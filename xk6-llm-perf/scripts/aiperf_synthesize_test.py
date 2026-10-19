import copy
import importlib.util
import json
import math
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('synthesis', Path(__file__).with_name('aiperf-synthesize.py'))
bridge = importlib.util.module_from_spec(spec)
spec.loader.exec_module(bridge)


def fixture():
    return {'schema_version': 1, 'seed': 7, 'grouped': False, 'params': {'block_size': 4}, 'traces': [
        {'timestamp': 15.5, 'input_length': 12, 'output_length': 5, 'hash_ids': [1, 2, 3], 'extra': {'kept': [None, True, 'fixture']}},
        {'timestamp': 30, 'input_length': 12, 'output_length': 7, 'hash_ids': [1, 2, 4], 'id': 'second'},
        {'timestamp': 40, 'text_input': 'literal input', 'output_length': 0, 'custom': 'kept'}]}


class DecodeTest(unittest.TestCase):
    def test_strict_json(self):
        for data in (b'{"x":1,"x":2}', b'{"x":NaN}', b'{"x":1e999}', b'[' * 65 + b'0' + b']' * 65):
            with self.assertRaises((ValueError, RecursionError)):
                bridge.decode(data)


@unittest.skipUnless(os.environ.get('AIPERF_SYNTHESIS_REFERENCE') == '1', 'actual pinned AIPerf runtime required')
class ReferenceTest(unittest.TestCase):
    def test_actual_oracle(self):
        bridge.runtime_identity()
        from aiperf.common import random_generator as rng
        from aiperf.dataset.synthesis.models import SynthesisParams
        from aiperf.dataset.synthesis.synthesizer import Synthesizer
        for params in ({}, {'prefix_len_multiplier': 1.5, 'prompt_len_multiplier': .5, 'output_len_multiplier': .5, 'speedup_ratio': 2},
                       {'prefix_len_multiplier': .5, 'prefix_root_multiplier': 3},
                       {'prefix_len_multiplier': 0, 'prompt_len_multiplier': 0, 'max_isl': 2, 'max_osl': 3},
                       {'renormalize_hash_ids': True, 'prefix_root_multiplier': 4}):
            for grouped in (False, True):
                with self.subTest(params=params, grouped=grouped):
                    config = fixture()
                    config['params'].update(params)
                    config['grouped'] = grouped
                    if grouped:
                        config['traces'][0]['session_id'] = 'embedded-overridden'
                        config['traces'] = {'z-first': config['traces'][:2], 'empty': [], 'a-last': config['traces'][2:]}
                    result = bridge.synthesize(copy.deepcopy(config))
                    rng.reset()
                    rng.init(config['seed'])
                    oracle = Synthesizer(SynthesisParams(**config['params']))
                    expected = oracle.synthesize_grouped_traces(config['traces']) if grouped else oracle.synthesize_traces(config['traces'])
                    self.assertEqual(result['traces'], expected)
                    self.assertEqual(result, bridge.synthesize(copy.deepcopy(config)))
                    if grouped:
                        self.assertEqual(list(result['traces']), ['z-first', 'empty', 'a-last'])
                        self.assertEqual(result['traces']['empty'], [])
                    else:
                        self.assertEqual(result['traces'][0]['extra'], fixture()['traces'][0]['extra'])
                        self.assertEqual(result['traces'][2]['text_input'], 'literal input')
                        self.assertNotIn('input_length', result['traces'][2])
        for grouped, traces in ((False, []), (True, {}), (True, {'empty': []})):
            config = fixture()
            config.update(grouped=grouped, traces=traces)
            self.assertEqual(bridge.synthesize(config)['traces'], traces)
        config = fixture()
        config['params'].update(speedup_ratio=2, output_len_multiplier=.5)
        traces = bridge.synthesize(config)['traces']
        self.assertEqual(traces[0]['timestamp'], 7)
        self.assertEqual(traces[0]['output_length'], 2)  # round(2.5) ties to even
        self.assertEqual(traces[1]['output_length'], 4)
        self.assertEqual(traces[2]['output_length'], 1)
        config = fixture()
        config['params']['prefix_root_multiplier'] = 8
        first = bridge.synthesize(config)['traces']
        config['seed'] = 19
        self.assertNotEqual(first, bridge.synthesize(config)['traces'])

    def test_abuse_before_synthesis(self):
        for key, value in [('seed', True), ('seed', -1), ('grouped', 1)]:
            config = fixture()
            config[key] = value
            with self.assertRaises(ValueError):
                bridge.validate(config)
        for params in ({'prefix_len_multiplier': math.inf}, {'prompt_len_multiplier': -1}, {'prefix_root_multiplier': 65},
                       {'output_len_multiplier': 17}, {'block_size': True}, {'speedup_ratio': .001}, {'renormalize_hash_ids': 1},
                       {'prefix_len_multiplier': True}, {'extra': 1}):
            config = fixture()
            config['params'].update(params)
            with self.assertRaises(ValueError):
                bridge.validate(config)
        for field, value in [('input_length', -1), ('output_length', -1), ('timestamp', math.nan),
                             ('hash_ids', [9007199254740992]), ('hash_ids', [True]), ('hash_ids', [-1])]:
            config = fixture()
            config['traces'][0][field] = value
            with self.assertRaises(ValueError):
                bridge.validate(config)
        config = fixture()
        config['traces'] *= 334
        with self.assertRaises(ValueError):
            bridge.validate(config)
        config = fixture()
        config['params']['prompt_len_multiplier'] = 16
        config['traces'] = [{'input_length': 999999, 'hash_ids': [1], 'output_length': 1}]
        with self.assertRaises(ValueError):
            bridge.validate(config)
        config = fixture()
        config['params'].update(block_size=1, prefix_len_multiplier=16, prompt_len_multiplier=16)
        config['traces'] = [{'input_length': 1024, 'hash_ids': list(range(1024))}] * 2
        with self.assertRaises(ValueError):
            bridge.validate(config)

    def test_bounded_cli_grouped_order(self):
        root = Path.home() / 'tmp/xk6-llm-perf/20261018-aiperf-parity/task/synthesis/tmp'
        root.mkdir(parents=True, exist_ok=True)
        with tempfile.TemporaryDirectory(dir=root) as work:
            directory = Path(work)
            config = fixture()
            config.update(grouped=True, traces={'z-first': config['traces'][:2], 'empty': [], 'a-last': config['traces'][2:]})
            source = directory / 'input.json'
            source.write_text(json.dumps(config))
            process = subprocess.run([sys.executable, str(Path(bridge.__file__)), str(source), '--output', str(directory / 'output')], capture_output=True, timeout=40)
            self.assertEqual(process.returncode, 0, process.stderr.decode())
            output = json.loads((directory / 'output/traces.json').read_bytes())
            rows = [json.loads(line) for line in (directory / 'output/traces.jsonl').read_bytes().splitlines()]
            self.assertEqual(list(output), ['z-first', 'empty', 'a-last'])
            self.assertEqual([r['session_id'] for r in rows], ['z-first', 'z-first', 'a-last'])
            receipt = json.loads((directory / 'output/receipt.json').read_bytes())
            self.assertEqual(receipt['input_sha256'], bridge.sha(source.read_bytes()))
            self.assertEqual(receipt['output_sha256']['traces.json'], bridge.sha((directory / 'output/traces.json').read_bytes()))


if __name__ == '__main__':
    unittest.main()
