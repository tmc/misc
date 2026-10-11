import copy
import importlib.util
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch


def load(name):
    spec = importlib.util.spec_from_file_location(name, Path(__file__).with_name(name + '.py'))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


categorical = load('optuna-campaign')
numeric = load('optuna-search')
outcomes = load('outcome-search')


class ReplayPolicyTest(unittest.TestCase):
    def categorical_config(self, script):
        return {'schema_version': 1, 'campaign_binary': os.environ['CAMPAIGN_SMOKE_BINARY'],
                'sampler': 'tpe', 'seed': 7, 'max_trials': 1,
                'trial_timeout_seconds': 20, 'max_duration_seconds': 90,
                'campaign': {'binary': os.environ['K6_SMOKE_BINARY'], 'script': str(script),
                             'dimensions': [{'name': 'SCORE', 'values': ['3']}], 'mode': 'grid',
                             'seed': 1, 'objective': {'name': 'score', 'stat': 'avg'}, 'minimize': False,
                             'constraints': [{'metric': {'name': 'score', 'stat': 'avg'}, 'direction': 'le', 'bound': 2}],
                             'max_candidates': 1, 'max_attempts': 2, 'min_runs': 2, 'max_runs': 2,
                             'retries': 0, 'convergence': 'range', 'tolerance': 0,
                             'timeout': '5s', 'max_duration': '15s'}}

    def configs(self, script):
        categorical_config = self.categorical_config(script)
        numeric_config = copy.deepcopy(categorical_config)
        numeric_config['campaign'].pop('dimensions')
        numeric_config['campaign'].pop('objective')
        numeric_config['campaign'].pop('minimize')
        numeric_config['domains'] = [{'name': 'SCORE', 'type': 'categorical', 'values': ['3']}]
        numeric_config['objectives'] = [{'metric': {'name': 'score', 'stat': 'avg'}, 'direction': 'maximize'}]
        outcome_config = copy.deepcopy(numeric_config)
        outcome_config.update(outcome_constraints=[], proposal_timeout_seconds=90)
        return ((categorical, categorical_config, 'optuna.json'),
                (numeric, numeric_config, 'optuna-search.json'),
                (outcomes, outcome_config, 'optuna-search.json'))

    @unittest.skipUnless(os.environ.get('K6_SMOKE_BINARY') and os.environ.get('CAMPAIGN_SMOKE_BINARY'), 'actual registered binaries required')
    def test_actual_policy_rebinding_refused(self):
        work = Path.home() / 'tmp/xk6-llm-perf/20261010-search-policy-controls/tmp'
        work.mkdir(parents=True, exist_ok=True)
        with tempfile.TemporaryDirectory(dir=work) as temporary:
            root = Path(temporary)
            script = root / 'score.js'
            script.write_text("import {Trend} from 'k6/metrics'; const score=new Trend('score'); export const options={vus:1,iterations:1}; export default function(){score.add(Number(__ENV.SCORE));}\n")
            for module, config, checkpoint in self.configs(script):
                with self.subTest(driver=module.__name__):
                    output = root / module.__name__
                    before = module.run(config, output)
                    self.assertEqual(before['trials'][0]['constraint_violations'], [1, 0])
                    self.assertEqual(module.run(config, output)['trials'], before['trials'])
                    manifest_path = output / 'trial-0000/campaign.json'
                    original_manifest = categorical.read_json(manifest_path)
                    original_state = categorical.read_json(output / checkpoint)
                    for change in ('constraints', 'mean', 'convergence', 'converged-flag', 'failed-flag', 'dimensions', 'attempt-value', 'number-bool', 'value-bool', 'mean-bool', 'converged-number', 'failed-number', 'passed-number', 'exit-bool'):
                        with self.subTest(change=change):
                            manifest = copy.deepcopy(original_manifest)
                            state = copy.deepcopy(original_state)
                            trial = state['trials'][0]
                            if change == 'constraints':
                                manifest['config']['constraints'] = []
                                manifest['candidates'][0]['feasible'] = True
                                trial['constraint_violations'] = [0]
                                trial['constraint_violation'] = 0
                            elif change == 'mean':
                                manifest['candidates'][0]['mean'] = 999
                                if module is categorical:
                                    trial['value'] = 999
                                else:
                                    trial['values'] = [999]
                            elif change == 'convergence':
                                manifest['config']['tolerance'] = 1
                            elif change == 'converged-flag':
                                manifest['candidates'][0]['converged'] = False
                            elif change == 'failed-flag':
                                manifest['candidates'][0]['failed'] = True
                            elif change in ('number-bool', 'value-bool', 'passed-number', 'exit-bool'):
                                key = {'number-bool': 'number', 'value-bool': 'value', 'passed-number': 'passed', 'exit-bool': 'exit_code'}[change]
                                manifest['candidates'][0]['attempts'][0][key] = 0 if change == 'passed-number' else False if change == 'exit-bool' else True
                            elif change in ('mean-bool', 'converged-number', 'failed-number'):
                                key = {'mean-bool': 'mean', 'converged-number': 'converged', 'failed-number': 'failed'}[change]
                                manifest['candidates'][0][key] = True if change == 'mean-bool' else 1 if change == 'converged-number' else 0
                            elif change == 'dimensions':
                                manifest['config']['dimensions'][0]['values'] = ['4']
                            else:
                                manifest['candidates'][0]['attempts'][0]['value'] = 999
                            categorical.write_json(manifest_path, manifest)
                            trial['artifact_sha256']['campaign.json'] = categorical.file_sha(manifest_path)
                            categorical.write_json(output / checkpoint, state)
                            with self.assertRaises(ValueError):
                                module.run(config, output)
                            categorical.write_json(manifest_path, original_manifest)
                            categorical.write_json(output / checkpoint, original_state)

    @unittest.skipUnless(os.environ.get('K6_SMOKE_BINARY') and os.environ.get('CAMPAIGN_SMOKE_BINARY'), 'hashed input binaries required')
    def test_exhausted_entry_starts_no_planner_or_validator(self):
        work = Path.home() / 'tmp/xk6-llm-perf/20261010-search-policy-controls/tmp'
        work.mkdir(parents=True, exist_ok=True)
        with tempfile.TemporaryDirectory(dir=work) as temporary:
            root = Path(temporary)
            script = root / 'unused.js'
            script.write_text('export default function(){}\n')
            validator = root / 'slow-validator.py'
            marker = root / 'validator-invoked'
            validator.write_text('#!' + os.sys.executable + '\nimport time\nfrom pathlib import Path\nPath(' + repr(str(marker)) + ').write_text("invoked")\ntime.sleep(2)\n')
            validator.chmod(0o700)
            for module, config, checkpoint in self.configs(script):
                config['campaign_binary'] = str(validator)
                output = root / module.__name__
                output.mkdir(mode=0o700)
                config['max_duration_seconds'] = 1
                identity = module.identities(config) if module is categorical else module.identity(config)
                categorical.write_json(output / checkpoint, {'schema_version': 1, 'identity': identity, 'elapsed_seconds': 1, 'trials': []})
                if module is categorical:
                    with patch.object(module, 'create_study', side_effect=AssertionError('planner launched')), patch.object(module, 'run_child', side_effect=AssertionError('validator launched')):
                        result = module.run(config, output)
                    self.assertEqual(result['stop_reason'], 'duration_budget')
                else:
                    target = module if module is numeric else module.search
                    with patch.object(target, 'create_study', side_effect=AssertionError('planner launched')), patch.object(target.core, 'run_child', side_effect=AssertionError('validator launched')):
                        with self.assertRaises(numeric.PlannerDeadline if module is numeric else target.PlannerDeadline):
                            module.run(config, output)
                self.assertGreaterEqual(categorical.read_json(output / checkpoint)['elapsed_seconds'], 1)
                # Exercise the actual slow executable without run_child mocking.
                if module is categorical:
                    result = module.run(config, output)
                    self.assertEqual(result['stop_reason'], 'duration_budget')
                else:
                    target = module if module is numeric else module.search
                    with self.assertRaises(target.PlannerDeadline):
                        module.run(config, output)
                self.assertFalse(marker.exists())

    @unittest.skipUnless(os.environ.get('K6_SMOKE_BINARY') and os.environ.get('CAMPAIGN_SMOKE_BINARY'), 'hashed input binaries required')
    def test_slow_identity_and_setup_start_no_evaluation(self):
        work = Path.home() / 'tmp/xk6-llm-perf/20261010-search-policy-controls/tmp'
        work.mkdir(parents=True, exist_ok=True)
        with tempfile.TemporaryDirectory(dir=work) as temporary:
            root = Path(temporary)
            script = root / 'unused.js'
            script.write_text('export default function(){}\n')
            for module, config, _ in self.configs(script):
                config['max_duration_seconds'] = 1
                target = module if module in (categorical, numeric) else module.search
                core = target if module is categorical else target.core
                for cause in ('identity', 'setup'):
                    with self.subTest(driver=module.__name__, cause=cause):
                        clock = [0]
                        calls = []
                        identity_name = 'identities' if module is categorical else 'identity'
                        identity_owner = module if module is outcomes else target
                        identity_function = getattr(identity_owner, identity_name)
                        identity_calls = [0]
                        writer = core.write_json
                        def identity(value):
                            result = identity_function(value)
                            identity_calls[0] += 1
                            if cause == 'identity' and identity_calls[0] == 2:
                                clock[0] += 2
                            return result
                        def write(path, value):
                            writer(path, value)
                            if cause == 'setup' and path.name == 'config.json' and path.parent.name.startswith('trial-'):
                                clock[0] += 2
                        def child(arguments, directory, timeout):
                            calls.append(arguments)
                            self.assertIn('-validate', arguments)
                            self.assertLessEqual(timeout, 1)
                            return 0
                        with patch.object(target.time, 'monotonic', side_effect=lambda: clock[0]), patch.object(identity_owner, identity_name, side_effect=identity), patch.object(core, 'write_json', side_effect=write), patch.object(core, 'run_child', side_effect=child):
                            result = module.run(config, root / (module.__name__ + '-' + cause))
                        self.assertEqual(result['stop_reason'], 'duration_budget')
                        self.assertEqual(len(calls), 1)
                        if cause == 'setup':
                            self.assertEqual(result['trials'][0]['state'], 'FAIL')
                            self.assertIn('evaluation not started', result['trials'][0]['reason'])


    @unittest.skipUnless(os.environ.get('K6_SMOKE_BINARY') and os.environ.get('CAMPAIGN_SMOKE_BINARY'), 'hashed input binaries required')
    def test_replay_stops_after_budget_exhausting_proposal(self):
        work = Path.home() / 'tmp/xk6-llm-perf/20261010-search-policy-controls/tmp'
        work.mkdir(parents=True, exist_ok=True)
        with tempfile.TemporaryDirectory(dir=work) as temporary:
            root = Path(temporary)
            script = root / 'unused.js'
            script.write_text('export default function(){}\n')
            for module, config, checkpoint in self.configs(script):
                with self.subTest(driver=module.__name__):
                    config.update(max_duration_seconds=1, max_trials=2)
                    output = root / module.__name__
                    output.mkdir(mode=0o700)
                    identity = module.identities(config) if module is categorical else module.identity(config)
                    categorical.write_json(output / checkpoint, {'schema_version': 1, 'identity': identity, 'elapsed_seconds': 0, 'trials': [{'number': number, 'state': 'FAIL'} for number in range(2)]})
                    target = module if module in (categorical, numeric) else module.search
                    core = target if module is categorical else target.core
                    clock, count = [0], [0]
                    original_ask = target.ask
                    def ask(*args):
                        result = original_ask(*args)
                        count[0] += 1
                        clock[0] += 2
                        return result
                    with patch.object(target.time, 'monotonic', side_effect=lambda: clock[0]), patch.object(target, 'ask', side_effect=ask), patch.object(core, 'run_child', side_effect=AssertionError('validation after exhausted replay')):
                        if module is categorical:
                            self.assertEqual(module.run(config, output)['stop_reason'], 'duration_budget')
                        else:
                            with self.assertRaises(target.PlannerDeadline):
                                module.run(config, output)
                    self.assertEqual(count[0], 1)
                    self.assertGreaterEqual(categorical.read_json(output / checkpoint)['elapsed_seconds'], 2)


    @unittest.skipUnless(os.environ.get('K6_SMOKE_BINARY') and os.environ.get('CAMPAIGN_SMOKE_BINARY'), 'actual registered binaries required')
    def test_utf8_and_html_safe_cell_identity(self):
        work = Path.home() / 'tmp/xk6-llm-perf/20261010-search-policy-controls/tmp'
        work.mkdir(parents=True, exist_ok=True)
        with tempfile.TemporaryDirectory(dir=work) as temporary:
            root = Path(temporary)
            script = root / 'score.js'
            script.write_text("import {Trend} from 'k6/metrics'; const score=new Trend('score'); export const options={vus:1,iterations:1}; export default function(){score.add(Number(__ENV.SCORE));}\n")
            for module, config, _ in self.configs(script):
                label = {'name': 'LABEL', 'values': ['é<&>\u2028\u2029']}
                if module is categorical:
                    config['campaign']['dimensions'].append(label)
                else:
                    config['domains'].append(dict(label, type='categorical'))
                output = root / module.__name__
                first = module.run(config, output)
                self.assertEqual(first['trials'][0]['state'], 'COMPLETE')
                self.assertEqual(module.run(config, output)['trials'], first['trials'])


    @unittest.skipUnless(os.environ.get('K6_SMOKE_BINARY') and os.environ.get('CAMPAIGN_SMOKE_BINARY'), 'actual registered binaries required')
    def test_boolean_aliases_of_unit_objective_refused(self):
        work = Path.home() / 'tmp/xk6-llm-perf/20261010-search-policy-controls/tmp'
        work.mkdir(parents=True, exist_ok=True)
        with tempfile.TemporaryDirectory(dir=work) as temporary:
            root = Path(temporary)
            script = root / 'unit.js'
            script.write_text("import {Trend} from 'k6/metrics'; const score=new Trend('score'); export const options={vus:1,iterations:1}; export default function(){score.add(1);}\n")
            for module, config, checkpoint in self.configs(script):
                if module is categorical:
                    config['campaign']['dimensions'][0]['values'] = ['1']
                else:
                    config['domains'][0]['values'] = ['1']
                output = root / module.__name__
                first = module.run(config, output)
                self.assertEqual(first['trials'][0]['state'], 'COMPLETE')
                manifest_path = output / 'trial-0000/campaign.json'
                baseline_manifest = categorical.read_json(manifest_path)
                baseline_state = categorical.read_json(output / checkpoint)
                for location, key, value in (('cell', 'mean', True), ('attempt', 'value', True),
                                             ('cell', 'converged', 1), ('cell', 'failed', 0),
                                             ('attempt', 'passed', 1), ('attempt', 'exit_code', False),
                                             ('attempt', 'number', True)):
                    with self.subTest(driver=module.__name__, location=location, key=key):
                        manifest = copy.deepcopy(baseline_manifest)
                        state = copy.deepcopy(baseline_state)
                        cell = manifest['candidates'][0]
                        target = cell if location == 'cell' else cell['attempts'][0]
                        target[key] = value
                        categorical.write_json(manifest_path, manifest)
                        state['trials'][0]['artifact_sha256']['campaign.json'] = categorical.file_sha(manifest_path)
                        categorical.write_json(output / checkpoint, state)
                        with self.assertRaises(ValueError):
                            module.run(config, output)
                        categorical.write_json(manifest_path, baseline_manifest)
                        categorical.write_json(output / checkpoint, baseline_state)



if __name__ == '__main__':
    unittest.main()
