#!/usr/bin/env python3
"""Safe admission tests; these never execute a generated answer."""
import asyncio
import importlib.util
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("bridge", Path(__file__).with_name("accuracy_external.py"))
bridge = importlib.util.module_from_spec(spec)
spec.loader.exec_module(bridge)


class CgroupAdmissionTest(unittest.TestCase):
    def test_effective_limits(self):
        cases = [
            ("bounded leaf", "max", "max", "2147483648", "64", True),
            ("bounded parent", "1073741824", "32", "max", "max", True),
            ("unbounded", "max", "max", "max", "max", False),
            ("memory too large", "max", "max", "4294967296", "64", False),
            ("pids too large", "max", "max", "2147483648", "65", False),
            ("invalid limit", "max", "max", "0", "64", False),
        ]
        for name, parent_memory, parent_pids, memory, pids, admitted in cases:
            with self.subTest(name=name), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                leaf = root / "job"
                leaf.mkdir()
                for path, value in [(root / "memory.max", parent_memory), (root / "pids.max", parent_pids),
                                    (leaf / "memory.max", memory), (leaf / "pids.max", pids)]:
                    path.write_text(value)
                membership = root / "membership"
                membership.write_text("0::/job\n")
                if admitted:
                    bridge.check_cgroup_limits(root, membership)
                else:
                    with self.assertRaises(ValueError):
                        bridge.check_cgroup_limits(root, membership)

    def test_root_without_controller_files(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            leaf = root / "job"
            leaf.mkdir()
            (leaf / "memory.max").write_text("2147483648")
            (leaf / "pids.max").write_text("64")
            membership = root / "membership"
            membership.write_text("0::/job\n")
            bridge.check_cgroup_limits(root, membership)

    def test_invalid_membership(self):
        for value in ["", "1:memory:/job", "0::/../job", "0::/missing", "0::/\n0::/"]:
            with self.subTest(value=value), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                membership = root / "membership"
                membership.write_text(value)
                with self.assertRaises(ValueError):
                    bridge.check_cgroup_limits(root, membership)


class DependencyAdmissionTest(unittest.TestCase):
    def test_version_mismatch_refuses_before_upstream_import(self):
        request = {"dependency_versions": {"fixture": "expected"}}
        with patch.object(bridge.importlib.metadata, "version", return_value="different"):
            with self.assertRaisesRegex(ValueError, "installed dependency versions differ"):
                asyncio.run(bridge.grade(request, Path("unused")))


class CodeWorkerGuardTest(unittest.TestCase):
    def test_inherited_fd_closed_once_even_when_number_is_reused(self):
        with patch.object(bridge.os, "close") as close:
            inherited = bridge.close_in_forked_child(42)
            inherited()
            inherited()
            close.assert_called_once_with(42)

    def test_code_worker_refuses_host_execution(self):
        with self.assertRaisesRegex(ValueError, "confined mount layout"):
            bridge.codegen_worker()


if __name__ == "__main__":
    unittest.main()
