#!/usr/bin/env python3
"""Record bounded nvidia-smi query intervals; compare asynchronous NVML readings."""
import argparse
import csv
import hashlib
import json
import math
import pathlib
import subprocess
import time

FIELDS = {
    "gpu_utilization_pct": ("utilization.gpu", "percent", 1, 15),
    "memory_utilization_pct": ("utilization.memory", "percent", 1, 15),
    "power_milliwatts": ("power.draw", "milliwatts", 1000, 10000),
    "temperature_celsius": ("temperature.gpu", "celsius", 1, 3),
    "sm_clock_mhz": ("clocks.current.sm", "mhz", 1, 300),
    "memory_clock_mhz": ("clocks.current.memory", "mhz", 1, 300),
}


def record(args):
    query = "uuid," + ",".join(v[0] for v in FIELDS.values())
    end = time.monotonic() + args.duration
    with open(args.output, "x") as out:
        while time.monotonic() < end:
            start_ns = time.time_ns()
            result = subprocess.run(
                ["nvidia-smi", "--query-gpu=" + query, "--format=csv,noheader,nounits"],
                capture_output=True, text=True, timeout=5, check=True)
            finish_ns = time.time_ns()
            for row in csv.reader(result.stdout.splitlines()):
                if len(row) != len(FIELDS) + 1:
                    raise ValueError("unexpected nvidia-smi field count")
                values = {}
                for (name, (_, unit, scale, _)), value in zip(FIELDS.items(), row[1:]):
                    try:
                        value = float(value.strip()) * scale
                    except ValueError:
                        value = None
                    if value is not None and not math.isfinite(value):
                        raise ValueError("non-finite reference reading")
                    values[name] = {"value": value, "unit": unit}
                out.write(json.dumps({"device_uuid": row[0].strip(),
                                      "query_start_unix": start_ns,
                                      "query_end_unix": finish_ns,
                                      "readings": values}) + "\n")
                out.flush()
            time.sleep(args.interval)


def native_value(reading):
    """Use the producer integer value, including omitted integer zero."""
    is_int = reading.get("is_int", False)
    if not isinstance(is_int, bool):
        raise ValueError("invalid native integer flag")
    if is_int:
        value = reading.get("int_value", 0)
        if type(value) is not int or not 0 <= value <= 2**64 - 1:
            raise ValueError("invalid native integer reading")
    else:
        value = reading.get("value")
        if type(value) not in (int, float) or not math.isfinite(value):
            raise ValueError("invalid native numeric reading")
    return value



def validate_reference(records):
    for record in records:
        start = record.get("query_start_unix")
        end = record.get("query_end_unix")
        if (type(start) is not int or type(end) is not int or
                not 0 < start <= end <= 2**63 - 1):
            raise ValueError("invalid reference query interval")
        for name, reading in record.get("readings", {}).items():
            if name not in FIELDS:
                continue
            if reading.get("unit") != FIELDS[name][1]:
                raise ValueError("reference unit mismatch: " + name)
            value = reading.get("value")
            if value is None:
                continue
            try:
                valid = type(value) in (int, float) and math.isfinite(value)
            except OverflowError:
                valid = False
            if not valid:
                raise ValueError("invalid reference numeric reading")


def compare(args):
    reference = [json.loads(s) for s in pathlib.Path(args.reference).read_text().splitlines()]
    validate_reference(reference)
    observed = [json.loads(s) for s in pathlib.Path(args.native).read_text().splitlines()]
    manifests = [r["manifest"] for r in observed if "manifest" in r]
    if not manifests or not manifests[-1].get("is_complete") or manifests[-1].get("lost_records", 0):
        raise ValueError("native capture lacks complete, lossless final manifest")
    checks = []
    for obj in observed:
        for reading in obj.get("sample", {}).get("readings", []):
            name = reading.get("name")
            if name not in FIELDS:
                continue
            check = {"metric": name, "device_uuid": reading.get("device_uuid")}
            if reading.get("evidence") != "measured":
                check.update(status="UNAVAILABLE", reason=reading.get("unavailable_reason", "not measured"))
            else:
                if reading.get("unit") != FIELDS[name][1]:
                    raise ValueError("native unit mismatch: " + name)
                stamp = reading.get("timestamp_unix")
                if type(stamp) is not int or not 0 < stamp <= 2**63 - 1:
                    raise ValueError("native reading lacks wall timestamp")
                candidates = [r for r in reference if r["device_uuid"] == check["device_uuid"]
                              and r["readings"][name]["value"] is not None]
                if not candidates:
                    check.update(status="UNTESTED", reason="no available same-device reference")
                else:
                    nearest = min(candidates, key=lambda r: abs((r["query_start_unix"] + r["query_end_unix"]) // 2 - stamp))
                    skew = abs((nearest["query_start_unix"] + nearest["query_end_unix"]) // 2 - stamp)
                    if skew > args.max_skew_ms * 1e6:
                        check.update(status="UNTESTED", reason="reference outside declared pairing interval")
                    else:
                        value = native_value(reading)
                        if not math.isfinite(value):
                            raise ValueError("invalid native numeric reading")
                        delta = abs(value - nearest["readings"][name]["value"])
                        check.update(status="PASS" if delta <= FIELDS[name][3] else "GAP",
                                     delta=delta, tolerance=FIELDS[name][3], pairing_skew_nanos=skew)
            checks.append(check)
    if not any(r["status"] in ("PASS", "GAP") for r in checks):
        raise ValueError("no comparable measured native readings")
    print(json.dumps({"qualification": "DIAGNOSTIC", "reference": "nvidia-smi; shared NVML backend",
                      "limitations": ["asynchronous queries; no independent sensor truth",
                                       "no MIG or sensor-exact energy qualification"],
                      "schema_version": manifests[-1].get("schema_version"),
                      "native_sha256": hashlib.sha256(pathlib.Path(args.native).read_bytes()).hexdigest(),
                      "reference_sha256": hashlib.sha256(pathlib.Path(args.reference).read_bytes()).hexdigest(),
                      "max_pairing_skew_ms": args.max_skew_ms, "checks": checks}, indent=2))
    return 1 if any(r["status"] == "GAP" for r in checks) else 0


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="command", required=True)
    rec = sub.add_parser("record")
    rec.add_argument("--output", required=True)
    rec.add_argument("--duration", type=float, default=10)
    rec.add_argument("--interval", type=float, default=.2)
    comp = sub.add_parser("compare")
    comp.add_argument("--reference", required=True)
    comp.add_argument("--native", required=True)
    comp.add_argument("--max-skew-ms", type=float, default=250)
    args = parser.parse_args()
    if args.command == "record":
        if not 0 < args.duration <= 300 or not .05 <= args.interval <= 10:
            parser.error("duration must be (0,300] seconds and interval [0.05,10]")
        record(args)
        return 0
    if not 0 < args.max_skew_ms <= 1000:
        parser.error("pairing skew must be (0,1000] milliseconds")
    return compare(args)


if __name__ == "__main__":
    raise SystemExit(main())
