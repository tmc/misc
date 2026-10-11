# xk6-llm-perf

A k6 extension for measuring inference APIs and replaying benchmark records.
It supports streaming and non-streaming requests, shared HTTP connections,
round-robin endpoints, and a shared prefill concurrency limit.

## Build and run

Requires Go 1.26.9 or newer. Builds pin k6 v2.3.0 and xk6 v1.4.14.

```bash
git clone https://github.com/tmc/misc
cd misc/xk6-llm-perf
make build

ENDPOINT_URL=http://localhost:8000/v1 \
OPENAI_API_KEY=test \
./dist/k6 run examples/sample-load-test.js
```

## Usage

Both default and named imports are supported. Configuration keys use camelCase;
request and response fields use OpenAI snake_case. Unknown options are rejected.
See [index.d.ts](./index.d.ts) for the complete API.

```javascript
import { Client } from 'k6/x/llm-perf';
import { check } from 'k6';

export const options = {
    vus: 10,
    duration: '30s',
    thresholds: {
        checks: ['rate==1'],
        http_req_failed: ['rate<0.01'],
        llm_ttft: ['p(95)<2000'],
        llm_request_latency: ['p(95)<5000'],
    },
};

const client = new Client({
    apiKey: __ENV.OPENAI_API_KEY,
    baseURL: __ENV.ENDPOINT_URL,
    model: __ENV.MODEL || 'test-model',
    timeout: '30s',
});

export default function () {
    const response = client.chat.completions.create({
        messages: [{ role: 'user', content: 'Explain HTTP keep-alive.' }],
        max_tokens: 256,
        stream: true,
    });
    check(response, {
        'completion successful': (r) => r.status === 200,
        'has content': (r) => r.choices?.[0]?.message?.content?.length > 0,
    });
}
```

Operational failures return a JavaScript Error object. Check the response and
set a `checks` threshold so failed checks fail the run.

## Configuration

| Option | Behavior |
|--------|----------|
| `apiKey` | Bearer token sent to the endpoint |
| `baseURL` | Single OpenAI-compatible API base URL, including any `/v1` prefix |
| `baseURLs` | Optional endpoint list; overrides `baseURL` and rotates per client |
| `model` | Default model; may be overridden per request |
| `timeout` | Positive Go duration string; defaults to `30s` |
| `networkRTT` | Milliseconds subtracted from adjusted latency metrics; disabled at zero |
| `tokenMultiplier` | Word-count multiplier for heuristic token estimates; defaults to 1.33 |
| `prefillConcurrency` | Shared cap for clients with the same endpoint set, model, and limit; unlimited at zero |
| `requestConcurrency` | Shared cap for complete requests; unlimited at zero |
| `controlGroup` | Explicit process-wide mutable admission group; excludes static ceilings |
| `useK6Transport` | Use the VU transport and its k6 network configuration |
| `strictMeasurement` | Omit heuristic token counts |
| `warmupCount` | Initial requests per client excluded from LLM and HTTP metrics |
| `maxTTFT` | First-token SLO in milliseconds; disabled at zero |
| `maxTTFO` | First-output SLO in milliseconds; disabled at zero |
| `maxTokenLatency` | Content-chunk interval SLO in milliseconds; disabled at zero |
| `maxTPOT` | Token-normalized last-content ITL SLO in milliseconds; disabled at zero |
| `maxE2E` | Dispatch-to-last-content SLO in milliseconds; disabled at zero |

SLO limits accept finite non-negative fractional milliseconds. The Go `Config`
and `SLOReceipt` latency fields use `float64`; programmatic integer variables
must be explicitly converted. New receipt pointers distinguish unavailable
TPOT/E2E limits from explicit disabled zero.

Streaming requests include `stream_options.include_usage` unless explicit
stream options are supplied. Provider token counts are preferred; missing counts
use a word-and-character estimate. These estimates are not model tokenization.

## Metrics

Latency metrics use milliseconds. The extension also emits k6 HTTP timing,
request-count, and failure metrics, including `http_reqs`, `http_req_duration`,
`http_req_waiting`, and `http_req_failed`.

