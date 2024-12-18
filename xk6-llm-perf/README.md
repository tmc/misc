# xk6-llm-perf

A k6 extension for load testing LLM APIs with real-time metrics for both streaming and non-streaming responses.

## Features

- Streaming-specific metrics:
  - Time to First Token (TTFT)
  - Inter-token Latency
  - Streaming Token Rate

- Non-streaming metrics:
  - Total Completion Time
  - Overall Token Rate

- Common metrics:
  - Total Tokens Generated
  - Request Duration
  - Error Count

## Usage

```bash
# Run streaming test
K6_STREAMING=true k6 run script.js

# Run non-streaming test
k6 run script.js
```

## Metrics

### Streaming Mode
- `llm_ttft`: Time to first token (trend, milliseconds)
- `llm_token_latency`: Inter-token latency (trend, milliseconds)
- `llm_tokens_per_second_stream`: Token generation rate (gauge)

### Non-streaming Mode
- `llm_completion_time`: Total completion time (trend, milliseconds)
- `llm_tokens_per_second_sync`: Overall token rate (gauge)

### Common Metrics
- `llm_total_tokens`: Total tokens generated (counter)
- `llm_request_duration`: Overall request duration (trend, milliseconds)
- `llm_errors`: Error count (counter)

## Example

```javascript
import llm from 'k6/x/llm-perf';

export const options = {
    thresholds: {
        'llm_ttft': ['p95<2000'],
        'llm_token_latency': ['avg<100'],
        'llm_completion_time': ['p95<10000'],
        'llm_errors': ['count<10'],
    },
};

const client = new llm.Client({
    baseURL: 'https://your-llm-api.com/v1',
    isStreaming: true,
});

export default function() {
    const response = client.complete({
        messages: [{
            role: "user",
            content: "Your prompt here"
        }],
        stream: true,
    });
}
```

## License

MIT
