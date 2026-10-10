# xk6-llm-perf

A k6 extension for load testing OpenAI-compatible chat completion APIs.
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
| `warmupCount` | Initial requests per client excluded from LLM and HTTP metrics |
| `maxTTFT` | First-token SLO in milliseconds; disabled at zero |
| `maxTTFO` | First-output SLO in milliseconds; disabled at zero |
| `maxTokenLatency` | Content-chunk interval SLO in milliseconds; disabled at zero |

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
| `llm_good_request` | Rate | SLO-compliant fraction among successful recorded requests |
| `llm_errors` | Counter | Recorded request failures |

Adjusted metrics are emitted only when `networkRTT` is positive. A configured
SLO fails when its required timing is unavailable; TTFT and TTFO are available
only for streaming requests.

## Measurement limits

- Request timing includes local preparation, prefill waiting, and trailing usage
  events. It does not isolate server latency or stop at the last content chunk.
- `llm_token_latency` measures chunk spacing, not token-normalized ITL.
- `llm_tokens_per_second` is a per-request Gauge, not aggregate throughput.
- `llm_good_request` excludes failures from its denominator. Use failure and
  checks thresholds alongside it; it is not goodput in requests per second.
- RTT adjustment uses a supplied value; the extension does not calibrate RTT.
- Streaming output collects the first choice's text. Tool calls and multimodal
  content are not supported.
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
otherwise `first_chunk_source` records `assumed_one`. Non-streaming TTFT and
ITL remain unavailable. New `llm_slo_compliance` includes measured failures;
legacy `llm_good_request` retains its successful-request population.

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
queries, fragments, or another host. The current response adapters are
`openai-chat`, `openai-completions`, `openai-responses`, `anthropic`, and `json`.
The JSON adapter returns `body` and leaves generated-token counts unavailable
unless supplied. Responses and Anthropic adapters normalize text/reasoning and
usage; they do not yet implement every event or tool lifecycle. Audio uploads,
asynchronous generation, and full multi-choice streams remain pending.

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
Automatic discovery, normalized window summaries, and a command integration
remain pending.

Record retention is bounded by both `maxRecords` and `maxRecordBytes` (default
16 MiB, maximum 32 MiB). Exceeding either budget increases `dropped_records`
without changing aggregate accounting. Caller tokenizer identity is retained
as `client_tokenizer`; it does not identify the provider's tokenizer.

`workerId` assigns record identities without adding high-cardinality metric
tags. `MergeRuns` requires explicit workers, complete records, compatible
workload/trace identities, and admitted clock mappings. It detects duplicate
workers/requests and preserves clock receipts. Missing workers require explicit
partial admission and remain listed in `missing_workers`. Kubernetes/operator
deployment, asset portability, and live multi-worker qualification remain open.

Makefile builds retain `extension_revision`, `extension_dirty`, and
`extension_source_hash` in the run manifest. The hash covers local Go sources,
example JavaScript, module files and Makefile, and is checked again after the
build. External vocabulary, template and dataset assets still require their
own retained hashes. A local development version string alone does not pin
extension source.