| Metric | Type | Meaning |
|--------|------|---------|
| `llm_ttft` | Trend | Time to first reasoning or output content in a stream |
| `llm_ttfo` | Trend | Time to first non-reasoning output content in a stream |
| `llm_token_latency` | Trend | Intervals between content-bearing chunks, which may contain multiple tokens |
| `llm_inter_chunk_latency` | Trend | Intervals between parsed JSON SSE events, including usage events |
| `llm_completion_time` | Trend | Time through full response consumption |
| `llm_request_latency` | Trend | Same full-response timing as `llm_completion_time` |
| `llm_ttft_adjusted` | Trend | TTFT minus configured RTT, clamped to zero |
| `llm_ttfo_adjusted` | Trend | TTFO minus configured RTT, clamped to zero |
| `llm_request_latency_adjusted` | Trend | Full-response latency minus configured RTT, clamped to zero |
| `llm_input_sequence_length` | Trend | Reported or estimated prompt tokens |
| `llm_output_sequence_length` | Trend | Reported or estimated completion tokens |
| `llm_prompt_token_discrepancy` | Trend | Absolute estimate difference as a percentage of reported prompt tokens |
| `llm_completion_token_discrepancy` | Trend | Absolute estimate difference as a percentage of reported completion tokens |
| `llm_tokens_per_second` | Gauge | Completion tokens divided by full-response duration for each request |
| `llm_prompt_tokens` | Counter | Reported or estimated prompt tokens |
| `llm_completion_tokens` | Counter | Reported or estimated completion tokens |
| `llm_total_tokens` | Counter | Prompt plus completion tokens |
| `llm_usage_discrepancies` | Counter | Differing prompt/completion count comparisons; up to two per request |
| `llm_requests` | Counter | Successful recorded requests |
| `llm_good_request` | Rate | SLO-compliant fraction among attempted recorded requests |
| `llm_errors` | Counter | Recorded request failures |

Adjusted metrics are emitted only when `networkRTT` is positive. A configured
SLO fails when its required timing is unavailable; TTFT and TTFO are available
only for streaming requests.

## Measurement limits

- Request timing includes local preparation, prefill waiting, and trailing usage
  events. It does not isolate server latency or stop at the last content chunk.
- `llm_token_latency` measures chunk spacing, not token-normalized ITL.
- `llm_tokens_per_second` is a per-request Gauge, not aggregate throughput.
- `llm_good_request` and `llm_slo_compliance` include measured failures in their
  denominator. The `llm_good_requests` counter rate uses the k6 engine window;
  completed content-span goodput uses `llm_content_span_goodput`.
- RTT adjustment uses a supplied value. Opt-in calibration retains its probe basis and overhead.
- Streaming timing selects choice 0; response content retains all choice indices.
  Request-level usage is not allocated to individual choices.
- The shared transport uses Go's proxy handling and TLS verification. k6-specific
  DNS, TLS, connection, and network byte accounting options are not fully
  integrated.

Local HTTP/SSE fixtures validate the protocol and k6 integration. Live provider
compatibility and AIPerf measurement parity have not been established.

## Examples

The scripts read environment variables and pass them to the client. These are
script settings, not environment variables read directly by the Go extension.

- [llm-test.js](./examples/llm-test.js): local streaming test.
- [openai-test.js](./examples/openai-test.js): hosted OpenAI example.
- [sample-load-test.js](./examples/sample-load-test.js): ramping load with a JSON summary.
- [trace-replay.js](./examples/trace-replay.js): replay request rows from a JSON trace.
- [Live dashboard](examples/dashboard/README.md): streaming LLM charts and HTML reports.

See each script for its environment settings. The load-test example accepts
`ENDPOINT_URL`, `ENDPOINT_URLS`, `OPENAI_API_KEY`, `MODEL`, `TIMEOUT`,
`NETWORK_RTT`, `TOKEN_MULTIPLIER`, `PREFILL_CONCURRENCY`, `WARMUP_COUNT`, and
`STREAM_MODE`.

