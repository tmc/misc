#!/usr/bin/env python3
"""Bounded offline adapter for the pinned AIPerf trace synthesizer."""
import argparse
import hashlib
import importlib.metadata
import json
import math
import os
from pathlib import Path
import resource
import signal
import subprocess
import sys
import tempfile

REFERENCE = 'f10eb50a1776775d80d7f13396b0f668f14e2c03'
SOURCE = '144bd0b340d8405ddf0b91a01368a009dd48da508b9560387cc130aacedac716'
PLUGINS = 'd10bf1dded99edfb98968e470b407d875bf24c7651c001d90c8c6403f05191de'
INPUT_LIMIT = 8 << 20
OUTPUT_LIMIT = 16 << 20
BLOCK_LIMIT = 50000
SAFE_INTEGER = 9007199254740991


def encoded(value, sort=True):
    return json.dumps(value, ensure_ascii=False, sort_keys=sort, separators=(',', ':'), allow_nan=False).encode()


def sha(data):
    return hashlib.sha256(data).hexdigest()


def decode(data):
    if len(data) > INPUT_LIMIT:
        raise ValueError('input exceeds 8 MiB')
    def pairs(items):
        result = {}
        for key, value in items:
            if key in result:
                raise ValueError('duplicate JSON member')
            result[key] = value
        return result
    def constant(value):
        raise ValueError('non-finite JSON number')
    value = json.loads(data, object_pairs_hook=pairs, parse_constant=constant)
    def visit(item, depth=0):
        if depth > 64:
            raise ValueError('JSON nesting exceeds 64')
        if isinstance(item, float) and not math.isfinite(item):
            raise ValueError('non-finite number')
        if isinstance(item, dict):
            for child in item.values():
                visit(child, depth + 1)
        elif isinstance(item, list):
            for child in item:
                visit(child, depth + 1)
    visit(value)
    return value


def runtime_identity():
    import aiperf
    root = Path(aiperf.__file__).parent
    hashes = {str(p.relative_to(root)): sha(p.read_bytes()) for p in sorted(root.rglob('*.py'))}
    if len(hashes) != 1025 or sha(encoded(hashes)) != SOURCE or sha((root / 'plugin/plugins.yaml').read_bytes()) != PLUGINS:
        raise ValueError('AIPerf source or plugin defaults differ from pin')
    if importlib.metadata.version('aiperf') != '0.13.0':
        raise ValueError('AIPerf package version differs from pin')
    return {'reference': REFERENCE, 'source_sha256': SOURCE, 'plugins_sha256': PLUGINS,
            'python': sys.version, 'packages': {p.metadata['Name']: p.version for p in importlib.metadata.distributions()}}


def bounded_integer(value, minimum, maximum, name):
    if type(value) is not int or not minimum <= value <= maximum:
        raise ValueError('invalid ' + name)


