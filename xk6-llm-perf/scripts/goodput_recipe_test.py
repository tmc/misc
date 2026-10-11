import copy
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import importlib.util
import math
import os
from pathlib import Path
import tempfile
import threading
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('recipes', Path(__file__).with_name('search-recipes.py'))
recipes = importlib.util.module_from_spec(spec)
spec.loader.exec_module(recipes)

class GoodputWitnessTest(unittest.TestCase):
    def test_content_span_controls(self):
        bridge = recipes.bridge
        policy = {"max_ttft_ms": 1, "max_tpot_ms": 2, "max_e2e_ms": 3}
        metrics = {name: {"values": {"count": value}} for name, value in
                   (("llm_attempts", 2), ("llm_successes", 1), ("llm_failures", 1), ("llm_good_requests", 1), ("llm_content_span_windows", 1))}
        metrics["llm_slo_compliance"] = {"values": {"value": .5}}
        for name, value in (("duration_ms", 20), ("good_requests", 1), ("attempts", 2), ("goodput", 50)):
            metrics["llm_content_span_" + name] = {"values": {"avg": value}}
        for name, value in (("ttft", 1), ("tpot", 2), ("e2e", 3)):
            metrics["llm_slo_" + name + "_ms"] = {"values": {"min": value, "max": value}}
        cases = [("valid", None, None, "COMPLETE"), ("multiple", "llm_content_span_windows", 2, "FAIL"),
                 ("zero-duration", "llm_content_span_duration_ms", 0, "FAIL"),
                 ("nan-duration", "llm_content_span_duration_ms", float("nan"), "FAIL"),
                 ("wrong-good", "llm_content_span_good_requests", 2, "FAIL"),
                 ("wrong-attempts", "llm_content_span_attempts", 3, "FAIL"),
                 ("missing", "llm_content_span_duration_ms", None, "FAIL"),
                 ("wrong-rate", "llm_content_span_goodput", 51, "FAIL"),
                 ("valid-zero-good", None, None, "COMPLETE")]
        work = Path.home() / "tmp/xk6-llm-perf/20261010-parity-botorch/tmp"
        work.mkdir(parents=True, exist_ok=True)
        with tempfile.TemporaryDirectory(dir=work) as temporary:
            directory = Path(temporary)
            (directory / "run").mkdir()
            bridge.core.write_json(directory / "campaign.json", {"candidates": [{"attempts": [{"directory": "run", "exit_code": 0, "value": 1}]}]})
            original = bridge.search.observed
            bridge.search.observed = lambda *args: {"state": "COMPLETE", "constraint_violations": [], "artifact_sha256": "fixture"}
            try:
                for label, name, value, state in cases:
                    with self.subTest(label=label):
                        changed = copy.deepcopy(metrics)
                        if label == "valid-zero-good":
                            changed["llm_good_requests"]["values"]["count"] = 0
                            changed["llm_slo_compliance"]["values"]["value"] = 0
                            changed["llm_content_span_good_requests"]["values"]["avg"] = 0
                            changed["llm_content_span_goodput"]["values"]["avg"] = 0
                        if name and value is None:
                            del changed[name]
                        elif name:
                            changed[name]["values"]["count" if name.endswith("windows") else "avg"] = value
                        config = {"goodput_accounting": "all_attempts", "applied_slo": policy, "outcome_constraints": []}
                        if label == "nan-duration":
                            # JSON rejects NaN before observation; also guard nonfinite
                            # values supplied by an in-memory summary reader.
                            with self.assertRaises(ValueError):
                                bridge.core.write_json(directory / "run/summary.json", {"metrics": changed})
                            reader = bridge.core.read_json
                            with patch.object(bridge.core, "read_json", side_effect=lambda path: {"metrics": changed} if Path(path).name == "summary.json" else reader(path)):
                                result = bridge.observed(directory, {}, config)
                        else:
                            bridge.core.write_json(directory / "run/summary.json", {"metrics": changed})
                            result = bridge.observed(directory, {}, config)
                        self.assertEqual(result["state"], state)
            finally:
                bridge.search.observed = original


