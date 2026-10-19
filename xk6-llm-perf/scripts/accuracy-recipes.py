#!/usr/bin/env python3
"""Render pinned AIPerf recipes from explicit local dataset snapshots, offline."""
import argparse
import asyncio
import hashlib
import importlib
import importlib.metadata
import json
import os
from pathlib import Path
import sys
from types import SimpleNamespace
from unittest.mock import patch

REFERENCE = 'f10eb50a1776775d80d7f13396b0f668f14e2c03'
SOURCE_HASH = '144bd0b340d8405ddf0b91a01368a009dd48da508b9560387cc130aacedac716'
PLUGIN_HASH = 'd10bf1dded99edfb98968e470b407d875bf24c7651c001d90c8c6403f05191de'
EVALUATORS = {'mmlu': 'mmlu-v1', 'mmlu_pro': 'mmlu-pro-v1', 'aime': 'math-symbolic-v1',
              'hellaswag': 'aiperf-exact-match-v1', 'bigbench': 'aiperf-exact-match-v1',
              'aime24': 'lighteval-expr-v1', 'aime25': 'lighteval-expr-v1',
              'math_500': 'lighteval-latex-v1', 'gsm8k': 'gsm8k-v1',
              'gpqa_diamond': 'lighteval-gpqa-v1', 'lcb_codegeneration': 'code-execution-v1'}


def encoded(value, sort=True):
    return json.dumps(value, ensure_ascii=False, sort_keys=sort, separators=(',', ':'), allow_nan=False).encode()


def digest(data):
    return hashlib.sha256(data).hexdigest()


def go_json(value):
    # Go encoding/json escapes HTML-sensitive runes and the two JS separators.
    text = encoded(value, sort=False).decode()
    for char in ('&', '<', '>', '\u2028', '\u2029'):
        text = text.replace(char, '\\u%04x' % ord(char))
    return text.encode()


def read_json(path):
    with Path(path).open('rb') as stream:
        data = stream.read((16 << 20) + 1)
    if len(data) > 16 << 20:
        raise ValueError('input exceeds 16 MiB')
    def pairs(items):
        result = {}
        for key, value in items:
            if key in result:
                raise ValueError('duplicate JSON member')
            result[key] = value
        return result
    def constant(value):
        raise ValueError('non-finite JSON number')
    return json.loads(data, object_pairs_hook=pairs, parse_constant=constant), digest(data)


def runtime_identity():
    import aiperf
    root = Path(aiperf.__file__).parent
    hashes = {str(p.relative_to(root)): digest(p.read_bytes()) for p in sorted(root.rglob('*.py'))}
    if len(hashes) != 1025 or digest(encoded(hashes)) != SOURCE_HASH:
        raise ValueError('AIPerf source differs from pinned reference')
    if digest((root / 'plugin/plugins.yaml').read_bytes()) != PLUGIN_HASH:
        raise ValueError('AIPerf plugin defaults differ from pinned reference')
    if importlib.metadata.version('aiperf') != '0.13.0':
        raise ValueError('AIPerf package version differs from pin')
    return {'reference': REFERENCE, 'source_sha256': SOURCE_HASH, 'plugins_sha256': PLUGIN_HASH,
            'python': sys.version, 'packages': {d.metadata['Name']: d.version for d in importlib.metadata.distributions()}}


