#!/usr/bin/env python3
"""Pinned Optuna search over independently repeated llm-campaign cells."""
import argparse
import copy
import contextlib
import fcntl
import hashlib
import importlib.metadata
import json
import math
import os
from pathlib import Path
import signal
import subprocess
import sys
import time

REFERENCE = "f10eb50a1776775d80d7f13396b0f668f14e2c03"
PINS = {"optuna": "4.9.0", "numpy": "2.4.6", "scipy": "1.17.1",
        "SQLAlchemy": "2.0.52", "alembic": "1.19.1", "colorlog": "6.12.0",
        "packaging": "26.3", "PyYAML": "6.0.3", "tqdm": "4.70.0",
        "Mako": "1.4.1", "MarkupSafe": "3.0.3", "typing-extensions": "4.16.0"}


def encoded(value):
    return json.dumps(value, allow_nan=False, sort_keys=True, separators=(",", ":")).encode()


def sha(value):
    return hashlib.sha256(encoded(value)).hexdigest()


def file_sha(path):
    result = hashlib.sha256()
    with open(path, "rb") as stream:
        while block := stream.read(1 << 20):
            result.update(block)
    return result.hexdigest()


def read_json(path, maximum=64 << 20):
    with open(path, "rb") as stream:
        raw = stream.read(maximum + 1)
    if len(raw) > maximum:
        raise ValueError("JSON exceeds limit")
    def pairs(items):
        result = {}
        for key, value in items:
            if key in result:
                raise ValueError("duplicate JSON key")
            result[key] = value
        return result
    def constant(_value):
        raise ValueError("invalid JSON number")
    return json.loads(raw, object_pairs_hook=pairs, parse_constant=constant)


def write_json(path, value):
    temporary = path.with_suffix(path.suffix + ".new")
    with open(temporary, "wb") as stream:
        stream.write(encoded(value) + b"\n")
        stream.flush()
        os.fsync(stream.fileno())
    os.replace(temporary, path)


def validate(config):
    fields = {"schema_version", "campaign_binary", "campaign", "sampler", "seed", "max_trials", "trial_timeout_seconds", "max_duration_seconds"}
    if not isinstance(config, dict) or set(config) != fields or type(config["schema_version"]) is not int or config["schema_version"] != 1:
        raise ValueError("invalid configuration schema")
    for key, maximum in (("seed", (1 << 32) - 1), ("max_trials", 10000), ("trial_timeout_seconds", 86400), ("max_duration_seconds", 604800)):
        minimum = 0 if key == "seed" else 1
        if type(config[key]) is not int or not minimum <= config[key] <= maximum:
            raise ValueError("invalid " + key)
    if config["sampler"] != "tpe" or not isinstance(config["campaign"], dict):
        raise ValueError("unsupported sampler or campaign")
    dimensions = config["campaign"].get("dimensions")
    if not isinstance(dimensions, list) or not 1 <= len(dimensions) <= 12:
        raise ValueError("invalid dimensions")
    names, cardinality = set(), 1
    for dimension in dimensions:
        if not isinstance(dimension, dict) or set(dimension) != {"name", "values"}:
            raise ValueError("invalid dimension fields")
        name, values = dimension["name"], dimension["values"]
        if not isinstance(name, str) or name in names or not isinstance(values, list) or not values or len(values) > 1000:
            raise ValueError("invalid dimension")
        if any(not isinstance(value, str) or len(value) > 1024 for value in values) or len(set(values)) != len(values):
            raise ValueError("invalid dimension values")
        names.add(name)
        cardinality *= len(values)
        if cardinality > 10000:
            raise ValueError("search space exceeds 10000 cells")
    if config["campaign"].get("convergence") not in ("range", "cv", "ci95"):
        raise ValueError("search requires run-level range/cv/ci95 convergence")
    # Every cell is evaluated as one fixed grid candidate by the existing engine.
    campaign = copy.deepcopy(config["campaign"])
    campaign["mode"], campaign["max_candidates"] = "grid", 1
    return campaign, cardinality