@unittest.skipUnless(os.environ.get('K6_GOODPUT_BINARY') and os.environ.get('CAMPAIGN_SMOKE_BINARY'), 'new applied-policy binary required')
class GoodputRecipeTest(unittest.TestCase):
    def test_actual_policy_attempts_boundary_resume(self):
        class Handler(BaseHTTPRequestHandler):
            def do_POST(self):
                self.rfile.read(int(self.headers.get('Content-Length', '0')))
                if self.path == '/error':
                    self.send_response(500)
                    self.end_headers()
                    self.wfile.write(b'error')
                    return
                self.send_response(200)
                self.send_header('Content-Type', 'text/event-stream')
                self.end_headers()
                self.wfile.write(b'data: {"choices":[{"index":0,"delta":{"content":"a"}}]}\n\ndata: {"choices":[],"usage":{"prompt_tokens":1,"completion_tokens":3,"total_tokens":4}}\n\ndata: [DONE]\n\n')
                self.wfile.flush()
            def log_message(self, *args):
                pass
        server = ThreadingHTTPServer(('127.0.0.1', 0), Handler)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        work = Path.home()/'tmp/xk6-llm-perf/20261010-parity-botorch/tmp'
        work.mkdir(parents=True, exist_ok=True)
        try:
            with tempfile.TemporaryDirectory(dir=work) as temporary:
                root = Path(temporary)
                script = root/'goodput.js'
                script.write_text("import llm from 'k6/x/llm-perf'; import {sleep} from 'k6'; export const options={vus:Number(__ENV.VUS||1),iterations:1}; export default function(){ llm.beginMeasurement({runId:'goodput-recipe',maxRecords:8}); const c=new llm.Client({baseURL:'http://127.0.0.1:"+str(server.server_port)+"',model:'fixture',strictMeasurement:true,maxTTFT:Number(__ENV.SLO_TTFT_MS),maxTPOT:Number(__ENV.SLO_TPOT_MS),maxE2E:Number(__ENV.SLO_E2E_MS)}); for(const path of ['/good','/error']) c.request({path,protocol:'openai-chat',body:{messages:[{role:'user',content:'x'}],stream:true}}); llm.endMeasurement(); sleep(1); }\n")
                config = {'recipe':'max-goodput-under-slo','streaming':True,'concurrency':{'name':'VUS','low':1,'high':1},
                          'slos':{'max_ttft_ms':5000.5,'max_tpot_ms':.25,'max_e2e_ms':5000.25},'attainment_fraction':.5,
                          'goodput_metric':{'name':'llm_content_span_goodput','stat':'avg'},'attainment_metric':{'name':'llm_slo_compliance','stat':'value'},
                          'search':{'schema_version':1,'campaign_binary':os.environ['CAMPAIGN_SMOKE_BINARY'],'sampler':'tpe','seed':7,
                                    'max_trials':1,'trial_timeout_seconds':30,'max_duration_seconds':90,'proposal_timeout_seconds':90,'outcome_constraints':[],
                                    'campaign':{'binary':os.environ['K6_GOODPUT_BINARY'],'script':str(script),'seed':1,'max_attempts':2,'min_runs':2,'max_runs':2,
                                                'retries':0,'convergence':'range','tolerance':1,'timeout':'10s','max_duration':'25s'}}}
                bridge = recipes.bridge
                compiled = recipes.expand(config)
                first = bridge.run(compiled, root/'passing')
                self.assertEqual(first['pareto_trials'], [0], first)
                trial = first['trials'][0]
                self.assertEqual(trial['applied_slo'], config['slos'])
                self.assertEqual(trial['applied_slo_runs'], 2)
                for accounting in trial['goodput_accounting']:
                    self.assertEqual(accounting['fraction'], .5)
                    self.assertEqual(accounting['counts'], {'llm_attempts':2,'llm_successes':1,'llm_failures':1,'llm_good_requests':1})
                    span = accounting['content_span']
                    self.assertEqual(span['llm_content_span_windows'], 1)
                    self.assertEqual(span['llm_content_span_attempts'], 2)
                    self.assertEqual(span['llm_content_span_good_requests'], 1)
                    self.assertAlmostEqual(span['llm_content_span_goodput'], 1000/span['llm_content_span_duration_ms'])
                self.assertEqual(bridge.run(compiled, root/'passing')['trials'], first['trials'])
                failing = copy.deepcopy(config)
                failing['attainment_fraction'] = math.nextafter(.5, math.inf)
                self.assertEqual(bridge.run(recipes.expand(failing), root/'strict-negative')['pareto_trials'], [])
                missing = copy.deepcopy(config)
                missing_script = root/'missing.js'
                missing_script.write_text("import {Counter,Rate} from 'k6/metrics'; const good=new Counter('llm_good_requests'), fraction=new Rate('llm_slo_compliance'); export const options={vus:1,iterations:1}; export default function(){good.add(1);fraction.add(true);}\n")
                missing['search']['campaign']['script'] = str(missing_script)
                rejected = bridge.run(recipes.expand(missing), root/'missing-policy')
                self.assertEqual(rejected['pareto_trials'], [])
                self.assertEqual(rejected['trials'][0]['state'], 'FAIL')
        finally:
            server.shutdown()
            server.server_close()
            thread.join(5)

if __name__ == '__main__':
    unittest.main()
