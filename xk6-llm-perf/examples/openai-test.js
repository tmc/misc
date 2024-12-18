import llm from 'k6/x/llm-perf';

export const options = {
    vus: 1,
    duration: '30s',
    thresholds: {
        'llm_ttft': ['p(95)<2000'],
        'llm_token_latency': ['avg<100'],
        'llm_tokens_per_second': ['value>10'],
        'llm_openai_tokens_per_second': ['avg>10'],
        'llm_cost_total': ['value<10'],
    },
};

export default function() {
    const client = new llm.Client({
        provider: 'openai',
        baseURL: 'https://api.openai.com/v1',
        model: 'gpt-4',
        apiKey: __ENV.OPENAI_API_KEY,
    });

    const response = client.complete({
        messages: [{
            role: "user",
            content: "Write a haiku about load testing"
        }],
        stream: true,
    });

    check(response, {
        'is successful': (r) => r.status === 200,
        'has metadata': (r) => r.metadata !== null,
        'tokens per second > 10': (r) => {
            const metadata = JSON.parse(r.metadata.provider_metadata);
            return metadata.tokens_per_second > 10;
        },
    });
}
