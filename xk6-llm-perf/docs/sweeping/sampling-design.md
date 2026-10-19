# Scalar sampling designs and multi-tier SLO search

Reference: AIPerf `f10eb50a1776775d80d7f13396b0f668f14e2c03`.

`scripts/sweep-design.py` expands ordered scalar environment proposals and runs
each through the existing `llm-campaign` worker. It supports these explicit
modes, without treating Halton as Sobol or Latin hypercube:

- `zip`: equal-length `parameters` lists paired element by element.
- `scenarios`: unique named `runs`, each with scalar `parameters` overriding
  the scalar `base` map. This is the flat scalar subset of upstream scenario
  deep merging; nested BenchmarkConfig overrides are rejected.
- `sobol`: the pinned SciPy Sobol engine, default scrambled, with an explicit
  seed and sample count. Powers of two preserve the engine's balance property.
- `latin_hypercube`: the pinned SciPy LatinHypercube engine with explicit seed
  and optional `optimization` (`null`, `random-cd`, or `lloyd`); default
  `random-cd`, matching the upstream configuration model.

QMC dimensions have an environment `name` and either scalar `choices`, or
numeric `lo`/`hi` plus optional `scale` (`linear`/`log`) and `kind`
(`real`/`int`). Mapping follows upstream floor-based categorical selection,
linear/log scaling and ties-to-even integer rounding clamped to integer bounds.
The adapter requires NumPy 2.4.6 and SciPy 1.17.1. It bounds designs to 4096
proposals and 12 axes and accepts environment names rather than upstream dotted
BenchmarkConfig paths. It does not reproduce arbitrary nested configuration,
upstream cooldown scheduling, iteration-order policies or per-variation seeds.

For a plan without worker execution, save this JSON and run:

```json
{"type":"zip","parameters":{"ISL":[256,1024],"OSL":[32,64]}}
```

```sh
python3 scripts/sweep-design.py --config design.json --output proposals.json --plan-only
```

For execution, wrap the design in an object with `schema_version: 1`, `design`,
`campaign_binary` (absolute path), `campaign` (the existing campaign worker
configuration), `trial_timeout_seconds` and `max_duration_seconds`. Pass the
wrapper to the same command without `--plan-only`, choosing a private output
directory. Every variation has an index, label, parameters and proposal hash.
Equal tuples caused by discretization retain separate indexed worker trials.
The checkpoint binds runtime, driver, worker/script/assets, design and policy.
Resume replays proposals and verifies raw worker summaries. Failed or interrupted
trials remain terminal; they are not silently rerun or removed.

`scripts/sla-planner.py` additionally accepts `style: "multi_tier"`,
`sla_replicates: 0`, and `tiers`, a list of 2 to 16 unique `label`/`filters`
objects using the existing filter schema. The remaining planner configuration
and explicit metric mappings stay required; global filters compose with each
tier. The adapter invokes the actual pinned `MultiTierPlanner`, with one integer
concurrency axis. Existing repeated process results feed the upstream any-trial
feasibility rule; this is not a statistical equivalence test.

Multi-tier search shares each concurrency observation across SLO tiers, detects
tier ordering, establishes brackets and bisects the widest unresolved bracket.
It does not promote candidates through fidelity tiers or change infrastructure
budgets. The upstream planner owns warmup, ordering inference, nonmonotonic-bound
repair and convergence. Resume reconstructs its state by exact ask/tell replay
and verifies retained worker artifacts. Runtime source/dependency hashes must
match the existing planner pins before dispatch.

CPU reference controls and historical worker interface tests establish software
behavior only. Real model capacity, optimal settings and performance equivalence
require separately approved workload plans, resources and live evidence.

## Typed upstream fixed-trial plans

The scalar modes above retain their existing contracts. A separate
`--aiperf-plan-only` mode accepts the actual pinned AIPerf envelope schema:

```json
{"random_seed":7,"benchmark":{"model":"local","endpoint":{"type":"chat","url":"http://unused.invalid"},"dataset":{"type":"synthetic","entries":1},"phases":[{"name":"profiling","type":"concurrency","concurrency":1,"requests":1}]},"sweep":{"type":"zip","parameters":{"concurrency":[1,2]},"cooldown_seconds":0.25},"multi_run":{"num_runs":2,"cooldown_seconds":0.5,"disable_warmup_after_first":false}}
```

```sh
/path/to/pinned/python scripts/sweep-design.py --config envelope.json \
  --output typed-plan.json --aiperf-plan-only
```

