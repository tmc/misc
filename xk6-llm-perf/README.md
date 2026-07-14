# xk6-llm-perf

A k6 extension for load testing OpenAI-compatible chat completion APIs.

## Features

- OpenAI-style `chat.completions.create` API
- Metrics for time to first token, time to first output token, token latency, goodput, token rate, total tokens, completion time, and errors
- Native k6 HTTP metrics for chat completion requests, including `http_reqs`, `http_req_duration`, and `http_req_waiting`
- Streaming and non-streaming chat completion responses
- Configurable endpoint set, model, API key, request timeout, RTT adjustment, token fallback, and prefill concurrency

## Quick Start

```bash
# Install k6 with the extension
go install go.k6.io/xk6/cmd/xk6@latest
xk6 build --with github.com/tmc/misc/xk6-llm-perf@latest

# Run a load test
ENDPOINT_URL=https://api.openai.com/v1 \
OPENAI_API_KEY=your-api-key \
./k6 run examples/sample-load-test.js
```

## Usage

```javascript
import llm from 'k6/x/llm-perf';
import { check } from 'k6';

export const options = {
    vus: 10,
    duration: '30s',
    thresholds: {
        'llm_ttft': ['p(95)<2000'],         // Time to First Token
        'llm_ttfo': ['p(95)<2500'],         // Time to First Output Token
        'llm_token_latency': ['avg<100'],    // Inter-token latency
        'llm_inter_chunk_latency': ['p(95)<250'],
        'llm_request_latency': ['p(95)<5000'],
        'llm_request_latency_adjusted': ['p(95)<4900'],
        'llm_good_request': ['rate>0.95'],   // SLO-compliant requests
        'llm_tokens_per_second': ['value>5'], // Output token generation rate
        'llm_completion_tokens': ['count>0'],
        'llm_errors': ['count<10'],
    },
};

const client = new llm.Client({
    apiKey: __ENV.OPENAI_API_KEY,
    baseURL: __ENV.ENDPOINT_URL,
    baseURLs: (__ENV.ENDPOINT_URLS || '').split(',').filter(Boolean),
    model: 'gpt-4',
    timeout: '30s',
    networkRTT: 25,
    tokenMultiplier: 1.33,
    prefillConcurrency: 32,
    warmupCount: 10,
    maxTTFT: 2000,
    maxTTFO: 2500,
    maxTokenLatency: 100,
});

export default function() {
    const response = client.chat.completions.create({
        messages: [
            { role: "user", content: "What is the meaning of life?" }
        ],
        temperature: 0.7,
        max_tokens: 1000,
        stream: true,  // Enable streaming responses
    });

    check(response, {
        'completion successful': (r) => r.status === 200,
        'has content': (r) => r.choices?.[0]?.message?.content?.length > 0,
    });
}
```

## Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `llm_ttft` | Trend | Time to First Token - measures initial response latency |
| `llm_ttfo` | Trend | Time to First Output Token - first non-reasoning text token in streaming responses |
| `llm_token_latency` | Trend | Inter-token latency for streaming responses |
| `llm_inter_chunk_latency` | Trend | Latency between consecutive SSE chunks |
| `llm_completion_time` | Trend | End-to-end completion latency |
| `llm_request_latency` | Trend | End-to-end request latency |
| `llm_ttft_adjusted` | Trend | Time to first token minus configured `networkRTT` |
| `llm_ttfo_adjusted` | Trend | Time to first output token minus configured `networkRTT` |
| `llm_request_latency_adjusted` | Trend | End-to-end request latency minus configured `networkRTT` |
| `llm_input_sequence_length` | Trend | Prompt token count from usage data, or an estimated count for streaming responses |
| `llm_output_sequence_length` | Trend | Completion token count from usage data, or an estimated count for streaming responses |
| `llm_prompt_token_discrepancy` | Trend | Absolute percent difference between estimated and reported prompt tokens |
| `llm_completion_token_discrepancy` | Trend | Absolute percent difference between estimated and reported completion tokens |
| `llm_tokens_per_second` | Gauge | Output token generation rate |
| `llm_prompt_tokens` | Counter | Prompt tokens reported or estimated |
| `llm_completion_tokens` | Counter | Completion tokens reported or estimated |
| `llm_total_tokens` | Counter | Prompt plus completion tokens |
| `llm_usage_discrepancies` | Counter | Count of requests where reported and estimated token counts differ |
| `llm_requests` | Counter | Successful requests |
| `llm_good_request` | Rate | Fraction of successful requests satisfying configured `maxTTFT`, `maxTTFO`, and `maxTokenLatency` SLOs |
| `llm_errors` | Counter | Number of failed requests |

