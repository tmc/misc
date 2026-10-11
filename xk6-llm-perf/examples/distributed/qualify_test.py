#!/usr/bin/env python3
"""Lifecycle guard tests; mocks are not cluster qualification."""
import importlib.util
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import Mock

spec = importlib.util.spec_from_file_location("qualify", Path(__file__).with_name("qualify.py"))
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


def result(value):
    return subprocess.CompletedProcess([], 0, json.dumps(value), "")


class QualificationTest(unittest.TestCase):
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
