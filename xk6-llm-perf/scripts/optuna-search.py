#!/usr/bin/env python3
"""Numeric and Pareto Optuna searches over repeated llm-campaign evaluations."""
import argparse
import contextlib
import types
import copy
from decimal import Decimal, localcontext
import fcntl
import importlib.util
import importlib.metadata
import math
import os
import platform
from pathlib import Path
import signal
import socket
import struct
import json
import subprocess
import sys
import time

spec = importlib.util.spec_from_file_location("optuna_campaign", Path(__file__).with_name("optuna-campaign.py"))
core = importlib.util.module_from_spec(spec)
spec.loader.exec_module(core)
TORCH_PIN = "2.13.0"
TORCH_VERSIONS = (TORCH_PIN, TORCH_PIN + "+cpu")


def validate(config):
    fields = {"schema_version", "campaign_binary", "campaign", "sampler", "seed", "max_trials", "trial_timeout_seconds", "max_duration_seconds", "domains", "objectives"}
    if not isinstance(config, dict) or set(config) != fields or config["sampler"] not in ("tpe", "gp"):
        raise ValueError("invalid search configuration")
    objectives = config["objectives"]
    if not isinstance(objectives, list) or not 1 <= len(objectives) <= 8:
        raise ValueError("invalid objective count")
    seen = set()
    for objective in objectives:
        if not isinstance(objective, dict) or set(objective) != {"metric", "direction"} or objective["direction"] not in ("minimize", "maximize"):
            raise ValueError("invalid objective")
        metric = objective["metric"]
        if not isinstance(metric, dict) or set(metric) != {"name", "stat"} or any(not isinstance(value, str) or not value or len(value) > 256 for value in metric.values()):
            raise ValueError("invalid objective metric")
        key = core.sha(metric)
        if key in seen:
            raise ValueError("duplicate objective")
        seen.add(key)
    domains = config["domains"]
    if not isinstance(domains, list) or not 1 <= len(domains) <= 12:
        raise ValueError("invalid domain count")
    dimensions, names = [], set()
    for domain in domains:
        if not isinstance(domain, dict) or not isinstance(domain.get("name"), str) or domain["name"] in names:
            raise ValueError("invalid or duplicate domain")
        names.add(domain["name"])
        kind = domain.get("type")
        if kind == "categorical":
            if set(domain) != {"name", "type", "values"} or not isinstance(domain["values"], list) or not 1 <= len(domain["values"]) <= 1000:
                raise ValueError("invalid categorical domain")
            if any(not isinstance(value, str) or len(value) > 1024 for value in domain["values"]) or len(set(domain["values"])) != len(domain["values"]):
                raise ValueError("invalid categorical values")
            initial = domain["values"][0]
        elif kind in ("int", "float"):
            if not {"name", "type", "low", "high"} <= set(domain) or set(domain) - {"name", "type", "low", "high", "step", "log"}:
                raise ValueError("invalid numeric domain fields")
            for name in ("low", "high"):
                value = domain[name]
                if isinstance(value, bool) or not isinstance(value, (int, float)) or not math.isfinite(value) or kind == "int" and (type(value) is not int or abs(value) > (1 << 53) - 1):
                    raise ValueError("invalid numeric bound")
            if domain["low"] > domain["high"] or not math.isfinite(domain["high"] - domain["low"]) or type(domain.get("log", False)) is not bool or domain.get("log", False) and domain["low"] <= 0:
                raise ValueError("invalid numeric range")
            if "step" in domain:
                step = domain["step"]
                if isinstance(step, bool) or not isinstance(step, (float, int)) or not math.isfinite(step) or step <= 0 or kind == "int" and type(step) is not int or domain.get("log", False) and (kind == "float" or step != 1):
                    raise ValueError("invalid numeric step")
                # Refuse Optuna's silent effective-high adjustment.
                with localcontext() as context:
                    context.prec = 1000
                    remainder = ((Decimal(str(domain["high"])) - Decimal(str(domain["low"]))) % Decimal(str(step)))
                if remainder != 0:
                    raise ValueError("step does not divide numeric range")
            initial = format(domain["low"], ".17g") if kind == "float" else str(domain["low"])
        else:
            raise ValueError("unsupported domain type")
        dimensions.append({"name": domain["name"], "values": [initial]})
    base = copy.deepcopy(config["campaign"])
    base.update(dimensions=dimensions, objective=objectives[0]["metric"], minimize=objectives[0]["direction"] == "minimize", mode="grid", max_candidates=1)
    if base.get("convergence") not in ("range", "cv", "ci95"):
        raise ValueError("search requires run-level range/cv/ci95 convergence")
    compatibility = {key: value for key, value in config.items() if key not in ("domains", "objectives")}
    compatibility["campaign"], compatibility["sampler"] = base, "tpe"
    core.validate(compatibility)
    return base


