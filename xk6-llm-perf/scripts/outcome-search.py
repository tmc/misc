#!/usr/bin/env python3
"""Pinned outcome constraints and BoTorch proposals over saved campaigns."""
import argparse
import copy
import importlib.metadata
import importlib.util
import math
from pathlib import Path
import signal
import shutil
import sys
import tempfile

spec = importlib.util.spec_from_file_location("numeric_search", Path(__file__).with_name("optuna-search.py"))
search = importlib.util.module_from_spec(spec)
spec.loader.exec_module(search)
core = search.core
PINS = {"torch": "2.13.0", "botorch": "0.18.1", "gpytorch": "1.15.2", "linear_operator": "0.6.1", "optuna-integration": "4.9.0", "scikit-learn": "1.9.1", "pyre-extensions": "0.0.32",
        "multipledispatch": "1.0.0", "threadpoolctl": "3.7.0", "ninja": "1.13.2",
        "joblib": "1.6.0", "typing-inspect": "0.9.0", "mypy-extensions": "1.1.0", "narwhals": "2.27.1"}


def dependency_versions():
    versions = {name: importlib.metadata.version(name) for name in PINS}
    if any(version != PINS[name] for name, version in versions.items() if name != "torch") or versions["torch"] not in ("2.13.0", "2.13.0+cpu"):
        raise ValueError("BoTorch dependencies differ from pins")
    return versions


def base_config(config):
    result = copy.deepcopy(config)
    for key in ("outcome_constraints", "proposal_timeout_seconds", "objective_parameter", "applied_slo", "goodput_accounting"):
        result.pop(key, None)
    if result.get("sampler") == "botorch":
        result["sampler"] = "tpe"
    return result


def validate(config):
    if not isinstance(config, dict) or "outcome_constraints" not in config or "proposal_timeout_seconds" not in config:
        raise ValueError("outcome configuration fields missing")
    constraints = config["outcome_constraints"]
    if not isinstance(constraints, list) or len(constraints) > 16:
        raise ValueError("invalid outcome constraints")
    seen = set()
    for constraint in constraints:
        if not isinstance(constraint, dict) or set(constraint) != {"metric", "op", "bound"} or constraint["op"] not in ("<=", ">=", "=="):
            raise ValueError("invalid outcome constraint")
        metric, bound = constraint["metric"], constraint["bound"]
        if not isinstance(metric, str) or not metric or len(metric) > 256 or isinstance(bound, bool) or not isinstance(bound, (float, int)) or not math.isfinite(bound):
            raise ValueError("invalid outcome metric or bound")
        identity = core.sha(constraint)
        if identity in seen:
            raise ValueError("duplicate outcome constraint")
        seen.add(identity)
    if type(config["proposal_timeout_seconds"]) is not int or not 1 <= config["proposal_timeout_seconds"] <= 180:
        raise ValueError("invalid proposal deadline")
    if config.get("sampler") not in ("tpe", "gp", "botorch"):
        raise ValueError("unsupported outcome sampler")
    if "goodput_accounting" in config and (config["goodput_accounting"] != "all_attempts" or config.get("applied_slo") is None):
        raise ValueError("invalid required goodput accounting")
    policy = config.get("applied_slo")
    if policy is not None and (not isinstance(policy, dict) or set(policy) != {"max_ttft_ms", "max_tpot_ms", "max_e2e_ms"} or any(isinstance(value, bool) or not isinstance(value, (float, int)) or not math.isfinite(value) or value <= 0 for value in policy.values())):
        raise ValueError("invalid required applied SLO policy")
    parameter = config.get("objective_parameter")
    if parameter is not None and (not isinstance(parameter, str) or not any(domain.get("name") == parameter and domain.get("type") == "int" for domain in config.get("domains", []))):
        raise ValueError("objective parameter must name an integer domain")
    return search.validate(base_config(config))


def identity(config):
    result = search.identity(base_config(config))
    result["config_sha256"] = core.sha(config)
    result["outcome_driver_sha256"] = core.file_sha(__file__)
    result["numeric_driver_sha256"] = core.file_sha(Path(__file__).with_name("optuna-search.py"))
    if config["sampler"] == "botorch":
        versions = dependency_versions()
        result["dependency_versions"].update(versions)
        result["proposal_worker_sha256"] = core.file_sha(Path(__file__).with_name("botorch-proposal.py"))
    return result


