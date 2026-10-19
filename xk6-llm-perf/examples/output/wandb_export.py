#!/usr/bin/env python3
"""Consume a version-1 or version-2 llm-output JSON document using the official W&B SDK.

Offline is the default. Online export requires --online and separate SDK
credentials. Only the explicitly supplied sanitized summary is uploaded.
"""
import argparse
import hashlib
import json
import math
import tempfile
from pathlib import Path


def main():
    p = argparse.ArgumentParser()
    p.add_argument("summary", type=Path)
    p.add_argument("--project", required=True)
    p.add_argument("--entity")
    p.add_argument("--directory", type=Path, required=True)
    p.add_argument("--online", action="store_true")
    args = p.parse_args()
    # Bound the actual read, rather than a separate stat of a mutable path.
    with args.summary.open("rb") as source:
        snapshot = source.read(4 * 1024 * 1024 + 1)
    if len(snapshot) > 4 * 1024 * 1024:
        p.error("summary exceeds 4 MiB")
    def invalid_constant(value):
        raise ValueError("non-finite JSON number: " + value)
    try:
        document = json.loads(snapshot, parse_constant=invalid_constant)
    except (ValueError, UnicodeError):
        p.error("invalid summary JSON")
    summary_hash = hashlib.sha256(snapshot).hexdigest()
    required = {"schema_version", "run_id", "source_versions", "attempts",
                "successes", "failures", "dropped_records", "metrics", "policy", "workload", "trace_hash", "seed", "duration_seconds", "window_policy", "warmup_policy"}
    if type(document.get("schema_version")) is not int or document["schema_version"] not in (1, 2):
        p.error("unsupported summary schema")
    if document["schema_version"] == 2:
        required |= {"percentile_method", "standard_deviation_method", "retained_records",
                     "retained_successes", "excluded_boundary_requests", "inflight_at_end",
                     "missing_input_counts", "missing_output_counts", "estimated_input_counts",
                     "estimated_output_counts", "timeline", "ttst_policy", "content_interval_policy",
                     "content_interval_unavailable_requests", "content_interval_truncated_requests"}
    work_counts = {"work_qualified", "work_accepted", "work_rejected", "work_unavailable", "work_good_requests"}
    optional = {"timeline_unavailable", "measurement_policy", "work_policy", "serving_manifest"} | work_counts
    allowed = required | (optional if document["schema_version"] == 2 else set())
    if not required <= set(document) or not set(document) <= allowed:
        p.error("unsupported summary schema")
    if work_counts & set(document) or "work_policy" in document:
        for key in work_counts:
            if type(document.get(key, 0)) is not int or document.get(key, 0) < 0:
                p.error("invalid work population")
        qualified = document.get("work_qualified", 0)
        accepted = document.get("work_accepted", 0)
        if (type(document["attempts"]) is not int or type(document["successes"]) is not int
                or qualified > document["attempts"] or accepted > document["successes"]
                or qualified != accepted + document.get("work_rejected", 0) + document.get("work_unavailable", 0)
                or document.get("work_good_requests", 0) > accepted):
            p.error("invalid work population")
        if qualified or "work_policy" in document:
            policy = document.get("work_policy")
            if not qualified or not isinstance(policy, str) or not policy or len(policy) > 1024:
                p.error("invalid work policy")
    if "serving_manifest" in document:
        receipt = document["serving_manifest"]
        if (not isinstance(receipt, dict) or "manifest_digest" not in receipt
                or not set(receipt) <= {"manifest_digest", "work_hash", "work_unavailable"}):
            p.error("invalid serving manifest receipt")
        for key in ("manifest_digest", "work_hash"):
            if key not in receipt:
                continue
            value = receipt[key]
            if not isinstance(value, str) or len(value) != 64 or any(c not in "0123456789abcdef" for c in value):
                p.error("invalid serving manifest digest")
        reason = receipt.get("work_unavailable")
        if "work_unavailable" in receipt and (not isinstance(reason, str) or not reason or len(reason) > 1024):
            p.error("invalid serving work reason")
        if ("work_hash" in receipt) == ("work_unavailable" in receipt):
            p.error("serving work hash or unavailable reason required")
    if not isinstance(document["metrics"], list) or len(document["metrics"]) > 1000:
        p.error("too many metrics")
    columns = ["name", "unit", "count", "mean", "min", "max", "p50", "p90", "p99", "std"]
    value_columns = list(columns)
    columns += ["population", "percentile_method", "standard_deviation_method", "token_sources"]
    rows = []
    for metric in document["metrics"]:
        if not set(value_columns) <= set(metric) or not set(metric) <= set(value_columns) | {"sources"}:
            p.error("invalid metric schema")
        if not isinstance(metric["name"], str) or not isinstance(metric["unit"], str) or len(metric["name"])>128 or len(metric["unit"])>128:
            p.error("invalid metric identity")
        count_limit = 4096 * 100000 if metric["name"] == "content_inter_chunk_latency" else 100000
        if type(metric["count"]) is not int or not 0 <= metric["count"] <= count_limit:
            p.error("invalid metric count")
        for key in value_columns[3:]:
            value = metric[key]
            if value is not None and (type(value) not in (int,float) or not math.isfinite(value) or value < 0):
                p.error("invalid metric value")
        if "sources" in metric:
            sources = metric["sources"]
            if not isinstance(sources, dict) or any(
                key not in {"reported", "reported_derived", "client_declared", "estimated", "locally_tokenized", "unavailable"}
                or type(value) is not int or not 0 <= value <= 100000
                for key, value in sources.items()
            ):
                p.error("invalid token sources")
        population = document["policy"]
        percentile = document.get("percentile_method", "see schema-1 policy")
        deviation = document.get("standard_deviation_method", "see schema-1 policy")
        if document["schema_version"] == 2:
            population = "successful retained request observations; unavailable values omitted"
            if metric["name"] == "content_inter_chunk_latency":
                population = document["content_interval_policy"]
            elif metric["name"] == "ttst":
                population += "; " + document["ttst_policy"]
            elif metric["name"] in {"goodput", "work_goodput", "input_throughput", "output_throughput"}:
                population = "aggregate counts divided by declared run window; idle time included"
                if metric["name"] == "work_goodput":
                    population += "; " + document.get("work_policy", "work policy unavailable")
                percentile = deviation = "not applied"
        rows.append([metric[key] for key in value_columns] +
                    [population, percentile, deviation, json.dumps(metric.get("sources", {}), sort_keys=True)])
    config = {key: value for key, value in document.items() if key != "metrics"}
    config["summary_sha256"] = summary_hash
    config["token_count_sources"] = {metric["name"]: metric["sources"]
                                    for metric in document["metrics"] if "sources" in metric}
    import wandb
    args.directory.mkdir(parents=True, exist_ok=True)
    # Keep one validated byte snapshot alive through SDK completion. The producer's
    # original path may be replaced or extended while the SDK initializes.
    with tempfile.TemporaryDirectory(prefix="llmperf-summary-", dir=args.directory) as staging:
        staged = Path(staging) / "summary.json"
        staged.write_bytes(snapshot)
        staged.chmod(0o400)
        with wandb.init(project=args.project, entity=args.entity,
                        name=document["run_id"], dir=str(args.directory),
                        mode="online" if args.online else "offline", config=config) as run:
            run.log({"summary_metrics": wandb.Table(columns=columns, data=rows),
                     "attempts": document["attempts"], "successes": document["successes"],
                     "failures": document["failures"]})
            artifact = wandb.Artifact("llmperf-" + run.id, type="llmperf-run",
                                      metadata={"summary_sha256": summary_hash,
                                                "schema_version": document["schema_version"]})
            artifact.add_file(str(staged), name="summary.json")
            run.log_artifact(artifact)


if __name__ == "__main__":
    main()
