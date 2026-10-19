#!/usr/bin/env python3
"""Lifecycle guard tests; mocks are not cluster qualification."""
import importlib.util
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import Mock, patch

spec = importlib.util.spec_from_file_location("qualify", Path(__file__).with_name("qualify.py"))
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


def result(value):
    return subprocess.CompletedProcess([], 0, json.dumps(value), "")


class QualificationTest(unittest.TestCase):
    def test_timeout_retains_partial_command_receipt(self):
        with tempfile.TemporaryDirectory() as directory:
            q = module.Qualification("test", Path(directory), 10, None)
            receipt = {"command": ["kubectl"], "returncode": None, "timed_out": True,
                       "stdout": "partial", "stderr": "stalled", "capture_complete": False}
            with patch.object(module, "bounded_command", return_value=receipt):
                with self.assertRaises(subprocess.TimeoutExpired):
                    q.kubectl("get", "pods")
            receipt = json.loads((Path(directory) / "1.json").read_text())
            self.assertTrue(receipt["timed_out"])
            self.assertIsNone(receipt["returncode"])
            self.assertEqual(receipt["stdout"], "partial")
            self.assertEqual(receipt["stderr"], "stalled")

    def test_streaming_capture_valid_output(self):
        receipt = module.bounded_command([sys.executable, "-c", 'import os; os.write(1, b"{}" ); os.write(2, b"warning")'], 5)
        self.assertEqual(receipt["returncode"], 0)
        self.assertTrue(receipt["capture_complete"])
        self.assertEqual(receipt["stdout"], "{}")
        self.assertEqual(receipt["stderr"], "warning")
        self.assertEqual(module.bounded_json(receipt["stdout"]), {})

    def test_streaming_capture_kills_oversized_stdout_and_stderr(self):
        for pipe in (1, 2):
            with self.subTest(pipe=pipe):
                receipt = module.bounded_command([sys.executable, "-c",
                    "import os; os.write(" + str(pipe) + ", b'x' * 2097152)"], 5)
                name = "stdout" if pipe == 1 else "stderr"
                self.assertEqual(len(receipt[name]), module.OUTPUT_LIMIT)
                self.assertTrue(receipt[name + "_truncated"])
                self.assertFalse(receipt["capture_complete"])
                self.assertFalse(receipt["timed_out"])
                with tempfile.TemporaryDirectory() as directory:
                    q = module.Qualification("test", Path(directory), 10, None)
                    with patch.object(module, "bounded_command", return_value=receipt):
                        with self.assertRaisesRegex(RuntimeError, "output truncated"):
                            q.kubectl("get", "events", required=False)
                    saved = json.loads((Path(directory) / "1.json").read_text())
                    self.assertTrue(saved[name + "_truncated"])

    def test_oversized_apply_output_still_cleans_owned_namespace(self):
        with tempfile.TemporaryDirectory() as directory:
            q = module.Qualification("test", Path(directory), 10, None)
            q.validate = Mock(return_value="test")
            q.recover_ownership = Mock(side_effect=lambda: setattr(q, "uid", "owned"))
            q.capture = Mock()
            q.cleanup = Mock()
            valid = {"command": [], "returncode": 0, "timed_out": False,
                     "stdout": "{}", "stderr": "", "capture_complete": True}
            oversized = dict(valid, capture_complete=False, stdout_truncated=True)
            with patch.object(module, "bounded_command", side_effect=[valid] * 4 + [oversized]):
                with self.assertRaisesRegex(RuntimeError, "output truncated"):
                    q.run(Path("manifest"), Path("assets"))
            q.capture.assert_called_once()
            q.cleanup.assert_called_once()
            self.assertTrue(json.loads((Path(directory) / "5.json").read_text())["stdout_truncated"])
            self.assertTrue(json.loads((Path(directory) / "cleanup.json").read_text())["complete"])

    def test_streaming_timeout_keeps_partial_output(self):
        receipt = module.bounded_command([sys.executable, "-c",
            "import os,time; os.write(1,b'partial'); os.write(2,b'warning'); time.sleep(30)"], 0.5)
        self.assertTrue(receipt["timed_out"])
        self.assertFalse(receipt["capture_complete"])
        self.assertEqual(receipt["stdout"], "partial")
        self.assertEqual(receipt["stderr"], "warning")

    def test_json_budget_rejects_before_parser(self):
        for text in ('{"data":"' + 'x' * module.OUTPUT_LIMIT + '"}',
                     '{"data":' + '[' * 65 + '0' + ']' * 65 + '}',
                     '{"data":[' + ','.join(['0'] * 100001) + ']}'):
            with patch.object(module.json, "loads", side_effect=AssertionError("parser called")):
                with self.assertRaisesRegex(ValueError, "budget"):
                    module.bounded_json(text)
        self.assertEqual(module.bounded_json('{"quoted":"[{}]\\\"", "items":[]}'),
                         {"quoted": '[{}]"', "items": []})

    def test_command_uses_remaining_aggregate_deadline(self):
        with tempfile.TemporaryDirectory() as directory:
            q = module.Qualification("test", Path(directory), 10, None)
            q.deadline = 105
            receipt = {"command": [], "returncode": 0, "timed_out": False,
                       "stdout": "{}", "stderr": "", "capture_complete": True}
            with patch.object(module.time, "monotonic", return_value=100), patch.object(
                    module, "bounded_command", return_value=receipt) as run:
                q.kubectl("get", "pods")
            self.assertEqual(run.call_args.args[1], 5)

    def test_capture_continues_after_failed_command_and_bounds_logs(self):
        for count in (2, 20):
            with self.subTest(count=count), tempfile.TemporaryDirectory() as directory:
                q = module.Qualification("test", Path(directory), 10, None)
                q.kubectl = Mock(side_effect=[TimeoutError("stalled"), result({}),
                    result({"items": [{"metadata": {"name": str(i)}} for i in range(count)]})]
                    + [result({})] * min(count, 16))
                q.capture()
                self.assertEqual(q.kubectl.call_count, 3 + min(count, 16))
                self.assertIn("--limit-bytes=1048576", q.kubectl.call_args.args)
                self.assertIsNone(q.deadline)
                receipt = json.loads((Path(directory) / "capture.json").read_text())
                self.assertFalse(receipt["complete"])
                self.assertIn("stalled", receipt["errors"][0])

    def test_capture_nonzero_retains_failure(self):
        with tempfile.TemporaryDirectory() as directory:
            q = module.Qualification("test", Path(directory), 10, None)
            q.kubectl = Mock(side_effect=[subprocess.CompletedProcess([], 1, "", "forbidden"),
                result({}), result({"items": []})])
            q.capture()
            receipt = json.loads((Path(directory) / "capture.json").read_text())
            self.assertFalse(receipt["complete"])
            self.assertEqual(q.kubectl.call_count, 3)

    def test_capture_success_has_complete_receipt(self):
        with tempfile.TemporaryDirectory() as directory:
            q = module.Qualification("test", Path(directory), 10, None)
            q.kubectl = Mock(side_effect=[result({}), result({}), result({"items": []})])
            q.capture()
            self.assertEqual(json.loads((Path(directory) / "capture.json").read_text()),
                             {"complete": True, "errors": []})

    def test_finished_cancelled_and_execution_deadline_cleanup(self):
        for mode in ("finished", "cancelled", "timeout"):
            with self.subTest(mode=mode), tempfile.TemporaryDirectory() as directory:
                q = module.Qualification("test", Path(directory), 10, 1 if mode == "cancelled" else None)
                q.validate = Mock(return_value="test")
                q.recover_ownership = Mock(side_effect=lambda: setattr(q, "uid", "owned"))
                q.capture = Mock()
                q.cleanup = Mock()
                def api(*args, **kwargs):
                    if args[:2] == ("apply", "--dry-run=server"):
                        self.assertEqual(q.deadline, 110)
                    return result({"status": {"stage": "finished"}})
                q.kubectl = api
                times = [100, 111 if mode == "timeout" else 102]
                with patch.object(module.time, "monotonic", side_effect=times):
                    if mode == "timeout":
                        with self.assertRaises(TimeoutError):
                            q.run(Path("manifest"), Path("assets"))
                    else:
                        self.assertIn(mode, q.run(Path("manifest"), Path("assets")))
                q.capture.assert_called_once()
                q.cleanup.assert_called_once()
                self.assertIsNone(q.deadline)

    def test_manifest_refuses_cluster_or_foreign_resources(self):
        for value in [{"kind": "Namespace"}, {"kind": "TestRun", "metadata": {"namespace": "foreign"}},
                      {"kind": "List", "items": []}]:
            q = module.Qualification("test", Path("unused"), 10, None)
            q.kubectl = Mock(return_value=result(value))
            with self.assertRaises(ValueError):
                q.validate(Path("unused"))

    def test_client_normalization_uses_owned_namespace(self):
        q = module.Qualification("test", Path("unused"), 10, None)
        q.kubectl = Mock(return_value=result({"kind": "TestRun", "metadata": {
            "namespace": q.namespace, "name": "test"}}))
        self.assertEqual(q.validate(Path("manifest.yaml")), "test")
        q.kubectl.assert_called_once_with("create", "--dry-run=client", "-n", q.namespace,
                                         "-f", "manifest.yaml", "-o", "json")

    def test_changed_namespace_never_deleted(self):
        q = module.Qualification("test", Path("unused"), 10, None)
        q.uid = "owned"
        q.kubectl = Mock(return_value=result({"metadata": {"uid": "replacement"}}))
        with self.assertRaises(RuntimeError):
            q.cleanup()
        self.assertEqual(q.kubectl.call_count, 1)

    def test_delete_uid_precondition_handles_replacement_race(self):
        for replacement in (False, True):
            with self.subTest(replacement=replacement), tempfile.TemporaryDirectory() as directory:
                q = module.Qualification("test", Path(directory), 10, None)
                q.uid = "owned"
                state = {"uid": "owned", "deleted": False, "waited": False}

                def api(*args, **kwargs):
                    if args[0] == "get":
                        response = result({"metadata": {"uid": state["uid"]}})
                        if replacement:
                            state["uid"] = "foreign-replacement"
                        return response
                    if args[0] == "delete":
                        self.assertEqual(args[1], "--raw=/api/v1/namespaces/" + q.namespace)
                        self.assertEqual(args[2], "-f")
                        self.assertFalse(kwargs["required"])
                        body = json.loads(Path(args[3]).read_text())
                        self.assertEqual(body, {"apiVersion": "v1", "kind": "DeleteOptions",
                                                "preconditions": {"uid": "owned"}})
                        if body["preconditions"]["uid"] != state["uid"]:
                            return subprocess.CompletedProcess([], 1, "", "Error from server (Conflict): 409")
                        state["deleted"] = True
                        return result({})
                    self.assertEqual(args[0], "wait")
                    state["waited"] = True
                    return result({})

                q.kubectl = api
                if replacement:
                    with self.assertRaisesRegex(RuntimeError, "ownership lost"):
                        q.cleanup()
                    self.assertFalse(state["deleted"])
                    self.assertFalse(state["waited"])
                else:
                    q.cleanup()
                    self.assertTrue(state["deleted"])
                    self.assertTrue(state["waited"])

    def test_delete_failure_never_waits_or_retries(self):
        with tempfile.TemporaryDirectory() as directory:
            q = module.Qualification("test", Path(directory), 10, None)
            q.uid = "owned"
            q.kubectl = Mock(side_effect=[result({"metadata": {"uid": "owned"}}),
                subprocess.CompletedProcess([], 1, "", "server unavailable")])
            with self.assertRaisesRegex(RuntimeError, "deletion failed"):
                q.cleanup()
            self.assertEqual(q.kubectl.call_count, 2)

    def test_failed_creation_never_deletes_namespace(self):
        directory = tempfile.TemporaryDirectory()
        self.addCleanup(directory.cleanup)
        q = module.Qualification("test", Path(directory.name), 10, None)
        q.validate = Mock(return_value="test")
        q.kubectl = Mock(side_effect=[result({}), result({}), RuntimeError("already exists"),
                                     result({"metadata": {"uid": "foreign", "labels": {}}})])
        q.cleanup = Mock()
        with self.assertRaises(RuntimeError):
            q.run(Path("unused"), Path("unused"))
        q.cleanup.assert_not_called()

    def test_startup_failure_captures_and_cleans(self):
        with tempfile.TemporaryDirectory() as directory:
            q = module.Qualification("test", Path(directory), 10, None)
            q.validate = Mock(return_value="test")
            q.kubectl = Mock(side_effect=[result({}), result({}), result({}),
                                         result({"metadata": {"uid": "owned", "labels": {"llm-perf-qualification-owner": q.nonce}}}),
                                         RuntimeError("startup")])
            q.capture = Mock()
            q.cleanup = Mock()
            with self.assertRaises(RuntimeError):
                q.run(Path("unused"), Path("unused"))
            q.capture.assert_called_once()
            q.cleanup.assert_called_once()

    def test_lost_create_response_recovers_only_own_nonce(self):
        with tempfile.TemporaryDirectory() as directory:
            q = module.Qualification("test", Path(directory), 10, None)
            q.validate = Mock(return_value="test")
            q.kubectl = Mock(side_effect=[result({}), result({}), TimeoutError("lost response"),
                result({"metadata": {"uid": "owned", "labels": {"llm-perf-qualification-owner": q.nonce}}})])
            q.capture = Mock()
            q.cleanup = Mock()
            with self.assertRaises(TimeoutError):
                q.run(Path("unused"), Path("unused"))
            self.assertEqual(q.uid, "owned")
            q.cleanup.assert_called_once()
            self.assertTrue(json.loads((Path(directory) / "cleanup.json").read_text())["complete"])

    def test_cleanup_failure_retains_incomplete_receipt(self):
        with tempfile.TemporaryDirectory() as directory:
            q = module.Qualification("test", Path(directory), 10, None)
            q.validate = Mock(return_value="test")
            q.kubectl = Mock(side_effect=[result({}), result({}), TimeoutError("lost response"),
                result({"metadata": {"uid": "owned", "labels": {"llm-perf-qualification-owner": q.nonce}}})])
            q.capture = Mock()
            q.cleanup = Mock(side_effect=RuntimeError("namespace still terminating"))
            with self.assertRaises(RuntimeError):
                q.run(Path("unused"), Path("unused"))
            receipt = json.loads((Path(directory) / "cleanup.json").read_text())
            self.assertFalse(receipt["complete"])
            self.assertIn("still terminating", receipt["error"])


if __name__ == "__main__":
    unittest.main()
