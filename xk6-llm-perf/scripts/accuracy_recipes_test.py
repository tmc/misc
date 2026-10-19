import copy
import importlib.util
import os
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location('recipes', Path(__file__).with_name('accuracy-recipes.py'))
bridge = importlib.util.module_from_spec(spec)
spec.loader.exec_module(bridge)


def fixture(benchmark):
    simple = {'problem': 'What is 1+1?', 'answer': '2', 'solution': 'Add one. \\boxed{2}'}
    mmlu = {'question': 'What is 1+1?', 'choices': ['2', '3', '4', '5'], 'answer': 0}
    mpro = {'question': 'What is 1+1?', 'options': ['2', '3', 'N/A'], 'answer': 'A', 'category': 'math', 'cot_content': 'Add one. The answer is (A).'}
    data = {
        'gsm8k': ('openai/gsm8k', 'main', {'test': [{'question': 'What is 1+1?', 'answer': 'Add one. #### 2'}]}),
        'aime': ('Maxwell-Jia/AIME_2024', None, {'train': [{'Problem': 'What is 1+1?', 'Answer': '2', 'Solution': 'Add one.'}]}),
        'aime24': ('HuggingFaceH4/aime_2024', None, {'train': [simple]}),
        'aime25': ('yentinglin/aime_2025', None, {'train': [simple]}),
        'math_500': ('HuggingFaceH4/MATH-500', None, {'test': [simple]}),
        'gpqa_diamond': ('Idavidrein/gpqa', 'gpqa_diamond', {'train': [{'Question': 'What is 1+1?', 'Correct Answer': '2', 'Incorrect Answer 1': '3', 'Incorrect Answer 2': '4', 'Incorrect Answer 3': '5'}]}),
        'mmlu': ('lighteval/mmlu', 'abstract_algebra', {'test': [mmlu], 'dev': [mmlu]}),
        'mmlu_pro': ('TIGER-Lab/MMLU-Pro', None, {'test': [mpro], 'validation': [mpro]}),
        'lcb_codegeneration': ('livecodebench/code_generation_lite', 'v4_v5', {'test': [{'question_content': 'Read a number and print it.', 'starter_code': '', 'public_test_cases': '[]', 'private_test_cases': '[]', 'metadata': '{}'}]}),
    }
    name, config, splits = data[benchmark]
    cfg = {'schema_version': 1, 'benchmark': benchmark, 'model': 'fixture', 'model_revision': 'fixture-v1',
           'snapshots': [{'dataset': name, 'config': config, 'revision': 'fixture-v1', 'splits': splits}]}
    if benchmark == 'mmlu':
        cfg['tasks'] = ['abstract_algebra']
    if benchmark == 'mmlu_pro':
        cfg['tasks'] = ['math']
    return cfg


class AdmissionTest(unittest.TestCase):
    def test_missing_and_invalid_snapshots(self):
        config = fixture('gsm8k')
        for key, value in [('n_shots', True), ('enable_cot', 1), ('benchmark', 'invented'), ('model_revision', '')]:
            bad = copy.deepcopy(config)
            bad[key] = value
            with self.assertRaises(ValueError):
                bridge.validate(bad)
        snapshots = bridge.validate(config)
        with self.assertRaises(ValueError):
            snapshots.load('missing', split='test')
        with self.assertRaises(ValueError):
            snapshots.load('openai/gsm8k', 'main', split='train')

    def test_go_reference_encoding(self):
        self.assertEqual(bridge.go_json([{'id': '0', 'reference': '<&\u2028'}]), b'[{"id":"0","reference":"\\u003c\\u0026\\u2028"}]')


@unittest.skipUnless(os.environ.get('AIPERF_RECIPE_REFERENCE') == '1', 'actual pinned AIPerf runtime required')
class ReferenceTest(unittest.TestCase):
    def test_actual_recipes(self):
        # These are local row fixtures, not downloaded task data or scores.
        for benchmark in ('gsm8k', 'aime', 'aime24', 'aime25', 'math_500', 'gpqa_diamond', 'mmlu', 'mmlu_pro', 'lcb_codegeneration'):
            with self.subTest(benchmark=benchmark):
                output = bridge.render(fixture(benchmark))
                self.assertEqual(output['recipe.json']['population'], 1)
                self.assertEqual(output['artifact.json']['cases'][0]['prediction'], None)
                self.assertGreater(output['requests.json']['0']['max_tokens'], 0)
                self.assertEqual(output, bridge.render(fixture(benchmark)))
                request = output['requests.json']['0']
                if benchmark == 'gsm8k':
                    self.assertEqual(request, {'messages': [{'role': 'user', 'content': 'Question: What is 1+1?\nAnswer:'}], 'max_tokens': 256})
                elif benchmark == 'mmlu':
                    self.assertEqual([m['role'] for m in request['messages']], ['user', 'assistant', 'user'])
                    self.assertEqual(request['stop'], ['\n'])
                    self.assertEqual(request['max_tokens'], 5)
                elif benchmark == 'aime':
                    self.assertEqual(request['messages'][0], {'role': 'system', 'content': 'Please reason step by step, and put your final answer within \\boxed{}.'})
                    self.assertIn("Let's think step-by-step.", request['messages'][-1]['content'])
                elif benchmark == 'gpqa_diamond':
                    from aiperf.accuracy.benchmarks.gpqa_diamond import GPQADiamondBenchmark
                    row = fixture(benchmark)['snapshots'][0]['splits']['train'][0]
                    choices, gold = GPQADiamondBenchmark._build_choices(row)
                    self.assertEqual(output['artifact.json']['cases'][0]['reference'], gold)
                    for letter, choice in zip('ABCD', choices):
                        self.assertIn(f'{letter}) {choice}', request['messages'][0]['content'])

    def test_explicit_overrides(self):
        config = fixture('mmlu')
        config.update(n_shots=0, enable_cot=True, system_prompt='authored override')
        request = bridge.render(config)['requests.json']['0']
        self.assertEqual([m['role'] for m in request['messages']], ['system', 'user'])
        self.assertEqual(request['messages'][0]['content'], 'authored override')
        self.assertNotIn('stop', request)
        self.assertGreater(request['max_tokens'], 5)


if __name__ == '__main__':
    unittest.main()
