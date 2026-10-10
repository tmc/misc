#!/usr/bin/env python3
"""Consume a measurement stream in an explicitly provisioned local OTel collector."""
import argparse
import json
import socket
import subprocess
import time
from pathlib import Path


def main():
    p = argparse.ArgumentParser()
    p.add_argument("--directory", type=Path, required=True)
    p.add_argument("--collector", type=Path, required=True)
    p.add_argument("--binary", type=Path, required=True)
    args = p.parse_args()
    args.directory.mkdir(parents=True, exist_ok=True)
    with socket.socket() as s:
        s.bind(("127.0.0.1", 0))
        port = s.getsockname()[1]
    config = args.directory / "collector.yaml"
    config.write_text(f"""receivers:
  otlp:
    protocols:
      http:
        endpoint: 127.0.0.1:{port}
exporters:
  debug:
    verbosity: detailed
service:
  pipelines:
    metrics:
      receivers: [otlp]
      exporters: [debug]
""")
    log = args.directory / "collector.log"
    with log.open("w") as log_file:
        process = subprocess.Popen([str(args.collector.resolve()), "--config", str(config)], stdout=log_file, stderr=subprocess.STDOUT)
        try:
            for _ in range(100):
                try:
                    with socket.create_connection(("127.0.0.1", port), timeout=.2):
                        break
                except OSError:
                    if process.poll() is not None:
                        raise RuntimeError("collector failed")
                    time.sleep(.1)
            else:
                raise RuntimeError("collector start timeout")
            record = {"schema_version": 1, "phase": "measurement", "attempted": True,
                      "recorded": True, "outcome": "success", "started_at":1100,
                      "dispatched_at":1100, "finished_at":1200, "full_response_latency":100,
                      "input_tokens":{"source":"reported","value":0},
                      "output_tokens":{"source":"unavailable","value":None}}
            records = args.directory / "records.jsonl"
            records.write_text(json.dumps(record) + "\n")
            result = subprocess.run([str(args.binary.resolve()), "-input", str(records), "-stream", "-format", "otlp",
                                     "-stream-run", "fixture", "-endpoint", f"http://127.0.0.1:{port}/v1/metrics"],
                                    capture_output=True, text=True, check=True)
            receipt = json.loads(result.stdout)
            assert receipt["complete"] and receipt["records_accepted"] == 1
            for _ in range(100):
                if "gen_ai.client.operation.duration" in log.read_text():
                    break
                time.sleep(.1)
            else:
                raise RuntimeError("collector did not emit metric")
            exported = log.read_text()
            assert "Unit: s" in exported and "Value: 0.100000" in exported
            assert "gen_ai.token.type: Str(input)" in exported
            assert "gen_ai.token.type: Str(output)" not in exported
            receipt["consumer"] = "otelcol"
            receipt["version"] = subprocess.check_output([str(args.collector.resolve()), "--version"], text=True).strip()
            (args.directory / "receipt.json").write_text(json.dumps(receipt, indent=2))
            print(json.dumps(receipt))
        finally:
            process.terminate()
            process.wait(timeout=10)


if __name__ == "__main__":
    main()
