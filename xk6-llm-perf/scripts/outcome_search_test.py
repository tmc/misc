import copy
import importlib.util
import os
import math
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location("outcomes", Path(__file__).with_name("outcome-search.py"))
bridge = importlib.util.module_from_spec(spec)
spec.loader.exec_module(bridge)

class OutcomeTest(unittest.TestCase):
    def config(self):
        return {"schema_version": 1, "campaign_binary": os.environ.get("CAMPAIGN_SMOKE_BINARY", "/missing"),
                "sampler": "tpe", "seed": 3, "max_trials": 4, "trial_timeout_seconds": 30, "max_duration_seconds": 120,
                "proposal_timeout_seconds": 90,
                "domains": [{"name": "X", "type": "float", "low": 1, "high": 3}],
                "objectives": [{"metric": {"name": "score", "stat": "avg"}, "direction": "maximize"}],
                "outcome_constraints": [{"metric": "cost", "op": "<=", "bound": 2}],
                "campaign": {"binary": os.environ.get("K6_SMOKE_BINARY", "/missing"), "script": "/missing",
                             "seed": 1, "constraints": [], "max_attempts": 2, "min_runs": 2, "max_runs": 2,
                             "retries": 0, "convergence": "range", "tolerance": 0, "timeout": "10s", "max_duration": "25s"}}

    def test_signed_boundaries(self):
        for op, value, bound, expected in [("<=", 1, 2, -1), (">=", 1, 2, 1), ("==", 2, 2, 0), ("==", 1, 2, 1)]:
            self.assertEqual(bridge.signed(value, {"op": op, "bound": bound}), expected)
        self.assertEqual(bridge.signed(None, {"op": "<=", "bound": 2}), 1e6)
        with self.assertRaises(ValueError):
            bridge.signed(1e308, {"op": "<=", "bound": -1e308})

    def test_partial_outcome_unavailable(self):
        work = Path.home() / "tmp/xk6-llm-perf/20261010-parity-botorch/tmp"
        work.mkdir(parents=True, exist_ok=True)
        with tempfile.TemporaryDirectory(dir=work) as temporary:
            directory = Path(temporary)
            for index, metric in enumerate(({"cost": {"values": {"avg": 1}}}, {})):
                run = directory / str(index)
                run.mkdir()
                bridge.core.write_json(run / "summary.json", {"metrics": metric})
            bridge.core.write_json(directory / "campaign.json", {"candidates": [{"attempts": [{"directory": str(index), "exit_code": 0, "value": 1} for index in range(2)]}]})
            original = bridge.search.observed
            bridge.search.observed = lambda *args: {"state": "COMPLETE", "constraint_violations": [0]}
            try:
                result = bridge.observed(directory, {}, self.config())
            finally:
                bridge.search.observed = original
            self.assertEqual(result["outcome_values"], [None])
            self.assertEqual(result["outcome_run_counts"], [1])
            self.assertEqual(result["outcome_expected_runs"], 2)
            self.assertEqual(result["constraint_violations"][-1], 1e6)

    def test_configuration_refusals(self):
        config = self.config()
        bridge.validate(config)
        for change in [{"metric": "cost", "op": "<", "bound": 2}, {"metric": "cost", "op": "<=", "bound": True},
                       {"metric": "cost", "op": "<=", "bound": float("nan")}, {"expression": "cost+1", "op": "<=", "bound": 2}]:
            bad = copy.deepcopy(config)
            bad["outcome_constraints"] = [change]
            with self.assertRaises(ValueError):
                bridge.validate(bad)
        bad = copy.deepcopy(config)
        bad["proposal_timeout_seconds"] = 181
        with self.assertRaises(ValueError):
            bridge.validate(bad)

    @unittest.skipUnless(os.environ.get("K6_SMOKE_BINARY") and os.environ.get("CAMPAIGN_SMOKE_BINARY"), "actual campaign binaries required")
    def test_actual_outcome_replay_and_missing(self):
        work = Path.home() / "tmp/xk6-llm-perf/20261010-parity-botorch/tmp"
        work.mkdir(parents=True, exist_ok=True)
        with tempfile.TemporaryDirectory(dir=work) as temporary:
            directory = Path(temporary)
            script = directory / "work.js"
            script.write_text("import {Trend} from 'k6/metrics'; const score=new Trend('score'), cost=new Trend('cost'); export const options={vus:1,iterations:1}; export default function(){score.add(Number(__ENV.X));cost.add(Number(__ENV.X));}\n")
            config = self.config()
            config["campaign"]["script"] = str(script)
            numeric_functions = (bridge.search.validate, bridge.search.identity, bridge.search.observed, bridge.search.create_study)
            first = bridge.run(config, directory / "study")
            self.assertEqual(numeric_functions, (bridge.search.validate, bridge.search.identity, bridge.search.observed, bridge.search.create_study))
            self.assertFalse(hasattr(bridge, "proposal_root"))
            self.assertFalse(hasattr(bridge, "deadline"))
            self.assertEqual(len(first["trials"]), 4)
            for trial in first["trials"]:
                self.assertEqual(trial["outcome_run_counts"], [2])
                self.assertEqual(trial["constraint_violations"][-1], trial["outcome_values"][0] - 2)
            self.assertTrue(first["pareto_trials"])
            self.assertEqual(bridge.run(config, directory / "study")["trials"], first["trials"])
            missing = copy.deepcopy(config)
            missing["outcome_constraints"][0]["metric"] = "absent"
            missing["max_trials"] = 1
            unavailable = bridge.run(missing, directory / "missing")
            self.assertEqual(unavailable["trials"][0]["outcome_values"], [None])
            self.assertEqual(unavailable["trials"][0]["outcome_run_counts"], [0])
            self.assertEqual(unavailable["trials"][0]["constraint_violations"][-1], 1e6)
            self.assertEqual(unavailable["pareto_trials"], [])
            recipe_spec = importlib.util.spec_from_file_location("recipes", Path(__file__).with_name("search-recipes.py"))
            recipes = importlib.util.module_from_spec(recipe_spec)
            recipe_spec.loader.exec_module(recipes)
            common = copy.deepcopy(config)
            common.pop("domains")
            common.pop("objectives")
            common["max_trials"] = 1
            common["campaign"]["constraints"] = []
            recipe = {"recipe": "max-throughput-ttft-sla", "streaming": True,
                      "concurrency": {"name": "X", "low": 1, "high": 1},
                      "throughput_metric": {"name": "score", "stat": "avg"},
                      "latency_metric": {"name": "cost", "stat": "p(95)"}, "sla_ms": 1,
                      "search": common}
            boundary = bridge.run(recipes.expand(recipe), directory / "strict-boundary")
            self.assertEqual(boundary["pareto_trials"], [])
            self.assertGreater(boundary["trials"][0]["constraint_violations"][0], 0)
            recipe["sla_ms"] = math.nextafter(1., math.inf)
            passing = bridge.run(recipes.expand(recipe), directory / "strict-positive")
            self.assertEqual(passing["pareto_trials"], [0])
            concurrency_recipe = {"recipe": "max-concurrency-under-sla", "search_style": "optuna", "streaming": True,
                                  "concurrency": {"name": "X", "low": 1, "high": 1},
                                  "concurrency_metric": {"name": "score", "stat": "avg"},
                                  "sla_filters": [{"metric": {"name": "cost", "stat": "p(95)"}, "op": "<", "bound": 2, "streaming_only": True}],
                                  "search": common}
            passing = bridge.run(recipes.expand(concurrency_recipe), directory / "concurrency-positive")
            self.assertEqual(passing["pareto_trials"], [0])
            dishonest_script = directory / "dishonest.js"
            dishonest_script.write_text("import {Trend} from 'k6/metrics'; const score=new Trend('score'), cost=new Trend('cost'); export const options={vus:1,iterations:1}; export default function(){score.add(0);cost.add(Number(__ENV.X));}\n")
            concurrency_recipe["search"] = copy.deepcopy(common)
            concurrency_recipe["search"]["campaign"]["script"] = str(dishonest_script)
            rejected = bridge.run(recipes.expand(concurrency_recipe), directory / "concurrency-negative")
            self.assertEqual(rejected["pareto_trials"], [])
            self.assertEqual(rejected["trials"][0]["reason"], "objective differs from proposed concurrency")
            altered = copy.deepcopy(config)
            altered["outcome_constraints"][0]["bound"] = 3
            with self.assertRaises(ValueError):
                bridge.run(altered, directory / "study")
            checkpoint = directory / "study/optuna-search.json"
            state = bridge.core.read_json(checkpoint)
            state["trials"][0]["outcome_values"] = [0]
            bridge.core.write_json(checkpoint, state)
            with self.assertRaises(ValueError):
                bridge.run(config, directory / "study")

if __name__ == "__main__":
    unittest.main()
