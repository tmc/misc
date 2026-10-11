# Outcome constraints and named searches

`scripts/outcome-search.py` adds auxiliary outcome constraints to the
[numeric search adapter](optuna-search.md). Its configuration adds two fields:

```json
{
  "outcome_constraints": [
    {"metric":"cost", "op":"<=", "bound":10},
    {"metric":"quality", "op":">=", "bound":0.95}
  ],
  "proposal_timeout_seconds": 90
}
```

Keep all the numeric adapter's existing fields. Run with its pinned environment:

```sh
"$work/venv/bin/python" scripts/outcome-search.py \
  --config "$work/search.json" --output "$work/results"
```

The pinned AIPerf `f10eb50a` contract supports metric averages and finite
`<=`, `>=`, `==` bounds. It has no arithmetic outcome-expression language.
This adapter likewise does not evaluate authored Python expressions. Equality
requires exact equality of the measured average; it is not a tolerance.

Outcome values come from the same successful, hashed run summaries as the
objectives. Every successful run must expose a finite `avg` for the auxiliary
metric. A missing value makes the outcome unavailable and adds a positive
`1000000` violation. Receipts retain the measured count and expected successful
run count. This complete-population requirement is stricter than AIPerf's
average over only measurable successful runs. It prevents a partial series from
qualifying an entire repeated cell. An unavailable auxiliary outcome remains a
completed objective observation but cannot enter the feasible Pareto front.
Changing constraints or derived observations refuses resume.

## Throughput presets

`scripts/search-recipes.py` compiles `max-throughput-ttft-sla` and
`max-throughput-itl-sla` into an explicit outcome-search configuration. Its
input has these fields:

```json
{
  "recipe":"max-throughput-ttft-sla",
  "streaming":true,
  "concurrency":{"name":"VUS","low":1,"high":1000},
  "throughput_metric":{"name":"llm_completion_tokens","stat":"rate"},
  "latency_metric":{"name":"llm_dispatch_ttft","stat":"p(95)"},
  "sla_ms":200,
  "search":{}
}
```

Fill `search` with the outcome adapter's process, sampler, seed, budget,
`outcome_constraints`, and campaign fields. Omit `domains`, `objectives`, and
campaign constraints: the recipe supplies them and rejects collisions. The
campaign script must actually consume `VUS` and emit the authored metrics.
No metric, unit, or endpoint inference occurs. The compiler requires declared
streaming, integer concurrency bounds within 1..1000, and a positive finite
millisecond SLA. It emits logarithmic integer sampling, maximized throughput,
and an SLA on every repeated run. That last rule is stricter than averaging
latency statistics across successful runs.

```sh
"$work/venv/bin/python" scripts/search-recipes.py \
  --config "$work/recipe.json" --output "$work/search.json"
```

The strict `<` SLA becomes `<= nextafter(threshold, -infinity)` for finite
IEEE 754 doubles, so exact equality is rejected by both acquisition feasibility
and the exported front. The user declares budgets explicitly. These two presets
cover two of the eight pinned recipes. The other adapters are listed below.

## Concurrency and goodput recipes

The same compiler supports `max-concurrency-under-sla`. Omit `search_style` to
select the actual pinned upstream `smooth_isotonic` planner, or select
`monotonic` or `optuna` explicitly. Smooth and monotonic configurations use a
[SLA planner](sweeping/sla-planners.md) template under `search`, including its
explicit metric mappings. The compiler injects `style`, bounds, concurrency
parameter, and authored `sla_filters`. Default iteration budgets are 30 for
smooth and 20 for monotonic; automatic replicate selection is zero.

Each filter has `metric: {name, stat}`, `op: <|<=|>|>=`, `bound`, and
`streaming_only: true|false`. For boundary planners, `name` and `stat` name the
canonical mapped metric and statistic, such as `time_to_first_token` and `p95`.
For Optuna, they name the authored k6 summary metric and statistic, such as
`llm_dispatch_ttft` and `p(95)`. Streaming is required when any filter declares
it. The actual boundary planner preserves upstream strict filter operators and
its documented any-trial feasibility rule.