def validate(config):
    from aiperf.dataset.synthesis.models import SynthesisParams
    if not isinstance(config, dict) or set(config) != {'schema_version', 'seed', 'grouped', 'params', 'traces'} or type(config['schema_version']) is not int or config['schema_version'] != 1:
        raise ValueError('invalid synthesis schema')
    bounded_integer(config['seed'], 0, (1 << 32) - 1, 'seed')
    if type(config['grouped']) is not bool or not isinstance(config['params'], dict):
        raise ValueError('invalid grouped mode or parameters')
    fields = set(SynthesisParams.model_fields)
    if set(config['params']) - fields:
        raise ValueError('unknown synthesis parameter')
    # Populate actual pinned defaults before budgeting, without allocating paths.
    params = SynthesisParams(**config['params'])
    values = params.model_dump()
    for key in ('speedup_ratio', 'prefix_len_multiplier', 'prompt_len_multiplier', 'output_len_multiplier'):
        supplied = config['params'].get(key, values[key])
        if type(supplied) not in (int, float) or not math.isfinite(supplied) or not 0 <= supplied <= 16:
            raise ValueError('multiplier exceeds finite 0..16 budget: ' + key)
    if 0 < params.speedup_ratio < 1 / 16:
        raise ValueError('speedup ratio would amplify timestamps beyond budget')
    for key, minimum, maximum in (('prefix_root_multiplier', 1, 64), ('block_size', 1, 4096), ('max_isl', 1, 1000000), ('max_osl', 1, 1000000)):
        value = config['params'].get(key, values[key])
        if value is not None:
            bounded_integer(value, minimum, maximum, key)
    if type(values['renormalize_hash_ids']) is not bool or 'renormalize_hash_ids' in config['params'] and type(config['params']['renormalize_hash_ids']) is not bool:
        raise ValueError('invalid renormalization flag')
    data = config['traces']
    if config['grouped']:
        if not isinstance(data, dict) or len(data) > 1000 or any(not key or len(key.encode()) > 256 or not isinstance(rows, list) for key, rows in data.items()):
            raise ValueError('invalid grouped traces')
        if sum(len(group) for group in data.values()) > 1000:
            raise ValueError('trace count exceeds 1000')
        rows = [row for group in data.values() for row in group]
    else:
        if not isinstance(data, list):
            raise ValueError('flat traces must be a list')
        rows = data
    if len(rows) > 1000:
        raise ValueError('trace count exceeds 1000')
    original_blocks = projected_blocks = 0
    max_id = 0
    for row in rows:
        if not isinstance(row, dict):
            raise ValueError('trace must be an object')
        for key in ('input_length', 'output_length'):
            if key in row:
                bounded_integer(row[key], 0, 1000000, key)
        if 'timestamp' in row and (type(row['timestamp']) not in (int, float) or not math.isfinite(row['timestamp']) or not 0 <= row['timestamp'] <= 86400000):
            raise ValueError('invalid timestamp')
        ids = row.get('hash_ids', [])
        if not isinstance(ids, list) or len(ids) > 1024:
            raise ValueError('hash count exceeds 1024 per trace')
        for value in ids:
            bounded_integer(value, 0, SAFE_INTEGER, 'hash ID')
            max_id = max(max_id, value)
        original_blocks += len(ids)
        length = row.get('input_length', 0)
        if ids and length < len(ids) * params.block_size:
            raise ValueError('input length does not cover declared hash blocks')
        # Conservative upper bound: all blocks may be shared, and all input
        # tokens may also need newly allocated unique-prompt blocks. max_isl is
        # applied only AFTER allocation upstream, so it cannot waive this bound.
        prefix = math.ceil(len(ids) * params.prefix_len_multiplier) + 1
        prompt = math.ceil(length * params.prompt_len_multiplier / params.block_size)
        blocks = prefix + prompt if ids else 0
        projected_blocks += blocks
        if (prefix * params.block_size + int(length * params.prompt_len_multiplier) > 1000000 or
                row.get('output_length', 0) * params.output_len_multiplier > 1000000):
            raise ValueError('projected token length exceeds 1000000 before caps')
        if row.get('timestamp', 0) / (params.speedup_ratio or 1) > 86400000:
            raise ValueError('projected timestamp exceeds one day')
    if original_blocks > BLOCK_LIMIT or projected_blocks > BLOCK_LIMIT:
        raise ValueError('input or projected hash blocks exceed 50000')
    if ((max_id + 1) * max(1, int(params.prefix_len_multiplier)) + projected_blocks + 2) * params.prefix_root_multiplier > SAFE_INTEGER:
        raise ValueError('projected hash IDs exceed exact JavaScript range')
    return params, {'rows': len(rows), 'input_blocks': original_blocks, 'projected_block_upper_bound': projected_blocks}


