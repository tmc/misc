import copy
from datetime import datetime, timezone
import importlib.util
from pathlib import Path
import tempfile
import os
import shutil
import signal
import subprocess
import time
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

    def policy(self):
        return {'binary': '/pinned/k6', 'script': '/pinned/workload.js', 'dimensions': [],
                'mode': 'grid', 'seed': 5, 'objective': {'name': 'score', 'stat': 'avg'},
                'minimize': False, 'constraints': [{'metric': {'name': 'score', 'stat': 'avg'}, 'direction': 'le', 'bound': 2}],
                'max_candidates': 1, 'max_attempts': 2, 'min_runs': 2, 'max_runs': 2,
                'retries': 0, 'convergence': 'range', 'tolerance': 0,
                'timeout': '5s', 'max_duration': '10s'}

    def fixture(self, root, config, values):
        config.setdefault('campaign', self.policy())
        cells=[]
        for index,cell in enumerate(grid.plan(config)):
            directory=root/str(index);directory.mkdir()
            metrics={name:{'values':{stat:value}} for (name,stat),value in values(index,cell)}
            metrics['score']={'values':{'avg':1}}
            parameters={k:str(v) for k,v in cell.items()}
            identity=grid.core.campaign_cell_id(parameters)
            attempts=[]
            for number in (1,2):
                relative=identity+'/attempt-%04d'%number
                attempt=directory/relative;attempt.mkdir(parents=True)
                grid.core.write_json(attempt/'summary.json',{'metrics':metrics})
                attempts.append({'number':number,'directory':relative,'exit_code':0,'value':1,'passed':True,'summary_sha256':grid.core.file_sha(attempt/'summary.json')})
            manifest={'schema_version':1,'identity':grid.core.sha(parameters),
                      'config':grid.core.cell_policy(config['campaign'],parameters),
                      'script_sha256':'a'*64,'binary_sha256':'b'*64,
                      'started_at':'2026-10-19T00:00:00Z','elapsed_seconds':1,
                      'candidates':[{'id':identity,'parameters':parameters,'attempts':attempts,'mean':1,'failed':False,'converged':True,'feasible':True}],
                      'best_id':identity,'stop_reason':'candidate_budget'}
            grid.core.write_json(directory/'campaign.json',manifest)
            cells.append({'parameters':cell,'directory':str(directory)})
        return cells

    def summary(self, cell):
        directory=Path(cell['directory'])
        manifest=grid.core.read_json(directory/'campaign.json')
        return directory/manifest['candidates'][0]['attempts'][0]['directory']/'summary.json'

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
            self.summary(cells[0]).write_text('{}')
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
            summary=self.summary(cells[0]);summary.unlink();os.mkfifo(summary)
            with self.assertRaisesRegex(ValueError,'regular'):grid.process(config,cells)

    def test_invalid_streaming_axes_and_metrics(self):
        for config in [{'recipe':'prefill-ttft-curve','streaming':False},{'recipe':'concurrency-ramp','streaming':False,'concurrency':[True,10,2]},{'recipe':'pareto-sweep','streaming':True,'pairs':[[1,1],[1,1]],'concurrency':[1]}]:
            with self.assertRaises(ValueError):grid.plan(config)
        for value in [None,True,float('nan'),-1]:
            with self.assertRaises(ValueError):grid.number(value)



    def test_policy_is_required_and_independent(self):
        config={'recipe':'concurrency-ramp','streaming':False,'concurrency':[1,2,2]}
        self.assertEqual(len(grid.plan(config)),2)
        with self.assertRaisesRegex(ValueError,'caller-pinned'):
            grid.process(config,[])
        changes = {
            'objective': lambda m: m['config'].update(objective={'name':'other','stat':'avg'}),
            'constraints': lambda m: m['config'].update(constraints=[]),
            'minimum runs': lambda m: m['config'].update(min_runs=1),
            'maximum runs': lambda m: m['config'].update(max_runs=3),
            'tolerance': lambda m: m['config'].update(tolerance=1),
            'mode': lambda m: m['config'].update(mode='halton'),
            'dimensions': lambda m: m['config'].update(dimensions=[]),
            'schema': lambda m: m.update(schema_version=2),
            'schema bool': lambda m: m.update(schema_version=True),
            'candidate identity': lambda m: m['candidates'][0].update(id='f'*24),
            'attempt number bool': lambda m: m['candidates'][0]['attempts'][0].update(number=True),
            'attempt directory': lambda m: m['candidates'][0]['attempts'][0].update(directory='elsewhere'),
            'attempt objective': lambda m: m['candidates'][0]['attempts'][0].update(value=2),
            'attempt passed': lambda m: m['candidates'][0]['attempts'][0].update(passed=False),
            'attempt failed': lambda m: m['candidates'][0]['attempts'][0].update(error='injected failure'),
            'candidate mean': lambda m: m['candidates'][0].update(mean=2),
            'candidate convergence': lambda m: m['candidates'][0].update(converged=False),
        }
        for label,change in changes.items():
            with self.subTest(change=label), tempfile.TemporaryDirectory(dir=self.work) as work:
                cfg=copy.deepcopy(config)
                cells=self.fixture(Path(work),cfg,lambda i,c:[(('llm_request_latency','p(99)'),10)])
                path=Path(cells[0]['directory'])/'campaign.json'
                manifest=grid.core.read_json(path)
                change(manifest);grid.core.write_json(path,manifest)
                with self.assertRaises((ValueError,FileNotFoundError)):
                    grid.process(cfg,cells)

    def test_constraint_values_are_recomputed(self):
        config={'recipe':'concurrency-ramp','streaming':False,'concurrency':[1,2,2]}
        with tempfile.TemporaryDirectory(dir=self.work) as work:
            cells=self.fixture(Path(work),config,lambda i,c:[(('llm_request_latency','p(99)'),10)])
            path=Path(cells[0]['directory'])/'campaign.json';manifest=grid.core.read_json(path)
            attempt=manifest['candidates'][0]['attempts'][0]
            summary_path=Path(cells[0]['directory'])/attempt['directory']/'summary.json'
            summary=grid.core.read_json(summary_path);summary['metrics']['score']['values']['avg']=3
            grid.core.write_json(summary_path,summary)
            attempt.update(value=3,summary_sha256=grid.core.file_sha(summary_path))
            manifest['candidates'][0].update(mean=2,converged=False)
            grid.core.write_json(path,manifest)
            with self.assertRaises(ValueError):grid.process(config,cells)

    def test_typed_policy_bounds(self):
        for key,value in [('min_runs',True),('max_runs',1),('max_attempts',10001),('max_candidates',2),('retries',-1),('seed',-1),('minimize',1),('timeout','0s'),('max_duration','169h'),('convergence','distribution'),('tolerance',float('nan')),('dimensions',[{'name':'A','values':['1']}]),('script','relative.js'),('unknown',1)]:
            with self.subTest(key=key):
                policy=self.policy();policy[key]=value
                with self.assertRaises(ValueError):grid.campaign_policy(policy)
        for value in [None,[],{'name':'score','stat':True},{'name':'score','stat':'avg','extra':1}]:
            policy=self.policy();policy['objective']=value
            with self.assertRaises(ValueError):grid.campaign_policy(policy)
        policy=self.policy();policy['constraints'][0]['bound']=True
        with self.assertRaises(ValueError):grid.campaign_policy(policy)

    @unittest.skipUnless(os.environ.get('GRID_WORKER_SMOKE') == '1', 'explicit four-process CPU smoke grant required')
    def test_actual_worker_policy_and_replay(self):
        root=Path(os.environ['GRID_SMOKE_OUTPUT']);root.mkdir(mode=0o700)
        script=root/'score.js'
        script.write_text("import {Trend} from 'k6/metrics';const score=new Trend('score');export const options={vus:1,iterations:1};export default function(){score.add(Number(__ENV.CONCURRENCY));}\n")
        policy=self.policy()
        policy.update(binary=os.environ['K6_SMOKE_BINARY'],script=str(script),constraints=[])
        config={'recipe':'concurrency-ramp','streaming':False,'concurrency':[1,2,2],'metric':'score','stat':'avg','campaign':policy}
        grid.core.write_json(root/'recipe.json',config)
        cells=[];commands=[];deadline=time.monotonic()+20
        for index,parameters in enumerate(grid.plan(config)):
            directory=root/('cell-%d'%index);directory.mkdir()
            cell=grid.core.cell_policy(policy,{k:str(v) for k,v in parameters.items()})
            grid.core.write_json(directory/'config.json',cell)
            command=[os.environ['CAMPAIGN_SMOKE_BINARY'],'-config',str(directory/'config.json'),'-output',str(directory)]
            with (directory/'worker.log').open('w') as log:
                process=subprocess.Popen(command,stdout=log,stderr=subprocess.STDOUT,start_new_session=True,env=dict(os.environ,K6_NO_USAGE_REPORT='true'))
                try:
                    code=process.wait(timeout=min(10,deadline-time.monotonic()))
                except BaseException:
                    os.killpg(process.pid,signal.SIGKILL);process.wait(timeout=1)
                    raise
            commands.append({'command':command,'exit_code':code})
            self.assertEqual(code,0)
            cells.append({'parameters':parameters,'directory':str(directory)})
        first=grid.process(config,cells);second=grid.process(config,cells)
        self.assertEqual(first,second)
        self.assertEqual(first['knee_concurrency'],2)
        self.assertEqual(first['campaign_policy_sha256'],grid.core.sha(policy))
        grid.core.write_json(root/'result.json',first)
        grid.core.write_json(root/'cells.json',cells)
        tampered=root/'tampered';shutil.copytree(Path(cells[0]['directory']),tampered)
        manifest=grid.core.read_json(tampered/'campaign.json')
        manifest['config']['constraints']=[{'metric':{'name':'score','stat':'avg'},'direction':'le','bound':0}]
        grid.core.write_json(tampered/'campaign.json',manifest)
        changed=copy.deepcopy(cells);changed[0]['directory']=str(tampered)
        with self.assertRaisesRegex(ValueError,'campaign policy differs'):
            grid.process(config,changed)
        grid.core.write_json(root/'receipt.json',{'commands':commands,'k6_sha256':grid.core.file_sha(policy['binary']),'campaign_binary_sha256':grid.core.file_sha(os.environ['CAMPAIGN_SMOKE_BINARY']),'script_sha256':grid.core.file_sha(script),'policy_sha256':grid.core.sha(policy),'worker_processes':4,'replay':'equal','tamper':'rejected','elapsed_seconds':20-(deadline-time.monotonic())})

if __name__=='__main__':unittest.main()