This mode verifies the installed AIPerf 0.13.0 package, all 1,025 Python source
files and plugin defaults against the pinned reference before importing its
loader. It calls `load_config_from_mapping` with environment substitution
disabled, then `build_benchmark_plan`. The actual `MultiRunOrchestrator` executes
against a capture-only executor: its sleeps are recorded and no benchmark runs.
Output retains full normalized benchmark bodies, normalized authored fields,
variation metadata, variation seeds and per-event trial/variation coordinates,
actual run seed and cooldown intervals. It also retains the Python/package and
source-file identities. Only trusted authored configs/templates are supported;
Jinja/config expansion is upstream code, not an execution sandbox.

Normalization includes singular dataset promotion, named dataset scenario
merges and per-variation Jinja rendering. Upstream rejects ambiguous singular
scenario datasets against multiple base datasets. `repeated` is trials outer,
variations inner; `independent` is variations outer, trials inner. Inter-variation
cooldown comes from `sweep.cooldown_seconds`, and inter-trial cooldown from
`multi_run.cooldown_seconds`. No delay is added after the final event.
The actual upstream seed resolver supplies base-plus-index seeds unless
`same_seed` is true; `vary_seed_per_trial` uses its label/trial SHA derivation.
Without an explicit seed, upstream can generate one when `set_consistent_seed`
is true. With that option false, absent seeds remain null. Zero stays distinct.

Typed mode currently accepts explicit `grid`, `zip` and `scenarios` sweeps and
fixed trials only. Other typed sweep variants, convergence, postprocessing,
SLA filters, plots and unknown envelope/sweep options are refused. Existing
scalar Sobol/LHS and separate search modes remain available. Limits are 12
scalar axes, 256 values per axis, 256 variations, 1,024 total events, 60 seconds
per cooldown and 1 MiB input/generated-plan JSON. Upstream supports additional
configuration forms; this mode does not claim all of them.

## Explicit script realization

`--aiperf-run` executes a narrower contract. It requires a wrapper with exactly:

- `schema_version: 1` and `envelope` as above;
- `environment`, mapping 1–12 environment names to normalized authored benchmark
  leaf paths, such as `{"MODEL":"models.items.0.name"}`;
- absolute `binary` and `script` paths, plus optional absolute `assets`;
- `seed_environment`, a distinct allowed environment name;
- `trial_timeout_seconds` (1–60) and `max_duration_seconds` (1–600).

Every normalized authored benchmark leaf must map exactly once, including base
settings and fields inferred by normalization. Lists use zero-based decimal
path segments. Authored empty collections, dotted dictionary keys and null leaves cannot
become scalar parameters;
unsupported mappings fail before dispatch. Inspect `authored_benchmark` in a
plan to construct the exact mapping. Execution additionally requires explicit
integer `random_seed` and `disable_warmup_after_first: false`; it does not
silently translate upstream warmup removal into a script setting.

The binary is invoked once per ordered event as `binary run --log-format raw --summary-export
<cell>/summary.json --env NAME=value ... script`. The seed is passed through the
chosen seed environment name. This directly runs a fixed trial and leaves the
existing campaign worker's convergence semantics unchanged. The script must
read those environment values and log exactly one receipt:

```js
console.log('SWEEP_APPLIED '+JSON.stringify({parameters, seed}));
```

`parameters` must contain the exact string values actually read by the script,
and `seed` the exact decimal seed string actually read. This avoids JavaScript
number rounding for upstream SHA-derived seeds. Missing, duplicate or mismatched
receipts, trailing non-whitespace or a second JSON document fail the cell, even when the process exits zero. This guard verifies
script-reported consumption; it does not prove engine application or prevent a
trusted script from lying. A valid summary JSON file is also required.
Use ordinary script checks/thresholds to validate the workload itself.

Each attempted cell retains its exact argv, normalized event, exit/error,
worker log and summary hashes in `typed-design.json`. Resume revalidates source,
plan, script/binary/assets and the exact required driver.log/summary.json artifact
keys and hashes without rerunning completed
cells. A failed or interrupted cell remains terminal. Elapsed time and
cooldowns count toward the duration budget. The CLI bounds CPU to 20 seconds,
address space to 2 GiB and files to 16 MiB; typed planning has a 30-second alarm,
and execution a 630-second alarm. Child processes inherit these OS limits. SIGTERM, SIGHUP and the wall-clock SIGALRM
raise an interruption error so owned child-process cleanup and failure checkpoint
saving run before the CLI exits.
Logs and configs can contain authored data and must be protected accordingly.

Typed normalization and script realization are separate evidence classes.
Mapped execution does not implement all BenchmarkConfig defaults in k6,
translate arbitrary datasets into requests or establish live AIPerf, capacity
or performance equivalence. The script implements the declared mapped settings;
engine effects require separate named runtime evidence.
