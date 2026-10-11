import copy
import importlib.util
import os
import signal
import subprocess
import time
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('sla_planner', Path(__file__).with_name('sla-planner.py'))
bridge = importlib.util.module_from_spec(spec)
spec.loader.exec_module(bridge)


def configuration():
    return {'schema_version': 1, 'style': 'smooth_isotonic', 'low': 1, 'high': 32, 'max_iterations': 30, 'sla_replicates': 3,
            'filters': [{'metric_tag': 'time_to_first_token', 'stat': 'p95', 'op': 'lt', 'threshold': 10}],
            'metrics': {'time_to_first_token': {'name': 'score', 'unit': 'ms', 'stats': {'p95': 'p(95)'}}, 'output_token_throughput': {'name': 'score', 'unit': 'tok/s', 'stats': {'avg': 'avg'}}},
            'campaign_binary': os.environ.get('CAMPAIGN_SMOKE_BINARY', '/missing'), 'concurrency_parameter': 'CONCURRENCY', 'trial_timeout_seconds': 30, 'max_duration_seconds': 300,
            'campaign': {'binary': os.environ.get('K6_SMOKE_BINARY', '/missing'), 'script': '/missing', 'dimensions': [{'name': 'CONCURRENCY', 'values': ['1']}], 'mode': 'grid', 'seed': 5, 'objective': {'name': 'score', 'stat': 'avg'}, 'minimize': False, 'constraints': [], 'max_candidates': 1, 'max_attempts': 2, 'min_runs': 2, 'max_runs': 2, 'retries': 0, 'convergence': 'range', 'tolerance': 0, 'timeout': '10s', 'max_duration': '25s'}}


