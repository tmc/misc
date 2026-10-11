import copy
import importlib.util
import importlib.metadata
import os
from pathlib import Path
import tempfile
import unittest
import time

SCRIPT = Path(__file__).with_name("optuna-search.py")
spec = importlib.util.spec_from_file_location("optuna_search", SCRIPT)
bridge = importlib.util.module_from_spec(spec)
spec.loader.exec_module(bridge)


class NumericParetoSearchTest(unittest.TestCase):
    def configuration(self):
        return {"schema_version": 1, "campaign_binary": os.environ.get("CAMPAIGN_SMOKE_BINARY", "/missing-campaign"),
                "sampler": "tpe", "seed": 2, "max_trials": 4, "trial_timeout_seconds": 30, "max_duration_seconds": 120,
                "domains": [{"name": "FLOAT", "type": "float", "low": .5, "high": 1.5, "log": True},
                            {"name": "INTEGER", "type": "int", "low": 1, "high": 3},
                            {"name": "LABEL", "type": "categorical", "values": ["a", "b"]}],
                "objectives": [{"metric": {"name": "score", "stat": "avg"}, "direction": "maximize"},
                               {"metric": {"name": "cost", "stat": "avg"}, "direction": "minimize"}],
                "campaign": {"binary": os.environ.get("K6_SMOKE_BINARY", "/missing-k6"), "script": "/missing-script",
                             "seed": 5, "constraints": [{"metric": {"name": "score", "stat": "avg"}, "direction": "le", "bound": 3}],
                             "max_attempts": 2, "min_runs": 2, "max_runs": 2, "retries": 0,
                             "convergence": "range", "tolerance": 0, "timeout": "10s", "max_duration": "25s"}}

    def test_numeric_and_pareto_actual_study_replay(self):
        config = self.configuration()
        bridge.validate(config)
        first = bridge.create_study(config)
        trials = []
        for number in range(15):
            trial, suggested, parameters = bridge.ask(first, config["domains"])
            self.assertEqual(float(parameters["FLOAT"]), suggested["FLOAT"])
            self.assertTrue(.5 <= suggested["FLOAT"] <= 1.5)
            self.assertTrue(1 <= suggested["INTEGER"] <= 3)
            self.assertIs(type(suggested["INTEGER"]), int)
            values = [suggested["FLOAT"] + suggested["INTEGER"], float(suggested["INTEGER"]) ** 2]
            receipt = {"number": number, "parameters": parameters, "suggested_values": suggested, "state": "COMPLETE", "values": values, "constraint_violations": [values[0] - 3, 0]}
            trials.append(receipt)
            bridge.tell(first, trial, receipt)
        state = {"trials": trials}
        self.assertEqual(bridge.pareto(state, config["objectives"]), [trial.number for trial in first.best_trials])
        second = bridge.create_study(config)
        for receipt in trials:
            trial, suggested, parameters = bridge.ask(second, config["domains"])
            self.assertEqual(suggested, receipt["suggested_values"])
            self.assertEqual(parameters, receipt["parameters"])
            bridge.tell(second, trial, receipt)
        self.assertEqual(bridge.ask(first, config["domains"])[1:], bridge.ask(second, config["domains"])[1:])

    def test_owned_planner_deadline(self):
        started = time.monotonic()
        planner = bridge.OwnedPlanner(self.configuration(), lambda: .001 - (time.monotonic() - started))
        try:
            with self.assertRaises(bridge.PlannerDeadline):
                planner.proposal()
        finally:
            planner.close()
        self.assertIsNotNone(planner.process.poll())
        self.assertLess(time.monotonic() - started, 4)

    def test_domain_refusal(self):
        for domain in [{"name": "X", "type": "float", "low": -1, "high": 1, "log": True},
                       {"name": "X", "type": "float", "low": 0, "high": 1, "step": .3},
                       {"name": "X", "type": "float", "low": 0, "high": 1.00000000001, "step": .1},
                       {"name": "X", "type": "int", "low": 0, "high": 3, "step": 2},
                       {"name": "X", "type": "int", "low": True, "high": 3},
                       {"name": "X", "type": "float", "low": float("nan"), "high": 3},
                       {"name": "X", "type": "float", "low": -1e308, "high": 1e308},
                       {"name": "X", "type": "int", "low": 0, "high": 1 << 53}]:
            config = self.configuration()
            config["domains"] = [domain]
            with self.subTest(domain=domain), self.assertRaises(ValueError):
                bridge.validate(config)
        config = self.configuration()
        config["domains"] = [{"name": "X", "type": "float", "low": 0, "high": .3, "step": .1}]
        bridge.validate(config)

    def test_all_objectives_must_converge(self):
        policy = {"min_runs": 2, "convergence": "range", "tolerance": 0}
        self.assertTrue(bridge.core.converged([0, 0], policy))
        self.assertFalse(bridge.core.converged([1, 2], policy))
        self.assertFalse(bridge.core.converged([1], policy))
        self.assertFalse(bridge.core.converged([-1, 1], policy))

    def test_gp_dependency_refusal_is_eager(self):
        try:
            version = importlib.metadata.version("torch")
        except importlib.metadata.PackageNotFoundError:
            version = None
        if version in bridge.TORCH_VERSIONS:
            self.skipTest("pinned Torch available; actual GP-fit qualification separate")
        config = self.configuration()
        config["sampler"] = "gp"
        with self.assertRaises((ValueError, importlib.metadata.PackageNotFoundError)):
            bridge.create_study(config)

    @unittest.skipUnless(os.environ.get("K6_SMOKE_BINARY") and os.environ.get("CAMPAIGN_SMOKE_BINARY"), "set both binaries for actual search execution")
    def test_actual_multiobjective_campaign_and_resume(self):
        work = Path.home() / "tmp/xk6-llm-perf/20261010-parity-contracts/tmp"
        work.mkdir(parents=True, exist_ok=True)
        with tempfile.TemporaryDirectory(dir=work) as temporary:
            root = Path(temporary)
            script = root / "workload.js"
            script.write_text("import { Trend } from 'k6/metrics'; const score = new Trend('score'), cost = new Trend('cost'); export const options = { vus: 1, iterations: 1 }; export default function () { score.add(Number(__ENV.FLOAT)+Number(__ENV.INTEGER)); cost.add(Number(__ENV.INTEGER)**2); }\n")
            config = self.configuration()
            config["campaign"]["script"] = str(script)
            output = root / "study"
            first = bridge.run(config, output)
            self.assertEqual(len(first["trials"]), 4)
            self.assertTrue(first["pareto_trials"])
            self.assertTrue(all(receipt["state"] == "COMPLETE" and receipt["objective_run_counts"] == [2, 2] for receipt in first["trials"]))
            exported = bridge.core.read_json(output / "pareto.json")
            self.assertEqual([trial["number"] for trial in exported["trials"]], first["pareto_trials"])
            second = bridge.run(config, output)
            self.assertEqual(second["trials"], first["trials"])
            self.assertEqual(second["pareto_trials"], first["pareto_trials"])
            missing_script = root / "missing-secondary.js"
            missing_script.write_text("import { Trend } from 'k6/metrics'; const score = new Trend('score'); export const options = { vus: 1, iterations: 1 }; export default function () { score.add(1); }\n")
            missing = copy.deepcopy(config)
            missing["campaign"]["script"] = str(missing_script)
            missing["max_trials"] = 1
            unavailable = bridge.run(missing, root / "missing-study")
            self.assertEqual(unavailable["trials"][0]["state"], "FAIL")
            self.assertEqual(unavailable["trials"][0]["reason"], "objective unavailable")
            self.assertEqual(unavailable["pareto_trials"], [])
            changed = copy.deepcopy(config)
            changed["domains"][0]["high"] = 2
            with self.assertRaises(ValueError):
                bridge.run(changed, output)
            trial = first["trials"][0]
            summary = next(path for path in trial["artifact_sha256"] if path.endswith("summary.json"))
            (output / "trial-0000" / summary).write_text("{}")
            with self.assertRaises(ValueError):
                bridge.run(config, output)


if __name__ == "__main__":
    unittest.main()