def signed(observed, constraint):
    if observed is None:
        return 1e6
    if constraint["op"] == "<=":
        value = observed - constraint["bound"]
    elif constraint["op"] == ">=":
        value = constraint["bound"] - observed
    else:
        value = abs(observed - constraint["bound"])
    if not math.isfinite(value):
        raise ValueError("outcome violation overflow")
    return value


def observed(directory, parameters, config):
    receipt = search.observed(directory, parameters, base_config(config))
    if receipt["state"] != "COMPLETE":
        return receipt
    manifest = core.read_json(directory / "campaign.json")
    samples = [[] for _ in config["outcome_constraints"]]
    expected = 0
    accounting = []
    for attempt in manifest["candidates"][0]["attempts"]:
        if attempt.get("error") or attempt.get("exit_code") != 0 or attempt.get("value") is None:
            continue
        expected += 1
        summary = core.read_json(directory / attempt["directory"] / "summary.json")
        if config.get("goodput_accounting") == "all_attempts":
            counts = {}
            for name in ("llm_attempts", "llm_successes", "llm_failures", "llm_good_requests"):
                metric = summary.get("metrics", {}).get(name, {})
                metric = metric.get("values", metric)
                value = metric.get("count")
                if isinstance(value, bool) or not isinstance(value, (float, int)) or not math.isfinite(value) or not 0 <= value <= (1 << 53) - 1 or value != int(value):
                    return {"state": "FAIL", "reason": "goodput accounting unavailable", "artifact_sha256": receipt["artifact_sha256"]}
                counts[name] = int(value)
            metric = summary.get("metrics", {}).get("llm_slo_compliance", {})
            metric = metric.get("values", metric)
            fraction = metric.get("value")
            total = counts["llm_attempts"]
            if total <= 0 or counts["llm_successes"] + counts["llm_failures"] != total or counts["llm_good_requests"] > counts["llm_successes"] or isinstance(fraction, bool) or not isinstance(fraction, (float, int)) or not math.isfinite(fraction) or not 0 <= fraction <= 1 or not math.isclose(fraction, counts["llm_good_requests"] / total, rel_tol=0, abs_tol=1e-12):
                return {"state": "FAIL", "reason": "goodput accounting contradicts all-attempts population", "artifact_sha256": receipt["artifact_sha256"]}
            span = {}
            for name, stat in (("llm_content_span_windows", "count"), ("llm_content_span_duration_ms", "avg"), ("llm_content_span_good_requests", "avg"), ("llm_content_span_attempts", "avg"), ("llm_content_span_goodput", "avg")):
                metric = summary.get("metrics", {}).get(name, {})
                value = metric.get("values", metric).get(stat)
                if isinstance(value, bool) or not isinstance(value, (float, int)) or not math.isfinite(value):
                    return {"state": "FAIL", "reason": "content-span goodput unavailable", "artifact_sha256": receipt["artifact_sha256"]}
                span[name] = value
            duration = span["llm_content_span_duration_ms"]
            if span["llm_content_span_windows"] != 1 or duration <= 0 or span["llm_content_span_good_requests"] != counts["llm_good_requests"] or span["llm_content_span_attempts"] != total or not math.isclose(span["llm_content_span_goodput"], counts["llm_good_requests"] * 1000 / duration, rel_tol=1e-12, abs_tol=1e-12):
                return {"state": "FAIL", "reason": "content-span goodput contradicts observation window", "artifact_sha256": receipt["artifact_sha256"]}
            accounting.append({"directory": attempt["directory"], "counts": counts, "fraction": fraction, "content_span": span})
        for field, threshold in config.get("applied_slo", {}).items():
            name = {"max_ttft_ms": "llm_slo_ttft_ms", "max_tpot_ms": "llm_slo_tpot_ms", "max_e2e_ms": "llm_slo_e2e_ms"}[field]
            metric = summary.get("metrics", {}).get(name, {})
            metric = metric.get("values", metric)
            if any(isinstance(metric.get(stat), bool) or metric.get(stat) != threshold for stat in ("min", "max")):
                return {"state": "FAIL", "reason": "applied SLO policy unavailable or mismatched", "artifact_sha256": receipt["artifact_sha256"]}
        parameter = config.get("objective_parameter")
        if parameter is not None:
            metric = summary.get("metrics", {}).get(config["objectives"][0]["metric"]["name"], {})
            metric = metric.get("values", metric)
            value = metric.get(config["objectives"][0]["metric"]["stat"])
            if isinstance(value, bool) or value != int(parameters[parameter]):
                return {"state": "FAIL", "reason": "objective differs from proposed concurrency", "artifact_sha256": receipt["artifact_sha256"]}
        for index, constraint in enumerate(config["outcome_constraints"]):
            metric = summary.get("metrics", {}).get(constraint["metric"], {})
            metric = metric.get("values", metric)
            value = metric.get("avg")
            if not isinstance(value, bool) and isinstance(value, (float, int)) and math.isfinite(value):
                samples[index].append(value)
    means = [sum(value / len(series) for value in series) if series and len(series) == expected else None for series in samples]
    if config.get("goodput_accounting") == "all_attempts":
        receipt["goodput_accounting"] = accounting
    if config.get("applied_slo") is not None:
        receipt["applied_slo"] = config["applied_slo"]
        receipt["applied_slo_runs"] = expected
    receipt["outcome_values"] = means
    receipt["outcome_run_counts"] = [len(series) for series in samples]
    receipt["outcome_expected_runs"] = expected
    receipt["constraint_violations"] += [signed(value, constraint) for value, constraint in zip(means, config["outcome_constraints"])]
    return receipt


