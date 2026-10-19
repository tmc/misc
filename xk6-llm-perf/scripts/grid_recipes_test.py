import copy
from datetime import datetime, timezone
import importlib.util
from pathlib import Path
import tempfile
import os
import unittest

spec=importlib.util.spec_from_file_location('grid',Path(__file__).with_name('grid-recipes.py'))
grid=importlib.util.module_from_spec(spec);spec.loader.exec_module(grid)

class GridRecipesTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        date = datetime.now(timezone.utc).strftime("%Y%m%d")
        fallback = Path.home()/"tmp/xk6-llm-perf"/(date+"-tests")/"tmp"
        cls.work = Path(os.environ.get("TMPDIR") or fallback)
        cls.work.mkdir(parents=True, exist_ok=True)

    def fixture(self, root, config, values):
        cells=[]
        for index,cell in enumerate(grid.plan(config)):
            directory=root/str(index);directory.mkdir(); attempt=directory/'run';attempt.mkdir()
            metrics={name:{'values':{stat:value}} for (name,stat),value in values(index,cell)}
            grid.core.write_json(attempt/'summary.json',{'metrics':metrics})
            parameters={k:str(v) for k,v in cell.items()}
            manifest={'config':{'constraints':[]},'candidates':[{'parameters':parameters,'attempts':[{'directory':'run','exit_code':0,'value':1,'summary_sha256':grid.core.file_sha(attempt/'summary.json')}],'mean':1,'failed':False,'converged':True,'feasible':True}]}
            grid.core.write_json(directory/'campaign.json',manifest)
            cells.append({'parameters':cell,'directory':str(directory)})
        return cells

    def test_plans_preserve_pairs(self):
        config={'recipe':'pareto-sweep','streaming':True,'pairs':[[128,64],[512,256]],'concurrency':[1,4]}
        self.assertEqual(len(grid.plan(config)),4)
        self.assertNotIn({'ISL':128,'OSL':256,'CONCURRENCY':1},grid.plan(config))
        self.assertEqual(grid.logspace(1,1000,8),[1,3,7,19,52,139,373,1000])

    def test_retained_knee_and_hash_rejection(self):
        config={'recipe':'concurrency-ramp','streaming':False,'concurrency':[1,4,3]}
        with tempfile.TemporaryDirectory(dir=self.work) as work:
            cells=self.fixture(Path(work),config,lambda i,c:[(('llm_request_latency','p(99)'),[10,12,13][i])])
            self.assertEqual(grid.process(config,cells)['knee_concurrency'],4)
            (Path(cells[0]['directory'])/'run/summary.json').write_text('{}')
            with self.assertRaisesRegex(ValueError,'hash mismatch'):grid.process(config,cells)

    def test_curve_and_constant_below_floor(self):
        config={'recipe':'prefill-ttft-curve','streaming':True,'isl':[1,1000,8]}
        for constant in (False,True):
            with tempfile.TemporaryDirectory(dir=self.work) as work:
                cells=self.fixture(Path(work),config,lambda i,c:[(('llm_ttft','avg'),5 if constant else (c['ISL']-500)**2+2)])
                result=grid.process(config,cells)
                self.assertEqual(result['below_floor'],constant)
                if not constant:self.assertEqual(result['fit_form'],'quadratic')

    def test_surface_missing_cell_is_null(self):
        config={'recipe':'decode-itl-curve','streaming':True,'concurrency':[1,2,2],'osl':[64,128,2]}
        with tempfile.TemporaryDirectory(dir=self.work) as work:
            cells=self.fixture(Path(work),config,lambda i,c:[(('llm_inter_token_latency','avg'),i)])
            result=grid.process(config,cells[:-1]);self.assertIsNone(result['surface']['itl_grid'][1][1])
            with self.assertRaisesRegex(ValueError,'duplicate'):grid.process(config,cells+[cells[0]])

    def test_global_pareto_ties_and_dominance(self):
        config={'recipe':'pareto-sweep','streaming':True,'pairs':[[128,64],[512,256]],'concurrency':[1,4]}
        with tempfile.TemporaryDirectory(dir=self.work) as work:
            cells=self.fixture(Path(work),config,lambda i,c:[(('llm_ttft','p(95)'),[10,20,10,30][i]),(('llm_completion_tokens','rate'),[100,90,100,120][i])])
            self.assertEqual(len(grid.process(config,cells)['pareto_front']),3)

    def test_null_boolean_and_fifo_summary_reject(self):
        config={'recipe':'concurrency-ramp','streaming':False,'concurrency':[1,2,2]}
        for value in (None,True,-1):
            with tempfile.TemporaryDirectory(dir=self.work) as work:
                cells=self.fixture(Path(work),config,lambda i,c:[(('llm_request_latency','p(99)'),value)])
                with self.assertRaises(ValueError):grid.process(config,cells)
        with tempfile.TemporaryDirectory(dir=self.work) as work:
            cells=self.fixture(Path(work),config,lambda i,c:[(('llm_request_latency','p(99)'),10)])
            summary=Path(cells[0]['directory'])/'run/summary.json';summary.unlink();os.mkfifo(summary)
            with self.assertRaisesRegex(ValueError,'regular'):grid.process(config,cells)

    def test_invalid_streaming_axes_and_metrics(self):
        for config in [{'recipe':'prefill-ttft-curve','streaming':False},{'recipe':'concurrency-ramp','streaming':False,'concurrency':[True,10,2]},{'recipe':'pareto-sweep','streaming':True,'pairs':[[1,1],[1,1]],'concurrency':[1]}]:
            with self.assertRaises(ValueError):grid.plan(config)
        for value in [None,True,float('nan'),-1]:
            with self.assertRaises(ValueError):grid.number(value)

if __name__=='__main__':unittest.main()
