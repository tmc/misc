#!/usr/bin/env python3
"""Run pinned AIPerf boundary planners over repeated xk6 campaign artifacts."""
import argparse
import copy
import fcntl
import importlib.metadata
import importlib.util
import math
import multiprocessing
import os
import pickle
import signal
import socket
import struct
from pathlib import Path
import sys
import time

spec = importlib.util.spec_from_file_location('campaign_core', Path(__file__).with_name('optuna-campaign.py'))
core = importlib.util.module_from_spec(spec)
spec.loader.exec_module(core)
SOURCE_SHA = '144bd0b340d8405ddf0b91a01368a009dd48da508b9560387cc130aacedac716'
PATH = 'phases.profiling.concurrency'


def runtime_identity():
    import aiperf
    root = Path(aiperf.__file__).parent
    hashes = {str(p.relative_to(root)): core.file_sha(p) for p in sorted(root.rglob('*.py'))}
    if len(hashes) != 1025 or core.sha(hashes) != SOURCE_SHA:
        raise ValueError('AIPerf runtime source differs from pinned reference')
    versions = {name: importlib.metadata.version(name) for name in ('aiperf', 'numpy', 'scipy')}
    if versions != {'aiperf': '0.13.0', 'numpy': '2.4.6', 'scipy': '1.17.1'}:
        raise ValueError('planner dependency versions differ from pins')
    return {'reference': core.REFERENCE, 'source_sha256': SOURCE_SHA, 'versions': versions, 'python': sys.version, 'dependencies': {item.metadata['Name']: item.version for item in importlib.metadata.distributions()}}


def validate(config):
    fields = {'schema_version', 'style', 'low', 'high', 'max_iterations', 'sla_replicates', 'filters', 'metrics', 'campaign_binary', 'campaign', 'concurrency_parameter', 'trial_timeout_seconds', 'max_duration_seconds'}
    if not isinstance(config, dict) or set(config) != fields or type(config['schema_version']) is not int or config['schema_version'] != 1:
        raise ValueError('invalid planner configuration')
    if config['style'] not in ('monotonic', 'smooth_isotonic'):
        raise ValueError('unsupported boundary planner')
    for name, minimum, maximum in [('low', 1, 1000000), ('high', 1, 1000000), ('max_iterations', 1, 1000), ('sla_replicates', 0, 100), ('trial_timeout_seconds', 1, 86400), ('max_duration_seconds', 1, 604800)]:
        if type(config[name]) is not int or not minimum <= config[name] <= maximum:
            raise ValueError('invalid ' + name)
    if config['low'] > config['high'] or not isinstance(config['concurrency_parameter'], str) or not config['concurrency_parameter']:
        raise ValueError('invalid concurrency domain')
    if not isinstance(config['filters'], list) or not 1 <= len(config['filters']) <= 8:
        raise ValueError('at least one SLA filter required')
    for item in config['filters']:
        if not isinstance(item, dict) or set(item) != {'metric_tag', 'stat', 'op', 'threshold'} or item['op'] not in ('lt', 'le', 'gt', 'ge'):
            raise ValueError('invalid SLA filter')
        if isinstance(item['threshold'], bool) or not isinstance(item['threshold'], (int, float)) or not math.isfinite(item['threshold']):
            raise ValueError('invalid SLA threshold')
    metrics = config['metrics']
    if not isinstance(metrics, dict) or not 1 <= len(metrics) <= 16:
        raise ValueError('invalid metric mappings')
    for tag, mapping in metrics.items():
        if not isinstance(tag, str) or not isinstance(mapping, dict) or set(mapping) != {'name', 'unit', 'stats'} or not isinstance(mapping['name'], str) or not isinstance(mapping['unit'], str) or not isinstance(mapping['stats'], dict) or not mapping['stats']:
            raise ValueError('invalid metric mapping')
        if any(stat not in ('avg', 'p50', 'p90', 'p95', 'p99', 'min', 'max', 'std') or not isinstance(source, str) for stat, source in mapping['stats'].items()):
            raise ValueError('invalid metric statistic')
    if 'output_token_throughput' not in metrics or 'avg' not in metrics['output_token_throughput']['stats']:
        raise ValueError('output-token throughput objective required')
    for item in config['filters']:
        if item['metric_tag'] not in metrics or item['stat'] not in metrics[item['metric_tag']]['stats']:
            raise ValueError('SLA metric mapping missing')
    campaign = config['campaign']
    if not isinstance(campaign, dict) or campaign.get('convergence') not in ('range', 'cv', 'ci95') or campaign.get('min_runs', 0) < 2:
        raise ValueError('independent repeated converged runs required')
    if campaign.get('constraints'):
        raise ValueError('configure SLA filters in planner, not campaign constraints')
    base = copy.deepcopy(campaign)
    base.update(mode='grid', max_candidates=1, dimensions=[{'name': config['concurrency_parameter'], 'values': [str(config['low'])]}])
    return base


