# Serving qualification

Use this lane to compare complete HTTP services. The comparison unit is an
independent server process run, paired with another independently started arm.
The lane adds five contracts:

| Contract | Evidence | Reference |
| --- | --- | --- |
| Effective manifest | Asset digests, configured/effective settings, observed evidence | [Manifest](../serving-manifests.md) |
| Expected work | Model, finish reason, reported output bounds, exact usage and optional digests | [Work acceptance](../work-acceptance.md) |
| Supervised lifecycle | Executable identity, readiness, warmup, workload, drain and process cleanup | [Lifecycle](lifecycle.md) |
| Client capacity | Offered arrivals, lag/drops, sampled client resources and sample delivery | [Capacity](capacity.md) |
| Paired analysis | Preregistered order, retained independent pairs, verified gates and uncertainty | [Paired analysis](paired.md) |

Protocol success, SLO goodput and accepted-work goodput are separate populations.
An absent expectation does not qualify expected work. Missing provider token IDs
remain unavailable. Pipeline counters belong to one VU; collect each contributing
VU's receipt rather than treating one VU as the complete client process.

Read the [confound ledger](CONFOUNDS.md) before collecting results. Local fixture
controls validate the harness, while real-model, hardware and performance claims
require receipts for that exact workload, candidate and environment.

Build the command with `make build-serving`. Run a pinned lifecycle configuration
with `dist/llm-serving lifecycle -config lifecycle.json`. Evaluate supplied
capacity evidence with `dist/llm-serving capacity -input capacity.json`; supplied
evidence remains diagnostic. On Linux, `capacity -pid PID -samples 100` captures
raw process resource observations without claiming a complete capacity gate.

On Linux, retain a fixed-response capacity control with
`python3 scripts/capacity-probe.py --k6 /path/to/k6 --output /task/new-capacity`.
Then exercise the complete lane with
`python3 scripts/serving-qualification-control.py --k6 /path/to/k6 --llm-serving /path/to/llm-serving --capacity /task/new-capacity --output /task/new-comparison`.
The second control starts four fresh fixture servers and four registered k6
workloads. It verifies the captured manifests, accepted work, sampled capacity
and paired receipts. It qualifies this fixture protocol only; it makes no
real-model performance claim.
