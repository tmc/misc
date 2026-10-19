#!/usr/bin/env python3
"""Execute bounded scalar zip, scenario and pinned SciPy sampling designs."""
import argparse
import copy
import fcntl
import importlib.metadata
import importlib.util
import math
import re
import sys
import time
from pathlib import Path

spec = importlib.util.spec_from_file_location('campaign_core', Path(__file__).with_name('optuna-campaign.py'))
core = importlib.util.module_from_spec(spec)
spec.loader.exec_module(core)
REFERENCE = 'f10eb50a1776775d80d7f13396b0f668f14e2c03'


def scalar(value):
    if type(value) not in (str, int, float, bool) or isinstance(value, float) and not math.isfinite(value):
        raise ValueError('parameter must be a finite JSON scalar')
    result = str(value).lower() if type(value) is bool else str(value)
    if len(result) > 1024:
        raise ValueError('parameter exceeds 1024 characters')
    return result


def name(value):
    if not isinstance(value, str) or not re.fullmatch('[A-Z][A-Z0-9_]{0,127}', value) or value in ('PATH', 'HOME', 'TMPDIR'):
        raise ValueError('invalid environment parameter')
    return value


def fields(value, required, optional=()):
    if not isinstance(value, dict) or not set(required) <= set(value) or set(value) - set(required) - set(optional):
        raise ValueError('invalid design fields')


