# Per-request goodput SLOs

A request is good only when it succeeds and meets every enabled client limit.
Limits accept finite non-negative fractional milliseconds. Zero disables a limit;
an enabled limit with unavailable timing fails qualification.

```javascript
const client = new llm.Client({
  baseURL: __ENV.ENDPOINT_URL,
  model: __ENV.MODEL,
  strictMeasurement: true,
  maxTTFT: 500,
  maxTPOT: 15.5,
  maxE2E: 2000,
});
```

`maxTTFT` uses dispatch-to-first recognized content. `maxTPOT` uses
`measurement.itl`: `(last_content_latency - ttft) / decode_tokens`.
`decode_tokens` subtracts a valid first-content-chunk count from the output
count, or assumes one token when that count is unavailable or inconsistent.
The original count and fallback provenance remain in the measurement.
Output counts below two, missing counts, multi-choice responses, and missing
content timing make TPOT unavailable. This is a client observation; token
accuracy requires a separate tokenizer and serving-template qualification.

`maxE2E` uses dispatch-to-last recognized content. Usage-only trailers and HTTP
completion after the final content do not change either TPOT or E2E. These
boundaries match AIPerf's `inter_token_latency` and `request_latency` at the
pinned source revision used by the alignment review. Full HTTP completion
remains available separately as `full_response_latency` and
`full_response_itl`. Non-streaming TPOT remains unavailable.

`llm_good_requests` counts good requests; its k6 counter rate gives requests
per second over the k6 engine window.
It is not the named recipe's content-span goodput objective. `llm_slo_compliance` and `llm_good_request` both use all recorded
attempted requests as their denominator, including failed, cancelled, and
timed-out requests. Warmup and requests rejected before dispatch are excluded
from that attempted population. This changes the former successful-only
population of `llm_good_request`.

`llm_slo_ttft_ms`, `llm_slo_tpot_ms`, and `llm_slo_e2e_ms` retain the actual
applied limits as trends, one observation per recorded attempt. Search workers
can verify their minimum and maximum against a declared policy instead of
trusting environment variables alone. These trends describe applied limits,
not observed latency distributions.

Run records retain numeric limits in `slo`. New clients explicitly retain
`max_tpot_ms` and `max_e2e_ms`, including disabled zero. Archived records without
these fields, or with null fields, remain readable with the old numeric policy;
they cannot establish a complete extended policy. Policy equality and hashing
include both values and their availability. Comparison outputs show absent
limits as unavailable.

The Go `Config` latency limits and the existing `SLOReceipt` scalar limits
now use `float64` instead of `int`. Programmatic integer variables require an
explicit conversion. JSON and JavaScript names are unchanged, and historical
integer-valued receipt JSON retains its canonical encoding. The new receipt
limits are `*float64` so absence cannot become disabled zero.

## Observation goodput

`endMeasurement()` computes an additional `observation` receipt from complete
retained records from one process clock domain. Merged worker records, clock
mappings and missing-worker evidence cannot qualify this policy. Its
`single-process-successful-content-span-v1` policy starts at the earliest
successful request with content and ends at the final successful content
response. Failed requests do not define those bounds, including failures before
or after the content span. They remain in `observation.attempts` and the SLO
attainment denominator. The good count divided by the observed duration in
seconds gives `observation.goodput`.

This is AIPerf's default benchmark-duration policy. AIPerf can instead use
explicit analysis-window bounds; the existing `RunSummary.goodput` retains this
project's declared begin/end window semantics. These are distinct receipts.
Initialization, warmup, manual idle time outside the content span, trailing
usage and post-window drain cannot change the content-span objective.

An all-failed or empty-content run, nonpositive span, dropped records, boundary
exclusions or in-flight requests leave observation goodput unavailable, with a
reason in `observation_unavailable`. No zero duration or qualified zero goodput
is invented. A zero-good run with a valid positive successful content span does
retain measured zero goodput. Retained receipts are recomputed during replay;
changed bounds, duration, population, policy or derived value reject.

A qualified `endMeasurement()` emits one observation into each of
`llm_content_span_goodput`, `llm_content_span_duration_ms`,
`llm_content_span_good_requests`, and `llm_content_span_attempts`, and increments
`llm_content_span_windows`. Named goodput searches require exactly one complete
window per process and verify the duration, population and objective witnesses.
Multiple window averages cannot silently replace one benchmark span.

For multiple VUs, start the process-wide window once in `setup()` and end it
once in `teardown()` after all measured requests have completed. Each VU uses
its own client. The final window aggregates the shared request population:

```javascript
export function setup() {
  llm.beginMeasurement({runId: 'serving-search', maxRecords: 10000});
}
export default function() {
  client.chat.completions.create({messages, stream: true});
}
export function teardown() {
  llm.endMeasurement();
}
```

Set the scenario's VU count from the recipe's concurrency environment variable.
Per-iteration begin/end calls are unsuitable for a concurrent shared window.
