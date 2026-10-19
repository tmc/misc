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


def typed_runtime():
    spec = importlib.util.spec_from_file_location('typed_template', Path(__file__).with_name('aiperf-template.py'))
    adapter = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(adapter)
    return adapter


def aiperf_plan(envelope):
    """Capture actual upstream fixed-trial dispatches without running a workload."""
    import asyncio
    import uuid
    from unittest.mock import patch
    adapter = typed_runtime()
    adapter.decode(adapter.encode(envelope))
    identity = adapter.runtime_identity()
    if not isinstance(envelope, dict) or not isinstance(envelope.get('sweep'), dict):
        raise ValueError('explicit upstream sweep required')
    if envelope['sweep'].get('type') not in ('grid', 'zip', 'scenarios'):
        raise ValueError('typed mode supports grid, zip and scenarios only')
    if set(envelope) - {'schema_version', 'benchmark', 'sweep', 'multi_run', 'variables', 'random_seed'}:
        raise ValueError('unsupported upstream envelope option')
    sweep = envelope['sweep']
    if set(sweep) - {'type', 'parameters', 'runs', 'iteration_order', 'same_seed', 'cooldown_seconds'}:
        raise ValueError('unsupported typed sweep option')
    multi = envelope.get('multi_run', {})
    if not isinstance(multi, dict) or set(multi) - {'num_runs', 'cooldown_seconds', 'set_consistent_seed', 'vary_seed_per_trial', 'disable_warmup_after_first'}:
        raise ValueError('only fixed-trial multi_run options are supported')
    # Bound expansion BEFORE asking upstream to allocate a Cartesian product.
    kind = sweep['type']
    if kind == 'scenarios':
        runs = sweep.get('runs')
        if not isinstance(runs, list) or not 1 <= len(runs) <= 256:
            raise ValueError('scenarios must contain 1..256 runs')
    else:
        axes = sweep.get('parameters')
        if not isinstance(axes, dict) or not 1 <= len(axes) <= 12:
            raise ValueError('typed sweep requires 1..12 axes')
        count = 1
        for values in axes.values():
            if not isinstance(values, list) or not 1 <= len(values) <= 256:
                raise ValueError('typed axis must contain 1..256 values')
            for value in values:
                scalar(value)
            count *= len(values)
        if kind == 'grid' and count > 256:
            raise ValueError('typed grid exceeds 256 variations')
    from aiperf.config.loader.core import load_config_from_mapping
    from aiperf.config.loader.plan import build_benchmark_plan
    from aiperf.orchestrator.models import RunResult
    from aiperf.orchestrator.orchestrator import MultiRunOrchestrator
    config = load_config_from_mapping(copy.deepcopy(envelope), substitute_env=False)
    plan = build_benchmark_plan(config)
    if not 1 <= len(plan.configs) <= 256 or len(plan.configs) * plan.trials > 1024:
        raise ValueError('typed plan exceeds variation or trial bound')
    if plan.sweep.cooldown_seconds > 60 or plan.cooldown_seconds > 60:
        raise ValueError('typed cooldown exceeds 60 seconds')
    events, pending = [], []
    async def sleep(seconds):
        pending.append(seconds)
    class Capture:
        @staticmethod
        def derive_id(plan, variation, trial):
            return str(uuid.uuid5(uuid.NAMESPACE_OID, '%d:%d' % (variation, trial)))
        async def execute(self, run):
            events.append({'index': len(events), 'variation_index': run.variation.index,
                           'trial_index': run.trial, 'label': run.variation.label,
                           'seed': run.random_seed, 'cooldowns_before': list(pending),
                           'benchmark': run.cfg.model_dump(mode='json'),
                           'authored_benchmark': run.cfg.model_dump(mode='json', exclude_unset=True)})
            pending.clear()
            return RunResult(label=run.label, success=True)
    # The source orchestrator's sleep calls are recorded, never slept here.
    with patch('aiperf.orchestrator.orchestrator.asyncio.sleep', sleep):
        asyncio.run(MultiRunOrchestrator(Path('.')).execute(plan, Capture()))
    result = {'reference': REFERENCE, 'provenance': identity, 'variations': [v.model_dump(mode='json') for v in plan.variations],
            'variation_seeds': plan.variation_seeds, 'events': events,
            'scope': 'actual upstream normalization and successful fixed-trial schedule; no workload dispatched'}
    adapter.decode(adapter.encode(result))
    return result


def leaves(value, prefix=''):
    if isinstance(value, dict):
        result = {}
        for key, child in value.items():
            result.update(leaves(child, prefix + '.' + key if prefix else key))
        return result
    if isinstance(value, list):
        result = {}
        for index, child in enumerate(value):
            result.update(leaves(child, prefix + '.' + str(index)))
        return result
    return {prefix: value}