campaign_policy = validate


def identity(config):
    result = core.identities(config)
    result["search_driver_sha256"] = core.file_sha(__file__)
    result["python_version"] = sys.version
    result["platform"] = sys.platform
    result["machine"] = platform.machine()
    if config["sampler"] == "gp":
        version = importlib.metadata.version("torch")
        if version not in TORCH_VERSIONS:
            raise ValueError("Torch version differs from pin")
        result["dependency_versions"]["torch"] = version
    return result


def create_study(config):
    import optuna
    optuna.logging.set_verbosity(optuna.logging.WARNING)
    callback = lambda trial: trial.user_attrs.get("constraint_violations", [1e6])
    if config["sampler"] == "gp":
        if importlib.metadata.version("torch") not in TORCH_VERSIONS:
            raise ValueError("Torch version differs from pin")
        import torch
        sampler = optuna.samplers.GPSampler(seed=config["seed"], n_startup_trials=10, constraints_func=callback)
    else:
        sampler = optuna.samplers.TPESampler(seed=config["seed"], n_startup_trials=10, constraints_func=callback)
    return optuna.create_study(directions=[objective["direction"] for objective in config["objectives"]], sampler=sampler)


def ask(study, domains):
    trial, values, parameters = study.ask(), {}, {}
    for domain in domains:
        name, kind = domain["name"], domain["type"]
        if kind == "categorical":
            value = trial.suggest_categorical(name, domain["values"])
        else:
            options = {"log": domain.get("log", False)}
            if "step" in domain:
                options["step"] = domain["step"]
            method = trial.suggest_int if kind == "int" else trial.suggest_float
            value = method(name, domain["low"], domain["high"], **options)
        values[name] = value
        parameters[name] = format(value, ".17g") if kind == "float" else str(value)
    return trial, values, parameters


class PlannerDeadline(TimeoutError):
    """The owned CPU planner exceeded the remaining study budget."""


class PlannerChannel:
    def __init__(self, stream):
        self.stream = stream

    def send(self, value):
        data = json.dumps(value, allow_nan=False, separators=(",", ":")).encode()
        if len(data) > 65536:
            raise ValueError("planner message exceeds limit")
        self.stream.sendall(struct.pack("!I", len(data)) + data)

    def recv(self, remaining=None):
        def read(size):
            chunks = bytearray()
            while len(chunks) < size:
                if remaining is not None:
                    budget = remaining()
                    if budget <= 0:
                        raise PlannerDeadline("planner deadline exceeded")
                    self.stream.settimeout(budget)
                part = self.stream.recv(size - len(chunks))
                if not part:
                    raise EOFError("planner disconnected")
                chunks.extend(part)
            return bytes(chunks)
        size = struct.unpack("!I", read(4))[0]
        if size > 65536:
            raise ValueError("planner message exceeds limit")
        return json.loads(read(size))

    def close(self):
        self.stream.close()


def planner_worker(connection, config):
    try:
        study = create_study(config)
        pending = None
        while True:
            operation, payload = connection.recv()
            if operation == "ask":
                pending, values, parameters = ask(study, config["domains"])
                result = (values, parameters)
            elif operation == "tell":
                tell(study, pending, payload)
                pending = None
                result = None
            elif operation == "front":
                result = [trial.number for trial in study.best_trials]
            else:
                raise ValueError("invalid planner operation")
            connection.send((True, result))
    except BaseException as error:
        try:
            connection.send((False, type(error).__name__))
        except (BrokenPipeError, EOFError):
            pass
    finally:
        connection.close()