def planner(config):
    import numpy as np
    np.random.seed(0)
    from aiperf.config.config import BenchmarkConfig
    from aiperf.config.sweep import AdaptiveSearchSweep, Objective
    from aiperf.config.sweep.adaptive import SearchSpaceDimension, SLAFilter
    from aiperf.orchestrator.search_planner.monotonic import MonotonicSLASearchPlanner
    from aiperf.orchestrator.search_planner.smooth_isotonic import SmoothIsotonicSLAPlanner
    base = BenchmarkConfig.model_validate({'models': ['xk6-artifacts'], 'endpoint': {'urls': ['http://127.0.0.1'], 'type': 'chat'}, 'datasets': [{'name': 'profiling', 'type': 'synthetic'}], 'phases': [{'name': 'profiling', 'type': 'concurrency', 'concurrency': 1, 'requests': 10}]})
    cfg = AdaptiveSearchSweep(planner=('monotonic_sla' if config['style'] == 'monotonic' else config['style']), search_space=[SearchSpaceDimension(path=PATH, lo=config['low'], hi=config['high'], kind='int')], objectives=[Objective(metric='output_token_throughput', stat='avg', direction='maximize')], max_iterations=config['max_iterations'], n_initial_points=1, sla_filters=[SLAFilter(**item) for item in config['filters']], sla_replicates=config['sla_replicates'])
    return (SmoothIsotonicSLAPlanner if config['style'] == 'smooth_isotonic' else MonotonicSLASearchPlanner)(base, cfg)


def results(directory, parameters, variation, config):
    from aiperf.common.models.export_models import JsonMetricResult
    from aiperf.orchestrator.models import RunResult
    raw_manifest = core.read_json(directory / 'campaign.json')
    for cell in raw_manifest.get('candidates', []):
        for attempt in cell.get('attempts', []):
            relative = Path(attempt.get('directory', '')) / 'summary.json'
            if relative.is_absolute() or '..' in relative.parts or not (directory / relative).resolve().is_relative_to(directory.resolve()):
                raise ValueError('summary artifact escapes owned campaign')
    receipt = core.observed(directory, parameters)
    if receipt['state'] != 'COMPLETE':
        raise ValueError('campaign failed or did not converge')
    manifest = core.read_json(directory / 'campaign.json')
    output = []
    for index, attempt in enumerate(manifest['candidates'][0]['attempts']):
        if attempt.get('error') or attempt.get('exit_code') != 0 or attempt.get('value') is None:
            raise ValueError('failed repeated run cannot qualify planner observation')
        summary = core.read_json(directory / attempt['directory'] / 'summary.json')
        metrics = {}
        for tag, mapping in config['metrics'].items():
            raw = summary.get('metrics', {}).get(mapping['name'], {})
            raw = raw.get('values', raw)
            values = {}
            for stat, source in mapping['stats'].items():
                value = raw.get(source)
                if isinstance(value, bool) or not isinstance(value, (int, float)) or not math.isfinite(value):
                    raise ValueError('required planner metric unavailable')
                values[stat] = value
            metrics[tag] = JsonMetricResult(unit=mapping['unit'], **values)
        output.append(RunResult(label=str(index), success=True, summary_metrics=metrics, variation_label=variation.label, variation_values=variation.values))
    if len(output) < config['campaign']['min_runs']:
        raise ValueError('insufficient repeated runs')
    return output, receipt


