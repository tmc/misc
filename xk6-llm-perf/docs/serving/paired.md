# Paired serving comparisons

Preregister an even number of independent process pairs and a metric family with
`serving.PlanPairs`. The plan balances AB and BA order and fixes its seeded shuffle.
The first declared metric is the explicit primary metric; its mean paired delta
is the primary estimand. Ratios are supplementary estimands. Execute the plan sequentially. Record every attempt; a failed arm invalidates the
experiment. Do not retry, trim runs, pool requests, or replace missing pairs.

`serving.AnalyzePaired` requires matched effective-work hashes, stable per-arm
binary hashes, unique process identities, retained artifact digests, successful
runs, accepted work and qualified client capacity. These fields are attestations:
the harness must verify the referenced artifacts and gates before constructing a
run record. Digest syntax alone is not verification of an external file.
Without both an artifact verifier and a policy verifier, analysis remains
diagnostic and every ratio states that provenance, gates or policy are unverified.

The estimand is the mean paired process-aggregate difference and the mean of
paired candidate/baseline ratios. It is not a ratio of pooled request means.
The seeded percentile bootstrap resamples whole independent pairs. Bonferroni
bounds jointly cover both delta and ratio estimands across the declared metric family under asymptotic bootstrap assumptions.
Insufficient bootstrap tail resolution is refused rather than reporting an
unsupported confidence level. Stable conditions and independence remain experimental obligations; balancing
order cannot prove absence of drift. Every ratio includes statistical status.
A zero baseline makes the ratio unavailable and retains the paired delta.

## Supervised execution

`llm-serving paired -config plan.json -output receipt.json` executes each declared
arm through the lifecycle supervisor. The configuration lists all executions in
plan order, with a fresh output artifact path and an archived capacity probe
folder for each arm. It persists `receipt.json.plan.json` before launching a
process. Command argument and environment values are hashed; probe bodies are
hashed and URL credentials, query strings and fragments are removed. The canonical original
configuration hash binds the user-owned input without copying secrets. The
sanitized plan is an audit artifact, not an executable resume configuration. Existing run artifacts are refused. Each attempted arm retains the
actual process nonce/PID/start, executable hashes, observed effective manifest,
validated work artifact digest and capacity archive receipt digest. The CLI
retains read-only snapshots under `receipt.json.artifacts/` and verifies those
snapshots, so later edits to the input archives cannot silently change evidence.

The runner checks every retained request's model, allowed finish reason, exact
usage and output-length bounds. It requires complete accepted populations and
matching effective work. A missing or failed lifecycle/work/capacity archive
stops execution immediately and retains that attempt; later arms are not run.

A functional null-server archive alone does not establish resource headroom.
Without measured resource bounds and offered-schedule evidence, completed
comparisons remain diagnostic. No retry or automatic gate relaxation occurs.
Supported process aggregates are `goodput`, `input_throughput`,
`output_throughput`, and request analysis metrics with a `.mean` suffix. Mean
metrics require complete population coverage. This serving comparison never
establishes native engine parity.

`llm-serving compare -input experiment.json` analyzes retained pair aggregates;
it remains diagnostic until an artifact verifier authenticates provenance and
gates through the Go API. Every ratio states its statistical status.

A comparison is qualified only when every archived capacity evaluation reports
`SAMPLED_CAPACITY`, the actual workload uses the same shared connection cap,
transport profile, nonstreaming protocol and request/response byte geometry,
and its complete offered schedule meets preregistered rate and lag bounds.
This status covers the finite-cadence sampling protocol; unsampled resource
spikes remain unknown. Unknown profiles or schedules stay diagnostic. Exceeded
bounds stop the experiment and retain the failure.

## Measurement and SLO policy

Preregister `plan.policy` with `measurement_policy`, `window_policy`,
`warmup_policy` and an explicit `slo` object. The supported metadata values are
`dispatch-content-v1/success-token-counts-v1/success-slo-v1`,
`attempt started and completed inside window; successful token totals`, and
`phase=measurement and per-client warmup exhausted`, respectively.
The SLO keys are `max_ttft_ms`, `max_ttfo_ms` and `max_chunk_latency_ms`.
Explicit zero limits disable those checks; an absent receipt is unavailable
policy evidence.

The supervised runner derives actual policy from validated run metadata and
every retained request's SLO receipt. Known differences within a run, between
arms or against the preregistered policy reject the experiment. Missing policy
metadata, missing request receipts or a missing preregistration leave the result
diagnostic. Identical strict policies retain unavailable timing and zero goodput;
a zero baseline still leaves the ratio unavailable.

The Go API records this evidence as `PairRun.Policy`, `PolicyComplete` and
`PolicyHash`. Qualification requires `PairedOptions.VerifyPolicy` to verify the
policy against retained actual run artifacts, in addition to `Verify` for other
provenance and gates. These callbacks are trusted harness attestations; policy
hashes and caller-supplied flags alone cannot authenticate execution. The CLI
provides both verifiers from its validated artifact snapshots. Imported aggregate
JSON remains diagnostic because it lacks these verifiers.
