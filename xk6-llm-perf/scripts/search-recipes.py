#!/usr/bin/env python3
"""Compile bounded throughput/SLA presets into explicit saved-run searches."""
import argparse
import copy
import importlib.util
import math
from pathlib import Path
import sys

spec = importlib.util.spec_from_file_location("outcome_search", Path(__file__).with_name("outcome-search.py"))
bridge = importlib.util.module_from_spec(spec)
spec.loader.exec_module(bridge)


def concurrency_recipe(config):
    style = config.get("search_style", "smooth_isotonic")
    fields = {"recipe", "streaming", "concurrency", "sla_filters", "search"}
    if style == "optuna":
        fields.add("concurrency_metric")
    allowed = fields | {"search_style"}
    if set(config) - allowed or not fields <= set(config) or style not in ("optuna", "monotonic", "smooth_isotonic") or type(config["streaming"]) is not bool:
        raise ValueError("invalid concurrency search style or fields")
    filters = config["sla_filters"]
    if not isinstance(filters, list) or not 1 <= len(filters) <= 16:
        raise ValueError("concurrency recipe requires SLA filters")
    constraints = []
    for rule in filters:
        if not isinstance(rule, dict) or set(rule) != {"metric", "op", "bound", "streaming_only"} or rule["op"] not in ("<", "<=", ">", ">=") or type(rule["streaming_only"]) is not bool:
            raise ValueError("invalid concurrency SLA filter")
        if rule["streaming_only"] and not config["streaming"]:
            raise ValueError("configured SLA requires streaming")
        bound = rule["bound"]
        if isinstance(bound, bool) or not isinstance(bound, (float, int)) or not math.isfinite(bound):
            raise ValueError("invalid SLA bound")
        if rule["op"] == "<":
            bound = math.nextafter(float(bound), -math.inf)
        elif rule["op"] == ">":
            bound = math.nextafter(float(bound), math.inf)
        if not math.isfinite(bound):
            raise ValueError("strict SLA boundary overflows")
        constraints.append({"metric": rule["metric"], "direction": "le" if rule["op"] in ("<", "<=") else "ge", "bound": bound})
    domain = config["concurrency"]
    if not isinstance(domain, dict) or set(domain) != {"name", "low", "high"} or any(type(domain[key]) is not int or not 1 <= domain[key] <= 1000 for key in ("low", "high")) or domain["low"] > domain["high"]:
        raise ValueError("invalid concurrency bounds")
    if style != "optuna":
        spec = importlib.util.spec_from_file_location("sla_planner", Path(__file__).with_name("sla-planner.py"))
        planner = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(planner)
        result = copy.deepcopy(config["search"])
        injected = {"style", "low", "high", "filters", "concurrency_parameter"}
        if not isinstance(result, dict) or set(result) & injected:
            raise ValueError("recipe conflicts with explicit planner settings")
        result.setdefault("max_iterations", 20 if style == "monotonic" else 30)
        result.setdefault("sla_replicates", 0)
        result.update(style=style, low=domain["low"], high=domain["high"], concurrency_parameter=domain["name"],
            filters=[{"metric_tag": rule["metric"]["name"], "stat": rule["metric"]["stat"],
                      "op": {"<": "lt", "<=": "le", ">": "gt", ">=": "ge"}[rule["op"]], "threshold": rule["bound"]} for rule in filters])
        planner.validate(result)
        return result
    result = copy.deepcopy(config["search"])
    if not isinstance(result, dict) or any(key in result for key in ("domains", "objectives", "objective_parameter")) or result.get("campaign", {}).get("constraints"):
        raise ValueError("recipe conflicts with explicit search settings")
    result["domains"] = [{**domain, "type": "int", "log": True}]
    result["objectives"] = [{"metric": config["concurrency_metric"], "direction": "maximize"}]
    result["objective_parameter"] = domain["name"]
    result["campaign"]["constraints"] = constraints
    bridge.validate(result)
    return result