Optuna configurations instead use the outcome-search template. Supply
`concurrency_metric: {name, stat}`: every successful summary must measure the
proposed integer concurrency exactly. A different objective value fails the
trial. This prevents an unrelated throughput metric from being reported as
maximum concurrency. Optuna constraints apply to every repeated run. The
compiler does not silently replace the default smooth algorithm with TPE.

`max-goodput-under-slo` requires declared streaming, concurrency bounds,
`slos: {max_ttft_ms, max_tpot_ms, max_e2e_ms}`, an `attainment_fraction` in
(0,1], `goodput_metric`, `attainment_metric`, and the outcome-search template.
All three SLO thresholds must be positive finite milliseconds; fractional
thresholds are supported. Use `llm_content_span_goodput.avg` for goodput and
`llm_slo_compliance.value` for all-attempts attainment. The compiler maximizes
observed goodput subject to attainment at least the declared fraction. The
default observation duration spans the first valid request start through
the last successful content. Engine setup, post-window sleeps, and drain
time do not enter that denominator. Each run must expose exactly one
qualified content window, with finite positive duration, matching good and
attempted counts, and goodput equal to good requests divided by its seconds.
Missing, incomplete, or multiple windows fail qualification.

It injects fixed `SLO_TTFT_MS`, `SLO_TPOT_MS`, and `SLO_E2E_MS` parameters.
The workload must apply these to the client's `maxTTFT`, `maxTPOT`, and
`maxE2E` configuration. Each successful summary must contain the actual applied
policy trends `llm_slo_ttft_ms`, `llm_slo_tpot_ms`, and `llm_slo_e2e_ms`, with
both min and max equal to the corresponding threshold. Missing or mismatched
policy evidence fails the cell. The policy and repeated-run count remain in
the receipt. This prevents legacy or unused settings from qualifying the new
recipe. Applied settings and measurements still require a source-reviewed
workload, not merely an arbitrary script emitting similarly named trends.

Add `--execute results-directory` to compile and invoke the matching adapter:

```sh
python scripts/search-recipes.py --config recipe.json --output compiled.json \
  --execute results/search
```

The complete pinned catalog is distributed across these adapters:

| Recipe | Adapter |
| --- | --- |
| max-throughput-ttft-sla | search-recipes / outcome-search |
| max-throughput-itl-sla | search-recipes / outcome-search |
| max-concurrency-under-sla | search-recipes / sla-planner or explicit Optuna |
| max-goodput-under-slo | search-recipes / outcome-search |
| concurrency-ramp | [grid-recipes](grid-recipes.md) |
| prefill-ttft-curve | grid-recipes |
| decode-itl-curve | grid-recipes |
| pareto-sweep | grid-recipes |

Grid plans and post-processing use retained independent campaign artifacts.
They do not create Kubernetes resources or supply inference workloads.

## BoTorch proposals

`sampler: "botorch"` selects actual Optuna `BoTorchSampler` with a separate
CPU proposal process. The additional environment pins are in
`scripts/botorch-search-requirements.txt`; Torch must be 2.13.0 or the official
2.13.0+cpu build. The release and local CPU tag remain in the identity. Install
Torch from the platform's official CPU wheel source, then install this file
without the integration package's `[botorch]` extra: that extra still declares
an old BoTorch upper bound incompatible with this explicit modern acquisition.

The worker uses qLogNEI for one objective and qLogNEHVI for multiple objectives,
Matern-5/2 ARD with the upstream Hvarfner dimension-scaled LogNormal prior,
standardized outputs, and explicit constraint GP outputs. The multiobjective
acquisition projects only objective outputs and derives the maximization-space
reference point from the worst observed objective with 5% slack. This corrects
the pinned upstream helper's apparent mismatch between constraint indices and
objective-only models; it is a scoped adapter, not identical upstream behavior.

Each fit has an owned process-group deadline of at most 180 seconds and uses
one CPU thread. Inputs are bounded to 1 MiB, 512 observations and 128 encoded
input dimensions. Returned candidates must be finite and within bounds. The
actual worker, numeric driver, outcome driver, environment, workload and raw
summaries are pinned for replay. No GPU jobs are needed. Algorithm and fixture
qualification do not establish global optimality, a performance advantage, or
universal AIPerf parity.