class Snapshots:
    """Serve only the dataset/config/split identities explicitly in the input."""
    def __init__(self, entries):
        if not isinstance(entries, list) or not 1 <= len(entries) <= 100:
            raise ValueError('invalid snapshot entries')
        self.entries = {}
        self.calls = []
        count = 0
        for item in entries:
            if not isinstance(item, dict) or set(item) != {'dataset', 'config', 'revision', 'splits'}:
                raise ValueError('invalid snapshot entry schema')
            for key in ('dataset', 'revision'):
                if not isinstance(item[key], str) or not item[key] or len(item[key]) > 256:
                    raise ValueError('missing snapshot identity')
            if item['config'] is not None and (not isinstance(item['config'], str) or not item['config'] or len(item['config']) > 256):
                raise ValueError('invalid dataset config')
            identity = (item['dataset'], item['config'])
            if identity in self.entries or not isinstance(item['splits'], dict) or not item['splits']:
                raise ValueError('duplicate snapshot identity or missing splits')
            for split, rows in item['splits'].items():
                if not isinstance(split, str) or not isinstance(rows, list) or any(not isinstance(r, dict) for r in rows):
                    raise ValueError('invalid snapshot rows')
                count += len(rows)
            self.entries[identity] = item
        if count > 10000:
            raise ValueError('snapshot exceeds 10000 rows')

    def load(self, dataset, config=None, *, split=None, **kwargs):
        from datasets import Dataset, DatasetDict
        if set(kwargs) - {'trust_remote_code'}:
            raise ValueError('unsupported upstream dataset load options')
        item = self.entries.get((dataset, config))
        if item is None or split is not None and split not in item['splits']:
            raise ValueError('required local dataset/config/split missing: ' + repr((dataset, config, split)))
        self.calls.append({'dataset': dataset, 'config': config, 'split': split, 'revision': item['revision'],
                           'snapshot_sha256': digest(encoded(item)), 'upstream_options': kwargs})
        if split is not None:
            return Dataset.from_list(item['splits'][split])
        return DatasetDict({key: Dataset.from_list(rows) for key, rows in item['splits'].items()})


def validate(config):
    required = {'schema_version', 'benchmark', 'model', 'model_revision', 'snapshots'}
    optional = {'tasks', 'n_shots', 'enable_cot', 'system_prompt'}
    if not isinstance(config, dict) or not required <= set(config) or set(config) - required - optional:
        raise ValueError('invalid recipe configuration')
    if type(config['schema_version']) is not int or config['schema_version'] != 1 or config['benchmark'] not in EVALUATORS:
        raise ValueError('unsupported recipe')
    for key in ('model', 'model_revision'):
        if not isinstance(config[key], str) or not config[key] or len(config[key]) > 256:
            raise ValueError('missing model identity')
    if 'n_shots' in config and (type(config['n_shots']) is not int or not 0 <= config['n_shots'] <= 100):
        raise ValueError('invalid shot count')
    if 'enable_cot' in config and type(config['enable_cot']) is not bool:
        raise ValueError('invalid CoT selection')
    if 'tasks' in config and (not isinstance(config['tasks'], list) or not 1 <= len(config['tasks']) <= 100 or any(not isinstance(t, str) or len(t) > 256 for t in config['tasks'])):
        raise ValueError('invalid task selection')
    if 'system_prompt' in config and (not isinstance(config['system_prompt'], str) or len(config['system_prompt'].encode()) > 65536):
        raise ValueError('invalid system prompt')
    return Snapshots(config['snapshots'])


