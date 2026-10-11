#!/usr/bin/env python3
"""Qualify llm-output against a local MLflow server; never posts remotely."""
import argparse
import json
import socket
import subprocess
import time
import urllib.request
from pathlib import Path


def main():
    p = argparse.ArgumentParser()
    p.add_argument("--directory", required=True, type=Path)
    p.add_argument("--binary", required=True, type=Path)
    args = p.parse_args()
    args.directory.mkdir(parents=True, exist_ok=True)
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        port = sock.getsockname()[1]
    base = "http://127.0.0.1:" + str(port)
    log = (args.directory / "mlflow-server.log").open("w")
    server = subprocess.Popen(["mlflow", "server", "--host", "127.0.0.1", "--port", str(port), "--workers", "1",
                               "--backend-store-uri", (args.directory / "mlruns").as_uri(),
                               "--default-artifact-root", (args.directory / "artifacts").as_uri()],
                              stdout=log, stderr=subprocess.STDOUT)
    try:
        for _ in range(600):
            try:
                with urllib.request.urlopen(base + "/health", timeout=1):
                    break
            except OSError:
                if server.poll() is not None:
                    raise RuntimeError("local MLflow server failed")
                time.sleep(.1)
        else:
            raise RuntimeError("local MLflow health timeout")
        from mlflow.tracking import MlflowClient
        client = MlflowClient(tracking_uri=base)
        experiment = client.create_experiment("llmperf-fixture")
        run = client.create_run(experiment)
        fixture = {"schema_version": 1, "run_id": "fixture", "started_at": 1000,
                   "finished_at": 2000, "duration": 1, "attempts": 0,
                   "successes": 0, "failures": 0, "records": []}
        input_file = args.directory / "run.json"
        input_file.write_text(json.dumps(fixture))
        subprocess.run([str(args.binary.resolve()), "-input", str(input_file), "-format", "mlflow",
                        "-mlflow-run", run.info.run_id,
                        "-endpoint", base + "/api/2.0/mlflow/runs/log-batch"], check=True)
        received = client.get_run(run.info.run_id)
        assert received.data.metrics["goodput.mean"] == 0
        assert "ttft.mean" not in received.data.metrics
        assert received.data.params["unit.ttft"] == "ms"
        summary = args.directory / "summary.json"
        with summary.open("wb") as target:
            subprocess.run([str(args.binary.resolve()), "-input", str(input_file)], stdout=target, check=True)
        record = {"schema_version": 1, "phase": "measurement", "attempted": True, "recorded": True, "outcome": "success", "started_at":1100,"dispatched_at":1100,"finished_at":1200,"full_response_latency":100,"last_content_latency":70,"ttft":20,"ttfo":30,"input_tokens":{"source":"reported","value":0},"output_tokens":{"source":"unavailable","value":None}}
        records = args.directory / "records.jsonl"
        records.write_text(json.dumps(record) + "\n")
        subprocess.run([str(args.binary.resolve()),"-input",str(records),"-stream","-format","mlflow","-mlflow-run",run.info.run_id,"-endpoint",base+"/api/2.0/mlflow/runs/log-batch"],check=True)
        received = client.get_run(run.info.run_id)
        assert received.data.metrics["live.gen_ai.client.operation.duration"] == .1
        assert received.data.metrics["live.llmperf.client.full_response_latency"] == .1
        assert received.data.metrics["live.llmperf.client.last_content_latency"] == .07
        assert received.data.params["unit.last_content_latency"] == "ms"
        observed = dict(fixture, attempts=1, successes=1, records=[record], missing_output_counts=1)
        observed_file = args.directory / "observed-run.json"
        observed_file.write_text(json.dumps(observed))
        subprocess.run([str(args.binary.resolve()), "-input", str(observed_file), "-format", "mlflow",
                        "-mlflow-run", run.info.run_id,
                        "-endpoint", base + "/api/2.0/mlflow/runs/log-batch"], check=True)
        received = client.get_run(run.info.run_id)
        assert received.data.metrics["full_response_latency.mean"] == 100
        assert received.data.metrics["last_content_latency.mean"] == 70
        assert received.data.metrics["request_latency.mean"] == 100
        assert received.data.metrics["live.gen_ai.client.input.token.usage"] == 0
        assert "live.gen_ai.client.output.token.usage" not in received.data.metrics
        client.log_artifact(run.info.run_id, str(summary))
        assert client.list_artifacts(run.info.run_id)[0].path == "summary.json"
        receipt = {"schema_version": 1, "consumer": "mlflow", "version": "3.4.0",
                   "run_id": run.info.run_id, "missing_ttft_omitted": True,
                   "zero_goodput_retained": True, "units_retained": True,
                   "artifact_received": True, "stream_received":True, "distinct_latency_boundaries":True}
        (args.directory / "receipt.json").write_text(json.dumps(receipt, indent=2))
        print(json.dumps(receipt))
    finally:
        server.terminate()
        try:
            server.wait(timeout=10)
        except subprocess.TimeoutExpired:
            server.kill()
            server.wait()
        log.close()


if __name__ == "__main__":
    main()
