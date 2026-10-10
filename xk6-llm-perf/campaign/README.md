# k6 campaigns

Run from the extension directory:

```sh
ENDPOINT_URL=http://localhost:8000 MODEL=fixture \
  go run ./cmd/llm-campaign -config examples/campaign/grid.json \
  -output outputs/campaign
```

The command executes the supplied k6 binary directly. It writes private
`campaign.json` checkpoints and a directory per stable parameter ID and numbered
attempt, including `attempt.json`, `summary.json` when produced, and `process.log`.
Logs retain at most 16 MiB per attempt; the receipt reports truncation. Summary
input is limited to 16 MiB. Campaigns allow at most 10,000 cells and 10,000 total
attempts, 12 dimensions, 2,048 bytes of parameters per cell, and seven days of
active execution. Processes run sequentially, with a timeout and total budget.
Process environment is inherited; credentials must stay in environment, not
parameter dimensions or configuration. Scripts and provider errors can print
sensitive data to logs: retain artifacts in a private directory.

The checkpoint identity includes configuration, the primary script, explicitly
listed assets, and binary SHA-256 hashes. Declare every imported module, dataset,
and tokenizer asset in `assets`; the runner does not discover transitive assets.
Environment-controlled endpoint/model/seed/phase settings should be declared
as dimensions when safe to record. Unrecorded environment changes are outside
the reproducibility claim. Every summary is hashed and checked when resuming; completed checkpoint
attempts must match their retained attempt receipts. Binary/script/assets are
rehash-checked around each launch, so mid-campaign drift retains a failed
attempt and stops subsequent work.
A lock refuses concurrent writers. After a killed runner, inspect the checkpoint
and ensure the process has ended before manually removing its stale `.lock`.
Interrupted attempts remain failures; their directories are never overwritten.

`grid` follows declared dimension/value order. `halton` maps a seeded Halton
sequence to the finite grid and uses a deterministic grid fallback for collisions.
`adaptive` explores immediate neighbors of the best feasible converged cell,
then uses Halton exploration. `bayesian` fits a bounded RBF Gaussian process on ordinal grid
coordinates and chooses expected improvement discounted by a local feasibility
estimate. The latest 64 observations bound fitting cost; declared value order
sets the geometry. The fixed length scale/noise and heuristic feasibility model
are recorded algorithm policy, not equivalence with upstream Optuna. `scale` requires a strictly increasing positive numeric
one-dimensional grid and SLA constraints. It doubles control values while cells
pass, then bisects the passing/failing bracket. It assumes monotone feasibility;
retain the full grid when that assumption is unjustified. A process failure or
unconverged cell stops scale assessment. Outer-run scale does not change load
within one k6 run.

Every successful execution supplies the configured k6 metric/statistic. Both
legacy flattened `--summary-export` metrics and nested `values` objects are
supported; absent/null metrics fail the attempt. Nonzero exits are failures even
if summary metrics exist. Failed attempts remain visible and permanently make
the cell infeasible, including when explicitly allowed retries later succeed.
`best_id` considers only cells that are feasible, converged, and have not exhausted
an unsuccessful run budget. It uses the mean of all successful objective values,
not the most favorable run. SLA constraints must pass on every successful run.
A metric spelling mistake cannot converge.

Convergence requires `min_runs >= 2`. `cv` compares sample standard deviation /
absolute mean; `ci95` compares full normal-approximation 95% interval width /
absolute mean; `range` compares observed range / absolute mean. The normal interval
is not an exact small-sample Student interval or a distribution-equivalence test.
All-zero values converge only when every value is zero. `distribution` requires `distribution_file`, a relative path emitted by the
script containing a JSON array of 2–10,000 finite observations per run. It bounds
every pairwise empirical Kolmogorov–Smirnov distance by the tolerance across
at most 32 repetitions. This is an effect-size rule, not a significance test.
Artifacts are hashed and checked during resume; absence fails the attempt.
`max_runs`, explicit
`retries`, candidate/attempt/time bounds, cancellation, and exhausted space give
explicit stopping rules. Resume preserves budgets and selection; it cannot turn
a failed campaign into a favorable-only result.

## Single-run scaling

