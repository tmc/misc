# SLA boundary planners

`scripts/sla-planner.py` runs the actual AIPerf monotonic and smooth-isotonic
planners against independently repeated `llm-campaign` evaluations. This is an
optional Python adapter: it requires AIPerf 0.13.0 at source revision
`f10eb50a1776775d80d7f13396b0f668f14e2c03`, NumPy 2.4.6 and SciPy 1.17.1.
It verifies all 1,025 installed Python source files before using the planner.
It never invokes AIPerf's inference client.

The smooth planner retains upstream exponential bracketing, internal probes,
PAVA and PCHIP fitting, sigma-normalized constraint margins, replicate
bootstrap boundaries, cliff recovery and termination reasons. The monotonic
planner retains upstream bracketing and binary search. The adapter fixes the
legacy NumPy RNG seed to zero for replay. An upstream degenerate bootstrap can
produce nonfinite bounds; the exported boundary then has `boundary_ci: null`
and a reason. That result supplies no confidence interval qualification.

Each proposal fixes one authored concurrency environment parameter and runs a
campaign with at least two independent successful converged executions.
The adapter checks campaign summary hashes and replays every prior ask/tell
before continuation. A failed or interrupted probe is terminal and cannot be
retried in the same output directory. Planner creation, ask and tell execute in owned fork workers with full framed
IPC deadlines and bounded responses. The private worker returns parent-created
model state and NumPy RNG state; no checkpoint pickle is read. Fit, bootstrap
and stalled partial responses cannot hold the parent past the operation deadline.
Owned worker groups are killed and reaped after every call. Campaign process
deadlines and owned process-group cleanup are supplied by the campaign driver.
Initial source checks and replay are charged to the same total wall budget; the
remaining budget is checked again after ask before starting an evaluation. Output directories are private and
locked against concurrent writers. Source, config, executable and script changes
reject resume. Repeated probes requested by the upstream replicate phase are
intentional new evaluations, each with separate raw artifacts.

Metric mappings explicitly name the authored k6 metric, each retained statistic,
and its unit. For example, an authored millisecond trend's `p(95)` can map to
`time_to_first_token.p95`; token throughput must be supplied in tokens per
second. The adapter does not infer units or convert an unrelated trend into
an inference metric. Such mappings are a workload contract, requiring review
against the authored script and retained execution output.

Upstream planner feasibility means **at least one successful trial passes every
SLA filter**. The adapter reports this rule explicitly. It does not substitute
the stricter xk6 campaign rule that every run must satisfy configured constraints;
configure planner filters separately and leave campaign constraints empty.
Strict `<` and `>` filters remain strict at equality. Failed runs and unavailable
metrics cannot qualify an observation.

Run an authored configuration with:

```sh
python scripts/sla-planner.py --config planner.json --output results/boundary
```

This adapter closes planner mechanics for the pinned implementation. CPU fixture
results do not establish a physical capacity limit, statistically qualified
performance, provider equivalence or universal AIPerf parity.
