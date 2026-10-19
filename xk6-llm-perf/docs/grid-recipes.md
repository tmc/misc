# Retained grid recipes

`scripts/grid-recipes.py` plans and post-processes four grid recipes from
AIPerf revision `f10eb50a1776775d80d7f13396b0f668f14e2c03`.
It does not launch benchmarks. Run each planned cell through `llm-campaign`
with matching environment dimensions, retain its manifest and summaries,
then supply the cell directories. A workload must actually apply those
settings; parameter names alone do not prove applied engine configuration.

```sh
python scripts/grid-recipes.py recipe.json > plan.json
python scripts/grid-recipes.py recipe.json --cells cells.json > result.json
```

A concurrency ramp configuration is:

```json
{"recipe":"concurrency-ramp","streaming":false,"concurrency":[1,1000,8],"threshold":0.2}
```

Each axis uses `[minimum, maximum, log-spaced steps]`; rounded duplicates
collapse. Other configurations are:

```json
{"recipe":"prefill-ttft-curve","streaming":true,"isl":[256,32768,8]}
```

```json
{"recipe":"decode-itl-curve","streaming":true,"concurrency":[1,200,6],"osl":[64,1024,4]}
```

```json
{"recipe":"pareto-sweep","streaming":true,"pairs":[[128,128],[512,256]],"concurrency":[1,4,16]}
```

A retained cell entry is:

```json
[{"parameters":{"CONCURRENCY":1},"directory":"/absolute/path/to/campaign-cell"}]
```

The list must contain the full plan except the decode surface may omit
cells. Every cell must be a complete feasible single-candidate campaign.
Every successful attempt needs a matching manifest summary SHA256. Failed,
missing, null, negative and non-finite observations reject. Output retains
per-cell manifest and summary hashes. The manifest is not an externally
signed attestation; freeze its output hashes separately when qualifying a run.

| Recipe | Measurement and post-process |
| --- | --- |
| concurrency-ramp | `llm_request_latency` `p(99)` in ms; lowest concurrency baseline, first strictly greater than baseline × 1.2. Override `metric`, `stat`, `threshold` explicitly. |
| prefill-ttft-curve | `llm_ttft` `avg` in ms, concurrency fixed at 1. Linear least squares; try quadratic below R² 0.85 with at least three points, retain better fit; constant measurements remain below floor. |
| decode-itl-curve | `llm_inter_token_latency` `avg` in ms. Measured concurrency × OSL grid; missing observed-axis cells remain null. No interpolation. |
| pareto-sweep | `llm_ttft` `p(95)` in ms minimized and `llm_completion_tokens` `rate` in token/s maximized; global non-dominated frontier. ISL/OSL pairs stay paired, crossed only with concurrency. Equal objective ties remain. |

Repeated-run statistics average the selected statistic across successful
attempts, matching upstream per-combination mean semantics. They do not pool
requests across runs. The completion counter rate uses k6's measured run
window; this is an explicit timing-window mapping, not proof that AIPerf's
throughput window is identical. Streaming policies are declarations checked
before planning; retained summaries cannot prove provider wire semantics.

Prefill fitting requires NumPy **2.4.6**, available through
`scripts/optuna-campaign-requirements.txt`. Other recipes use Python's
standard library. Plans contain 2–4096 distinct cells, positive integer axes
at most 1,000,000, and at most 64 log steps. This bridge has stricter failed-cell
handling than upstream's quarantine/sentinel paths and does not implement
upstream artifact schemas, automatic plots, dataset generation or engine
application.

Post-processing additionally requires a caller-pinned `campaign` policy in
`recipe.json`. Planning without `--cells` may omit it. Use the existing
`campaign.Config` fields, with `mode: "grid"`, `max_candidates: 1` and empty
base `dimensions`; replace dimensions with each planned cell when executing
`llm-campaign`. For example, add this field to the recipe object:

```json
"campaign": {
  "binary": "/absolute/path/to/k6",
  "script": "/absolute/path/to/workload.js",
  "dimensions": [],
  "mode": "grid",
  "seed": 5,
  "objective": {"name": "llm_request_latency", "stat": "p(99)"},
  "minimize": true,
  "constraints": [],
  "max_candidates": 1,
  "max_attempts": 2,
  "min_runs": 2,
  "max_runs": 2,
  "retries": 0,
  "convergence": "range",
  "tolerance": 0,
  "timeout": "5s",
  "max_duration": "10s"
}
```

Freeze this policy independently before reading retained manifests. The verifier
derives the expected per-cell configuration from it and checks the recorded
objective, constraints, run/retry bounds, convergence and other policy fields.
It does not obtain its expected policy from the manifest it is verifying.
Only `range`, `cv` and `ci95` repeated-run convergence are supported here.
Optional `assets` must be unique absolute paths. Durations must be positive
Go-style durations of at most seven days. Binary/script paths must be absolute;
policy identity alone does not prove which executable actually ran.

A valid version-1 manifest needs the exact candidate ID derived from its
parameters and numbered `<candidate-id>/attempt-0001` summary directories.
The verifier recomputes objective values, constraint violations, feasibility
and convergence from raw summaries, checks all summary hashes, and refuses
failed attempts even if later attempts converged. Output includes
`campaign_policy_sha256` alongside retained artifact hashes. Preserve the
caller configuration and its hash separately; an attacker able to replace both
caller policy and receipts is outside this local consistency check.
