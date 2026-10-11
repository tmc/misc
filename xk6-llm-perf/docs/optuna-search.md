# Numeric and Pareto Optuna search

`scripts/optuna-search.py` extends the separate finite categorical campaign
adapter with actual Optuna numeric distributions and multiple objectives. It
uses the same pinned runtime and existing `llm-campaign` execution engine.
Install [the campaign environment](optuna-campaign.md) first.

The configuration uses the campaign adapter's process and budget fields,
replaces categorical dimensions with typed `domains`, and declares objective
metrics and their directions explicitly:

```json
{
  "schema_version": 1,
  "campaign_binary": "/absolute/path/llm-campaign",
  "sampler": "tpe",
  "seed": 42,
  "max_trials": 30,
  "trial_timeout_seconds": 180,
  "max_duration_seconds": 1800,
  "domains": [
    {"name":"RATE","type":"float","low":1,"high":20,"log":true},
    {"name":"VUS","type":"int","low":1,"high":16},
    {"name":"MODEL","type":"categorical","values":["model-a","model-b"]}
  ],
  "objectives": [
    {"metric":{"name":"llm_successes","stat":"rate"},"direction":"maximize"},
    {"metric":{"name":"llm_dispatch_ttft","stat":"avg"},"direction":"minimize"}
  ],
  "campaign": {
    "binary":"/absolute/path/k6",
    "script":"/absolute/path/load.js",
    "seed":42,
    "constraints":[{"metric":{"name":"llm_dispatch_ttft","stat":"p(95)"},"direction":"le","bound":1000}],
    "max_attempts":5,
    "min_runs":3,
    "max_runs":5,
    "retries":0,
    "convergence":"range",
    "tolerance":0.1,
    "timeout":"30s",
    "max_duration":"150s"
  }
}
```

The load script must consume the declared environment variables and preserve
comparable work. Check units and metric names against its actual k6 summary.
The example does not define a universal workload or performance policy.

```sh
"$work/venv/bin/python" scripts/optuna-search.py \
  --config "$work/search.json" --output "$work/search"
```

Float domains support continuous, logarithmic, or explicitly stepped sampling.
Integer domains support bounded integer steps or positive logarithmic sampling
with step one. Numeric bounds must be finite; integer bounds stay within exact
JSON integers. Steps must divide the declared interval exactly using the same
`Decimal(str(value))` interpretation as Optuna, so silent upper-bound adjustment
is refused. Floats serialize with 17 significant digits for exact binary-float
round trips into the campaign environment. Both typed suggestions and their
string parameters remain in each receipt. Categorical domains retain authored
string values.

One to eight objectives are supported. Each trial uses the first objective to
collect repeated runs through `llm-campaign`; all objectives are then read from
the same retained, hashed run summaries. Every objective must independently
satisfy the declared run-level `range`, `cv`, or `ci95` stability rule. A missing
or nonconverged secondary objective fails the trial. The adapter does not collect
extra runs after the primary campaign finishes to rescue a secondary objective.
Run counts for every objective are retained. The `ci95` rule is the campaign's
normal-approximation stability check, not a small-sample significance test or
request-level independence claim.

Constrained actual TPE supports joint scalar or multiobjective proposals. Signed
SLA violations, missing-observation penalties, and failed-attempt outcome
constraints use the campaign adapter's raw-summary contract. The final
`pareto.json` includes every observed, feasible, converged, nondominated trial,
its parameter map, objective vector, objective run counts, constraints, and
source artifact hashes. Equal nondominated vectors retain both trials. The
exported front is cross-checked against actual Optuna `study.best_trials`.
It describes sampled cells; it proves neither global optimality nor a
statistically significant difference between cells.

Resume reconstructs actual seeded sampler state through proposal/tell replay,
checks typed suggestions and source/summary hashes, and refuses changed input
or derived objective vectors. The shared adapter also binds each retained
campaign's objectives, SLAs, convergence, and fixed cell to the requested policy,
and recomputes those results from raw summaries. Replay proposals and validation
stop when the cumulative wall budget is exhausted. The identity also records Python version,
platform and machine architecture. Failed, duplicate, interrupted and timed-out
trials remain retained and are not rerun. Interrupted trial reservations charge
the full allowed duration plus cleanup grace, matching the campaign adapter.
Exclusive private output, bounded process-group cleanup and source immutability
obligations are unchanged. Configuration validation, deterministic replay and proposal fitting consume the
same cumulative study time budget. GP runs in an owned CPU process with bounded
64 KiB JSON messages and deadline-controlled socket reads/writes. Deadline or
worker failure retains elapsed checkpoint time; resume verifies proposals again.

Set `sampler` to `gp` only with the upstream-lock Torch **2.13.0** installed in
the same pinned runtime. The adapter constructs actual Optuna `GPSampler` with
constraints, seed and ten startup trials; it fails before any campaign starts
if Torch is absent or its version differs. The official `2.13.0+cpu` wheel is
also accepted, with its actual version retained in the study identity. A pinned
Linux arm64 CPU runtime exercised actual post-startup GP proposals and exact
seeded replay. These controls qualify search mechanics with synthetic objectives;
they do not establish model performance or global optimality. The tool does not
install dependencies automatically.

BoTorch acquisition and upstream search-recipe/outcome-expression configuration
are not implemented by this adapter. Multiobjective TPE and Pareto export do not
imply BoTorch acquisition equivalence. The source basis remains AIPerf
`f10eb50a1776775d80d7f13396b0f668f14e2c03`, its `_optuna_helpers.py`, and
locked Optuna/Torch versions.

Tests exercise real multiobjective TPE beyond startup, exact continuous/log/int
proposal replay, finite-front equality with native Optuna, invalid numeric
ranges, exact step adjustment refusal, all-objective convergence, eager GP
dependency refusal, and real repeated k6 campaigns followed by no-rerun resume
and configuration/raw-artifact corruption refusal:

```sh
K6_SMOKE_BINARY="$PWD/dist/k6" CAMPAIGN_SMOKE_BINARY="$work/llm-campaign" \
  "$work/venv/bin/python" -m unittest discover -s scripts \
  -p optuna_search_test.py -v
```
