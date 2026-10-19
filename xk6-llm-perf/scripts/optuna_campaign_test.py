import copy
import importlib.util
import json
import os
import signal
from pathlib import Path
import tempfile
import subprocess
import sys
import time
import unittest
from unittest.mock import patch

SCRIPT = Path(__file__).with_name("optuna-campaign.py")
spec = importlib.util.spec_from_file_location("optuna_campaign", SCRIPT)
bridge = importlib.util.module_from_spec(spec)
spec.loader.exec_module(bridge)


class OptunaCampaignTest(unittest.TestCase):
    def configuration(self):
        return {"schema_version": 1, "campaign_binary": os.environ.get("CAMPAIGN_SMOKE_BINARY", "/missing-campaign"),
                "sampler": "tpe", "seed": 2, "max_trials": 12, "trial_timeout_seconds": 30, "max_duration_seconds": 120,
                "campaign": {"binary": os.environ.get("K6_SMOKE_BINARY", "/missing-k6"), "script": "/missing-script",
                             "dimensions": [{"name": "SCORE", "values": ["1", "2", "3"]}],
                             "mode": "grid", "seed": 5, "objective": {"name": "score", "stat": "avg"}, "minimize": False,
                             "constraints": [{"metric": {"name": "score", "stat": "avg"}, "direction": "le", "bound": 2}],
                             "max_candidates": 3, "max_attempts": 2, "min_runs": 2, "max_runs": 2, "retries": 0,
                             "convergence": "range", "tolerance": 0, "timeout": "10s", "max_duration": "25s"}}

    def test_actual_tpe_ask_tell_replay(self):
        config = self.configuration()
        config["campaign"]["dimensions"][0]["values"] = [str(number) for number in range(20)]
        first = bridge.create_study(config)
        proposals = []
        for number in range(15):
            trial, parameters = bridge.ask(first, config["campaign"]["dimensions"])
            receipt = {"state": "FAIL"} if number == 4 else {"state": "COMPLETE", "value": float(parameters["SCORE"]), "constraint_violation": int(int(parameters["SCORE"]) > 10)}
            proposals.append((parameters, receipt))
            bridge.tell(first, trial, receipt)
        second = bridge.create_study(config)
        for expected, receipt in proposals:
            trial, parameters = bridge.ask(second, config["campaign"]["dimensions"])
            self.assertEqual(parameters, expected)
            bridge.tell(second, trial, receipt)
        self.assertEqual(bridge.ask(first, config["campaign"]["dimensions"])[1], bridge.ask(second, config["campaign"]["dimensions"])[1])

    def test_invalid_geometry(self):
        for key, value in [("seed", True), ("max_trials", 0), ("sampler", "botorch"), ("trial_timeout_seconds", 86401)]:
            config = self.configuration()
            config[key] = value
            with self.assertRaises(ValueError):
                bridge.validate(config)
        config = self.configuration()
        config["campaign"]["dimensions"][0]["values"] = ["1", "1"]
        with self.assertRaises(ValueError):
            bridge.validate(config)

    def test_owned_group_cleanup_after_leader_exits(self):
        work = Path.home() / "tmp/xk6-llm-perf/20261010-parity-contracts/tmp"
        work.mkdir(parents=True, exist_ok=True)
        for mode in ("timeout", "success"):
            with self.subTest(mode=mode), tempfile.TemporaryDirectory(dir=work) as temporary:
                directory = Path(temporary)
                stub = directory / "stub.py"
                pid_path = directory / "child.pid"
                stub.write_text("import os, signal, sys, time\nfrom pathlib import Path\npid = os.fork()\nif pid == 0:\n signal.signal(signal.SIGTERM, signal.SIG_IGN)\n Path(sys.argv[1]).write_text(str(os.getpid()))\n time.sleep(60)\nelse:\n while not Path(sys.argv[1]).exists(): time.sleep(.01)\n if sys.argv[2] == 'success': sys.exit(0)\n signal.signal(signal.SIGTERM, lambda *_: sys.exit(0))\n time.sleep(60)\n")
                try:
                    if mode == "timeout":
                        with self.assertRaises(subprocess.TimeoutExpired):
                            bridge.run_child([sys.executable, "-S", str(stub), str(pid_path), mode], directory, 3)
                    else:
                        self.assertEqual(bridge.run_child([sys.executable, "-S", str(stub), str(pid_path), mode], directory, 5), 0)
                    pid = int(pid_path.read_text())
                    time.sleep(.1)
                    result = subprocess.run(["ps", "-o", "stat=", "-p", str(pid)], capture_output=True, text=True, timeout=3)
                    self.assertTrue(not result.stdout.strip() or result.stdout.strip().startswith("Z"), result.stdout)
                finally:
                    if pid_path.exists():
                        try:
                            os.kill(int(pid_path.read_text()), signal.SIGKILL)
                        except ProcessLookupError:
                            pass

    def test_signal_while_spawning_cleans_up_child(self):
        # Deliver SIGTERM while Popen waits for exec: the child exists but
        # Popen has not returned. The handler must not orphan the session.
        class OS:
            pid = None
            def __getattr__(self, name):
                return getattr(os, name)
            def read(self, fd, size):
                if self.pid is None:
                    self.pid = sys._getframe(1).f_locals["self"].pid
                    os.kill(os.getpid(), signal.SIGTERM)
                return os.read(fd, size)
        def interrupted(_signum, _frame):
            raise KeyboardInterrupt()
        shim = OS()
        previous = signal.signal(signal.SIGTERM, bridge.deferrable(interrupted))
        try:
            with tempfile.TemporaryDirectory() as temporary, patch.object(bridge.subprocess, "os", shim):
                with self.assertRaises(KeyboardInterrupt):
                    bridge.run_child(["/bin/sleep", "30"], Path(temporary), 20)
            self.assertIsNotNone(shim.pid)
            time.sleep(.1)
            result = subprocess.run(["ps", "-o", "stat=", "-p", str(shim.pid)], capture_output=True, text=True, timeout=3)
            self.assertTrue(not result.stdout.strip() or result.stdout.strip().startswith("Z"), result.stdout)
        finally:
            signal.signal(signal.SIGTERM, previous)
            if shim.pid:
                try:
                    os.kill(shim.pid, signal.SIGKILL)
                except ProcessLookupError:
                    pass

    def test_cleanup_errors_keep_original_error(self):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            killpg = os.killpg
            def zombie_group(pid, number):
                # Emulate macOS EPERM for a group holding only zombies.
                try:
                    killpg(pid, number)
                except ProcessLookupError:
                    pass
                raise PermissionError(1, "Operation not permitted")
            with patch.object(bridge.os, "killpg", side_effect=zombie_group), self.assertRaises(subprocess.TimeoutExpired):
                bridge.run_child(["/bin/sleep", "30"], directory, .2)

    @unittest.skipUnless(os.environ.get("K6_SMOKE_BINARY") and os.environ.get("CAMPAIGN_SMOKE_BINARY"), "set both binaries for actual campaign execution")
    def test_actual_campaign_resume_and_mutation_refusal(self):
        work = Path.home() / "tmp/xk6-llm-perf/20261010-parity-contracts/tmp"
        work.mkdir(parents=True, exist_ok=True)
        with tempfile.TemporaryDirectory(dir=work) as directory:
            root = Path(directory)
            script = root / "workload.js"
            script.write_text("import { Trend } from 'k6/metrics'; const score = new Trend('score'); export const options = { vus: 1, iterations: 1 }; export default function () { score.add(Number(__ENV.SCORE)); }\n")
            config = self.configuration()
            config["campaign"]["script"] = str(script)
            interrupted_output = root / "interrupted-study"
            interrupted_output.mkdir(mode=0o700)
            study = bridge.create_study(config)
            _, proposal = bridge.ask(study, config["campaign"]["dimensions"])
            pending = {"schema_version": 1, "identity": bridge.identities(config), "elapsed_seconds": 119,
                       "trials": [{"number": 0, "parameters": proposal, "state": "RUNNING", "reserved_duration_seconds": 36}]}
            bridge.write_json(interrupted_output / "optuna.json", pending)
            interrupted = bridge.run(config, interrupted_output)
            self.assertEqual(interrupted["stop_reason"], "duration_budget")
            self.assertGreaterEqual(interrupted["elapsed_seconds"], 155)
            self.assertEqual(len(interrupted["trials"]), 1)
            self.assertEqual(interrupted["trials"][0]["state"], "FAIL")
            self.assertFalse((interrupted_output / "trial-0000").exists())
            output = root / "study"
            first = bridge.run(config, output)
            self.assertEqual(first["stop_reason"], "space_exhausted")
            self.assertTrue(all(receipt["state"] == "COMPLETE" or receipt.get("reason") == "duplicate proposal; not rerun" for receipt in first["trials"]))
            best = first["trials"][first["best_trial"]]
            self.assertEqual(best["value"], 2)
            trial_count = len(first["trials"])
            second = bridge.run(config, output)
            self.assertEqual(len(second["trials"]), trial_count)
            self.assertEqual(second["best_trial"], first["best_trial"])
            config_changed = copy.deepcopy(config)
            config_changed["seed"] += 1
            with self.assertRaises(ValueError):
                bridge.run(config_changed, output)
            receipt = next(value for value in first["trials"] if value["state"] == "COMPLETE")
            summary = next(path for path in receipt["artifact_sha256"] if path.endswith("summary.json"))
            path = output / ("trial-%04d" % receipt["number"]) / summary
            path.write_text("{}")
            with self.assertRaises(ValueError):
                bridge.run(config, output)


if __name__ == "__main__":
    unittest.main()