def create_study(config, proposal_root, remaining_budget):
    if config["sampler"] != "botorch":
        return search.create_study(base_config(config))
    versions = dependency_versions()
    import optuna
    import torch
    from optuna_integration import BoTorchSampler
    def candidates(x, objectives, constraints, bounds, pending):
        remaining = min(config["proposal_timeout_seconds"], remaining_budget())
        if remaining <= 0:
            raise ValueError("proposal campaign budget exhausted")
        reference = [float(column.min() - .05 * column.min().abs()) for column in objectives.T]
        data = {"x": x.tolist(), "objectives": objectives.tolist(), "constraints": None if constraints is None else constraints.tolist(),
                "bounds": bounds.tolist(), "pending": None if pending is None else pending.tolist(),
                "reference_point": reference, "seed": int(torch.randint(0, (1 << 31) - 1, (1,)).item())}
        with tempfile.TemporaryDirectory(prefix="proposal-", dir=proposal_root) as temporary:
            directory = Path(temporary)
            core.write_json(directory / "input.json", data)
            try:
                status = core.run_child([sys.executable, str(Path(__file__).with_name("botorch-proposal.py")),
                                         str(directory / "input.json"), str(directory / "output.json")], directory, remaining)
                if status:
                    raise ValueError("BoTorch proposal worker failed")
            except BaseException:
                failure = Path(tempfile.mkdtemp(prefix="proposal-failure-", dir=proposal_root))
                for path in directory.iterdir():
                    if path.is_file() and path.stat().st_size <= 2 << 20:
                        shutil.copyfile(path, failure / path.name)
                raise
            candidate = torch.tensor(core.read_json(directory / "output.json", 1 << 20)["candidate"], dtype=x.dtype, device=x.device)
            if candidate.shape != (1, x.shape[1]) or not torch.isfinite(candidate).all() or not ((candidate >= bounds[0]) & (candidate <= bounds[1])).all():
                raise ValueError("BoTorch proposal worker returned invalid point")
            return candidate
    sampler = BoTorchSampler(seed=config["seed"], n_startup_trials=10, candidates_func=candidates,
        constraints_func=lambda trial: trial.user_attrs.get("constraint_violations", [1e6]))
    return optuna.create_study(directions=[obj["direction"] for obj in config["objectives"]], sampler=sampler)


def run(config, output):
    return search.run(config, output, validate_config=validate, pin_identity=identity,
                      observe=observed, make_study=create_study)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--config", required=True)
    parser.add_argument("--output", required=True)
    args = parser.parse_args()
    def interrupted(_signal, _frame):
        raise KeyboardInterrupt()
    signal.signal(signal.SIGTERM, interrupted)
    try:
        result = run(core.read_json(args.config, 1 << 20), Path(args.output).absolute())
        print(result["stop_reason"] + ": " + str(len(result["pareto_trials"])) + " feasible Pareto cells")
        return 0
    except BaseException as error:
        sys.stderr.write("outcome search failed (" + type(error).__name__ + ")\n")
        return 1


if __name__ == "__main__":
    sys.exit(main())
