"""Offline controls for snapshot identity and displayed statistic populations."""
import datetime
import hashlib
import json
from pathlib import Path
import runpy
import sys
import tempfile
import types
import unittest
from unittest.mock import patch


class ExportTest(unittest.TestCase):
    def setUp(self):
        root = Path.home() / "tmp" / "xk6-llm-perf" / (datetime.date.today().strftime("%Y%m%d") + "-wandb-tests")
        root.mkdir(parents=True, exist_ok=True)
        self.temp = tempfile.TemporaryDirectory(dir=root)
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.source = self.root / "summary.json"
        self.output = self.root / "offline"
        self.document = dict(schema_version=2, run_id="fixture", source_versions={}, attempts=2,
                             successes=2, failures=0, dropped_records=0, policy="request policy",
                             workload="fixture", trace_hash="hash", seed=1, duration_seconds=1,
                             window_policy="whole contained", warmup_policy="excluded",
                             percentile_method="nearest_rank", standard_deviation_method="population",
                             retained_records=2, retained_successes=2, excluded_boundary_requests=1,
                             inflight_at_end=0, missing_input_counts=1, missing_output_counts=0,
                             estimated_input_counts=0, estimated_output_counts=0, timeline=None,
                             ttst_policy="second recognized event; not a physical token",
                             content_interval_policy="pooled successful recognized-content intervals",
                             content_interval_unavailable_requests=0, content_interval_truncated_requests=1)
        base = dict(unit="ms", count=3, mean=25, min=20, max=30, p50=25, p90=30, p99=30, std=(50/3)**0.5)
        self.document["metrics"] = [dict(base, name="content_inter_chunk_latency"),
                                    dict(base, name="input_tokens", unit="token", count=1,
                                         min=25, max=25, p50=25, p90=25, p99=25, std=0, sources={"reported":1, "unavailable":1})]
        self.bytes = json.dumps(self.document).encode()
        self.source.write_bytes(self.bytes)
        self.received = {}
        received = self.received

        class Run:
            id = "fixture"
            def __enter__(self):
                return self
            def __exit__(self, *args):
                # The SDK may stage/upload the file only when the run completes.
                received["artifact_bytes"] = received["artifact_path"].read_bytes()
                return False
            def log(self, items):
                received["table"] = items["summary_metrics"]
            def log_artifact(self, artifact):
                received["artifact_logged"] = True

        class Artifact:
            def __init__(self, *args, **kwargs):
                received["artifact_metadata"] = kwargs["metadata"]
            def add_file(self, path, **kwargs):
                received["artifact_path"] = Path(path)


        def init(**kwargs):
            received["config"] = kwargs["config"]
            self.source.write_bytes(b'{"replacement":"not validated"}')
            return Run()

        self.sdk = types.SimpleNamespace(init=init, Artifact=Artifact, Table=lambda **kwargs: kwargs)

    def run_export(self):
        argv = ["wandb_export.py", str(self.source), "--project", "fixture", "--directory", str(self.output)]
        with patch.dict(sys.modules, {"wandb": self.sdk}), patch.object(sys, "argv", argv):
            runpy.run_path(str(Path(__file__).with_name("wandb_export.py")), run_name="__main__")

    def test_original_mutation_preserves_snapshot_and_population(self):
        source_opens = []
        original_open = Path.open
        reads = []
        class Reader:
            def __init__(self, file): self.file = file
            def __enter__(self): return self
            def __exit__(self, *args): self.file.close()
            def read(self, limit=-1):
                reads.append(limit)
                return self.file.read(limit)
        def tracked_open(path, mode="r", *args, **kwargs):
            if path == self.source and mode == "rb":
                source_opens.append(mode)
                return Reader(original_open(path, mode, *args, **kwargs))
            return original_open(path, mode, *args, **kwargs)
        with patch.object(Path, "open", tracked_open):
            self.run_export()
        self.assertEqual(self.received["artifact_bytes"], self.bytes)
        self.assertEqual(source_opens, ["rb"])
        self.assertEqual(reads, [4 * 1024 * 1024 + 1])
        digest = hashlib.sha256(self.bytes).hexdigest()
        self.assertEqual(self.received["config"]["summary_sha256"], digest)
        self.assertEqual(self.received["artifact_metadata"]["summary_sha256"], digest)
        self.assertFalse(self.received["artifact_path"].exists())
        table = self.received["table"]
        row = dict(zip(table["columns"], table["data"][0]))
        self.assertEqual(row["population"], self.document["content_interval_policy"])
        self.assertEqual(row["percentile_method"], "nearest_rank")
        self.assertEqual(row["standard_deviation_method"], "population")
        self.assertEqual(json.loads(table["data"][1][-1]), {"reported":1,"unavailable":1})
        for key in ("retained_records", "content_interval_truncated_requests", "content_interval_policy"):
            self.assertEqual(self.received["config"][key], self.document[key])

    def test_growth_beyond_limit_refused_despite_stale_stat(self):
        self.source.write_bytes(self.bytes + b" " * (4 * 1024 * 1024 + 1 - len(self.bytes)))
        original_stat = Path.stat
        def stale_stat(path, *args, **kwargs):
            if path == self.source:
                return types.SimpleNamespace(st_size=1)
            return original_stat(path, *args, **kwargs)
        with patch.object(Path, "stat", stale_stat), self.assertRaises(SystemExit) as err:
            self.run_export()
        self.assertEqual(err.exception.code, 2)
        self.assertNotIn("config", self.received)


if __name__ == "__main__":
    unittest.main()