def goodput_recipe(config):
    fields = {"recipe", "streaming", "concurrency", "slos", "attainment_fraction", "goodput_metric", "attainment_metric", "search"}
    if set(config) != fields or config["streaming"] is not True:
        raise ValueError("goodput recipe requires streaming and all SLO fields")
    policy = config["slos"]
    if not isinstance(policy, dict) or set(policy) != {"max_ttft_ms", "max_tpot_ms", "max_e2e_ms"} or any(isinstance(value, bool) or not isinstance(value, (float, int)) or not math.isfinite(value) or value <= 0 for value in policy.values()):
        raise ValueError("goodput recipe requires positive finite millisecond SLOs")
    fraction = config["attainment_fraction"]
    if isinstance(fraction, bool) or not isinstance(fraction, (float, int)) or not math.isfinite(fraction) or not 0 < fraction <= 1:
        raise ValueError("invalid attainment fraction")
    domain = config["concurrency"]
    if not isinstance(domain, dict) or set(domain) != {"name", "low", "high"} or any(type(domain[key]) is not int or not 1 <= domain[key] <= 1000 for key in ("low", "high")) or domain["low"] > domain["high"]:
        raise ValueError("invalid concurrency bounds")
    result = copy.deepcopy(config["search"])
    if not isinstance(result, dict) or any(key in result for key in ("domains", "objectives", "applied_slo")) or result.get("campaign", {}).get("constraints"):
        raise ValueError("recipe conflicts with explicit search settings")
    result["domains"] = [{**domain, "type": "int", "log": True}]
    for field, name in (("max_ttft_ms", "SLO_TTFT_MS"), ("max_tpot_ms", "SLO_TPOT_MS"), ("max_e2e_ms", "SLO_E2E_MS")):
        result["domains"].append({"name": name, "type": "categorical", "values": [format(policy[field], ".17g")]})
    result["objectives"] = [{"metric": config["goodput_metric"], "direction": "maximize"}]
    result["campaign"]["constraints"] = [{"metric": config["attainment_metric"], "direction": "ge", "bound": fraction}]
    if config["goodput_metric"] != {"name": "llm_content_span_goodput", "stat": "avg"} or config["attainment_metric"] != {"name": "llm_slo_compliance", "stat": "value"}:
        raise ValueError("goodput recipe requires canonical all-attempts metrics")
    result["applied_slo"] = policy
    result["goodput_accounting"] = "all_attempts"
    bridge.validate(result)
    return result


def expand(config):
    if isinstance(config, dict) and config.get("recipe") == "max-goodput-under-slo":
        return goodput_recipe(config)
    if isinstance(config, dict) and config.get("recipe") == "max-concurrency-under-sla":
        return concurrency_recipe(config)
    if not isinstance(config, dict) or set(config) != {"recipe", "streaming", "concurrency", "throughput_metric", "latency_metric", "sla_ms", "search"}:
        raise ValueError("invalid recipe configuration")
    if config["recipe"] not in ("max-throughput-ttft-sla", "max-throughput-itl-sla") or config["streaming"] is not True:
        raise ValueError("recipe requires supported name and streaming")
    if not isinstance(config["latency_metric"], dict) or config["latency_metric"].get("stat") != "p(95)":
        raise ValueError("throughput recipe requires p95 latency statistic")
    threshold = config["sla_ms"]
    if isinstance(threshold, bool) or not isinstance(threshold, (float, int)) or not math.isfinite(threshold) or threshold <= 0:
        raise ValueError("recipe requires positive finite SLA in milliseconds")
    concurrency = config["concurrency"]
    if not isinstance(concurrency, dict) or set(concurrency) != {"name", "low", "high"} or any(type(concurrency[key]) is not int or not 1 <= concurrency[key] <= 1000 for key in ("low", "high")) or concurrency["low"] > concurrency["high"]:
        raise ValueError("invalid concurrency bounds")
    result = copy.deepcopy(config["search"])
    if not isinstance(result, dict) or any(key in result for key in ("domains", "objectives")) or result.get("campaign", {}).get("constraints"):
        raise ValueError("recipe conflicts with explicit search settings")
    result["domains"] = [{**concurrency, "type": "int", "log": True}]
    result["objectives"] = [{"metric": config["throughput_metric"], "direction": "maximize"}]
    # Finite IEEE754 doubles satisfy x < T exactly when x <= nextafter(T,-inf).
    # The signed acquisition constraint and final exported front agree at T.
    result["campaign"]["constraints"] = [{"metric": config["latency_metric"], "direction": "le", "bound": math.nextafter(float(threshold), -math.inf)}]
    bridge.validate(result)
    return result


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--config", required=True)
    parser.add_argument("--output", required=True)
    parser.add_argument("--execute", help="also execute the compiled configuration in this private results directory")
    args = parser.parse_args()
    try:
        compiled = expand(bridge.core.read_json(args.config, 1 << 20))
        bridge.core.write_json(Path(args.output), compiled)
        if args.execute:
            if compiled.get("style") in ("monotonic", "smooth_isotonic"):
                spec = importlib.util.spec_from_file_location("sla_planner", Path(__file__).with_name("sla-planner.py"))
                planner = importlib.util.module_from_spec(spec)
                spec.loader.exec_module(planner)
                planner.run(compiled, Path(args.execute).absolute())
            else:
                bridge.run(compiled, Path(args.execute).absolute())
    except Exception as error:
        sys.stderr.write("recipe expansion failed (" + type(error).__name__ + ")\n")
        sys.exit(1)
