"""Offline controls for snapshot identity and displayed statistic populations."""
import copy
import datetime
import hashlib
import json
import os
from pathlib import Path
import runpy
import subprocess
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

    def test_work_and_serving_provenance_and_rate_population(self):
        self.document.update(work_qualified=2, work_accepted=1, work_rejected=0,
                             work_unavailable=1, work_good_requests=1,
                             work_policy="accepted opted-in requests per declared window",
                             serving_manifest={"manifest_digest": "a" * 64,
                                               "work_unavailable": "serving work identity unavailable"})
        rate = dict(name="work_goodput", unit="request/s", count=1,
                    mean=1, min=None, max=None, p50=None, p90=None, p99=None, std=None)
        self.document["metrics"].append(rate)
        self.source.write_text(json.dumps(self.document))
        self.run_export()
        for key in ("work_qualified", "work_accepted", "work_rejected", "work_unavailable",
                    "work_good_requests", "work_policy", "serving_manifest"):
            self.assertEqual(self.received["config"][key], self.document[key])
        table = self.received["table"]
        row = dict(zip(table["columns"], table["data"][-1]))
        self.assertIn("aggregate counts divided by declared run window", row["population"])
        self.assertIn(self.document["work_policy"], row["population"])
        self.assertEqual(row["percentile_method"], "not applied")
        self.assertEqual(row["standard_deviation_method"], "not applied")

    def test_invalid_extended_schema_rejected_before_sdk(self):
        edits = [
            {"unknown": 1}, {"work_qualified": True}, {"work_accepted": -1},
            {"work_unavailable": "1"}, {"work_good_requests": None},
            {"work_qualified": 1}, {"work_good_requests": 1},
            {"work_qualified": 1, "work_accepted": 1, "work_policy": None},
            {"work_qualified": 1, "work_accepted": 1, "work_policy": "x" * 1025},
            {"work_policy": "unbound"}, {"serving_manifest": None},
            {"serving_manifest": {"manifest_digest": "A" * 64, "work_hash": "b" * 64}},
            {"serving_manifest": {"manifest_digest": "a" * 64}},
            {"serving_manifest": {"manifest_digest": "a" * 64, "work_hash": "bad"}},
            {"serving_manifest": {"manifest_digest": "a" * 64, "work_unavailable": ""}},
            {"serving_manifest": {"manifest_digest": "a" * 64, "work_unavailable": True}},
            {"serving_manifest": {"manifest_digest": "a" * 64, "work_hash": "b" * 64, "work_unavailable": "why"}},
            {"serving_manifest": {"manifest_digest": "a" * 64, "work_hash": "b" * 64, "endpoint": "private"}},
        ]
        for edit in edits:
            with self.subTest(edit=edit):
                document = copy.deepcopy(self.document)
                document.update(edit)
                self.source.write_text(json.dumps(document))
                with self.assertRaises(SystemExit) as error:
                    self.run_export()
                self.assertEqual(error.exception.code, 2)
                self.assertNotIn("config", self.received)

    def work_document(self):
        document = copy.deepcopy(self.document)
        document.update(work_qualified=1, work_accepted=1, work_good_requests=1,
                        work_policy="accepted opted-in requests per declared window",
                        duration_seconds=2,
                        serving_manifest={"manifest_digest": "a" * 64, "work_hash": "b" * 64})
        document["metrics"].append(dict(name="work_goodput", unit="request/s", count=1,
                                         mean=0.5, min=None, max=None, p50=None, p90=None,
                                         p99=None, std=None))
        return document

    def test_duplicate_decoded_json_members_refused_before_sdk(self):
        raw = json.dumps(self.work_document())
        invalid = [
            raw.replace('"work_qualified": 1', '"work_qualified": true, "work_qualified": 1'),
            raw.replace('"work_qualified": 1', r'"work_\u0071ualified": false, "work_qualified": 1'),
            raw.replace('"work_hash": "' + 'b' * 64 + '"', '"work_hash": "bad", "work_hash": "' + 'b' * 64 + '"'),
            raw.replace('"work_hash": "' + 'b' * 64 + '"', r'"work_\u0068ash": "bad", "work_hash": "' + 'b' * 64 + '"'),
            raw.replace('"mean": 0.5', '"mean": 999, "mean": 0.5'),
            raw.replace('"sources": {"reported": 1', '"sources": {"reported": 2, "reported": 1'),
            raw.replace('"source_versions": {}', '"source_versions": {"producer": "bad", "producer": "good"}'),
        ]
        for document in invalid:
            with self.subTest(document=document):
                self.source.write_text(document)
                with self.assertRaises(SystemExit) as error:
                    self.run_export()
                self.assertEqual(error.exception.code, 2)
                self.assertNotIn("config", self.received)

    def test_work_goodput_population_coherence_before_sdk(self):
        base = self.work_document()
        invalid = []
        absent = copy.deepcopy(base)
        for key in list(absent):
            if key.startswith("work_"):
                del absent[key]
        invalid.append(absent)
        zero = copy.deepcopy(absent)
        zero.update(work_qualified=0, work_accepted=0, work_good_requests=0)
        invalid.append(zero)
        missing = copy.deepcopy(base)
        missing["metrics"].pop()
        invalid.append(missing)
        for changes in ({"mean": 999}, {"unit": "ms"}, {"count": 0}, {"count": 2},
                        {"count": True}, {"mean": None}, {"mean": True}, {"min": 0},
                        {"p50": 0.5}, {"std": 0}, {"sources": {}}):
            document = copy.deepcopy(base)
            document["metrics"][-1].update(changes)
            invalid.append(document)
        for duration in (None, 0, -1, True, float("nan"), float("inf"), 10 ** 400):
            document = copy.deepcopy(base)
            document["duration_seconds"] = duration
            invalid.append(document)
        duplicate = copy.deepcopy(base)
        duplicate["metrics"].append(copy.deepcopy(duplicate["metrics"][-1]))
        invalid.append(duplicate)
        duplicate = copy.deepcopy(base)
        duplicate["metrics"].append(dict(duplicate["metrics"][-1], mean=999))
        invalid.append(duplicate)
        policy = copy.deepcopy(base)
        del policy["work_policy"]
        invalid.append(policy)
        for document in invalid:
            with self.subTest(document=document):
                self.source.write_text(json.dumps(document))
                with self.assertRaises(SystemExit) as error:
                    self.run_export()
                self.assertEqual(error.exception.code, 2)
                self.assertNotIn("config", self.received)

    def test_actual_output_document_work_and_serving_receipts(self):
        binary = os.environ.get("LLM_OUTPUT_BINARY")
        if not binary:
            self.skipTest("set LLM_OUTPUT_BINARY to test actual output.Document summaries")
        module = Path(__file__).resolve().parents[2]
        fixtures = (module / "cmd/llm-output/testdata/cost.txt").read_text()
        run = json.loads(fixtures.split("-- routed.json --\n", 1)[1].split("\n-- ", 1)[0])
        run.update(duration=2, finished_at=3000, input_throughput=10, output_throughput=5)
        values = {"cache": {}, "compile": {}, "device": "fixture", "dtype": "fixture",
                  "max_output_tokens": 1, "resources": {}, "scheduler": {}, "seed": 0,
                  "temperature": 0, "top_k": 0, "top_p": 1}
        for complete in (False, True):
            with self.subTest(complete=complete):
                good = int(complete)
                run.update(good_requests=good, work_good_requests=good,
                           goodput=good / 2, work_goodput=good / 2)
                run["records"][1]["good"] = complete
                manifest = {"schema_version": 1, "arm": {"endpoint_identity": "private-endpoint"}}
                if complete:
                    manifest.update({key: "a" * 64 for key in ("model_digest", "tokenizer_digest",
                                     "template_digest", "workload_digest", "environment_digest")})
                    manifest["settings"] = {key: {"effective": value, "evidence_digest": "b" * 64}
                                            for key, value in values.items()}
                run["serving_manifest"] = manifest
                result = subprocess.run([binary, "-format", "json"], input=json.dumps(run),
                                        text=True, capture_output=True, check=True, timeout=15)
                document = json.loads(result.stdout)
                self.assertEqual(document["work_qualified"], 1)
                self.assertEqual(document["work_accepted"], 1)
                self.assertIn("work_policy", document)
                serving = document["serving_manifest"]
                self.assertEqual(len(serving["manifest_digest"]), 64)
                self.assertEqual("work_hash" in serving, complete)
                self.assertEqual("work_unavailable" in serving, not complete)
                self.assertNotIn("private-endpoint", result.stdout)
                self.source.write_text(result.stdout)
                self.run_export()
                self.assertEqual(self.received["config"]["serving_manifest"], serving)
                self.assertEqual(self.received["config"]["work_policy"], document["work_policy"])
                self.assertEqual(self.received["config"]["summary_sha256"], hashlib.sha256(result.stdout.encode()).hexdigest())
                self.assertEqual(self.received["artifact_bytes"], result.stdout.encode())
                rate = next(metric for metric in document["metrics"] if metric["name"] == "work_goodput")
                self.assertEqual(rate["count"], 1)
                self.assertEqual(rate["unit"], "request/s")
                self.assertEqual(rate["mean"], document.get("work_good_requests", 0) / document["duration_seconds"])
                forged = copy.deepcopy(run)
                forged["work_goodput"] = 999
                rejected = subprocess.run([binary, "-format", "json"], input=json.dumps(forged),
                                          text=True, capture_output=True, timeout=15)
                self.assertNotEqual(rejected.returncode, 0)
                self.assertEqual(rejected.stdout, "")
                self.assertIn("work", rejected.stderr)

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
