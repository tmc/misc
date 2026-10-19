"""Offline controls; invoke with the pinned AIPerf Python environment."""
import importlib.util
from pathlib import Path
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('adapter', Path(__file__).with_name('aiperf-template.py'))
adapter = importlib.util.module_from_spec(spec)
spec.loader.exec_module(adapter)


def request(template='{"text":{{ text|tojson }},"n":{{ turn.max_tokens|tojson }},"count":{{ turns|length }}}'):
    return {'schema_version': 1, 'responses': [], 'request_info': {
        'credit_num': 0, 'credit_phase': 'profiling', 'conversation_id': 'typed',
        'turn_index': 0, 'x_request_id': 'request', 'x_correlation_id': 'correlation',
        'model_endpoint': {'models': {'models': [{'name': 'local'}], 'model_selection_strategy': 'round_robin'},
                           'endpoint': {'type': 'template', 'streaming': False, 'template': {'body': template}}},
        'turns': [{'texts': [{'contents': ['first']}]}, {'texts': [{'contents': ['last']}], 'max_tokens': 1}]}}


class TypedTemplateTest(unittest.TestCase):
    def test_typed_last_turn_zero_and_absent(self):
        value = request()
        self.assertEqual(adapter.evaluate(value)['payload'], {'text': 'last', 'n': 1, 'count': 2})
        value['request_info']['turns'][-1]['max_tokens'] = 0
        with self.assertRaises(Exception):
            adapter.evaluate(value)
        del value['request_info']['turns'][-1]['max_tokens']
        self.assertIsNone(adapter.evaluate(value)['payload']['n'])

    def test_precedence_and_named_content(self):
        value = request('{"query":{{ query|tojson }},"n":1}')
        value['request_info']['turns'][-1]['texts'] = [{'name': 'query', 'contents': ['named']}]
        value['request_info']['model_endpoint']['endpoint']['extra'] = [['n', 2]]
        value['request_info']['turns'][-1]['extra_body'] = {'n': 0}
        self.assertEqual(adapter.evaluate(value)['payload'], {'query': 'named', 'n': 0})

    def test_captured_chunks_and_false_selection(self):
        value = request('{}')
        value['request_info']['model_endpoint']['endpoint']['template']['response_field'] = 'result'
        value['responses'] = [{'perf_ns': 0, 'text': '{"result":"yes"}'},
                              {'perf_ns': 1, 'text': '{"result":0}'},
                              {'perf_ns': 2, 'text': '{"other":"missing"}'}]
        parsed = adapter.evaluate(value)['parsed_responses']
        self.assertEqual(parsed[0]['perf_ns'], 0)
        self.assertEqual(parsed[1:], [None, None])

    def test_empty_turns_invalid_json_and_unknown_model_field(self):
        for change in ('empty', 'json', 'unknown'):
            value = request()
            if change == 'empty':
                value['request_info']['turns'] = []
            elif change == 'json':
                value['request_info']['model_endpoint']['endpoint']['template']['body'] = 'broken'
            else:
                value['request_info']['turns'][-1]['unknown_field'] = True
            with self.subTest(change=change), self.assertRaises(Exception):
                adapter.evaluate(value)

    def test_unused_nested_overflow_and_finite_numbers(self):
        for number in ('1e999', '-1e999'):
            value = request()
            value['request_info']['turns'][0]['extra_body'] = {'ignored': 'NUMBER'}
            raw = adapter.encode(value).replace(b'\"NUMBER\"', number.encode())
            with self.subTest(number=number), self.assertRaisesRegex(ValueError, 'nonfinite'):
                adapter.decode(raw)
        for number in (-1.5, 0, 0.0):
            value = request()
            value['request_info']['turns'][0]['extra_body'] = {'ignored': number}
            decoded = adapter.decode(adapter.encode(value))
            self.assertEqual(decoded['request_info']['turns'][0]['extra_body']['ignored'], number)
            self.assertEqual(adapter.evaluate(decoded)['payload']['text'], 'last')

    def test_source_mismatch_refused(self):
        with patch.object(adapter, 'SOURCE_HASH', '0' * 64):
            with self.assertRaisesRegex(ValueError, 'source differs'):
                adapter.evaluate(request())

    def test_decoder_bounds_and_duplicates(self):
        for raw in (b'{"a":0,"a":1}', b'NaN', b'[' * 66 + b'0' + b']' * 66, b' ' * (adapter.MAX_BYTES + 1)):
            with self.assertRaises(ValueError):
                adapter.decode(raw)


if __name__ == '__main__':
    unittest.main()
