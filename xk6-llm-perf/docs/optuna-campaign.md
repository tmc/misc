# Optuna TPE campaigns

`scripts/optuna-campaign.py` uses actual Optuna 4.9.0 TPE proposals to evaluate
finite parameter cells with `llm-campaign`. Each cell uses the existing repeated
k6-run convergence and constraint contract. Requests are not treated as
independent experimental replicates.

Build the campaign CLI and install the upstream-lock dependency versions in a
private virtual environment. Python 3.12 or later is recommended for these pins.

```sh
work="$HOME/tmp/xk6-llm-perf/20261010-optuna"
mkdir -p "$work"
go build -o "$work/llm-campaign" ./cmd/llm-campaign
python3.12 -m venv "$work/venv"
"$work/venv/bin/pip" install -r scripts/optuna-campaign-requirements.txt
```

The configuration wraps an existing campaign configuration. `campaign_binary`,
`campaign.binary`, `campaign.script`, and every asset path must be absolute.
Declare finite string-valued dimensions as usual. The adapter replaces
`campaign.mode` with `grid`, `max_candidates` with one, and each dimension with
one proposed value for each evaluation. All other run/convergence/constraint
settings pass to the campaign engine, which validates them before benchmarking.

```json
{
  "schema_version": 1,
  "campaign_binary": "/absolute/path/llm-campaign",
  "sampler": "tpe",
  "seed": 42,
  "max_trials": 50,
  "trial_timeout_seconds": 180,
  "max_duration_seconds": 1800,
  "campaign": {
    "binary": "/absolute/path/k6",
    "script": "/absolute/path/load.js",
    "dimensions": [{"name":"RATE","values":["1","2","4","8"]}],
    "mode": "grid",
    "seed": 42,
    "objective": {"name":"llm_successes","stat":"rate"},
    "minimize": false,
    "constraints": [{"metric":{"name":"llm_dispatch_ttft","stat":"p(95)"},"direction":"le","bound":1000}],
    "max_candidates": 4,
    "max_attempts": 5,
    "min_runs": 3,
    "max_runs": 5,
    "retries": 0,
    "convergence": "range",
    "tolerance": 0.1,
    "timeout": "30s",
    "max_duration": "150s"
  }
}
```

Check objective and constraint names/units against the chosen script's k6
summary. The configuration is illustrative, not a universal performance policy.
Create the JSON as `$work/config.json`, then run:

```sh
"$work/venv/bin/python" scripts/optuna-campaign.py \
  --config "$work/config.json" --output "$work/study"
```

The same command resumes the study. An exclusive advisory lock refuses concurrent
writers. The private output directory holds `optuna.json`, each fixed-cell
configuration, driver logs, and complete `llm-campaign` attempt artifacts. Source,
configuration, driver, binaries, script, assets, and dependency versions must match
on resume. Completed trial campaign/summary hashes are checked before reuse.
Actual seeded Optuna asks and tells are replayed and every proposed parameter map
must match, reconstructing sampler state without executable pickle files.
Objective, SLA constraints, convergence, and fixed cell dimensions come from
the identity-bound requested policy. Retained manifest policy must agree; raw
summaries determine objective means, convergence, and feasibility. Updating
manifest and checkpoint metadata cannot replace that contract.

Only run-level `range`, `cv`, and `ci95` convergence are supported. Distribution
convergence is rejected. Wall time begins at driver entry and charges identity
hashing, replay, validation, proposals, checkpoint writes, and evaluation setup.
Exhausted resumes start no planner or validator. The remaining budget is checked
before replay proposals and refreshed immediately before each child launch.
These checks are cooperative between synchronous Python operations; process
cleanup retains its documented bounded grace.

The TPE sampler uses ten startup trials, the upstream planner's actual constraint
callback interface, and the declared seed. The callback supplies actual signed SLA violations from hashed run summaries:
`observed - bound` for `le`, `bound - observed` for `ge`, and the upstream
`1e6` penalty when an observation is unavailable. It retains the worst signed
violation across repeated runs for each constraint, matching the campaign's
all-runs feasibility requirement. An additional outcome component is one if any
attempt failed and zero otherwise. The derived feasibility must agree with the
campaign receipt. This preserves actual violation magnitude and direction while
using an explicitly stricter repeated-run population than a single upstream run.
Infeasible converged cells remain complete objective observations and influence
constrained sampling.
Failed, nonconverged, timed-out, interrupted, and duplicate proposals are retained
as failed trials. Duplicate proposals consume the proposal budget without running
the same cell again. The final best cell is selected only among converged feasible
cells; it is not a significance or optimality claim. The study stops at its trial,
wall-time, or finite-space budget. Search does not guarantee that a finite budget
will visit every cell.

Each campaign process owns a new process group. Deadline/cancellation sends TERM,
waits up to three seconds, then KILL and waits up to three more seconds. Interrupted
running trials become failed on resume and are not automatically retried. Each
running checkpoint reserves the full allowed trial time plus six seconds of
cleanup grace. If the coordinator dies before recording actual elapsed time,
resume conservatively charges that entire reservation against the study budget.
This can stop a resumed study early; it cannot erase interrupted trial time. The
underlying campaign's explicit `retries` policy still applies inside a cell.
Escaped process groups and mutated input files during an active child remain
caller environment obligations; pinned hashes are checked between cells.

This adapter supports one scalar objective and categorical finite dimensions.
It does not implement continuous/log distributions, GP/BoTorch acquisition,
multi-objective Pareto search, upstream recipe configuration, or upstream outcome-expression surrogates. Upstream GPSampler requires Torch for its post-startup
fit; naming a custom Go GP planner is not evidence of Optuna GP equivalence.
Those are separate remaining capability cells.

The source basis is AIPerf `f10eb50a1776775d80d7f13396b0f668f14e2c03`,
`src/aiperf/orchestrator/search_planner/_optuna_helpers.py` and `uv.lock`.
Tests run actual TPE past the startup phase, replay failed/feasible/infeasible
observations, and invoke real `llm-campaign` and registered k6 processes against
an arithmetic workload. They verify repeated cell results, constraint-qualified
selection, resume without reruns, configuration mutation, and summary corruption
refusal. The arithmetic fixture does not qualify inference performance.

```sh
K6_SMOKE_BINARY="$PWD/dist/k6" CAMPAIGN_SMOKE_BINARY="$work/llm-campaign" \
  "$work/venv/bin/python" -m unittest discover -s scripts \
  -p optuna_campaign_test.py -v
```
