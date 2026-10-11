import copy
import importlib.util
import math
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location("recipes", Path(__file__).with_name("search-recipes.py"))
recipes = importlib.util.module_from_spec(spec)
spec.loader.exec_module(recipes)

class RecipeTest(unittest.TestCase):
    def config(self):
        return {"recipe": "max-throughput-ttft-sla", "streaming": True,
                "concurrency": {"name": "VUS", "low": 1, "high": 1000},
                "throughput_metric": {"name": "llm_completion_tokens", "stat": "rate"},
                "latency_metric": {"name": "llm_dispatch_ttft", "stat": "p(95)"}, "sla_ms": 200,
                "search": {"schema_version": 1, "campaign_binary": "/missing", "sampler": "tpe", "seed": 3,
                           "max_trials": 30, "trial_timeout_seconds": 30, "max_duration_seconds": 120,
                           "proposal_timeout_seconds": 90, "outcome_constraints": [],
                           "campaign": {"binary": "/missing", "script": "/missing", "seed": 1,
                                        "max_attempts": 2, "min_runs": 2, "max_runs": 2, "retries": 0,
                                        "convergence": "range", "tolerance": 0, "timeout": "10s", "max_duration": "25s"}}}

    def test_named_expansion_and_strict_boundary(self):
        config = self.config()
        result = recipes.expand(config)
        constraint = result["campaign"]["constraints"][0]
        self.assertEqual(constraint["bound"], math.nextafter(200., -math.inf))
        self.assertGreater(200 - constraint["bound"], 0)
        self.assertLessEqual(math.nextafter(200., -math.inf) - constraint["bound"], 0)
        self.assertEqual(result["domains"], [{"name": "VUS", "low": 1, "high": 1000, "type": "int", "log": True}])
        self.assertEqual(result["objectives"][0]["metric"], config["throughput_metric"])
        self.assertNotIn("constraints", config["search"]["campaign"])
        config["recipe"] = "max-throughput-itl-sla"
        config["latency_metric"] = {"name": "llm_full_response_itl", "stat": "p(95)"}
        recipes.expand(config)

    def test_concurrency_optuna_and_goodput_contracts(self):
        base = self.config()
        concurrency = {"recipe": "max-concurrency-under-sla", "search_style": "optuna", "streaming": True,
                       "concurrency": base["concurrency"], "concurrency_metric": {"name": "concurrency", "stat": "avg"},
                       "sla_filters": [{"metric": base["latency_metric"], "op": "<", "bound": 200, "streaming_only": True}],
                       "search": base["search"]}
        result = recipes.expand(concurrency)
        self.assertEqual(result["objective_parameter"], "VUS")
        self.assertEqual(result["campaign"]["constraints"][0]["bound"], math.nextafter(200., -math.inf))
        concurrency["search_style"] = "smooth_isotonic"
        with self.assertRaises(ValueError):
            recipes.expand(concurrency)
        boundary = {"recipe": "max-concurrency-under-sla", "streaming": True,
                    "concurrency": base["concurrency"],
                    "sla_filters": [{"metric": {"name": "time_to_first_token", "stat": "p95"}, "op": "<", "bound": 200, "streaming_only": True}],
                    "search": {"schema_version": 1, "trial_timeout_seconds": 30, "max_duration_seconds": 120,
                               "campaign_binary": "/missing", "campaign": base["search"]["campaign"],
                               "metrics": {"time_to_first_token": {"name": "llm_dispatch_ttft", "unit": "ms", "stats": {"p95": "p(95)"}},
                                           "output_token_throughput": {"name": "llm_completion_tokens", "unit": "tokens/sec", "stats": {"avg": "rate"}}}}}
        result = recipes.expand(boundary)
        self.assertEqual(result["style"], "smooth_isotonic")
        self.assertEqual(result["max_iterations"], 30)
        self.assertEqual(result["sla_replicates"], 0)
        self.assertEqual(result["filters"][0]["op"], "lt")
        boundary["search_style"] = "monotonic"
        self.assertEqual(recipes.expand(boundary)["max_iterations"], 20)
        goodput = {"recipe": "max-goodput-under-slo", "streaming": True, "concurrency": base["concurrency"],
                   "slos": {"max_ttft_ms": .5, "max_tpot_ms": 1.25, "max_e2e_ms": 10},
                   "attainment_fraction": .95, "goodput_metric": {"name": "llm_content_span_goodput", "stat": "avg"},
                   "attainment_metric": {"name": "llm_slo_compliance", "stat": "value"}, "search": base["search"]}
        result = recipes.expand(goodput)
        self.assertEqual(result["applied_slo"], goodput["slos"])
        self.assertEqual(result["domains"][1]["values"], ["0.5"])
        self.assertEqual(result["campaign"]["constraints"][0]["direction"], "ge")
        self.assertEqual(result["campaign"]["constraints"][0]["bound"], .95)
        for fraction in (0, 1.01, True, float("nan")):
            invalid = copy.deepcopy(goodput)
            invalid["attainment_fraction"] = fraction
            with self.assertRaises(ValueError):
                recipes.expand(invalid)
        invalid = copy.deepcopy(goodput)
        invalid["slos"].pop("max_tpot_ms")
        with self.assertRaises(ValueError):
            recipes.expand(invalid)

    def test_refuse_collisions_and_invalid_modes(self):
        base = self.config()
        for key, value in [("streaming", False), ("sla_ms", 0), ("sla_ms", float("inf")), ("recipe", "arbitrary")]:
            config = copy.deepcopy(base)
            config[key] = value
            with self.assertRaises(ValueError):
                recipes.expand(config)
        for key in ("domains", "objectives"):
            config = copy.deepcopy(base)
            config["search"][key] = []
            with self.assertRaises(ValueError):
                recipes.expand(config)
        config = copy.deepcopy(base)
        config["search"]["campaign"]["constraints"] = [{"anything": 1}]
        with self.assertRaises(ValueError):
            recipes.expand(config)

if __name__ == "__main__":
    unittest.main()