class OwnedPlanner:
    def __init__(self, config, remaining):
        parent_socket, child_socket = socket.socketpair()
        self.connection, child = PlannerChannel(parent_socket), PlannerChannel(child_socket)
        self.remaining = remaining
        self.process = subprocess.Popen([sys.executable, __file__, "--planner-worker", str(child_socket.fileno())], pass_fds=(child_socket.fileno(),), start_new_session=True)
        child.close()
        self.connection.stream.settimeout(max(.000001, self.remaining()))
        try:
            self.connection.send(config)
        except BaseException:
            self.close()
            raise

    def close(self):
        # Kill the entire owned group even when its leader has exited.
        try:
            os.killpg(self.process.pid, signal.SIGTERM)
        except ProcessLookupError:
            pass
        try:
            self.process.wait(timeout=.5)
        except subprocess.TimeoutExpired:
            pass
        try:
            os.killpg(self.process.pid, signal.SIGKILL)
        except ProcessLookupError:
            pass
        if self.process.poll() is None:
            self.process.kill()
        self.process.wait(timeout=3)
        self.connection.close()

    def rpc(self, operation, payload=None):
        remaining = self.remaining()
        if remaining <= 0:
            raise PlannerDeadline("planner deadline exceeded")
        # These messages are bounded domain/receipt objects, never raw summaries.
        self.connection.stream.settimeout(remaining)
        try:
            self.connection.send((operation, payload))
            self.connection.stream.settimeout(max(0.000001, self.remaining()))
            valid, result = self.connection.recv(self.remaining)
        except (TimeoutError, socket.timeout):
            self.close()
            raise PlannerDeadline("planner deadline exceeded")
        if not valid:
            raise ValueError("owned planner failed: " + result)
        if self.remaining() <= 0:
            raise PlannerDeadline("planner deadline exceeded")
        return result

    def proposal(self):
        values, parameters = self.rpc("ask")
        return None, values, parameters

    def receipt(self, receipt):
        self.rpc("tell", {key: receipt[key] for key in ("state", "values", "constraint_violations") if key in receipt})

    @property
    def best_trials(self):
        return [types.SimpleNamespace(number=number) for number in self.rpc("front")]


def observed(directory, parameters, config):
    policy = campaign_policy(config)
    receipt = core.observed(directory, parameters, policy)
    if receipt["state"] != "COMPLETE":
        return receipt
    manifest = core.read_json(directory / "campaign.json")
    samples = [[] for _ in config["objectives"]]
    for attempt in manifest["candidates"][0]["attempts"]:
        if attempt.get("error") or attempt.get("exit_code") != 0 or attempt.get("value") is None:
            continue
        summary = core.read_json(directory / attempt["directory"] / "summary.json")
        for index, objective in enumerate(config["objectives"]):
            metric = summary.get("metrics", {}).get(objective["metric"]["name"], {})
            metric = metric.get("values", metric)
            value = metric.get(objective["metric"]["stat"])
            if isinstance(value, bool) or not isinstance(value, (int, float)) or not math.isfinite(value):
                return {"state": "FAIL", "reason": "objective unavailable", "artifact_sha256": receipt["artifact_sha256"]}
            samples[index].append(value)
    if not all(core.converged(values, policy) for values in samples):
        return {"state": "FAIL", "reason": "not all objectives converged", "artifact_sha256": receipt["artifact_sha256"]}
    values = [sum(value / len(samples[index]) for value in samples[index]) for index in range(len(samples))]
    if not math.isclose(values[0], receipt["value"], rel_tol=1e-12, abs_tol=1e-12):
        raise ValueError("primary objective contradicts raw summaries")
    receipt.pop("value")
    receipt.update(values=values, objective_run_counts=[len(series) for series in samples])
    return receipt


def tell(study, trial, receipt):
    import optuna
    if receipt["state"] == "COMPLETE":
        trial.set_user_attr("constraint_violations", receipt["constraint_violations"])
        study.tell(trial, receipt["values"])
    else:
        study.tell(trial, state=optuna.trial.TrialState.FAIL)


def pareto(state, objectives):
    feasible = [trial for trial in state["trials"] if trial["state"] == "COMPLETE" and all(value <= 0 for value in trial["constraint_violations"])]
    front = []
    for trial in feasible:
        dominated = False
        for other in feasible:
            comparisons = [(a <= b, a < b) if objective["direction"] == "minimize" else (a >= b, a > b) for a, b, objective in zip(other["values"], trial["values"], objectives)]
            if all(weak for weak, _strict in comparisons) and any(strict for _weak, strict in comparisons):
                dominated = True
                break
        if not dominated:
            front.append(trial["number"])
    return front


