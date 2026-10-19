"""Strict admission and actual pinned-source/HF/Go realization controls."""
import copy
import contextlib
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

SCRIPT = Path(__file__).with_name('aiperf-realize-trace.py')
spec = importlib.util.spec_from_file_location('realize', SCRIPT)
bridge = importlib.util.module_from_spec(spec)
spec.loader.exec_module(bridge)


def fixture(targets=(32,), hashes=None, scope='file'):
    hashes = hashes or [[] for _ in targets]
    trace = {'format': 'mooncake-jsonl-v1', 'sourceHash': '1'*64, 'sessions': [
        {'session_id': 'one', 'contextMode': 'live', 'blockSize': 16, 'hashScope': scope,
         'turns': [{'id': str(i), 'sourceRow': i+1, 'inputMode': 'counts', 'inputTokens': target,
                    'inputTarget': None, 'outputBudget': 8, 'hashIDs': ids}
                   for i, (target, ids) in enumerate(zip(targets, hashes, strict=True))]}]}
    config = {'schema_version': 1, 'trace': '/missing', 'trace_sha256': '2'*64, 'source_sha256': '1'*64,
              'assets': '/missing', 'asset_manifest': '/missing', 'asset_manifest_sha256': '3'*64,
              'model': 'Qwen/Qwen3-0.6B', 'model_revision': 'local-pinned-assets', 'go_tokenizer': '/missing',
              'go_tokenizer_sha256': '4'*64, 'python_sha256': '5'*64, 'seed': 18}
    return config, trace


class Admission(unittest.TestCase):
    def test_strict_json(self):
        for data in (b'{"x":1,"x":2}', b'{"x":NaN}', b'{"x":"\xff"}'):
            with self.subTest(data=data), self.assertRaises((ValueError, UnicodeError)):
                bridge.decode(data)

    def test_boundaries_and_exact_hash_strings(self):
        for target in (0, 1, 4096):
            config, trace = fixture((target,))
            self.assertEqual(bridge.validate(config, trace)[0]['target'], target)
        config, trace = fixture((16,), [['9223372036854775807']])
        self.assertEqual(bridge.validate(config, trace)[0]['hash_ids'], ['9223372036854775807'])
        for target in (-1, 4097, True, None):
            config, trace = fixture((target,))
            with self.assertRaises(ValueError):
                bridge.validate(config, trace)

    def test_refusals(self):
        config, trace = fixture()
        for mutate in (
            lambda c, t: c.update(ignored_option=True),
            lambda c, t: t.update(sourceHash='0'*64),
            lambda c, t: t['sessions'][0].update(hashScope='global'),
            lambda c, t: t['sessions'][0]['turns'][0].update(hashIDs=[9007199254740993]),
            lambda c, t: t['sessions'][0]['turns'][0].update(hashIDs=['9223372036854775808']),
            lambda c, t: t['sessions'][0]['turns'][0].update(inputMode='messages'),
            lambda c, t: t['sessions'][0]['turns'][0].update(tools=[{}]),
            lambda c, t: t['sessions'][0]['turns'][0].update(outputBudget=0),
            lambda c, t: t['sessions'][0]['turns'][0].update(outputBudget=33),
        ):
            c, t = copy.deepcopy(config), copy.deepcopy(trace)
            mutate(c, t)
            with self.assertRaises(ValueError):
                bridge.validate(c, t)
        for targets in ((4096,)*5, (1,)*17):
            c, t = fixture(targets)
            with self.assertRaises(ValueError):
                bridge.validate(c, t)

    def test_prefix_namespaces(self):
        c, t = fixture((16,), [['9']], 'session-virtual')
        other = copy.deepcopy(t['sessions'][0])
        other['session_id'] = 'two'
        t['sessions'].append(other)
        rows = bridge.validate(c, t)
        self.assertNotEqual(rows[0]['scope'], rows[1]['scope'])
        for s in t['sessions']:
            s['hashScope'] = 'file'
        rows = bridge.validate(c, t)
        self.assertEqual(rows[0]['scope'], rows[1]['scope'])