def design(config):
    """Return ordered proposals; equal parameter tuples retain distinct indexes.

    The supported upstream subset is scalar environment axes. Scenarios are
    flat scalar overrides, not arbitrary AIPerf BenchmarkConfig deep merges.
    """
    if not isinstance(config, dict):
        raise ValueError('invalid design')
    kind = config.get('type')
    rows = []
    if kind == 'zip':
        fields(config, ('type', 'parameters'))
        parameters = config['parameters']
        if not isinstance(parameters, dict) or not 1 <= len(parameters) <= 12:
            raise ValueError('invalid zip axes')
        lengths = set()
        for key, values in parameters.items():
            name(key)
            if not isinstance(values, list) or not 1 <= len(values) <= 4096:
                raise ValueError('invalid zip values')
            lengths.add(len(values))
            for value in values:
                scalar(value)
        if len(lengths) != 1:
            raise ValueError('zip axes must have equal lengths')
        rows = [('zip_%04d' % i, dict(zip(parameters, values))) for i, values in enumerate(zip(*parameters.values(), strict=True))]
    elif kind == 'scenarios':
        fields(config, ('type', 'base', 'runs'))
        if not isinstance(config['base'], dict) or not isinstance(config['runs'], list) or not 1 <= len(config['runs']) <= 4096:
            raise ValueError('invalid scenario collection')
        labels = set()
        for run in config['runs']:
            fields(run, ('name', 'parameters'))
            label = run['name']
            if not isinstance(label, str) or not 1 <= len(label) <= 128 or label in labels or '/' in label or '\\' in label or '..' in label or set(label) == {'.'}:
                raise ValueError('invalid or duplicate scenario name')
            labels.add(label)
            if not isinstance(run['parameters'], dict):
                raise ValueError('invalid scenario parameters')
            rows.append((label, dict(config['base'], **run['parameters'])))
    elif kind in ('sobol', 'latin_hypercube'):
        fields(config, ('type', 'samples', 'seed', 'dimensions'), ('scramble',) if kind == 'sobol' else ('optimization',))
        samples, seed, dimensions = config['samples'], config['seed'], config['dimensions']
        if type(samples) is not int or not 2 <= samples <= 4096 or type(seed) is not int or not 0 <= seed < 2**32 or not isinstance(dimensions, list) or not 1 <= len(dimensions) <= 12:
            raise ValueError('invalid sampling budget or seed')
        paths = set()
        for dim in dimensions:
            fields(dim, ('name',), ('choices', 'lo', 'hi', 'scale', 'kind'))
            key = name(dim['name'])
            if key in paths:
                raise ValueError('duplicate dimension')
            paths.add(key)
            if 'choices' in dim:
                if set(dim) != {'name', 'choices'} or not isinstance(dim['choices'], list) or not 1 <= len(dim['choices']) <= 4096:
                    raise ValueError('invalid categorical axis')
                for value in dim['choices']:
                    scalar(value)
            else:
                lo, hi = dim.get('lo'), dim.get('hi')
                if any(type(v) not in (int, float) or not math.isfinite(v) for v in (lo, hi)) or hi <= lo or dim.get('scale', 'linear') not in ('linear', 'log') or dim.get('kind', 'real') not in ('int', 'real'):
                    raise ValueError('invalid numeric axis')
                if dim.get('scale') == 'log' and lo <= 0 or dim.get('kind') == 'int' and math.ceil(lo) > math.floor(hi):
                    raise ValueError('empty numeric axis')
        versions = {key: importlib.metadata.version(key) for key in ('numpy', 'scipy')}
        if versions != {'numpy': '2.4.6', 'scipy': '1.17.1'}:
            raise ValueError('QMC runtime differs from pinned NumPy/SciPy')
        from scipy.stats import qmc
        if kind == 'sobol':
            scramble = config.get('scramble', True)
            if type(scramble) is not bool:
                raise ValueError('scramble must be boolean')
            engine = qmc.Sobol(d=len(dimensions), scramble=scramble, seed=seed)
            # The power-of-two balance recommendation is retained as a warning.
            cube = engine.random_base2(m=samples.bit_length()-1) if samples & (samples-1) == 0 else engine.random(n=samples)
        else:
            optimization = config.get('optimization', 'random-cd')
            if optimization not in (None, 'random-cd', 'lloyd'):
                raise ValueError('unsupported LHS optimization')
            cube = qmc.LatinHypercube(d=len(dimensions), optimization=optimization, seed=seed).random(n=samples)
        for i, row in enumerate(cube):
            parameters = {}
            for u, dim in zip(row, dimensions, strict=True):
                u = float(u)
                if 'choices' in dim:
                    value = dim['choices'][min(math.floor(u*len(dim['choices'])), len(dim['choices'])-1)]
                else:
                    lo, hi = dim['lo'], dim['hi']
                    value = math.exp(math.log(lo)+u*(math.log(hi)-math.log(lo))) if dim.get('scale') == 'log' else lo+u*(hi-lo)
                    if dim.get('kind') == 'int':
                        value = max(math.ceil(lo), min(round(value), math.floor(hi)))
                parameters[dim['name']] = value
            rows.append(('%s_%04d' % (kind, i), parameters))
    else:
        raise ValueError('unsupported design; Halton is not Sobol or LHS')
    proposals = []
    for i, (label, parameters) in enumerate(rows):
        if not 1 <= len(parameters) <= 12:
            raise ValueError('proposal must contain 1..12 scalar axes')
        values = {name(key): scalar(value) for key, value in parameters.items()}
        if sum(len(k)+len(v) for k, v in values.items()) > 2048:
            raise ValueError('parameter cell exceeds 2048 bytes')
        proposal = {'index': i, 'label': label, 'parameters': values}
        proposal['proposal_sha256'] = core.sha(proposal)
        proposals.append(proposal)
    return proposals