def run(config, output, started=None, *, validate_config=None, pin_identity=None, observe=None, make_study=None):
    # Resolve defaults per call; outcome drivers supply their policy explicitly.
    validate_config = validate if validate_config is None else validate_config
    pin_identity = identity if pin_identity is None else pin_identity
    observe = observed if observe is None else observe
    if started is None:
        started = time.monotonic()
    base, pinned = validate_config(config), pin_identity(config)
    output.mkdir(parents=True, exist_ok=True, mode=0o700)
    if output.is_symlink() or output.stat().st_mode & 0o077:
        raise ValueError("output must be private and nonsymlink")
    with open(output / ".optuna-search-lock", "a+b") as lock, contextlib.ExitStack() as cleanup:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        checkpoint = output / "optuna-search.json"
        state = {"schema_version": 1, "identity": pinned, "trials": [], "elapsed_seconds": 0}
        if checkpoint.exists():
            state = core.read_json(checkpoint)
            if state.get("schema_version") != 1 or state.get("identity") != pinned:
                raise ValueError("search identity differs")
        trials, elapsed = state.get("trials"), state.get("elapsed_seconds")
        if not isinstance(trials, list) or len(trials) > config["max_trials"] or isinstance(elapsed, bool) or not isinstance(elapsed, (int, float)) or not math.isfinite(elapsed) or elapsed < 0:
            raise ValueError("invalid search checkpoint")
        previous = elapsed
        def remaining_budget():
            return config["max_duration_seconds"] - previous - (time.monotonic() - started)
        def retain_failure():
            state["elapsed_seconds"] = previous + time.monotonic() - started
            core.write_json(checkpoint, state)
        cleanup.callback(retain_failure)
        if remaining_budget() <= 0:
            raise PlannerDeadline("study budget exhausted before replay")
        if config["sampler"] == "gp":
            study = OwnedPlanner(config, remaining_budget)
        elif make_study is None:
            study = create_study(config)
        else:
            study = make_study(config, output, remaining_budget)
        if isinstance(study, OwnedPlanner):
            cleanup.callback(study.close)
        def proposal():
            return study.proposal() if isinstance(study, OwnedPlanner) else ask(study, config["domains"])
        def submit(trial, receipt):
            if isinstance(study, OwnedPlanner):
                study.receipt(receipt)
            else:
                tell(study, trial, receipt)
        used = set()
        for number, receipt in enumerate(trials):
            if remaining_budget() <= 0:
                raise PlannerDeadline("study budget exhausted during replay")
            trial, values, parameters = proposal()
            if remaining_budget() <= 0:
                raise PlannerDeadline("study budget exhausted during replay proposal")
            if type(receipt.get("number")) is not int or receipt["number"] != number or core.encoded(receipt.get("suggested_values")) != core.encoded(values) or receipt.get("parameters") != parameters:
                raise ValueError("search proposal replay differs")
            if receipt.get("state") == "RUNNING":
                reservation = receipt.get("reserved_duration_seconds")
                if isinstance(reservation, bool) or not isinstance(reservation, (int, float)) or not math.isfinite(reservation) or not 0 <= reservation <= config["trial_timeout_seconds"] + 6:
                    raise ValueError("invalid interrupted time reservation")
                elapsed += reservation
                previous += reservation
                state["elapsed_seconds"] = elapsed
                receipt.update(state="FAIL", reason="interrupted evaluation; not retried")
                core.write_json(checkpoint, state)
            elif receipt.get("state") == "COMPLETE":
                actual = observe(output / ("trial-%04d" % number), parameters, config)
                if any(core.encoded(receipt.get(key)) != core.encoded(value) for key, value in actual.items()):
                    raise ValueError("search trial artifacts differ")
            elif receipt.get("state") != "FAIL":
                raise ValueError("invalid trial state")
            for relative, expected in receipt.get("artifact_sha256", {}).items():
                path = Path(relative)
                if path.is_absolute() or ".." in path.parts or core.file_sha(output / ("trial-%04d" % number) / path) != expected:
                    raise ValueError("search artifact changed")
            used.add(core.sha(parameters))
            submit(trial, receipt)
        def save():
            state["elapsed_seconds"] = previous + time.monotonic() - started
            state["pareto_trials"] = pareto(state, config["objectives"])
            core.write_json(checkpoint, state)
            core.write_json(output / "pareto.json", {"schema_version": 1, "identity": pinned, "objectives": config["objectives"], "trials": [trials[number] for number in state["pareto_trials"]], "scope": "observed feasible converged cells; no optimality or significance claim"})
        validation = output / "validate"
        validation.mkdir(exist_ok=True, mode=0o700)
        core.write_json(validation / "config.json", base)
        if remaining_budget() <= 0:
            raise PlannerDeadline("replay exhausted study budget")
        if core.run_child([config["campaign_binary"], "-config", str(validation / "config.json"), "-validate"], validation, min(10, remaining_budget())) != 0:
            raise ValueError("campaign configuration rejected")
        while len(trials) < config["max_trials"]:
            remaining = config["max_duration_seconds"] - previous - (time.monotonic() - started)
            if remaining <= 0:
                state["stop_reason"] = "duration_budget"
                break
            if pin_identity(config) != pinned:
                raise ValueError("search pinned sources changed")
            if remaining_budget() <= 0:
                state["stop_reason"] = "duration_budget"
                break
            trial, values, parameters = proposal()
            remaining = remaining_budget()
            if remaining <= 0:
                state["stop_reason"] = "duration_budget"
                break
            number = len(trials)
            receipt = {"number": number, "parameters": parameters, "suggested_values": values, "state": "RUNNING", "reserved_duration_seconds": min(config["trial_timeout_seconds"], remaining) + 6}
            trials.append(receipt)
            save()
            key = core.sha(parameters)
            if key in used:
                receipt.update(state="FAIL", reason="duplicate proposal; not rerun")
            else:
                used.add(key)
                directory = output / ("trial-%04d" % number)
                directory.mkdir(mode=0o700)
                cell = copy.deepcopy(base)
                cell["dimensions"] = [{"name": name, "values": [value]} for name, value in parameters.items()]
                core.write_json(directory / "config.json", cell)
                try:
                    remaining = remaining_budget()
                    if remaining <= 0:
                        receipt.update(state="FAIL", reason="setup exhausted study budget; evaluation not started")
                        state["stop_reason"] = "duration_budget"
                        submit(trial, receipt)
                        save()
                        break
                    status = core.run_child([config["campaign_binary"], "-config", str(directory / "config.json"), "-output", str(directory)], directory, min(config["trial_timeout_seconds"], remaining))
                    if status != 0:
                        receipt.update(state="FAIL", reason="campaign process failed", exit_code=status)
                        for path in (directory / "campaign.json", directory / "driver.log"):
                            if path.exists():
                                receipt.setdefault("artifact_sha256", {})[path.name] = core.file_sha(path)
                    else:
                        receipt.update(observe(directory, parameters, config))
                except subprocess.TimeoutExpired:
                    receipt.update(state="FAIL", reason="deadline exceeded; not retried")
                except BaseException:
                    receipt.update(state="FAIL", reason="interrupted or invalid evaluation; not retried")
                    save()
                    raise
            submit(trial, receipt)
            save()
        else:
            state["stop_reason"] = "trial_budget"
        save()
        # Cross-check our exported finite front against Optuna's actual front.
        native = [trial.number for trial in study.best_trials]
        if native != state["pareto_trials"]:
            raise ValueError("native Optuna Pareto front differs")
        return state


def main():
    if len(sys.argv) == 3 and sys.argv[1] == "--planner-worker":
        channel = PlannerChannel(socket.socket(fileno=int(sys.argv[2])))
        planner_worker(channel, channel.recv())
        return 0
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--config", required=True)
    parser.add_argument("--output", required=True)
    args = parser.parse_args()
    def interrupted(_signum, _frame):
        raise KeyboardInterrupt()
    signal.signal(signal.SIGTERM, interrupted)
    try:
        result = run(core.read_json(args.config, 1 << 20), Path(args.output).absolute())
        print(result["stop_reason"] + ": " + str(len(result["trials"])) + " proposals, " + str(len(result["pareto_trials"])) + " Pareto cells")
        return 0
    except BaseException as error:
        sys.stderr.write("optuna search failed (" + type(error).__name__ + ")\n")
        return 1


if __name__ == "__main__":
    sys.exit(main())
