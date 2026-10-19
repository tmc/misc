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