def render(config):
    os.environ['HF_HUB_OFFLINE'] = '1'
    os.environ['HF_DATASETS_OFFLINE'] = '1'
    snapshots = validate(config)
    identity = runtime_identity()
    from aiperf.plugin import plugins
    from aiperf.plugin.enums import PluginType
    from aiperf.accuracy.benchmark_loader import load_benchmark_problems
    from aiperf.dataset.loader.accuracy_dataset_loader import AccuracyDatasetLoader
    import aiperf.accuracy.benchmarks._datasets_compat as compat
    benchmark = config['benchmark']
    cls = plugins.get_class(PluginType.ACCURACY_BENCHMARK, benchmark)
    module = importlib.import_module(cls.__module__)
    acc = SimpleNamespace(enabled=True, benchmark=benchmark, tasks=config.get('tasks'), n_shots=config.get('n_shots'),
                          enable_cot=config.get('enable_cot'), system_prompt=config.get('system_prompt'))
    cfg = SimpleNamespace(accuracy=acc, get_default_dataset=lambda: SimpleNamespace(random_seed=1))
    run = SimpleNamespace(cfg=cfg, random_seed=1)
    # No upstream remote dataset loader is ever called. The module-local name is
    # patched because recipes import load_dataset by value, often in a thread.
    with patch.object(module, 'load_dataset', snapshots.load), patch.object(compat, 'load_dataset', snapshots.load):
        problems = asyncio.run(load_benchmark_problems(run))
    if not 1 <= len(problems) <= 10000:
        raise ValueError('empty or oversized problem population')
    conversations = AccuracyDatasetLoader(run=run)._convert_to_conversations(problems)
    requests, prompts, cases, records = {}, {}, [], []
    for index, (problem, conversation) in enumerate(zip(problems, conversations, strict=True)):
        turn = conversation.turns[0]
        case_id = str(index)
        request = {'messages': turn.raw_messages, 'max_tokens': turn.max_tokens}
        if turn.extra_body and turn.extra_body.get('stop'):
            request['stop'] = turn.extra_body['stop']
        if not 1 <= len(request['messages']) <= 128 or sum(len(m['content'].encode()) for m in request['messages']) > 65536 or not 1 <= request['max_tokens'] <= 1048576 or len(request.get('stop', [])) > 16 or len(problem.ground_truth.encode()) > 65536:
            raise ValueError('recipe request or reference exceeds budget')
        requests[case_id] = request
        prompts[case_id] = problem.prompt
        cases.append({'id': case_id, 'reference': problem.ground_truth, 'prediction': None, 'outcome': 'success'})
        records.append({'id': case_id, 'task': problem.task, 'metadata': problem.metadata, 'request_sha256': digest(encoded(request))})
    references = [{'id': c['id'], 'reference': c['reference']} for c in cases]
    artifact = {'schema_version': 1, 'task': benchmark, 'task_version': REFERENCE, 'reference_version': digest(encoded(config['snapshots'])),
                'reference_sha256': digest(go_json(references)), 'evaluator': EVALUATORS[benchmark], 'evaluator_version': '1',
                'model': config['model'], 'model_revision': config['model_revision'], 'cases': cases}
    metadata = plugins.get_metadata(PluginType.ACCURACY_BENCHMARK, benchmark)
    receipt = {'schema_version': 1, 'runtime': identity, 'benchmark': benchmark, 'defaults': metadata,
               'selection': {key: config[key] for key in ('tasks', 'n_shots', 'enable_cot', 'system_prompt') if key in config},
               'dataset_calls': snapshots.calls, 'cases': records, 'population': len(cases),
               'qualification': 'offline recipe transformation only; no model or dataset authenticity claim'}
    return {'artifact.json': artifact, 'requests.json': requests, 'prompts.json': prompts, 'recipe.json': receipt}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('input', type=Path)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    # Fail closed on missing snapshots. Never call Hugging Face hub loaders.
    os.environ['HF_HUB_OFFLINE'] = '1'
    os.environ['HF_DATASETS_OFFLINE'] = '1'
    config, input_hash = read_json(args.input)
    outputs = render(config)
    outputs['recipe.json']['input_sha256'] = input_hash
    outputs['recipe.json']['adapter_sha256'] = digest(Path(__file__).read_bytes())
    for name, maximum in (('artifact.json', 16 << 20), ('requests.json', 4 << 20), ('prompts.json', 4 << 20), ('recipe.json', 16 << 20)):
        if len(encoded(outputs[name])) + 1 > maximum:
            raise ValueError(name + ' exceeds consumer budget')
    args.output.mkdir(parents=True, exist_ok=False)
    for name, value in outputs.items():
        (args.output / name).write_bytes(encoded(value) + b'\n')
    manifest = {name: digest((args.output / name).read_bytes()) for name in outputs}
    (args.output / 'manifest.json').write_bytes(encoded(manifest) + b'\n')


if __name__ == '__main__':
    main()