def identities(config):
    versions = {name: importlib.metadata.version(name) for name in PINS}
    if versions != PINS:
        raise ValueError("dependency versions differ from pins")
    campaign = config["campaign"]
    paths = [config["campaign_binary"], campaign["binary"], campaign["script"]] + campaign.get("assets", [])
    if any(not isinstance(path, str) or not Path(path).is_absolute() for path in paths):
        raise ValueError("binary, script and asset paths must be absolute")
    hashes = {str(Path(path)): file_sha(path) for path in paths}
    return {"reference_revision": REFERENCE, "dependency_versions": versions, "input_sha256": hashes, "config_sha256": sha(config), "driver_sha256": file_sha(__file__)}


def run_child(arguments, directory, timeout):
    with open(directory / "driver.log", "wb") as log:
        process = subprocess.Popen(arguments, stdout=log, stderr=log, start_new_session=True)
        try:
            status = process.wait(timeout=timeout)
            return status
        except BaseException:
            try:
                os.killpg(process.pid, signal.SIGTERM)
            except ProcessLookupError:
                pass
            try:
                process.wait(timeout=3)
            except subprocess.TimeoutExpired:
                pass
            # The leader can exit while a descendant ignores TERM. Always signal
            # the owned group, even after wait() has reaped the leader.
            try:
                os.killpg(process.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
            process.wait(timeout=3)
            raise
        finally:
            # A successful CLI must not leave background workers in its group.
            try:
                os.killpg(process.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass


def policy_equal(left, right):
    if isinstance(left, dict) and isinstance(right, dict):
        return left.keys() == right.keys() and all(policy_equal(left[key], right[key]) for key in left)
    if isinstance(left, list) and isinstance(right, list):
        return len(left) == len(right) and all(policy_equal(a, b) for a, b in zip(left, right))
    if isinstance(left, bool) or isinstance(right, bool):
        return type(left) is type(right) and left == right
    return left == right


def campaign_cell_id(parameters):
    # Match encoding/json's UTF-8 and HTML-safe map encoding.
    raw = json.dumps(parameters, ensure_ascii=False, sort_keys=True, separators=(",", ":"))
    for character, escape in (("&", "\\u0026"), ("<", "\\u003c"), (">", "\\u003e"), ("\u2028", "\\u2028"), ("\u2029", "\\u2029")):
        raw = raw.replace(character, escape)
    return hashlib.sha256(raw.encode()).hexdigest()[:24]


def cell_policy(policy, parameters):
    expected = copy.deepcopy(policy)
    expected["dimensions"] = [{"name": name, "values": [value]} for name, value in parameters.items()]
    return expected


def converged(values, policy):
    if len(values) < policy["min_runs"] or policy["min_runs"] < 2:
        return False
    mean = sum(value / len(values) for value in values)
    if mean == 0:
        return min(values) == max(values) == 0
    method = policy["convergence"]
    if method == "range":
        score = (max(values) - min(values)) / abs(mean)
    elif method in ("cv", "ci95"):
        sd = math.sqrt(sum((value - mean) ** 2 for value in values) / (len(values) - 1))
        score = sd / abs(mean) if method == "cv" else 2 * 1.96 * sd / math.sqrt(len(values)) / abs(mean)
    else:
        raise ValueError("search requires run-level range/cv/ci95 convergence")
    return math.isfinite(score) and score <= policy["tolerance"]


def observed(directory, parameters, policy):
    manifest_path = directory / "campaign.json"
    manifest = read_json(manifest_path)
    candidates = manifest.get("candidates", [])
    if len(candidates) != 1 or candidates[0].get("parameters") != parameters:
        raise ValueError("campaign did not evaluate the proposed cell")
    expected = cell_policy(policy, parameters)
    retained = manifest.get("config", {})
    for key in set(expected) | set(retained):
        default = [] if key in ("constraints", "assets") else False if key == "minimize" else 0 if key == "retries" else None
        left, right = expected.get(key, default), retained.get(key, default)
        if key in ("constraints", "assets"):
            left, right = left or [], right or []
        if not policy_equal(left, right):
            raise ValueError("campaign policy differs from pinned configuration: " + key)
    cell = candidates[0]
    if cell.get("id") != campaign_cell_id(parameters) or not isinstance(cell.get("attempts"), list) or len(cell["attempts"]) > min(policy["max_attempts"], policy["max_runs"] + policy.get("retries", 0)):
        raise ValueError("campaign cell identity or attempt bound differs")
    artifact_hashes = {"campaign.json": file_sha(manifest_path)}
    constraints = policy.get("constraints", [])
    values = []
    failures = 0
    violations = [-float("inf")] * len(constraints)
    failed_attempt = False
    for number, attempt in enumerate(cell.get("attempts", []), 1):
        if type(attempt.get("number")) is not int or attempt.get("number") != number or attempt.get("directory") != cell["id"] + "/attempt-%04d" % number:
            raise ValueError("campaign attempt identity differs")
        retained_value = attempt.get("value")
        if type(attempt.get("exit_code")) is not int or type(attempt.get("passed")) is not bool or retained_value is not None and (isinstance(retained_value, bool) or not isinstance(retained_value, (int, float)) or not math.isfinite(retained_value)):
            raise ValueError("campaign attempt metadata has invalid types")
        failed_attempt |= bool(attempt.get("error")) or attempt.get("exit_code") != 0 or attempt.get("value") is None
        digest = attempt.get("summary_sha256")
        summary = {}
        if digest:
            relative = Path(attempt["directory"]) / "summary.json"
            if relative.is_absolute() or ".." in relative.parts:
                raise ValueError("invalid summary artifact path")
            actual = file_sha(directory / relative)
            if actual != digest:
                raise ValueError("campaign summary hash mismatch")
            artifact_hashes[str(relative)] = actual
            summary = read_json(directory / relative)
        metric = summary.get("metrics", {}).get(policy["objective"]["name"], {})
        value = metric.get("values", metric).get(policy["objective"]["stat"])
        valid = not isinstance(value, bool) and isinstance(value, (int, float)) and math.isfinite(value)
        failed_attempt |= not valid
        passed = not attempt.get("error") and attempt.get("exit_code") == 0 and valid
        if passed:
            if attempt.get("value") != value:
                raise ValueError("campaign attempt objective contradicts raw summary")
            values.append(value)
        else:
            failures += 1
        for index, constraint in enumerate(constraints):
            metric = summary.get("metrics", {}).get(constraint["metric"]["name"], {})
            metric = metric.get("values", metric)
            value = metric.get(constraint["metric"]["stat"])
            if isinstance(value, bool) or not isinstance(value, (int, float)) or not math.isfinite(value):
                violation = 1e6
            elif constraint["direction"] == "le":
                violation = value - constraint["bound"]
            else:
                violation = constraint["bound"] - value
            violations[index] = max(violations[index], violation)
            passed &= violation <= 0
        if attempt.get("passed") is not passed:
            raise ValueError("campaign attempt feasibility contradicts raw summaries")
    mean = sum(value / len(values) for value in values) if values else None
    retained_mean = cell.get("mean")
    if type(cell.get("converged")) is not bool or type(cell.get("failed")) is not bool or retained_mean is not None and (isinstance(retained_mean, bool) or not isinstance(retained_mean, (int, float)) or not math.isfinite(retained_mean)):
        raise ValueError("campaign derived metadata has invalid types")
    convergence = converged(values, policy)
    failed = failures > policy.get("retries", 0) or len(values) >= policy["max_runs"] and not convergence or len(cell.get("attempts", [])) >= policy["max_runs"] + policy.get("retries", 0) and not convergence
    if cell.get("mean") != mean or cell.get("converged") != convergence or cell.get("failed") != failed:
        raise ValueError("campaign objective or convergence contradicts raw summaries")
    if failed or not convergence or mean is None:
        return {"state": "FAIL", "artifact_sha256": artifact_hashes}
    feasible = cell.get("feasible")
    if type(feasible) is not bool:
        raise ValueError("campaign feasibility missing")
    # Match upstream's signed SLA direction and missing-observation penalty.
    # Retain worst run per SLA because campaign feasibility requires every run.
    violations = [value if math.isfinite(value) else 1e6 for value in violations]
    violations.append(1 if failed_attempt else 0)
    if feasible != all(value <= 0 for value in violations):
        raise ValueError("campaign feasibility contradicts raw constraint values")
    return {"state": "COMPLETE", "value": mean, "constraint_violations": violations,
            "constraint_violation": max(0, *violations), "artifact_sha256": artifact_hashes}


def create_study(config):
    import optuna
    optuna.logging.set_verbosity(optuna.logging.WARNING)
    sampler = optuna.samplers.TPESampler(seed=config["seed"], n_startup_trials=10,
                                        constraints_func=lambda trial: trial.user_attrs.get("constraint_violations", [1e6]))
    return optuna.create_study(direction="minimize" if config["campaign"]["minimize"] else "maximize", sampler=sampler)


def ask(study, dimensions):
    trial = study.ask()
    parameters = {dimension["name"]: trial.suggest_categorical(dimension["name"], dimension["values"]) for dimension in dimensions}
    return trial, parameters


def tell(study, trial, receipt):
    import optuna
    if receipt["state"] == "COMPLETE":
        trial.set_user_attr("constraint_violations", receipt.get("constraint_violations", [receipt["constraint_violation"]]))
        study.tell(trial, receipt["value"])
    else:
        study.tell(trial, state=optuna.trial.TrialState.FAIL)


def run(config, output):
    started = time.monotonic()
    base, cardinality = validate(config)
    identity = identities(config)
    output.mkdir(parents=True, exist_ok=True, mode=0o700)
    if output.is_symlink() or output.stat().st_mode & 0o077:
        raise ValueError("output must be a private nonsymlink directory")
    with open(output / ".optuna-lock", "a+b") as lock, contextlib.ExitStack() as cleanup:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        checkpoint = output / "optuna.json"
        state = {"schema_version": 1, "identity": identity, "trials": [], "elapsed_seconds": 0}
        if checkpoint.exists():
            state = read_json(checkpoint)
            if state.get("schema_version") != 1 or state.get("identity") != identity:
                raise ValueError("study source/configuration/dependencies differ")
        trials = state.get("trials")
        elapsed = state.get("elapsed_seconds")
        if not isinstance(trials, list) or len(trials) > config["max_trials"] or isinstance(elapsed, bool) or not isinstance(elapsed, (float, int)) or not math.isfinite(elapsed) or elapsed < 0:
            raise ValueError("invalid study checkpoint")
        previous_elapsed = elapsed
        def remaining_budget():
            return config["max_duration_seconds"] - previous_elapsed - (time.monotonic() - started)
        def save():
            state["elapsed_seconds"] = previous_elapsed + time.monotonic() - started
            write_json(checkpoint, state)
        cleanup.callback(save)
        def exhausted():
            state["stop_reason"] = "duration_budget"
            state["best_trial"] = None
            return state
        if remaining_budget() <= 0:
            return exhausted()
        study, used = create_study(config), set()
        # Recreate actual sampler RNG by replaying every ask/tell, never unpickle
        # arbitrary checkpoint objects or silently seed a different continuation.
        for number, receipt in enumerate(trials):
            if remaining_budget() <= 0:
                return exhausted()
            trial, parameters = ask(study, base["dimensions"])
            if remaining_budget() <= 0:
                return exhausted()
            if type(receipt.get("number")) is not int or receipt.get("number") != number or receipt.get("parameters") != parameters:
                raise ValueError("Optuna replay proposal differs")
            if receipt.get("state") == "RUNNING":
                reservation = receipt.get("reserved_duration_seconds")
                if isinstance(reservation, bool) or not isinstance(reservation, (int, float)) or not math.isfinite(reservation) or not 0 <= reservation <= config["trial_timeout_seconds"] + 6:
                    raise ValueError("invalid interrupted trial reservation")
                elapsed += reservation
                previous_elapsed += reservation
                state["elapsed_seconds"] = elapsed
                receipt.update(state="FAIL", reason="interrupted evaluation; not retried")
                write_json(checkpoint, state)
            elif receipt.get("state") == "COMPLETE":
                actual = observed(output / ("trial-%04d" % number), parameters, base)
                if any(not policy_equal(receipt.get(key), value) for key, value in actual.items()):
                    raise ValueError("completed trial artifacts differ")
            elif receipt.get("state") != "FAIL":
                raise ValueError("invalid trial state")
            for relative, expected in receipt.get("artifact_sha256", {}).items():
                path = Path(relative)
                if path.is_absolute() or ".." in path.parts or file_sha(output / ("trial-%04d" % number) / path) != expected:
                    raise ValueError("trial artifact changed")
            if receipt.get("reason") != "duplicate proposal; not rerun":
                used.add(sha(parameters))
            tell(study, trial, receipt)
        # Validate full existing campaign contract before any benchmark execution.
        if remaining_budget() <= 0:
            return exhausted()
        validation = output / "validate"
        validation.mkdir(exist_ok=True, mode=0o700)
        write_json(validation / "config.json", base)
        remaining = remaining_budget()
        if remaining <= 0:
            return exhausted()
        if run_child([config["campaign_binary"], "-config", str(validation / "config.json"), "-validate"], validation, min(10, remaining)) != 0:
            raise ValueError("campaign configuration rejected")
        while len(trials) < config["max_trials"] and len(used) < cardinality:
            remaining = config["max_duration_seconds"] - (previous_elapsed + time.monotonic() - started)
            if remaining <= 0:
                state["stop_reason"] = "duration_budget"
                break
            if identities(config) != identity:
                raise ValueError("pinned source or dependencies changed")
            if remaining_budget() <= 0:
                return exhausted()
            trial, parameters = ask(study, base["dimensions"])
            remaining = remaining_budget()
            if remaining <= 0:
                return exhausted()
            number = len(trials)
            receipt = {"number": number, "parameters": parameters, "state": "RUNNING",
                       "reserved_duration_seconds": min(config["trial_timeout_seconds"], remaining) + 6}
            trials.append(receipt)
            save()
            key = sha(parameters)
            if key in used:
                receipt.update(state="FAIL", reason="duplicate proposal; not rerun")
            else:
                used.add(key)
                directory = output / ("trial-%04d" % number)
                directory.mkdir(mode=0o700)
                cell = copy.deepcopy(base)
                cell["dimensions"] = [{"name": name, "values": [value]} for name, value in parameters.items()]
                write_json(directory / "config.json", cell)
                try:
                    remaining = remaining_budget()
                    if remaining <= 0:
                        receipt.update(state="FAIL", reason="setup exhausted study budget; evaluation not started")
                        state["stop_reason"] = "duration_budget"
                        tell(study, trial, receipt)
                        save()
                        break
                    status = run_child([config["campaign_binary"], "-config", str(directory / "config.json"), "-output", str(directory)], directory, min(config["trial_timeout_seconds"], remaining))
                    if status != 0:
                        receipt.update(state="FAIL", reason="campaign process failed", exit_code=status)
                        for path in (directory / "campaign.json", directory / "driver.log"):
                            if path.exists():
                                receipt.setdefault("artifact_sha256", {})[path.name] = file_sha(path)
                    else:
                        receipt.update(observed(directory, parameters, base))
                except subprocess.TimeoutExpired:
                    receipt.update(state="FAIL", reason="campaign deadline exceeded; not retried")
                except BaseException:
                    receipt.update(state="FAIL", reason="evaluation interrupted or invalid; not retried")
                    save()
                    raise
            tell(study, trial, receipt)
            save()
        else:
            state["stop_reason"] = "space_exhausted" if len(used) == cardinality else "trial_budget"
        feasible = [receipt for receipt in trials if receipt["state"] == "COMPLETE" and receipt["constraint_violation"] == 0]
        state["best_trial"] = (min if base["minimize"] else max)(feasible, key=lambda receipt: receipt["value"])["number"] if feasible else None
        save()
        return state


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--config", required=True)
    parser.add_argument("--output", required=True)
    args = parser.parse_args()
    def interrupted(_signum, _frame):
        raise KeyboardInterrupt()
    signal.signal(signal.SIGTERM, interrupted)
    try:
        state = run(read_json(args.config, 1 << 20), Path(args.output).absolute())
        print(state["stop_reason"] + ": " + str(len(state["trials"])) + " proposals; best=" + str(state["best_trial"]))
        return 0
    except BaseException as error:
        sys.stderr.write("optuna campaign failed (" + type(error).__name__ + ")\n")
        return 1


if __name__ == "__main__":
    sys.exit(main())
