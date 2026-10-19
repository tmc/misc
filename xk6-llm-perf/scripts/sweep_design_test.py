import copy
import importlib.util
import math
import os
from pathlib import Path
from types import SimpleNamespace
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('sweep_design', Path(__file__).with_name('sweep-design.py'))
bridge = importlib.util.module_from_spec(spec)
spec.loader.exec_module(bridge)


class DesignTest(unittest.TestCase):
    def test_zip_and_scenarios(self):
        rows = bridge.design({'type': 'zip', 'parameters': {'ISL': [1, 2], 'OSL': [4, 8]}})
        self.assertEqual([r['parameters'] for r in rows], [{'ISL': '1', 'OSL': '4'}, {'ISL': '2', 'OSL': '8'}])
        rows = bridge.design({'type': 'scenarios', 'base': {'ISL': 8, 'OSL': 4}, 'runs': [{'name': 'first', 'parameters': {'OSL': 2}}, {'name': 'duplicate-values', 'parameters': {'OSL': 2}}]})
        self.assertEqual(rows[0]['parameters'], {'ISL': '8', 'OSL': '2'})
        self.assertEqual(rows[0]['parameters'], rows[1]['parameters'])
        self.assertNotEqual(rows[0]['proposal_sha256'], rows[1]['proposal_sha256'])

    def test_refusals(self):
        designs = [
            {'type': 'halton'}, {'type': 'zip', 'parameters': {'A': [1], 'B': [1, 2]}},
            {'type': 'zip', 'parameters': {'PATH': [1]}},
            {'type': 'scenarios', 'base': {}, 'runs': [{'name': '../escape', 'parameters': {'A': 1}}]},
            {'type': 'scenarios', 'base': {}, 'runs': [{'name': 'a', 'parameters': {'A': {'nested': 1}}}]},
            {'type': 'sobol', 'samples': 4097, 'seed': 1, 'dimensions': []},
            {'type': 'latin_hypercube', 'samples': 2, 'seed': True, 'dimensions': []},
            {'type': 'sobol', 'samples': 4, 'seed': 1, 'dimensions': [{'name': 'A', 'lo': 1, 'hi': float('inf')}]},
            {'type': 'sobol', 'samples': 4, 'seed': 1, 'dimensions': [{'name': 'A', 'lo': .1, 'hi': .2, 'kind': 'int'}]},
        ]
        for design in designs:
            with self.subTest(design=design), self.assertRaises(ValueError):
                bridge.design(design)

    def test_actual_scipy_source_oracle(self):
        source = os.environ.get('AIPERF_QMC_SOURCE')
        if not source:
            self.skipTest('set pinned upstream expand_qmc.py source')
        path = Path(source)
        expected = os.environ['AIPERF_QMC_SHA256']
        self.assertEqual(bridge.core.file_sha(path), expected)
        from aiperf.config.sweep.expand_qmc import _map_dim, _qmc_engine
        namespace = {'_map_dim': _map_dim, '_qmc_engine': _qmc_engine}
        dimensions = [{'name': 'A', 'lo': 1, 'hi': 64, 'kind': 'int', 'scale': 'log'}, {'name': 'B', 'lo': 2, 'hi': 8}, {'name': 'C', 'choices': ['x', 'y']}]
        for kind, options in [('sobol', {'scramble': True}), ('sobol', {'scramble': False}), ('latin_hypercube', {'optimization': None}), ('latin_hypercube', {'optimization': 'random-cd'})]:
            config = dict(type=kind, samples=8, seed=17, dimensions=dimensions, **options)
            rows = bridge.design(config)
            engine = namespace['_qmc_engine'](kind, 3, 17, options)
            cube = engine.random_base2(m=3) if kind == 'sobol' else engine.random(n=8)
            for row, coords in zip(rows, cube, strict=True):
                expected = {}
                for dim, u in zip(dimensions, coords, strict=True):
                    d = SimpleNamespace(choices=dim.get('choices'), lo=dim.get('lo'), hi=dim.get('hi'), scale=dim.get('scale', 'linear'), kind=dim.get('kind', 'real'))
                    expected[dim['name']] = bridge.scalar(namespace['_map_dim'](float(u), d))
                self.assertEqual(row['parameters'], expected)
            self.assertEqual(rows, bridge.design(config))
        # Discretization collisions are samples, not automatically deduplicated cells.
        rows = bridge.design({'type': 'sobol', 'samples': 4, 'seed': 1, 'dimensions': [{'name': 'A', 'choices': ['same']} ]})
        self.assertEqual(len(rows), 4)
        self.assertEqual(len({r['proposal_sha256'] for r in rows}), 4)

    def configuration(self):
        asset = str(Path(__file__).resolve())
        return {'schema_version': 1, 'design': {'type': 'zip', 'parameters': {'A': [1, 1]}}, 'campaign_binary': asset, 'campaign': {'binary': asset, 'script': asset, 'dimensions': [], 'mode': 'grid', 'convergence': 'range'}, 'trial_timeout_seconds': 2, 'max_duration_seconds': 10}

    def test_retained_resume_and_changed_proposal(self):
        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary)/'study'
            config = self.configuration()
            policies = []
            def observed(directory, parameters, policy):
                policies.append(copy.deepcopy(policy))
                return {'state': 'COMPLETE', 'artifact_sha256': {'fixture': bridge.core.sha(parameters)}}
            with patch.object(bridge.core, 'run_child', return_value=0) as child, patch.object(bridge.core, 'observed', observed):
                first = bridge.run(config, output)
                self.assertEqual(child.call_count, 2) # Duplicate values retain separate process attempts.
                self.assertEqual([r['state'] for r in first['trials']], ['COMPLETE', 'COMPLETE'])
                second = bridge.run(config, output)
                self.assertEqual(child.call_count, 2)
                self.assertEqual(first['trials'], second['trials'])
                self.assertTrue(all(p['max_candidates'] == 1 and p['mode'] == 'grid' for p in policies))
                checkpoint = bridge.core.read_json(output/'design.json')
                checkpoint['trials'][0]['parameters']['A'] = 'changed'
                bridge.core.write_json(output/'design.json', checkpoint)
                with self.assertRaisesRegex(ValueError, 'proposal replay'):
                    bridge.run(config, output)

    def test_failed_attempt_is_preserved(self):
        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary)/'study'
            with patch.object(bridge.core, 'run_child', return_value=7) as child:
                with self.assertRaisesRegex(ValueError, 'exit code 7'):
                    bridge.run(self.configuration(), output)
                state = bridge.core.read_json(output/'design.json')
                self.assertEqual(state['trials'][0]['state'], 'FAIL')
                self.assertEqual(state['stop_reason'], 'proposal_failed')
                with self.assertRaisesRegex(ValueError, 'cannot be retried'):
                    bridge.run(self.configuration(), output)
                self.assertEqual(child.call_count, 1)



