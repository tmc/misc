# Live AIPerf measurement comparison

`scripts/aiperf-live-verify.py ARCHIVE` checks a bounded streaming chat
comparison against a common observer's retained wire capture. It exits nonzero
for missing, malformed or contradictory evidence. It does not measure client
capacity, GPU collector overhead or cross-tool performance equivalence.

The archive contains AIPerf/xk6/xk6/AIPerf process runs at concurrency 1 and 2.
Each run sends two excluded sequential warmups and six measured requests.
The observer writes `wire.jsonl`, with arm, request, HTTP status, monotonic
start/end times, Unix start time and timestamped SSE lines. Each xk6 arm retains
`run.json`; each AIPerf arm retains `profile_export_aiperf.json` and
`profile_export.jsonl`. Preserve the authored scripts, source and binary hashes,
model digest, runtime versions, lock receipt and every failed trial separately.

The verifier requires one canonical payload, exact populations, observed ABBA
order, nonoverlapping process arms, sequential excluded warmups and observed
concurrency. It checks unique native request IDs, measurement phases and
record-to-wire start times within 15 ms. It refuses provider errors, missing or
duplicate SSE terminals, unavailable usage and inconsistent token totals.
Every event timestamp, including usage, terminal and non-data events, must be
an integer inside the request's integer start/end bounds and must not decrease
in captured order. Equal event timestamps are accepted. Present content,
reasoning and reasoning_content fields accept strings or null; absent, null and
empty strings do not trigger content. Tool calls accept null or a list of
objects with a nonnegative integer index; optional id/type and function
name/arguments fields accept strings or null, and function accepts an object or
null. Text objects, arrays, booleans and numbers are refused. A present primary-choice
index must be the exact integer zero; omission defaults to zero.
Every measured response must exercise reasoning content so the first-token
regression control cannot pass without its trigger.

Each tool is compared with its own wire measurements. First-token, first-output
and last-content mean residuals must be at most 15 ms; token-normalized ITL mean
residual must be at most 0.25 ms. ITL uses the provider completion count minus one,
with the declared assumption of one first token. This is a semantic regression
control, not physical token-spacing proof. Count totals must match exactly.
Bounds are fixed before the run; a failed run remains failed.

Arm labels and timestamps are observation consistency evidence. They do not
authenticate the observer or prove immutable producer execution. Keep independent
source/binary provenance and process supervision receipts. The verifier prints
`NOT statistically gated`; separately scheduled model timings and throughput
cannot establish performance parity from these small cells. Live completion,
Responses, embedding, reranking, hosted-provider failures, missing-usage estimates
require separate qualification cells.

For a different bounded concurrency cell, supply an explicit policy and its
exact byte hash; the verifier never infers a policy from measured rows:

```sh
python3 scripts/aiperf-live-verify.py ARCHIVE \
  --plan verification-policy.json --plan-sha256 "$POLICY_SHA256"
```

The JSON object has exactly these fields:

```json
{"schema_version":1,"concurrencies":[4,8],"measured_requests":8,
 "warmup_requests":2,"latency_limit_ms":15,"itl_limit_ms":0.25,
 "payload_sha256":"<64 lowercase hex characters>",
 "preregistration_sha256":"<SHA256 of ARCHIVE/PREREGISTRATION.md>"}
```

Concurrency values must be distinct, increasing integers from 1 through 8.
Each concurrency uses four ABBA arms. The measured count must support the
largest concurrency and total at most 64 across all arms. Two sequential
excluded warmups and the residual tolerances remain fixed. Both hash arguments
are required together. The declared canonical payload and retained
preregistration bytes must match. Hashes check retained identity; they do not
prove when a plan was authored. A policy materialized after a run must reference
its original retained preregistration and be described as a verification mapping,
not a newly preregistered measurement plan.