def synthesize(config):
    identity = runtime_identity()
    params, budget = validate(config)
    from aiperf.common import random_generator as rng
    from aiperf.dataset.synthesis.synthesizer import Synthesizer
    rng.reset()
    rng.init(config['seed'])
    synthesizer = Synthesizer(params)
    output = synthesizer.synthesize_grouped_traces(config['traces']) if config['grouped'] else synthesizer.synthesize_traces(config['traces'])
    if len(encoded(output)) > OUTPUT_LIMIT:
        raise ValueError('synthesis output exceeds 16 MiB')
    return {'traces': output, 'receipt': {'runtime': identity, 'params': params.model_dump(), 'params_sha256': sha(encoded(params.model_dump())),
            'seed': config['seed'], 'rng_derivation': 'dataset.synthesis.synthesizer', 'grouped': config['grouped'], 'budget': budget,
            'stats': synthesizer.get_stats(), 'qualification': 'count/cache targets only; no tokenizer, realized prompt or serving cache qualification'}}


def limits():
    resource.setrlimit(resource.RLIMIT_CPU, (20, 20))
    resource.setrlimit(resource.RLIMIT_AS, (2 << 30, 2 << 30))
    resource.setrlimit(resource.RLIMIT_FSIZE, (32 << 20, 32 << 20))
    resource.setrlimit(resource.RLIMIT_CORE, (0, 0))


def main():
    if sys.argv[1:] == ['--worker']:
        limits()
        signal.alarm(30)
        data = sys.stdin.buffer.read(INPUT_LIMIT + 1)
        result = synthesize(decode(data))
        sys.stdout.buffer.write(encoded(result, sort=False))
        return
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('input', type=Path)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    with args.input.open('rb') as stream:
        data = stream.read(INPUT_LIMIT + 1)
    config = decode(data)
    env = dict(os.environ, HF_HUB_OFFLINE='1', HF_DATASETS_OFFLINE='1', OPENBLAS_NUM_THREADS='1', OMP_NUM_THREADS='1', MKL_NUM_THREADS='1')
    with tempfile.TemporaryFile() as stdout, tempfile.TemporaryFile() as stderr:
        process = subprocess.Popen([sys.executable, str(Path(__file__).resolve()), '--worker'], stdin=subprocess.PIPE, stdout=stdout, stderr=stderr, env=env, preexec_fn=limits)
        try:
            process.communicate(data, timeout=30)
        except subprocess.TimeoutExpired:
            process.kill()
            process.wait()
            raise ValueError('synthesis exceeded 30 seconds') from None
        stdout.seek(0)
        body = stdout.read(OUTPUT_LIMIT + 1)
        if process.returncode != 0 or len(body) > OUTPUT_LIMIT:
            stderr.seek(0)
            raise ValueError('bounded synthesis worker failed: ' + stderr.read(4096).decode(errors='replace'))
    result = json.loads(body)
    traces = result['traces']
    if config['grouped']:
        flat = [{**row, 'session_id': key} for key, rows in traces.items() for row in rows]
    else:
        flat = traces
    outputs = {'traces.json': encoded(traces, sort=False) + b'\n', 'traces.jsonl': b''.join(encoded(row) + b'\n' for row in flat)}
    receipt = result['receipt']
    receipt.update(input_sha256=sha(data), canonical_input_sha256=sha(encoded(config)), adapter_sha256=sha(Path(__file__).read_bytes()),
                   output_sha256={name: sha(value) for name, value in outputs.items()},
                   jsonl_transformation='grouped session keys attached as session_id; empty groups retained only in traces.json' if config['grouped'] else 'flat rows retained',
                   budgets={'cpu_seconds': 20, 'wall_seconds': 30, 'address_space_bytes': 2 << 30})
    outputs['receipt.json'] = encoded(receipt) + b'\n'
    if any(len(value) > OUTPUT_LIMIT for value in outputs.values()):
        raise ValueError('export exceeds 16 MiB')
    args.output.mkdir(parents=True, exist_ok=False)
    for name, value in outputs.items():
        (args.output / name).write_bytes(value)


if __name__ == '__main__':
    main()
