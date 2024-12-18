# xk6-llm-perf

A k6 extension for load testing Large Language Model APIs with comprehensive metrics and OpenAI SDK-compatible interface.

## Features

- 🚀 OpenAI SDK-compatible API
- 📊 Comprehensive LLM-specific metrics
- 🌊 Support for both streaming and non-streaming responses
- 📈 Real-time performance monitoring
- 🔄 Automatic retry handling
- ⚡ Efficient token counting

## Quick Start

```bash
# Install k6 with the extension
go install go.k6.io/xk6/cmd/xk6@latest
xk6 build --with github.com/tmc/misc/xk6-llm-perf@latest

# Run a load test
K6_ENDPOINT_URL=https://api.openai.com/v1 \
K6_OPENAI_API_KEY=your-api-key \
./k6 run examples/basic-load-test.js
```

## Usage

```javascript
import llm from 'k6/x/llm-perf';

export const options = {
    vus: 10,
    duration: '30s',
    thresholds: {
        'llm_ttft': ['p(95)<2000'],         // Time to First Token
        'llm_token_latency': ['avg<100'],    // Token Generation Latency
        'llm_tokens_per_second': ['value>5'], // Token Generation Rate
        'llm_total_tokens': ['count>0'],     // Total Tokens Generated
        'llm_errors': ['count<10'],          // Error Count
    },
};

const client = new llm.Client({
    apiKey: __ENV.OPENAI_API_KEY,
    baseURL: __ENV.ENDPOINT_URL,
    model: 'gpt-4',
    timeout: '30s',
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
| `llm_token_latency` | Trend | Inter-token latency for streaming responses |
| `llm_tokens_per_second` | Gauge | Token generation rate |
| `llm_total_tokens` | Counter | Total tokens generated |
| `llm_errors` | Counter | Number of failed requests |

## Configuration

The client accepts the following configuration options:

```javascript
const client = new llm.Client({
    apiKey: 'your-api-key',      // API key for authentication
    baseURL: 'https://...',      // Base URL for the LLM API
    model: 'gpt-4',              // Default model to use
    timeout: '30s',              // Request timeout
    maxRetries: 3,               // Maximum number of retries
});
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `OPENAI_API_KEY` | API key for authentication | - |
| `ENDPOINT_URL` | Base URL for the LLM API | https://api.openai.com/v1 |
| `MODEL` | Default model to use | gpt-4 |
| `TIMEOUT` | Request timeout | 30s |
| `STREAM_MODE` | Enable streaming responses | false |

## Examples

Check out the [examples](./examples) directory for more usage scenarios:

- Basic load test
- Streaming responses
- Custom prompts
- Error handling
- Advanced scenarios

## Development

```bash
# Clone the repository
git clone https://github.com/tmc/misc/xk6-llm-perf
cd xk6-llm-perf

# Build
make build

# Run tests
make test

# Run example load test
make example-loadtest
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT License - see [LICENSE](LICENSE) for details.