Trace replay assigns each row once across VUs. Delays are relative sleeps before
each request, not an absolute arrival schedule. `TRACE_FILE` selects the trace;
`VUS` controls concurrent workers.

## Development

Run `make check` for race tests, vet, the signed extension build, HTTP/SSE smoke
checks, and a three-VU trace replay test. See [CONTRIBUTING.md](./CONTRIBUTING.md)
for the development workflow, [SECURITY.md](./SECURITY.md) for reporting issues,
and [CHANGELOG.md](./CHANGELOG.md) for changes. Licensed under [MIT](./LICENSE).

### Explicit measurement windows

Use `beginMeasurement` in `setup`, `endMeasurement` in `teardown`, and
`measurementSummary` in `handleSummary` to export a process-local run. See
[`examples/phased-load-test.js`](examples/phased-load-test.js) for warmup and
measurement scenarios. Only measured requests dispatched and finished inside
the window contribute to run totals. Successful requests contribute token
totals. Inflight and boundary work is recorded separately; retained records
are bounded by `maxRecords` (default zero). Dropped records prevent timeline
replay even when aggregate counters remain available.

The response's `measurement` object preserves unavailable values as `null`.
`strictMeasurement: true` disables heuristic counts. `measurement` request
options supply caller-declared counts and a tokenizer label; this does not
verify vocabulary or chat framing. Reported zero remains distinct from absent
usage. Admission wait is separate from dispatch-anchored full-response latency.
TTFT includes reasoning; TTFO starts at visible text or tool output. ITL uses
last content, excludes trailers, and subtracts first-chunk tokens when known;
otherwise `first_chunk_source` records `assumed_one`. Inconsistent explicit
first-chunk counts retain their original value in `first_chunk_tokens` and
record a reason in `first_chunk_unavailable`. Non-streaming TTFT and
ITL remain unavailable. `llm_slo_compliance` and `llm_good_request` include
measured failures. `maxTPOT` tests this ITL and `maxE2E` tests last-content
latency; a trailing usage event does not change either. Enabled limits require
available observations. Archived SLO receipts without the new limits remain
readable, but cannot qualify the extended SLO policy.

`requestConcurrency` limits complete requests across clients with the same
endpoint list, model, and ceiling. `prefillConcurrency` releases at first
content. Timeout includes both admission and the HTTP request. Optional
`cancelAfter` is a request-local cancellation deadline. `scheduledAt` supplies
an intended Unix millisecond arrival, without delaying dispatch; the record
contains dispatch lag and scheduled-to-completion latency. `useK6Transport`
opts into the VU's transport and k6 network configuration. Completed HTTP
stages are retained in `http_trace`; missing stages stay null. Network RTT
subtraction remains a supplied arithmetic adjustment.

### Authored endpoint payloads

`client.request({path, protocol, body})` accepts a bounded authored JSON body.
Paths are relative to the configured base URL and cannot contain traversal,
queries, fragments, or another host. Response adapters include chat, completions, Responses, Anthropic, TGI, audio,
image, video, embeddings, ranking and retrieval. See
[modality lifecycles](docs/modalities/README.md) and
[endpoint templates](docs/endpoint-adapters/templates.md).
The JSON adapter returns `body` and leaves generated-token counts unavailable
unless supplied. Responses and Anthropic adapters normalize protocol-specific content and usage.
Media output units remain separate from generated text tokens. Video requests
retain one job lifecycle; HTTP traces cover the submission, with polling bounded
by the same cancellation deadline.

### Offline replay and GPU energy

Run `go run ./cmd/llm-report -input outputs/run.json -slice 10s` to reconstruct
request/prefill/decode concurrency and completion-based timeslices. Effective
concurrency includes idle duration; active concurrency excludes idle duration.
These are client phase proxies. They do not measure device concurrency.
Replay refuses partial retained record sets.

An optional `-telemetry window.json` joins a benchmark-specific version-1
`TelemetryWindow` interchange. This is a consumer adapter format, not an
assertion that the lib GPU collector has delivered that wire schema. Preserve
the producer's raw readings and manifest, identified by collector revision and
source hash. Every observed endpoint needs an explicit device mapping, and
clock uncertainty must fit `-max-clock-uncertainty` (milliseconds).

