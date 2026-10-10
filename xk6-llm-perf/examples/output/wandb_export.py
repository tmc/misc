#!/usr/bin/env python3
"""Consume a version-1 llm-output JSON document using the official W&B SDK.

Offline is the default. Online export requires --online and separate SDK
credentials. Only the explicitly supplied sanitized summary is uploaded.
"""
import argparse
import json
import math
from pathlib import Path


def main():
    p = argparse.ArgumentParser()
    p.add_argument("summary", type=Path)
    p.add_argument("--project", required=True)
    p.add_argument("--entity")
    p.add_argument("--directory", type=Path, required=True)
    p.add_argument("--online", action="store_true")
    args = p.parse_args()
    if args.summary.stat().st_size > 4 * 1024 * 1024:
        p.error("summary exceeds 4 MiB")
    def invalid_constant(value):
        raise ValueError("non-finite JSON number: " + value)
    document = json.loads(args.summary.read_text(encoding="utf-8"), parse_constant=invalid_constant)
    required = {"schema_version", "run_id", "source_versions", "attempts",
                "successes", "failures", "dropped_records", "metrics", "policy", "workload", "trace_hash", "seed", "duration_seconds", "window_policy", "warmup_policy"}
    if set(document) != required or type(document["schema_version"]) is not int or document["schema_version"] != 1:
        p.error("unsupported summary schema")
    if not isinstance(document["metrics"], list) or len(document["metrics"]) > 1000:
        p.error("too many metrics")
    columns = ["name", "unit", "count", "mean", "min", "max", "p50", "p90", "p99", "std"]
    rows = []
    for metric in document["metrics"]:
        if set(metric) != set(columns):
            p.error("invalid metric schema")
        if not isinstance(metric["name"], str) or not isinstance(metric["unit"], str) or len(metric["name"])>128 or len(metric["unit"])>128:
            p.error("invalid metric identity")
        if type(metric["count"]) is not int or not 0 <= metric["count"] <= 100000:
            p.error("invalid metric count")
        for key in columns[3:]:
            value = metric[key]
            if value is not None and (type(value) not in (int,float) or not math.isfinite(value) or value < 0):
                p.error("invalid metric value")
        rows.append([metric[key] for key in columns])
    import wandb
    args.directory.mkdir(parents=True, exist_ok=True)
    with wandb.init(project=args.project, entity=args.entity,
                    name=document["run_id"], dir=str(args.directory),
                    mode="online" if args.online else "offline",
                    config={"schema_version": 1, "source_versions": document["source_versions"],
                            "policy": document["policy"], "workload":document["workload"],"trace_hash":document["trace_hash"],"seed":document["seed"],"duration_seconds":document["duration_seconds"],"window_policy":document["window_policy"],"warmup_policy":document["warmup_policy"]}) as run:
        run.log({"summary_metrics": wandb.Table(columns=columns, data=rows),
                 "attempts": document["attempts"], "successes": document["successes"],
                 "failures": document["failures"]})
        artifact = wandb.Artifact("llmperf-" + run.id, type="llmperf-run")
        artifact.add_file(str(args.summary), name="summary.json")
        run.log_artifact(artifact)


if __name__ == "__main__":
    main()