`examples/campaign/window-scale.js` supplies a bounded single-run controller for
one VU's synchronous request rate. It assesses whole-contained responses over
explicit windows, requires minimum completions, increases rate up to a ceiling,
and sustains the last passing rate for a bounded number of windows. It records
intended arrival lag and skips expired arrivals. Every window emits a JSON
receipt into the retained k6 log; exported counters and trends exercise native
k6 thresholds. Run it with the signed extension binary:

```sh
ENDPOINT_URL=http://localhost:8000 ./dist/k6 run \
  -e SCALE_MAX_RATE=32 -e SCALE_WINDOWS=8 \
  examples/campaign/window-scale.js
```

Its control is offered rate, bounded by a single synchronous VU; realized rate
and exclusions appear separately. It does not implement AIPerf's live global
concurrency/prefill controller. Global concurrency/prefill and multi-worker control use the separate
controller below.
Live provider and matched AIPerf comparison require independent receipts.

## Validation

`go test ./campaign ./cmd/llm-campaign` checks analytic planner/convergence
controls, failures, missing/null metrics, interruption/resume, asset drift,
timeouts, concurrent writer refusal, and CLI txtar contracts.
`K6_SMOKE_BINARY="$PWD/dist/k6" go test ./campaign -run TestBuiltK6 -v`
executes two cells twice against an in-process HTTP fixture, checks extension
metrics/thresholds and selects the expected cell from actual k6 summary exports.
These are host receipts; they do not establish live-provider or AIPerf parity.

## Global and multi-worker single-run scaling

```sh
ENDPOINT_URL=http://localhost:8000 go run ./cmd/llm-campaign \
  -scale -config examples/campaign/global-scale.json -output outputs/global-scale
```

`RunScale` starts bounded native k6 subprocesses on the same host and a loopback
assessment server. Every worker has a stable numeric ID and an explicitly shared
extension control group. Existing VUs call `setControlLimits` against that root;
request/prefill holders drain when limits decrease, while queued new admissions
wait. Request and prefill caps are partitioned by balanced integer worker quotas;
zero-quota workers idle and still participate in window heartbeats. Quota sums
never exceed the declared global limits. Each worker starts more VUs than its
peak assigned request quota where possible, so admission limits, rather than
only native VU count, bound the tested concurrency.

The selected variable is `request` or `prefill`. The other global cap stays fixed.
The controller doubles the selected cap between assessment windows, requires
minimum successful completions and a nearest-rank p95 full-response-latency SLA over successful contained requests,
checks a failure-ratio bound, and sustains the last passing cap for a bounded
number of windows. Every failed assessment stays in `scale.json`. Missing
workers, duplicate identities, or exhausted retention terminate assessment;
failed sustain and a window budget have distinct stopping labels. Controller
windows include only whole-contained dispatched requests and declare same-host
Unix epoch milliseconds. Crossing requests count as exclusions; sample retention
is bounded globally and incomplete records cannot pass an assessment.

Each window records its caps, participating workers, exclusions/drops, sanitized
request IDs, dispatch/finish times, response outcomes, and root-group holder
snapshots. Native process logs and summary hashes are retained per worker.
`ReadScale` reproduces assessments from samples and refuses contradictory counts
or incomplete running-window receipts. The scale output directory must be new;
scale does not restart a partially executed live epoch. Sweep checkpoints remain
the resumable mechanism for repeated candidate runs.

k6 v2.3.0's `api/v1/status_routes.go` rejects live VU/VUsMax changes. This adapter
uses the supported extension admission control rather than pretending REST VU
patches work. Multi-process same-host clocks/quotas are exercised by fixtures;
remote clocks, Kubernetes placement and actual provider deployments require
separate deployment receipts. No arbitrary remote clock is assumed equivalent
to the controller host. The controller binds only loopback and caps request
payloads, headers, read/write timeouts, windows, workers, retained samples and
process shutdown. Asset hashes are checked between assessment windows.

The fresh signed-binary gate `go test ./campaign -run TestBuiltK6 -v` also exercises
both global request and prefill control, two worker processes with multiple VUs,
observed HTTP overlap bounded by caps, successful sustain, replay, and a failed
HTTP assessment that cannot be promoted to a passing boundary.