def boundary(model):
    result = model.boundary_summary()
    if result and result.get('boundary_ci') and any(not math.isfinite(value) for value in result['boundary_ci'].values()):
        result['boundary_ci'] = None
        result['boundary_ci_unavailable_reason'] = 'pinned upstream bootstrap returned nonfinite bounds'
    return result


def bounded(model, action, args, deadline):
    remaining = deadline - time.monotonic()
    if remaining <= 0:
        raise TimeoutError('planner wall budget exhausted')
    import numpy as np
    context = multiprocessing.get_context('fork')
    reader, writer = socket.socketpair()
    def work():
        reader.close()
        os.setsid()
        try:
            current = planner(args[0]) if action == 'create' else model
            value = None if action == 'create' else getattr(current, action)(*args)
            payload = pickle.dumps((current, value, np.random.get_state()), protocol=5)
            if len(payload) > 64 << 20:
                raise ValueError('planner state exceeds limit')
            writer.sendall(struct.pack('!Q', len(payload)) + payload)
        finally:
            writer.close()
    process = context.Process(target=work)
    process.start()
    writer.close()
    try:
        def receive(size):
            chunks = []
            while size:
                remaining = deadline - time.monotonic()
                if remaining <= 0:
                    raise TimeoutError('planner operation deadline exceeded')
                reader.settimeout(remaining)
                chunk = reader.recv(min(size, 1 << 20))
                if not chunk:
                    raise ValueError('planner worker exited without a complete result')
                chunks.append(chunk)
                size -= len(chunk)
            return b''.join(chunks)
        size = struct.unpack('!Q', receive(8))[0]
        if size > 64 << 20:
            raise ValueError('planner response exceeds limit')
        payload = receive(size)
        if time.monotonic() >= deadline:
            raise TimeoutError('planner operation deadline exceeded')
        current, value, rng = pickle.loads(payload)
        np.random.set_state(rng)
        return current, value
    finally:
        reader.close()
        try:
            os.killpg(process.pid, signal.SIGKILL)
        except ProcessLookupError:
            process.kill() if process.is_alive() else None
        process.join(timeout=3)
        if process.is_alive():
            process.kill()
            process.join(timeout=3)