class WorkerTest(unittest.TestCase):
    @unittest.skipUnless(os.environ.get('CAMPAIGN_SMOKE_BINARY') and os.environ.get('K6_SMOKE_BINARY'), 'set existing historical worker binaries')
    def test_actual_campaign_worker_and_resume(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            workload = root/'workload.js'
            workload.write_text("import { Trend } from 'k6/metrics'; const score = new Trend('score'); export const options = {vus:1,iterations:1}; export default function(){score.add(Number(__ENV.A));}\n")
            campaign = {'binary': os.environ['K6_SMOKE_BINARY'], 'script': str(workload), 'dimensions': [], 'mode': 'grid', 'seed': 5, 'objective': {'name': 'score', 'stat': 'avg'}, 'minimize': False, 'constraints': [], 'max_candidates': 1, 'max_attempts': 2, 'min_runs': 2, 'max_runs': 2, 'retries': 0, 'convergence': 'range', 'tolerance': 0, 'timeout': '10s', 'max_duration': '25s'}
            config = {'schema_version': 1, 'design': {'type': 'zip', 'parameters': {'A': [2, 2]}}, 'campaign_binary': os.environ['CAMPAIGN_SMOKE_BINARY'], 'campaign': campaign, 'trial_timeout_seconds': 30, 'max_duration_seconds': 90}
            output = root/'study'
            first = bridge.run(config, output)
            self.assertEqual(first['stop_reason'], 'design_exhausted')
            self.assertEqual(len(first['trials']), 2)
            self.assertEqual([t['value'] for t in first['trials']], [2, 2])
            second = bridge.run(config, output)
            self.assertEqual(first['trials'], second['trials'])
            artifact = next((output/'trial-0000').glob('*/attempt-0001/summary.json'))
            artifact.write_text('{}')
            with self.assertRaisesRegex(ValueError, 'hash mismatch'):
                bridge.run(config, output)

class UpstreamExpansionTest(unittest.TestCase):
    def test_actual_zip_and_scalar_scenario_reference(self):
        from aiperf.config.sweep.expand import expand_sweep
        zip_config = {'type': 'zip', 'parameters': {'ISL': [1, 2], 'OSL': [4, 8]}}
        source = {'benchmark': {}, 'sweep': {'type': 'zip', 'parameters': {'variables.'+k: v for k, v in zip_config['parameters'].items()}}}
        expected = [variant['variables'] for variant, variation in expand_sweep(source)]
        self.assertEqual([r['parameters'] for r in bridge.design(zip_config)], [{k: bridge.scalar(v) for k, v in row.items()} for row in expected])
        config = {'type': 'scenarios', 'base': {'ISL': 8, 'OSL': 4}, 'runs': [{'name': 'first', 'parameters': {'OSL': 2}}, {'name': 'second', 'parameters': {'ISL': 16}}]}
        source = {'benchmark': {}, 'variables': config['base'], 'sweep': {'type': 'scenarios', 'runs': [{'name': r['name'], 'variables': r['parameters']} for r in config['runs']]}}
        expected = [variant['variables'] for variant, variation in expand_sweep(source)]
        self.assertEqual([r['parameters'] for r in bridge.design(config)], [{k: bridge.scalar(v) for k, v in row.items()} for row in expected])

class TypedSweepTest(unittest.TestCase):
    def envelope(self):
        return {'benchmark': {'model': 'local', 'endpoint': {'type': 'chat', 'url': 'http://unused.invalid'},
                              'dataset': {'type': 'synthetic', 'entries': 1},
                              'phases': [{'name': 'profiling', 'type': 'concurrency', 'concurrency': 1, 'requests': 1}]},
                'random_seed': 7, 'sweep': {'type': 'zip', 'parameters': {'concurrency': [1, 2]}, 'cooldown_seconds': 0.25},
                'multi_run': {'num_runs': 2, 'cooldown_seconds': 0.5, 'disable_warmup_after_first': False}}

    def test_actual_order_cooldowns_seeds_and_normalization(self):
        value = self.envelope()
        plan = bridge.aiperf_plan(value)
        self.assertEqual([(e['variation_index'], e['trial_index']) for e in plan['events']], [(0, 0), (1, 0), (0, 1), (1, 1)])
        self.assertEqual([e['cooldowns_before'] for e in plan['events']], [[], [0.25], [0.5], [0.25]])
        self.assertEqual([e['seed'] for e in plan['events']], [7, 8, 7, 8])
        self.assertEqual(plan['events'][0]['authored_benchmark']['datasets'][0]['name'], 'default')
        value['sweep']['iteration_order'] = 'independent'
        plan = bridge.aiperf_plan(value)
        self.assertEqual([(e['variation_index'], e['trial_index']) for e in plan['events']], [(0, 0), (0, 1), (1, 0), (1, 1)])
        self.assertEqual([e['cooldowns_before'] for e in plan['events']], [[], [0.5], [0.25], [0.5]])
        value['sweep']['same_seed'] = True
        self.assertEqual(bridge.aiperf_plan(value)['variation_seeds'], [7, 7])
        value['multi_run']['vary_seed_per_trial'] = True
        plan = bridge.aiperf_plan(value)
        from aiperf.common.random_generator import derive_variation_seed
        self.assertEqual([e['seed'] for e in plan['events']], [derive_variation_seed(7, e['label']+':trial:'+str(e['trial_index'])) for e in plan['events']])
        del value['random_seed']
        value['multi_run']['set_consistent_seed'] = False
        self.assertEqual([e['seed'] for e in bridge.aiperf_plan(value)['events']], [None]*4)

    def test_actual_scenario_dataset_merge(self):
        value = self.envelope()
        value['benchmark']['dataset']['prompts'] = {'isl': {'mean': 8, 'stddev': 0}}
        value['sweep'] = {'type': 'scenarios', 'runs': [
            {'name': 'short', 'benchmark': {'dataset': {'entries': 2, 'prompts': {'isl': {'mean': 4}}}}},
            {'name': 'same', 'benchmark': {'dataset': {'entries': 2, 'prompts': {'isl': {'mean': 4}}}}}]}
        plan = bridge.aiperf_plan(value)
        ds = plan['events'][0]['benchmark']['datasets'][0]
        self.assertEqual(ds['entries'], 2)
        self.assertEqual(ds['prompts']['isl']['mean'], 4)
        self.assertEqual(ds['prompts']['isl']['stddev'], 0)
        self.assertEqual([v['label'] for v in plan['variations']], ['short', 'same'])
        self.assertEqual(len(plan['events']), 4)

    def config(self):
        value = self.envelope()
        plan = bridge.aiperf_plan(value)
        mapping = {'B%d' % i: path for i, path in enumerate(bridge.leaves(plan['events'][0]['authored_benchmark']))}
        asset = str(Path(__file__).resolve())
        return {'schema_version': 1, 'envelope': value, 'environment': mapping,
                'binary': asset, 'script': asset, 'seed_environment': 'WORKLOAD_SEED',
                'trial_timeout_seconds': 5, 'max_duration_seconds': 20}

    def test_actual_console_log_format_and_receipt_refusals(self):
        import json
        event = {'parameters': {'A': '0'}, 'seed': 9007199254740993}
        payload = {'parameters': {'A': '0'}, 'seed': '9007199254740993'}
        message = 'SWEEP_APPLIED '+json.dumps(payload)
        actual = 'time=\"2026-10-18T19:41:15-07:00\" level=info msg='+json.dumps(message)+' source=console\n'
        with tempfile.TemporaryDirectory() as temporary:
            log = Path(temporary)/'driver.log'
            for text in (message+'\n', actual):
                log.write_text(text)
                bridge.applied_receipt(log, event)
            for text in (actual+actual, 'SWEEP_APPLIED broken\n', message.replace('9007199254740993', '7'), message.replace('\"seed\":', '\"seed\":\"1\",\"seed\":')):
                log.write_text(text)
                with self.assertRaises(ValueError):
                    bridge.applied_receipt(log, event)

    def test_mapped_execution_receipt_resume_failure_and_refusals(self):
        config = self.config()
        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary)/'run'
            def child(argv, directory, timeout):
                values = dict(argv[i+1].split('=', 1) for i, arg in enumerate(argv[:-1]) if arg == '--env')
                seed = values.pop('WORKLOAD_SEED')
                (directory/'driver.log').write_text('SWEEP_APPLIED '+bridge.core.encoded({'parameters': values, 'seed': seed}).decode()+'\n')
                bridge.core.write_json(directory/'summary.json', {'metrics': {}})
                return 0
            with patch.object(bridge.core, 'run_child', side_effect=child) as worker, patch.object(bridge.time, 'sleep') as sleep:
                first = bridge.aiperf_run(config, output)
                self.assertEqual(worker.call_count, 4)
                self.assertEqual([c.args[0] for c in sleep.call_args_list], [.25, .5, .25])
                second = bridge.aiperf_run(config, output)
                self.assertEqual(worker.call_count, 4)
                self.assertEqual(first['attempts'], second['attempts'])
                (output/'cell-0000/driver.log').write_text('changed')
                with self.assertRaisesRegex(ValueError, 'artifact differs'):
                    bridge.aiperf_run(config, output)
        for defect in ('mapping', 'seed', 'cooldown', 'convergence', 'type', 'warmup'):
            bad = copy.deepcopy(config)
            if defect == 'mapping':
                bad['environment'].pop(next(iter(bad['environment'])))
            elif defect == 'seed':
                del bad['envelope']['random_seed']
            elif defect == 'cooldown':
                bad['envelope']['sweep']['cooldown_seconds'] = 61
            elif defect == 'convergence':
                bad['envelope']['multi_run']['convergence'] = {'type': 'mean'}
            elif defect == 'type':
                bad['envelope']['sweep']['type'] = 'adaptive_search'
            else:
                bad['envelope']['multi_run']['disable_warmup_after_first'] = True
            with self.subTest(defect=defect), self.assertRaises(Exception):
                bridge.mapped_plan(bad)
        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary)/'failure'
            with patch.object(bridge.core, 'run_child', return_value=7) as worker:
                with self.assertRaisesRegex(ValueError, 'exit code 7'):
                    bridge.aiperf_run(config, output)
                with self.assertRaisesRegex(ValueError, 'failed/interrupted'):
                    bridge.aiperf_run(config, output)
                self.assertEqual(worker.call_count, 1)

if __name__ == '__main__':
    unittest.main()
