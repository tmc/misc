import importlib.util
import os
from pathlib import Path
import sys
import tempfile
import time
import unittest

spec = importlib.util.spec_from_file_location("outcomes", Path(__file__).with_name("outcome-search.py"))
bridge = importlib.util.module_from_spec(spec)
spec.loader.exec_module(bridge)

@unittest.skipUnless(os.environ.get("BOTORCH_QUALIFY") == "1", "pinned CPU BoTorch runtime required")
class BoTorchTest(unittest.TestCase):
    def config(self, objectives):
        return {"sampler": "botorch", "seed": 7, "proposal_timeout_seconds": 90,
                "objectives": [{"direction": direction} for direction in objectives]}

    def test_actual_poststartup_constraints_and_seeded_replay(self):
        bridge.dependency_versions()
        work = Path.home() / "tmp/xk6-llm-perf/20261010-parity-botorch/tmp"
        work.mkdir(parents=True, exist_ok=True)
        with tempfile.TemporaryDirectory(dir=work) as temporary:
            proposal_root = Path(temporary)
            deadline = time.monotonic() + 600
            remaining_budget = lambda: deadline - time.monotonic()
            for directions in (["maximize"], ["maximize", "minimize"]):
                config = self.config(directions)
                domains = [{"name": "X", "type": "float", "low": 0, "high": 1}]
                first = bridge.create_study(config, proposal_root, remaining_budget)
                receipts = []
                for number in range(11):
                    trial, values, parameters = bridge.search.ask(first, domains)
                    x = values["X"]
                    receipt = {"number": number, "state": "COMPLETE", "values": [-(x-.4)**2] if len(directions) == 1 else [x, x*x],
                               "constraint_violations": [x-.7], "parameters": parameters, "suggested_values": values}
                    bridge.search.tell(first, trial, receipt)
                    receipts.append(receipt)
                second = bridge.create_study(config, proposal_root, remaining_budget)
                for receipt in receipts:
                    trial, values, parameters = bridge.search.ask(second, domains)
                    self.assertEqual(values, receipt["suggested_values"])
                    self.assertEqual(parameters, receipt["parameters"])
                    bridge.search.tell(second, trial, receipt)
                state = {"trials": receipts}
                self.assertEqual(bridge.search.pareto(state, config["objectives"]), [trial.number for trial in first.best_trials])
                self.assertTrue(first.best_trials)
                self.assertTrue(all(trial.user_attrs["constraint_violations"][0] <= 0 for trial in first.best_trials))

    @unittest.skipUnless(os.environ.get("K6_SMOKE_BINARY") and os.environ.get("CAMPAIGN_SMOKE_BINARY"), "actual binaries required")
    def test_actual_campaign_resume_and_artifact_refusal(self):
        work = Path.home() / "tmp/xk6-llm-perf/20261010-parity-botorch/tmp"
        work.mkdir(parents=True, exist_ok=True)
        with tempfile.TemporaryDirectory(dir=work) as temporary:
            directory = Path(temporary)
            script = directory / "work.js"
            script.write_text("import {Trend} from 'k6/metrics'; const score=new Trend('score'), cost=new Trend('cost'); export const options={vus:1,iterations:1}; export default function(){score.add(Number(__ENV.X));cost.add(Number(__ENV.X)**2); }\n")
            config = {"schema_version": 1, "campaign_binary": os.environ["CAMPAIGN_SMOKE_BINARY"],
                      "sampler": "botorch", "seed": 7, "max_trials": 11, "trial_timeout_seconds": 30,
                      "max_duration_seconds": 600, "proposal_timeout_seconds": 90,
                      "domains": [{"name": "X", "type": "float", "low": 0, "high": 1}],
                      "objectives": [{"metric": {"name": "score", "stat": "avg"}, "direction": "maximize"},
                                     {"metric": {"name": "cost", "stat": "avg"}, "direction": "minimize"}],
                      "outcome_constraints": [{"metric": "cost", "op": "<=", "bound": .5}],
                      "campaign": {"binary": os.environ["K6_SMOKE_BINARY"], "script": str(script), "seed": 1,
                                   "constraints": [], "max_attempts": 2, "min_runs": 2, "max_runs": 2,
                                   "retries": 0, "convergence": "range", "tolerance": 0, "timeout": "10s", "max_duration": "25s"}}
            first = bridge.run(config, directory / "study")
            self.assertEqual(len(first["trials"]), 11)
            self.assertTrue(first["pareto_trials"])
            self.assertTrue(all(trial["state"] == "COMPLETE" and trial["objective_run_counts"] == [2, 2] and trial["outcome_run_counts"] == [2] for trial in first["trials"]))
            self.assertEqual(bridge.run(config, directory / "study")["trials"], first["trials"])
            checkpoint = directory / "study/optuna-search.json"
            state = bridge.core.read_json(checkpoint)
            state["trials"][0]["outcome_values"] = [123]
            bridge.core.write_json(checkpoint, state)
            with self.assertRaises(ValueError):
                bridge.run(config, directory / "study")

    def test_actual_worker_negative_controls(self):
        work = Path.home() / "tmp/xk6-llm-perf/20261010-parity-botorch/tmp"
        work.mkdir(parents=True, exist_ok=True)
        base = {"x": [[0.1], [0.8]], "objectives": [[1.], [2.]], "bounds": [[0.], [1.]],
                "constraints": None, "pending": None, "reference_point": [.9], "seed": 3}
        import copy
        for key, value in [("pending", [[2.]]), ("pending", [[.1, .2]]), ("x", [[-1.], [.8]]),
                           ("constraints", [[1.]]), ("bounds", [[1.], [1.]])]:
            with tempfile.TemporaryDirectory(dir=work) as temporary:
                directory = Path(temporary)
                invalid = copy.deepcopy(base)
                invalid[key] = value
                bridge.core.write_json(directory / "input.json", invalid)
                status = bridge.core.run_child([sys.executable, str(Path(__file__).with_name("botorch-proposal.py")),
                                              str(directory / "input.json"), str(directory / "output.json")], directory, 30)
                self.assertNotEqual(status, 0)
                self.assertFalse((directory / "output.json").exists())

if __name__ == "__main__":
    unittest.main()
