import llm from 'k6/x/llm-perf';

export const options = {
    vus: 1,
    duration: '30s',
    thresholds: {
        'llm_ttft': ['p(95)<2000'],
        'llm_ttfo': ['p(95)<2500'],
        'llm_token_latency': ['avg<100'],
        'llm_inter_chunk_latency': ['p(95)<250'],
        'llm_request_latency': ['p(95)<5000'],
        'llm_good_request': ['rate>0.95'],
        'llm_tokens_per_second': ['value>10'],
    },
};

const client = new llm.Client({
    baseURL: __ENV.ENDPOINT_URL || 'https://api.openai.com/v1',
    baseURLs: (__ENV.ENDPOINT_URLS || '').split(',').filter(Boolean),
    model: __ENV.MODEL || 'gpt-4',
    apiKey: __ENV.OPENAI_API_KEY,
    timeout: __ENV.TIMEOUT || '30s',
    networkRTT: Number(__ENV.NETWORK_RTT || '0'),
    tokenMultiplier: Number(__ENV.TOKEN_MULTIPLIER || '1.33'),
    prefillConcurrency: Number(__ENV.PREFILL_CONCURRENCY || '0'),
    warmupCount: Number(__ENV.WARMUP_COUNT || '0'),
    maxTTFT: 2000,
    maxTTFO: 2500,
    maxTokenLatency: 100,
});

export default function() {
    const response = client.chat.completions.create({
        messages: [{
            role: "user",
            content: "Write a haiku about load testing"
        }],
        stream: true,
    });

    check(response, {
        'is successful': (r) => r.status === 200,
        'has response content': (r) => r.choices?.[0]?.message?.content?.length > 0,
    });
}