## Configuration

The client accepts the following configuration options:

```javascript
const client = new llm.Client({
	apiKey: 'your-api-key',      // API key for authentication
	baseURL: 'https://...',      // Base URL for the LLM API
	baseURLs: ['https://...'],   // Optional round-robin endpoint set
	model: 'gpt-4',              // Default model to use
	timeout: '30s',              // Request timeout
	networkRTT: 25,              // Optional RTT in milliseconds for adjusted latency metrics
	tokenMultiplier: 1.33,       // Optional local token-estimation multiplier
	prefillConcurrency: 32,      // Optional shared cap on in-flight prefill work
	warmupCount: 10,             // Optional initial requests to exclude from metrics
	maxTTFT: 2000,               // Optional goodput SLO in milliseconds
	maxTTFO: 2500,               // Optional goodput SLO in milliseconds
	maxTokenLatency: 100,        // Optional goodput SLO in milliseconds
});
```

The extension also emits k6's built-in HTTP metrics for each recorded request.
Requests skipped by `warmupCount` are excluded from both `llm_*` metrics and
native `http_*` metrics.

When `baseURLs` is set, requests are distributed round-robin across the
normalized endpoint list. `baseURL` remains supported for single-endpoint tests.
When provider usage data is missing, local token estimates use a conservative
word-and-character fallback; `tokenMultiplier` tunes the word-count side of that
estimate.

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `OPENAI_API_KEY` | API key for authentication | - |
| `ENDPOINT_URL` | Base URL for the LLM API | https://api.openai.com/v1 |
| `ENDPOINT_URLS` | Comma-separated base URLs for round-robin dispatch | - |
| `MODEL` | Default model to use | gpt-4 |
| `TIMEOUT` | Request timeout | 30s |
| `NETWORK_RTT` | RTT in milliseconds subtracted from adjusted latency metrics | 0 |
| `TOKEN_MULTIPLIER` | Word-count multiplier for local token fallback | 1.33 |
| `PREFILL_CONCURRENCY` | Shared cap on in-flight prefill work for the same endpoint and model | 0 |
| `WARMUP_COUNT` | Initial requests per client to execute without recording LLM metrics | 0 |
| `STREAM_MODE` | Enable streaming responses | false |

## Examples

Check out the [examples](./examples) directory for more usage scenarios:

- `examples/llm-test.js`: local streaming smoke test.
- `examples/openai-test.js`: OpenAI-hosted endpoint example.
- `examples/sample-load-test.js`: ramping load test with JSON summary output.
- `examples/trace-replay.js`: trace/profile replay from `examples/trace.json`.

## Development

```bash
# Clone the repository
git clone https://github.com/tmc/misc/xk6-llm-perf
cd xk6-llm-perf

# Build
make build

# Run tests
make test

# Run all local checks
make check

# Run example load test
make example-loadtest
```

## Project Files

- [PRODUCTION-READINESS.md](./PRODUCTION-READINESS.md): state-of-the-art
  feature and disposition record.
- [CONTRIBUTING.md](./CONTRIBUTING.md): development loop and PR checklist.
- [SECURITY.md](./SECURITY.md): private reporting and operational guidance.
- [CHANGELOG.md](./CHANGELOG.md): unreleased changes.
- [LICENSE](./LICENSE): MIT license.

## Trace Replay

`examples/trace-replay.js` replays a JSON trace with per-row delay and request
shape fields. This keeps arrival modeling in k6's script layer while the
extension records LLM and native HTTP metrics.

```bash
ENDPOINT_URL=http://localhost:8000/v1 \
OPENAI_API_KEY=test \
./dist/k6 run examples/trace-replay.js
```

See [PRODUCTION-READINESS.md](./PRODUCTION-READINESS.md) for the current
state-of-the-art delta disposition.