class PlannerTest(unittest.TestCase):
    def test_invalid_inputs(self):
        for key, value in [('low', True), ('high', 0), ('sla_replicates', -1), ('style', 'fake')]:
            cfg = configuration()
            cfg[key] = value
            with self.assertRaises(ValueError):
                bridge.validate(cfg)
        cfg = configuration()
        cfg['metrics'].pop('time_to_first_token')
        with self.assertRaises(ValueError):
            bridge.validate(cfg)

    def test_actual_upstream_planner_edges_replay(self):
        bridge.runtime_identity()
        from aiperf.common.models.export_models import JsonMetricResult
        from aiperf.orchestrator.models import RunResult
        for style in ('monotonic', 'smooth_isotonic'):
            for curve in ('smooth', 'cliff', 'allpass', 'allfail'):
                cfg = configuration()
                cfg['style'] = style
                first, second = bridge.planner(cfg), bridge.planner(cfg)
                seen = []
                while not first.is_converged():
                    proposal = first.ask()
                    if proposal is None:
                        break
                    variation = proposal[1]
                    x = variation.values[bridge.PATH]
                    score = {'smooth': x, 'cliff': 0 if x < 10 else 100, 'allpass': 0, 'allfail': 100}[curve]
                    results = [RunResult(label=str(i), success=True, summary_metrics={'time_to_first_token': JsonMetricResult(unit='ms', p95=score), 'output_token_throughput': JsonMetricResult(unit='tok/s', avg=x)}, variation_label=variation.label, variation_values=variation.values) for i in range(2)]
                    replay = second.ask()
                    self.assertEqual(replay[1].values, variation.values)
                    first.tell(variation, results)
                    second.tell(replay[1], results)
                    seen.append(x)
                self.assertLessEqual(len(seen), 30)
                self.assertEqual(bridge.boundary(first), bridge.boundary(second))
                self.assertEqual(first.convergence_reason(), second.convergence_reason())
                if curve == 'allfail':
                    self.assertIsNone(first.feasible_max)
                if curve == 'allpass':
                    self.assertIsNone(first.infeasible_min)
                if curve in ('smooth', 'cliff'):
                    self.assertLess(first.feasible_max, 10)
                    self.assertGreaterEqual(first.infeasible_min, 10)

    def test_owned_planner_deadlines(self):
        class Slow:
            def ask(self):
                time.sleep(30)
            def tell(self):
                time.sleep(30)
        for action in ('ask', 'tell'):
            start = time.monotonic()
            with self.assertRaises(TimeoutError):
                bridge.bounded(Slow(), action, (), start + .15)
            self.assertLess(time.monotonic() - start, 2)
        work = Path.home() / 'tmp/xk6-llm-perf/20261010-parity-recipes/tmp'
        work.mkdir(parents=True, exist_ok=True)
        with tempfile.TemporaryDirectory(dir=work) as temporary:
            pidfile = Path(temporary) / 'child.pid'
            class Descendant:
                def ask(self):
                    pid = os.fork()
                    if pid == 0:
                        signal.signal(signal.SIGTERM, signal.SIG_IGN)
                        pidfile.write_text(str(os.getpid()))
                        time.sleep(30)
                        os._exit(0)
                    time.sleep(30)
            with self.assertRaises(TimeoutError):
                bridge.bounded(Descendant(), 'ask', (), time.monotonic() + .3)
            pid = int(pidfile.read_text())
            status = subprocess.run(['ps', '-o', 'stat=', '-p', str(pid)], text=True, capture_output=True, timeout=3).stdout.strip()
            self.assertTrue(not status or status.startswith('Z'), status)

    def test_failed_planner_operation_receipt_refuses_retry(self):
        work = Path.home() / 'tmp/xk6-llm-perf/20261010-parity-recipes/tmp'
        work.mkdir(parents=True, exist_ok=True)
        with tempfile.TemporaryDirectory(dir=work) as temporary:
            cfg = configuration()
            asset = str(Path(__file__).resolve())
            cfg['campaign_binary'] = cfg['campaign']['binary'] = cfg['campaign']['script'] = asset
            output = Path(temporary) / 'failed'
            def slow(*args):
                time.sleep(.1)
                raise TimeoutError('injected planner timeout')
            with patch.object(bridge, 'runtime_identity', lambda: {}), patch.object(bridge, 'bounded', slow):
                with self.assertRaises(TimeoutError):
                    bridge.run(cfg, output)
                state = bridge.core.read_json(output / 'planner.json')
                self.assertGreaterEqual(state['elapsed_seconds'], .1)
                self.assertTrue(state['terminal_error'])
                self.assertEqual(state['trials'], [])
                with self.assertRaises(ValueError):
                    bridge.run(cfg, output)

    def test_partial_frame_deadlines(self):
        original = bridge.socket.socketpair
        for count in (4, 9):
            def pair():
                reader, writer = original()
                class Partial:
                    def close(self):
                        writer.close()
                    def sendall(self, data):
                        writer.sendall(data[:count])
                        time.sleep(30)
                return reader, Partial()
            start = time.monotonic()
            with patch.object(bridge.socket, 'socketpair', pair), self.assertRaises(TimeoutError):
                bridge.bounded(bridge.planner(configuration()), 'ask', (), start + .3)
            self.assertLess(time.monotonic() - start, 2)

    def test_sigma_and_degenerate_bootstrap_controls(self):
        bridge.runtime_identity()
        from aiperf.orchestrator.search_planner._margin_normalize import normalize_margins
        from aiperf.orchestrator.search_planner._replicate_budget import boundary_ci
        from aiperf.orchestrator.search_planner._smooth_isotonic_fit import smooth_isotonic_fit, find_root
        normalized, key = normalize_margins({'ttft': 50, 'itl': 500}, {'ttft': 5, 'itl': 80}, {'ttft': 200, 'itl': 1000})
        self.assertEqual(key, 'ttft')
        self.assertEqual(normalized, 10)
        curve, denoised = smooth_isotonic_fit([1, 2, 3, 4, 5], [-2, -.9, -1.1, 1, 2])
        self.assertEqual(denoised, [-2, -1, -1, 1, 2])
        self.assertGreater(find_root(curve, 1, 5), 3)
        self.assertLess(find_root(curve, 1, 5), 4)
        self.assertIsNone(find_root(curve, 1, 2))
        self.assertEqual(boundary_ci([2]), (2, 2))
        lo, hi = boundary_ci([-2, -1, 0, 1, 2], n_resamples=1000)
        self.assertLess(lo, 0)
        self.assertGreater(hi, 0)

    @unittest.skipUnless(os.environ.get('CAMPAIGN_SMOKE_BINARY') and os.environ.get('K6_SMOKE_BINARY'), 'set actual registered binaries')
    def test_actual_repeated_campaign_and_hash_refusal(self):
        work = Path.home() / 'tmp/xk6-llm-perf/20261010-parity-recipes/tmp'
        work.mkdir(parents=True, exist_ok=True)
        with tempfile.TemporaryDirectory(dir=work) as temporary:
            root = Path(temporary)
            script = root / 'workload.js'
            script.write_text("import { Trend } from 'k6/metrics'; const score = new Trend('score'); export const options = { vus: 1, iterations: 1 }; export default function () { score.add(Number(__ENV.CONCURRENCY)); }\n")
            cfg = configuration()
            cfg.update(style='monotonic', high=4)
            cfg['filters'][0]['threshold'] = 3
            cfg['campaign']['script'] = str(script)
            output = root / 'study'
            first = bridge.run(cfg, output)
            self.assertTrue(first['trials'])
            self.assertEqual(first['boundary']['feasible_max']['value'], 2)
            self.assertEqual(first['boundary']['infeasible_min']['value'], 3)
            resumed = bridge.run(cfg, output)
            self.assertGreater(resumed['elapsed_seconds'], first['elapsed_seconds'])
            first_compare, resumed_compare = copy.deepcopy(first), copy.deepcopy(resumed)
            first_compare.pop('elapsed_seconds')
            resumed_compare.pop('elapsed_seconds')
            self.assertEqual(first_compare, resumed_compare)
            checkpoint = output / 'planner.json'
            corrupted = copy.deepcopy(first)
            corrupted['boundary']['feasible_max']['value'] = 4
            bridge.core.write_json(checkpoint, corrupted)
            with self.assertRaises(ValueError):
                bridge.run(cfg, output)
            corrupted = copy.deepcopy(first)
            corrupted['trials'][-1]['state'] = 'RUNNING'
            bridge.core.write_json(checkpoint, corrupted)
            with self.assertRaises(ValueError):
                bridge.run(cfg, output)
            bridge.core.write_json(checkpoint, first)
            trial = first['trials'][0]
            relative = next(p for p in trial['artifact_sha256'] if p.endswith('summary.json'))
            (output / 'trial-0000' / relative).write_text('{}')
            with self.assertRaises(ValueError):
                bridge.run(cfg, output)


if __name__ == '__main__':
    unittest.main()