Energy admission requires full coverage, matching counter/benchmark boundaries,
and a sample gap within `-max-telemetry-gap`. Counter resets, missing devices,
unknown clocks, and MIG allocation of whole-device energy are refused.
Cumulative millijoules retain integer precision before subtraction. Power
integration is labeled estimated. Energy ratios use successful output counts
with source provenance; zero denominators remain unavailable. The consumer
does not qualify native readings, collection overhead, or cross-tool parity.

`llm-telemetry` can admit complete native schema-2 captures with a hash-bound
clock and endpoint mapping. See [native window admission](docs/telemetry/NATIVE-ADMISSION.md)
for gauge, interval, MIG and query-estimate policies. Query estimates cannot
supply energy; unavailable energy does not invalidate independent gauges.

[`examples/lib/workload.js`](examples/lib/workload.js) supplies seeded constant,
Poisson and gamma arrival offsets, dataset validation/sampling, bounded authored
prefixes, absolute-arrival waits with explicit drop policy, synchronous DAG
replay, and same-VU conversations. It does not claim exact token budgets from
text length. `examples/workload-smoke.js` verifies unique seven-row replay across
three VUs and records scheduled-arrival lag. DAG validation rejects cycles
before executing any node; failed parents skip descendants.

`LatencyConfidence` provides an optional seeded percentile bootstrap of the
successful-request mean with explicit independent-observation acknowledgment,
a retained-record requirement, and a work budget. This assumption can fail for
concurrent load; request resampling is not run-level convergence evidence.

Dispatch-anchored first-content trends are `llm_dispatch_ttft` and
`llm_dispatch_ttfo`. Legacy `llm_ttft` and `llm_ttfo` keep their earlier
local-preparation/admission-inclusive anchors.

The `servermetrics` package parses bounded Prometheus text, validates cumulative
histogram buckets, retains unavailable/nonfinite readings, and rejects duplicate
identities. Counter rates require stable labels/types and a declared maximum gap;
resets remain errors. `Collect` writes serial, bounded JSONL scrape attempts with
raw exposition hashes, failed observations, missed ticks, and completeness.
Raw server labels are deliberately retained and may contain server metadata.
Configured URL and caller-obtained Kubernetes PodList discovery, normalized
window replay, and `cmd/llm-scrape` retain target changes and coverage. See
[server metrics](servermetrics/README.md).

Record retention is bounded by both `maxRecords` and `maxRecordBytes` (default
16 MiB, maximum 32 MiB). Exceeding either budget increases `dropped_records`
without changing aggregate accounting. Caller tokenizer identity is retained
as `client_tokenizer`; it does not identify the provider's tokenizer.

`workerId` assigns record identities without adding high-cardinality metric
tags. `MergeRuns` requires explicit workers, complete records, compatible
workload/trace identities, and admitted clock mappings. It detects duplicate
workers/requests and preserves clock receipts. Missing workers require explicit
partial admission and remain listed in `missing_workers`. Portable local worker and operator assets are in
[examples/distributed](examples/distributed/README.md). Local worker fixtures
and live cluster qualification remain separate evidence classes.

Makefile builds retain `extension_revision`, `extension_dirty`, and
`extension_source_hash` in the run manifest. The hash covers local Go sources,
example JavaScript, module files and Makefile, and is checked again after the
build. External vocabulary, template and dataset assets still require their
own retained hashes. A local development version string alone does not pin
extension source.

Streaming timing selects choice index 0; returned content retains all indices. Other choice
indices set `multi_choice`; provider request-level usage is retained, but
per-choice ITL is omitted because those counts cannot identify choice 0's
output. The record states `choice_policy`. Sync responses retain all choices
and their indices. Streaming responses aggregate text, reasoning and tools by choice index; no
per-choice token allocation is inferred.