@unittest.skipUnless(os.environ.get('AIPERF_REALIZE_REFERENCE') == '1', 'actual pinned AIPerf runtime/assets/Go binary required')
class ActualReference(unittest.TestCase):
    def execute(self, trace, directory):
        config, _ = fixture()
        root = Path(directory)
        trace_path = root/'trace.json'
        trace_path.write_bytes(bridge.encoded(trace))
        config.update(trace=str(trace_path), trace_sha256=bridge.digest(trace_path.read_bytes()),
                      assets=os.environ['REALIZE_ASSETS'], asset_manifest=os.environ['REALIZE_ASSET_MANIFEST'],
                      asset_manifest_sha256=bridge.digest(Path(os.environ['REALIZE_ASSET_MANIFEST']).read_bytes()),
                      go_tokenizer=os.environ['REALIZE_GO_TOKENIZER'], go_tokenizer_sha256=os.environ['REALIZE_GO_TOKENIZER_SHA256'],
                      python_sha256=bridge.digest(Path(sys.executable).resolve().read_bytes()))
        config_path = root/'config.json'
        config_path.write_bytes(bridge.encoded(config))
        output = root/'out'
        run = subprocess.run([sys.executable, str(SCRIPT), '--config', str(config_path), '--config-sha256', bridge.digest(config_path.read_bytes()), '--output', str(output)], capture_output=True, timeout=30)
        return run, output, config

    def temporary(self):
        root = Path(os.environ['TMPDIR'])
        if os.environ.get('REALIZE_RETAIN_TEST_ARTIFACTS') == '1':
            return contextlib.nullcontext(tempfile.mkdtemp(dir=root))
        return tempfile.TemporaryDirectory(dir=root)

    def test_count_only_success(self):
        _, trace = fixture((32,))
        with self.temporary() as tmp:
            run, output, _ = self.execute(trace, tmp)
            self.assertEqual(run.returncode, 0, run.stderr + run.stdout + (output/'status.json').read_bytes())
            cells = json.loads((output/'realization.json').read_bytes())['results']
            self.assertEqual(cells[0]['hf_count'], 32)
            self.assertEqual(cells[0]['hf_token_ids'], cells[0]['go_token_ids'])
            sessions = json.loads((output/'sessions.json').read_bytes())
            self.assertEqual(sessions[0]['turns'][0]['request']['body']['messages'][0]['content'], cells[0]['text'])

    def test_actual_singleton_and_batched_source_equivalence(self):
        _, trace = fixture((32, 32, 48), [['17', '18'], ['17', '18'], ['17', '18']])
        with self.temporary() as tmp:
            run, output, config = self.execute(trace, tmp)
            self.assertIn(run.returncode, (0, 2))
            report = json.loads((output/'realization.json').read_bytes())
            cells = report['results']
            assets = json.loads((output/'asset-identity.json').read_bytes())
            os.environ.update(HF_HUB_OFFLINE='1', TRANSFORMERS_OFFLINE='1', HF_HUB_CACHE=assets['cache'],
                              USE_TORCH='0', USE_TF='0', USE_FLAX='0', TOKENIZERS_PARALLELISM='false')
            from aiperf.common import random_generator as rng
            from aiperf.common.tokenizer import Tokenizer
            from aiperf.dataset.generator.prompt import PromptGenerator
            from aiperf.dataset.loader.hash_ids_synthesis import HashIdsPromptRequest, HashIdsPromptSynthesisMixin
            rng.reset()
            rng.init(config['seed'])
            tokenizer = Tokenizer.from_pretrained(assets['repo'], revision=assets['revision'], trust_remote_code=False, resolve_alias=False)
            pg = PromptGenerator(prompts=None, prefix_prompts=None, tokenizer=tokenizer)
            pg._hash_id_corpus_rng.set_trace_id(trace['sourceHash'])
            host = HashIdsPromptSynthesisMixin()
            host.prompt_generator, host._tokenizer_name = pg, assets['repo']
            host._trust_remote_code, host._tokenizer_revision, host._block_size = False, assets['revision'], 16
            expected = host.synthesize_prompts_from_hash_ids([HashIdsPromptRequest(str(i), list(map(int, ids)), n) for i, (n, ids) in enumerate([(32, ['17','18']), (32, ['17','18']), (48, ['17','18'])])])
            self.assertEqual([c['text'] for c in cells], [expected[str(i)] for i in range(3)])
            self.assertEqual(cells[0]['hf_token_ids'], cells[1]['hf_token_ids'])
            self.assertEqual(cells[0]['intended_token_ids'], cells[2]['intended_token_ids'][:32])
            for check in report['prefix_checks']:
                self.assertTrue(check['matches_prior'])
            for cell in cells:
                if cell['status'] == 'VERIFIED':
                    self.assertEqual(cell['hf_token_ids'], cell['go_token_ids'])
                else:
                    self.assertIn('reason', cell)
            self.assertEqual((output/'sessions.json').exists(), all(c['status']=='VERIFIED' for c in cells))

    def test_actual_session_virtual_scopes(self):
        _, trace = fixture((16,), [['17']], 'session-virtual')
        other = copy.deepcopy(trace['sessions'][0])
        other['session_id'] = 'two'
        trace['sessions'].append(other)
        with self.temporary() as tmp:
            run, output, _ = self.execute(trace, tmp)
            self.assertEqual(run.returncode, 0, run.stdout + run.stderr)
            report = json.loads((output/'realization.json').read_bytes())
            cells = report['results']
            self.assertNotEqual(cells[0]['scope'], cells[1]['scope'])
            for cell in cells:
                self.assertEqual(cell['hf_token_ids'], cell['go_token_ids'])
                self.assertEqual(cell['hf_count'], 16)
            self.assertTrue(all(not c['seen_before'] for c in report['prefix_checks']))
            self.assertEqual(len(json.loads((output/'sessions.json').read_bytes())), 2)

    def test_zero_and_conflicting_hash_size_preserve_unavailable(self):
        _, trace = fixture((0, 16, 8, 0), [[], ['17'], ['17'], ['19']])
        with self.temporary() as tmp:
            run, output, _ = self.execute(trace, tmp)
            self.assertEqual(run.returncode, 2)
            self.assertFalse((output/'sessions.json').exists())
            cells = json.loads((output/'realization.json').read_bytes())['results']
            self.assertEqual(cells[0]['target'], 0)
            self.assertEqual(cells[0]['hf_count'], 1)
            self.assertEqual(cells[0]['status'], 'UNAVAILABLE')
            self.assertEqual(cells[2]['reference_error_type'], 'ConfigurationError')
            self.assertIn('already materialized', cells[2]['reference_error_message'])
            self.assertEqual(cells[3]['reference_error_type'], 'ConfigurationError')
            self.assertIn('both be greater than 0', cells[3]['reference_error_message'])


if __name__ == '__main__':
    unittest.main()
