"""Independent wire parsing and fail-closed empty-population controls."""
import importlib.util
import json
import hashlib
import pathlib
import tempfile
import unittest

SPEC = importlib.util.spec_from_file_location('live_verify',
    pathlib.Path(__file__).with_name('aiperf-live-verify.py'))
VERIFY = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(VERIFY)


def wire(reasoning='reasoning'):
    def event(at, delta=None, usage=None):
        data = dict(choices=[dict(delta=delta)] if delta else [])
        if usage is not None:
            data['usage'] = usage
        return dict(ns=at, line='data: '+json.dumps(data))
    return dict(start_ns=0, end_ns=4_000_000, events=[
        event(1_000_000, {reasoning: 'think'}),
        event(2_000_000, {'content': 'hello'}),
        event(3_000_000, {'content': ' world'}),
        event(4_000_000, usage=dict(prompt_tokens=1, completion_tokens=3, total_tokens=4)),
        dict(ns=4_000_000, line='data: [DONE]')])


class WireTests(unittest.TestCase):
    def test_reasoning_aliases(self):
        for key in ('reasoning', 'reasoning_content'):
            with self.subTest(key=key):
                row = VERIFY.parse_wire(wire(key))
                self.assertEqual((row['ttft'], row['ttfo'], row['last_content_latency'], row['itl']), (1, 2, 3, 1))
                self.assertTrue(row['reasoning'])

    def test_invalid_usage(self):
        for usage in (None, [], "bad", dict(prompt_tokens=1, completion_tokens=3, total_tokens=5),
                      dict(prompt_tokens=True, completion_tokens=3, total_tokens=4)):
            with self.subTest(usage=usage):
                row = wire()
                row['events'][-2]['line'] = 'data: '+json.dumps(dict(choices=[], usage=usage))
                with self.assertRaises(ValueError):
                    VERIFY.parse_wire(row)

    def test_terminals(self):
        mutations = [lambda row: row['events'].pop(),
            lambda row: row['events'].append(dict(ns=4_000_000, line='data: [DONE]')),
            lambda row: row['events'].insert(-1, dict(ns=4_000_000, line='data: {"error":{"message":"failed"}}')),
            lambda row: row['events'].insert(-1, dict(ns=4_000_000, line='data: {"choices":[{"delta":null}]}'))]
        for mutate in mutations:
            with self.subTest(mutation=mutate):
                row = wire()
                mutate(row)
                with self.assertRaises(ValueError):
                    VERIFY.parse_wire(row)

    def test_invalid_json(self):
        for value in ('{"usage":1,"usage":2}', '{"value":NaN}', '{"value":1e999}'):
            with self.subTest(value=value):
                with self.assertRaises(ValueError):
                    VERIFY.loads(value)

    def test_invalid_timing(self):
        row = wire()
        row['start_ns'] = 2_000_000
        with self.assertRaises(ValueError):
            VERIFY.parse_wire(row)

    def test_primary_choice_index(self):
        for index in (False, 0.0, None, '0', 1):
            with self.subTest(index=index):
                row = wire()
                data = json.loads(row['events'][0]['line'][6:])
                data['choices'][0]['index'] = index
                row['events'][0]['line'] = 'data: '+json.dumps(data)
                with self.assertRaises(ValueError):
                    VERIFY.parse_wire(row)
        for explicit in (False, True):
            row = wire()
            if explicit:
                data = json.loads(row['events'][0]['line'][6:])
                data['choices'][0]['index'] = 0
                row['events'][0]['line'] = 'data: '+json.dumps(data)
            self.assertTrue(VERIFY.parse_wire(row)['reasoning'])

    def test_delta_text_types(self):
        for key in ('content', 'reasoning', 'reasoning_content'):
            for value in ({'unsupported': True}, [], True, 1, 1.5):
                with self.subTest(key=key, value=value):
                    row = wire()
                    row['events'].insert(1, dict(ns=1_000_000,
                        line='data: '+json.dumps(dict(choices=[dict(delta={key: value})]))))
                    with self.assertRaises(ValueError):
                        VERIFY.parse_wire(row)
            for delta in ({}, {key: None}, {key: ''}, {key: 'text'}):
                with self.subTest(delta=delta):
                    row = wire()
                    row['events'].insert(1, dict(ns=1_000_000,
                        line='data: '+json.dumps(dict(choices=[dict(delta=delta)]))))
                    self.assertTrue(VERIFY.parse_wire(row)['reasoning'])

    def test_tool_containers(self):
        invalid = [{}, 'tool', True, 1, [None], [{}], [{'index': True}],
            [{'index': -1}], [{'index': 0, 'function': []}],
            [{'index': 0, 'function': {'arguments': {}}}],
            [{'index': 0, 'id': 7}]]
        valid = [None, [], [{'index': 0}],
            [{'index': 0, 'id': 'call1', 'type': 'function',
              'function': {'name': 'f', 'arguments': '{'}}],
            [{'index': 0, 'function': {'arguments': ''}}]]
        for values, rejected in ((invalid, True), (valid, False)):
            for value in values:
                with self.subTest(value=value):
                    row = wire()
                    row['events'].insert(2, dict(ns=2_000_000,
                        line='data: '+json.dumps(dict(choices=[dict(delta={'tool_calls': value})]))))
                    if rejected:
                        with self.assertRaises(ValueError):
                            VERIFY.parse_wire(row)
                    else:
                        self.assertTrue(VERIFY.parse_wire(row)['reasoning'])

    def test_all_event_timestamps(self):
        for line in ('data: {"choices":[{"delta":{"content":"bad"}}]}',
                     'data: {"choices":[]}', ': heartbeat'):
            for at in (-1, 1_000_000, 5_000_000):
                with self.subTest(line=line, ns=at):
                    row = wire()
                    row['events'].insert(2, dict(ns=at, line=line))
                    with self.assertRaises(ValueError):
                        VERIFY.parse_wire(row)
            row = wire()
            row['events'].insert(2, dict(ns=2_000_000, line=line))
            self.assertTrue(VERIFY.parse_wire(row)['reasoning'])
        for bounds in ((True, 4_000_000), (0, 4.), (5, 4)):
            row = wire()
            row['start_ns'], row['end_ns'] = bounds
            with self.assertRaises(ValueError):
                VERIFY.parse_wire(row)

    def test_empty_archive(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = pathlib.Path(tmp)
            (path/'wire.jsonl').write_text('')
            result = VERIFY.verify(path)
            self.assertEqual(result['status'], 'GAP')
            self.assertFalse(next(c for c in result['checks'] if c['name'] == 'exact arm identities')['passed'])


class PlanTests(unittest.TestCase):
    def test_hash_and_bounds(self):
        valid = dict(schema_version=1, concurrencies=[4, 8], measured_requests=8,
            warmup_requests=2, payload_sha256='a'*64, preregistration_sha256='b'*64,
            latency_limit_ms=15, itl_limit_ms=.25)
        with tempfile.TemporaryDirectory() as tmp:
            path = pathlib.Path(tmp)/'plan.json'
            def read(plan, wrong_hash=False):
                raw = json.dumps(plan).encode()
                path.write_bytes(raw)
                return VERIFY.read_plan(path, '0'*64 if wrong_hash else hashlib.sha256(raw).hexdigest())
            self.assertEqual(read(valid), valid)
            with self.assertRaises(ValueError):
                read(valid, True)
            for field, value in [('concurrencies', [4, 4]), ('concurrencies', [8, 4]),
                    ('concurrencies', [True, 8]), ('concurrencies', [9]),
                    ('measured_requests', 9), ('measured_requests', 7),
                    ('warmup_requests', 0), ('latency_limit_ms', 16),
                    ('itl_limit_ms', 1), ('preregistration_sha256', 'bad'),
                    ('schema_version', True)]:
                with self.subTest(field=field, value=value):
                    bad = dict(valid)
                    bad[field] = value
                    with self.assertRaises(ValueError):
                        read(bad)


if __name__ == '__main__':
    unittest.main()