[`tools/llm-tokenize`](tools/llm-tokenize/README.md) is a separate offline
preprocessing command using a publicly pinned `lib/hf/hftokenizer` module.
It verifies vocabulary and literal role-frame hashes, known synthetic sequences,
and chat framing, and emits a retained counting receipt. Its framing dialect
is explicit; Jinja and unsupported tokenizer pipelines are refused. The
extension continues to identify imported counts as caller declarations.

### Host workflows

See [AIPerf compatibility and remaining gates](docs/aiperf-parity.md) for the
current scope of cross-tool qualification.

- [Serving qualification](docs/serving/README.md): observed manifests, expected work,
  supervised processes, measured client controls, and independent paired analysis.
- [Analysis and replay](docs/analysis/README.md): distributions, phase-rate proxies,
  strict JSONL reconciliation, and declared confidence methods.
- [Workloads](examples/workloads/README.md): seeded realized sequences, exact pinned
  count receipts, prefixes, absolute arrivals, conversations, DAGs and hooks.
- [Campaigns](campaign/README.md): bounded sweeps/search, failed attempts,
  checkpoints and declared convergence controls.
- [Telemetry replay](docs/telemetry/REPLAY.md): continuous raw readings, coverage,
  provenance and whole-device energy joins.
- [Outputs and accuracy](examples/output/README.md): local plots, OTLP/MLflow/W&B
  consumers and pinned evaluator workflows.
- [Calibration](docs/calibration.md): bounded HTTP and fresh TCP probes outside
  active measurement windows.

`make check` runs race/vet, the offline tokenizer checks, a signed extension build
and built-k6 fixtures. Native telemetry, live providers, collector overhead,
cluster deployment and matched AIPerf qualification require their own receipts.

For adaptive admission, initialize `setControlLimits('load', {request: 4,
prefill: 2})` and give clients `controlGroup: 'load'`. Limits apply to all clients
in that process naming the group. `controlSnapshot('load')` reports current
holders. Changing a ceiling affects new admission; decreases let existing
holders drain. Zero disables a ceiling. Multi-process controllers partition
quotas and preserve their assessment windows rather than assuming shared memory.

For dispatched requests, start and finish timestamps project monotonic elapsed time from the recorded dispatch Unix clock anchor; `timestamp_basis` identifies this mapping. Failed admission uses the request-start Unix anchor. Projected endpoints are not independent system wall-clock samples. Content timings use the same monotonic dispatch span, and scheduled response latency uses the projected finish. Admission wait remains the monotonic duration of the admission queue, excluding request preparation. This keeps timestamps consistent with latency when system wall-clock and monotonic readings advance differently. Replay checks timing against dispatch/finish timestamps within 0.002 ms
(microsecond truncation plus floating-point epoch rounding). Absent derived
rates are recomputed for compatible version-1 exports; explicit null rates are
refused. Request-analysis percentiles include successful requests only. AIPerf
can include failures in latency percentiles, so comparisons require matching
populations.

Streaming records retain up to 4096 content-only inter-event intervals in
`content_intervals_ms`; `content_intervals_truncated` marks an incomplete
vector. `ttst` measures dispatch to the second recognized content event, which
can contain multiple tokens. Role, usage, and terminal events do not enter this
population. The matching k6 trends are `llm_dispatch_ttst`,
`llm_mean_content_interval`, and `llm_max_content_interval`. The interval
trends each record one observation per request with a complete, nonempty
interval vector. Their percentiles describe per-request means and maxima, with
equal weight per request; raw exports retain the individual intervals for
pooled event analysis. Truncated and empty vectors contribute no interval trend
observations. Distributed merging requires a recognized measurement policy,
whole-contained windows and identical recorded SLO limits; older workers
without policy receipts must be rerun.

Replay requires eligible count and first-chunk provenance for token-normalized timing, and reconciles ITL against its content span and full-response ITL against its response span. Exact first-chunk sources require the retained original count; unavailable metrics remain unavailable. Arrival lag requires a schedule anchor. Projected admission wait may cover only part of request preparation. Partial manifests cannot add tokens when every dropped successful count in that dimension is missing; incomplete request-level analyses remain refused.
