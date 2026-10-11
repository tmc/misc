"""Regression controls for the CPU serving qualification fixture."""
import importlib.util
import json
import pathlib
import unittest

path = pathlib.Path(__file__).with_name('serving-qualification-control.py')
spec = importlib.util.spec_from_file_location('serving_control', path)
control = importlib.util.module_from_spec(spec)
spec.loader.exec_module(control)


class FixtureRequestTest(unittest.TestCase):
    def test_policy_is_explicit_and_independent(self):
        policy = control.paired_policy()
        self.assertEqual(set(policy['slo']), {'max_ttft_ms', 'max_ttfo_ms', 'max_chunk_latency_ms', 'max_tpot_ms', 'max_e2e_ms'})
        self.assertTrue(all(type(v) is int and v == 0 for v in policy['slo'].values()))
        self.assertTrue(all(policy[k] for k in ('measurement_policy', 'window_policy', 'warmup_policy')))
        policy['slo']['max_ttft_ms'] = 1
        self.assertEqual(control.paired_policy()['slo']['max_ttft_ms'], 0)

    def request(self):
        # The measured Go CompletionRequest omits stream=false. The lifecycle
        # warmup uses explicitly encoded false. Both perform identical work.
        return json.loads('{"model":"capacity-fixture","messages":[{"role":"user","content":"x"}],"max_tokens":1}')

    def test_omitted_stream_matches_explicit_false(self):
        request = self.request()
        self.assertTrue(control.valid_fixture_request(request))
        request['stream'] = False
        self.assertTrue(control.valid_fixture_request(request))

    def test_other_stream_values_rejected(self):
        for value in (True, None, 0, 1, '', 'false', [], {}):
            with self.subTest(value=value):
                request = self.request()
                request['stream'] = value
                self.assertFalse(control.valid_fixture_request(request))

    def test_different_work_rejected(self):
        for key, value in [('model', 'wrong'), ('messages', []),
                           ('max_tokens', 0), ('max_tokens', 2),
                           ('max_tokens', True), ('max_tokens', 1.0)]:
            with self.subTest(key=key, value=value):
                request = self.request()
                request[key] = value
                self.assertFalse(control.valid_fixture_request(request))
        self.assertFalse(control.valid_fixture_request(None))
        self.assertFalse(control.valid_fixture_request([]))


if __name__ == '__main__':
    unittest.main()