def mapped_plan(config):
    fields(config, ('schema_version', 'envelope', 'environment', 'binary', 'script', 'seed_environment', 'trial_timeout_seconds', 'max_duration_seconds'), ('assets',))
    if type(config['schema_version']) is not int or config['schema_version'] != 1:
        raise ValueError('unsupported mapped schema')
    mapping = config['environment']
    if not isinstance(mapping, dict) or not 1 <= len(mapping) <= 12 or len(set(mapping.values())) != len(mapping):
        raise ValueError('mapping requires 1..12 unique scalar paths')
    seed_name = name(config['seed_environment'])
    if seed_name in mapping:
        raise ValueError('seed environment name collides')
    for key, path in mapping.items():
        name(key)
        if not isinstance(path, str) or not path or len(path) > 256:
            raise ValueError('invalid normalized benchmark path')
    for key, limit in (('trial_timeout_seconds', 60), ('max_duration_seconds', 600)):
        if type(config[key]) is not int or not 1 <= config[key] <= limit:
            raise ValueError('invalid mapped duration budget')
    if type(config['envelope'].get('random_seed')) is not int:
        raise ValueError('mapped execution requires an explicit integer random_seed')
    if config['envelope'].get('multi_run', {}).get('disable_warmup_after_first', True):
        raise ValueError('mapped execution requires disable_warmup_after_first=false')
    def check_authored(value):
        if isinstance(value, dict):
            if not value or any('.' in key for key in value):
                raise ValueError('empty authored collections and dotted mapping keys are unsupported')
            for child in value.values():
                check_authored(child)
        elif isinstance(value, list):
            if not value:
                raise ValueError('empty authored collections are unsupported')
            for child in value:
                check_authored(child)
    check_authored(config['envelope'].get('benchmark', {}))
    for run in config['envelope']['sweep'].get('runs', []):
        if 'benchmark' in run:
            check_authored(run['benchmark'])
    result = aiperf_plan(config['envelope'])
    for event in result['events']:
        authored = leaves(event['authored_benchmark'])
        if set(authored) != set(mapping.values()):
            raise ValueError('every normalized authored benchmark leaf must be mapped exactly once')
        event['parameters'] = {key: scalar(authored[path]) for key, path in mapping.items()}
    return result


def applied_receipt(path, event):
    if path.stat().st_size > 16 << 20:
        raise ValueError('worker log exceeds 16 MiB')
    records = []
    # k6 raw logs end lines with newline only; splitlines would also split on
    # vertical tab, form feed and U+2028, hiding text after a receipt.
    for line in path.read_text().split('\n'):
        if 'SWEEP_APPLIED ' in line:
            if line.startswith('time=') and ' msg=\"' in line:
                quoted = line.split(' msg=', 1)[1]
                message, end = __import__('json').JSONDecoder().raw_decode(quoted)
                if quoted[end:].strip() != 'source=console' or not isinstance(message, str):
                    raise ValueError('invalid structured console receipt')
            else:
                message = line
            if not message.startswith('SWEEP_APPLIED '):
                raise ValueError('invalid applied receipt prefix')
            text = message[len('SWEEP_APPLIED '):]
            value = typed_runtime().decode(text.encode())
            records.append(value)
    expected = {'parameters': event['parameters'], 'seed': str(event['seed'])}
    if core.encoded(records) != core.encoded([expected]):
        raise ValueError('script must emit exactly one matching SWEEP_APPLIED receipt')