def run(config, output):
    fields(config, ('schema_version', 'design', 'campaign_binary', 'campaign', 'trial_timeout_seconds', 'max_duration_seconds'))
    if type(config['schema_version']) is not int or config['schema_version'] != 1:
        raise ValueError('unsupported schema')
    for key, maximum in (('trial_timeout_seconds', 86400), ('max_duration_seconds', 604800)):
        if type(config[key]) is not int or not 1 <= config[key] <= maximum:
            raise ValueError('invalid duration budget')
    proposals = design(config['design'])
    base = copy.deepcopy(config['campaign'])
    if not isinstance(base, dict) or base.get('convergence') not in ('range', 'cv', 'ci95'):
        raise ValueError('run-level convergence required')
    base.update(mode='grid', max_candidates=1)
    paths = [config['campaign_binary'], base['binary'], base['script']] + base.get('assets', [])
    if any(not isinstance(p, str) or not Path(p).is_absolute() for p in paths):
        raise ValueError('absolute asset paths required')
    identity = {'reference': REFERENCE, 'config_sha256': core.sha(config), 'design_sha256': core.sha(proposals), 'driver_sha256': core.file_sha(__file__), 'core_sha256': core.file_sha(core.__file__), 'assets': {p: core.file_sha(p) for p in paths}, 'python': sys.version, 'qmc_versions': {n: importlib.metadata.version(n) for n in ('numpy', 'scipy')} if config['design']['type'] in ('sobol', 'latin_hypercube') else {}}
    output.mkdir(parents=True, exist_ok=True, mode=0o700)
    if output.is_symlink() or output.stat().st_mode & 0o077:
        raise ValueError('private nonsymlink output required')
    start = time.monotonic()
    with open(output/'.design-lock', 'a+b') as lock:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        checkpoint = output/'design.json'
        state = core.read_json(checkpoint) if checkpoint.exists() else {'schema_version': 1, 'identity': identity, 'trials': [], 'elapsed_seconds': 0}
        if state.get('schema_version') != 1 or state.get('identity') != identity or not isinstance(state.get('trials'), list) or len(state['trials']) > len(proposals) or type(state.get('elapsed_seconds')) not in (int, float) or not math.isfinite(state['elapsed_seconds']) or state['elapsed_seconds'] < 0:
            raise ValueError('checkpoint identity or bounds differ')
        previous = state['elapsed_seconds']
        def save():
            state['elapsed_seconds'] = previous+time.monotonic()-start
            core.write_json(checkpoint, state)
        for i, trial in enumerate(state['trials']):
            if any(trial.get(k) != v for k, v in proposals[i].items()):
                raise ValueError('proposal replay differs')
            if trial.get('state') != 'COMPLETE':
                raise ValueError('failed or interrupted proposal cannot be retried')
            receipt = core.observed(output/('trial-%04d' % i), trial['parameters'], base)
            if any(core.encoded(trial.get(k)) != core.encoded(v) for k, v in receipt.items()):
                raise ValueError('retained campaign differs')
        try:
            for proposal in proposals[len(state['trials']):]:
                remaining = config['max_duration_seconds']-previous-(time.monotonic()-start)
                if remaining <= 0:
                    state['stop_reason'] = 'duration_budget'
                    return state
                trial = dict(proposal, state='RUNNING')
                state['trials'].append(trial)
                save()
                directory = output/('trial-%04d' % proposal['index'])
                directory.mkdir(mode=0o700)
                core.write_json(directory/'config.json', core.cell_policy(base, proposal['parameters']))
                try:
                    if any(core.file_sha(path) != digest for path, digest in identity['assets'].items()):
                        raise ValueError('campaign assets changed before dispatch')
                    status = core.run_child([config['campaign_binary'], '-config', str(directory/'config.json'), '-output', str(directory)], directory, min(remaining, config['trial_timeout_seconds']))
                    if status:
                        raise ValueError('campaign child failed with exit code %d' % status)
                    receipt = core.observed(directory, proposal['parameters'], base)
                    if receipt['state'] != 'COMPLETE':
                        raise ValueError('campaign did not converge')
                    trial.update(receipt, state='COMPLETE')
                except BaseException as error:
                    trial.update(state='FAIL', error=str(error))
                    state['stop_reason'] = 'proposal_failed'
                    raise
                save()
            state['stop_reason'] = 'design_exhausted'
            state['scope'] = 'ordered scalar proposals and retained worker artifacts; no capacity or performance qualification'
            return state
        finally:
            save()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--config', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--plan-only', action='store_true')
    args = parser.parse_args()
    config = core.read_json(args.config, 1 << 20)
    if args.plan_only:
        core.write_json(args.output, {'reference': REFERENCE, 'design': config, 'proposals': design(config)})
    else:
        run(config, args.output)


if __name__ == '__main__':
    try:
        main()
    except (ValueError, OSError, ImportError, TimeoutError) as error:
        sys.exit(str(error))