def run(config, output):
    start = time.monotonic()
    base = validate(config)
    runtime = runtime_identity()
    assets = [config['campaign_binary'], base['binary'], base['script']] + base.get('assets', [])
    if any(not isinstance(p, str) or not Path(p).is_absolute() for p in assets):
        raise ValueError('assets must have absolute paths')
    identity = {'runtime': runtime, 'config_sha256': core.sha(config), 'driver_sha256': core.file_sha(__file__), 'core_sha256': core.file_sha(core.__file__), 'assets': {p: core.file_sha(p) for p in assets}}
    output.mkdir(parents=True, exist_ok=True, mode=0o700)
    if output.is_symlink() or output.stat().st_mode & 0o077:
        raise ValueError('private nonsymlink output required')
    with open(output / '.planner-lock', 'a+b') as lock:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        checkpoint = output / 'planner.json'
        state = core.read_json(checkpoint) if checkpoint.exists() else {'schema_version': 1, 'identity': identity, 'trials': [], 'elapsed_seconds': 0}
        if type(state.get('schema_version')) is not int or state.get('schema_version') != 1 or state.get('identity') != identity or not isinstance(state.get('trials'), list) or len(state['trials']) > config['max_iterations'] or isinstance(state.get('elapsed_seconds'), bool) or not isinstance(state.get('elapsed_seconds'), (int, float)) or not math.isfinite(state['elapsed_seconds']) or state['elapsed_seconds'] < 0:
            raise ValueError('planner checkpoint identity invalid')
        if state.get('terminal_error'):
            raise ValueError('failed planner operation cannot be retried')
        previous = state['elapsed_seconds']
        deadline = start + max(0, config['max_duration_seconds'] - previous)
        def call(current, action, args):
            try:
                return bounded(current, action, args, deadline)
            except BaseException as error:
                state['elapsed_seconds'] = previous + time.monotonic() - start
                state['terminal_error'] = str(error)
                state['stop_reason'] = 'planner_operation_failed'
                core.write_json(checkpoint, state)
                raise
        model, _ = call(None, 'create', (config,))
        for number, trial in enumerate(state['trials']):
            model, proposal = call(model, 'ask', ())
            if not isinstance(trial, dict) or proposal is None or type(trial.get('number')) is not int or type(trial.get('concurrency')) is not int or trial.get('number') != number or trial.get('concurrency') != proposal[1].values[PATH]:
                raise ValueError('planner proposal replay differs')
            if trial.get('state') != 'COMPLETE':
                raise ValueError('interrupted or failed probe cannot be retried')
            observed, receipt = results(output / ('trial-%04d' % number), {config['concurrency_parameter']: str(trial['concurrency'])}, proposal[1], config)
            if any(core.encoded(trial.get(key)) != core.encoded(value) for key, value in receipt.items()):
                raise ValueError('planner retained artifacts changed')
            model, _ = call(model, 'tell', (proposal[1], observed))
        if model.is_converged():
            if state.get('boundary') != boundary(model) or state.get('stop_reason') != model.convergence_reason():
                raise ValueError('planner boundary replay differs')
            state['elapsed_seconds'] = previous + time.monotonic() - start
            core.write_json(checkpoint, state)
            return state
        while not model.is_converged():
            remaining = config['max_duration_seconds'] - previous - (time.monotonic() - start)
            if remaining <= 0:
                state['stop_reason'] = 'duration_budget'
                break
            if runtime_identity() != runtime or core.file_sha(__file__) != identity['driver_sha256'] or core.file_sha(core.__file__) != identity['core_sha256'] or any(core.file_sha(p) != digest for p, digest in identity['assets'].items()):
                raise ValueError('planner source or assets changed')
            remaining = config['max_duration_seconds'] - previous - (time.monotonic() - start)
            if remaining <= 0:
                state['stop_reason'] = 'duration_budget'
                break
            model, proposal = call(model, 'ask', ())
            if proposal is None:
                break
            remaining = config['max_duration_seconds'] - previous - (time.monotonic() - start)
            if remaining <= 0:
                state['stop_reason'] = 'duration_budget'
                break
            variation = proposal[1]
            value = variation.values[PATH]
            number = len(state['trials'])
            trial = {'number': number, 'concurrency': value, 'state': 'RUNNING', 'reserved_duration_seconds': min(remaining, config['trial_timeout_seconds']) + 6}
            state['trials'].append(trial)
            state['elapsed_seconds'] = previous + time.monotonic() - start
            core.write_json(checkpoint, state)
            directory = output / ('trial-%04d' % number)
            directory.mkdir(mode=0o700)
            cell = copy.deepcopy(base)
            parameters = {config['concurrency_parameter']: str(value)}
            cell['dimensions'] = [{'name': name, 'values': [item]} for name, item in parameters.items()]
            core.write_json(directory / 'config.json', cell)
            try:
                status = core.run_child([config['campaign_binary'], '-config', str(directory / 'config.json'), '-output', str(directory)], directory, min(remaining, config['trial_timeout_seconds']))
                if status:
                    raise ValueError('campaign process failed')
                observed, receipt = results(directory, parameters, variation, config)
                model, _ = call(model, 'tell', (variation, observed))
                trial.update(receipt, state='COMPLETE')
            except BaseException:
                trial['state'] = 'FAIL'
                state['elapsed_seconds'] = previous + time.monotonic() - start
                core.write_json(checkpoint, state)
                raise
            state['elapsed_seconds'] = previous + time.monotonic() - start
            state['boundary'] = boundary(model)
            state['stop_reason'] = model.convergence_reason()
            state['scope'] = 'actual pinned upstream planner; successful converged repeated xk6 runs; upstream any-trial SLA feasibility; no physical or statistical performance qualification'
            core.write_json(checkpoint, state)
        state['elapsed_seconds'] = previous + time.monotonic() - start
        state['boundary'] = boundary(model)
        core.write_json(checkpoint, state)
        return state


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--config', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    run(core.read_json(args.config), args.output)


if __name__ == '__main__':
    try:
        main()
    except (ValueError, OSError, ImportError, TimeoutError) as error:
        sys.exit(str(error))