def aiperf_run(config, output):
    """Run explicit scalar script realizations, retaining every attempted cell."""
    plan = mapped_plan(config)
    paths = [config['binary'], config['script']] + config.get('assets', [])
    if any(not isinstance(path, str) or not Path(path).is_absolute() for path in paths):
        raise ValueError('absolute binary/script/asset paths required')
    identity = {'config_sha256': core.sha(config), 'plan_sha256': core.sha(plan),
                'driver_sha256': core.file_sha(__file__), 'core_sha256': core.file_sha(core.__file__),
                'assets': {path: core.file_sha(path) for path in paths}}
    output.mkdir(mode=0o700, parents=True, exist_ok=True)
    if output.is_symlink() or output.stat().st_mode & 0o077:
        raise ValueError('private nonsymlink output required')
    start = time.monotonic()
    with open(output/'.typed-lock', 'a+b') as lock:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        checkpoint = output/'typed-design.json'
        state = core.read_json(checkpoint) if checkpoint.exists() else {'identity': identity, 'attempts': [], 'elapsed_seconds': 0}
        if state.get('identity') != identity or not isinstance(state.get('attempts'), list) or len(state['attempts']) > len(plan['events']):
            raise ValueError('typed checkpoint identity differs')
        previous = state.get('elapsed_seconds')
        if type(previous) not in (int, float) or not math.isfinite(previous) or previous < 0:
            raise ValueError('invalid retained elapsed duration')
        def save():
            state['elapsed_seconds'] = previous + time.monotonic() - start
            core.write_json(checkpoint, state)
        for index, attempt in enumerate(state['attempts']):
            if attempt.get('event') != plan['events'][index] or attempt.get('state') != 'COMPLETE':
                raise ValueError('failed/interrupted or changed cell cannot be replayed')
            artifacts = attempt.get('artifacts')
            expected = {'cell-%04d/driver.log' % index, 'cell-%04d/summary.json' % index}
            if not isinstance(artifacts, dict) or set(artifacts) != expected:
                raise ValueError('retained typed artifact keys differ')
            if type(attempt.get('exit_code')) is not int or attempt['exit_code'] != 0:
                raise ValueError('retained typed process did not succeed')
            if any(not isinstance(digest, str) or not re.fullmatch('[0-9a-f]{64}', digest) for digest in artifacts.values()):
                raise ValueError('invalid retained typed artifact digest')
            for path, digest in artifacts.items():
                target = output/path
                if target.is_symlink() or target.parent.is_symlink() or not target.is_file():
                    raise ValueError('retained typed artifact missing or symlink')
                if core.file_sha(target) != digest:
                    raise ValueError('retained typed artifact differs')
            applied_receipt(output/('cell-%04d/driver.log' % index), attempt['event'])
        try:
            for event in plan['events'][len(state['attempts']):]:
                cooldown = sum(event['cooldowns_before'])
                remaining = config['max_duration_seconds'] - previous - (time.monotonic() - start)
                if remaining <= cooldown:
                    state['stop_reason'] = 'duration_budget'
                    return state
                if cooldown:
                    time.sleep(cooldown)
                timeout = min(config['trial_timeout_seconds'], config['max_duration_seconds'] - previous - (time.monotonic()-start))
                if timeout <= 0:
                    state['stop_reason'] = 'duration_budget'
                    return state
                directory = output/('cell-%04d' % event['index'])
                directory.mkdir(mode=0o700)
                arguments = [config['binary'], 'run', '--log-format', 'raw', '--summary-export', str(directory/'summary.json')]
                parameters = dict(event['parameters'], **{config['seed_environment']: str(event['seed'])})
                for key, value in parameters.items():
                    arguments.extend(['--env', key + '=' + value])
                arguments.append(config['script'])
                attempt = {'event': event, 'arguments': arguments, 'state': 'RUNNING'}
                state['attempts'].append(attempt)
                save()
                try:
                    if any(core.file_sha(path) != digest for path, digest in identity['assets'].items()):
                        raise ValueError('typed execution assets changed')
                    status = core.run_child(arguments, directory, timeout)
                    attempt['exit_code'] = status
                    if status:
                        raise ValueError('typed script failed with exit code %d' % status)
                    applied_receipt(directory/'driver.log', event)
                    core.read_json(directory/'summary.json', 16 << 20)
                    attempt['artifacts'] = {str(path.relative_to(output)): core.file_sha(path) for path in (directory/'driver.log', directory/'summary.json')}
                    attempt['state'] = 'COMPLETE'
                except BaseException as error:
                    attempt.update(state='FAIL', error=str(error))
                    state['stop_reason'] = 'cell_failed'
                    raise
                save()
            state['stop_reason'] = 'schedule_exhausted'
            return state
        finally:
            save()


def interrupted(signum, frame):
    raise InterruptedError('typed execution interrupted by signal %d' % signum)


def handle_signals():
    """Turn termination, hangup and alarm signals into owned-child cleanup."""
    import signal
    for number in (signal.SIGTERM, signal.SIGHUP, signal.SIGALRM):
        signal.signal(number, interrupted)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--config', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    mode = parser.add_mutually_exclusive_group()
    mode.add_argument('--plan-only', action='store_true')
    mode.add_argument('--aiperf-plan-only', action='store_true')
    mode.add_argument('--aiperf-run', action='store_true')
    args = parser.parse_args()
    config = core.read_json(args.config, 1 << 20)
    if args.aiperf_plan_only or args.aiperf_run:
        import resource
        import signal
        handle_signals()
        resource.setrlimit(resource.RLIMIT_CPU, (20, 20))
        resource.setrlimit(resource.RLIMIT_AS, (2 << 30, 2 << 30))
        resource.setrlimit(resource.RLIMIT_FSIZE, (16 << 20, 16 << 20))
        config = typed_runtime().decode(args.config.read_bytes())
        signal.alarm(30 if args.aiperf_plan_only else 630)
    if args.aiperf_plan_only:
        core.write_json(args.output, aiperf_plan(config))
    elif args.aiperf_run:
        aiperf_run(config, args.output)
    elif args.plan_only:
        core.write_json(args.output, {'reference': REFERENCE, 'design': config, 'proposals': design(config)})
    else:
        run(config, args.output)


if __name__ == '__main__':
    try:
        main()
    except (ValueError, OSError, ImportError, TimeoutError) as error:
        sys.exit(str(error))
