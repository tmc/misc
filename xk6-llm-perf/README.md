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
