import llm from 'k6/x/llm-perf';
import { check, sleep } from 'k6';

export const options = {
    vus: 1,
    duration: '30s',
    thresholds: {
        'llm_ttft': ['p(95)<500'],
        'llm_ttfo': ['p(95)<750'],
        'llm_token_latency': ['avg<50'],
        'llm_inter_chunk_latency': ['p(95)<100'],
        'llm_request_latency': ['p(95)<5000'],
        'llm_good_request': ['rate>0.95'],
        'llm_tokens_per_second': ['value>10'],
    },
};

const client = new llm.Client({
    apiKey: __ENV.OPENAI_API_KEY || 'local-key',
    baseURL: __ENV.ENDPOINT_URL || 'http://localhost:8080/v1',
    baseURLs: (__ENV.ENDPOINT_URLS || '').split(',').filter(Boolean),
    model: __ENV.MODEL || 'gpt-4',
    timeout: __ENV.TIMEOUT || '30s',
    networkRTT: Number(__ENV.NETWORK_RTT || '0'),
    tokenMultiplier: Number(__ENV.TOKEN_MULTIPLIER || '1.33'),
    prefillConcurrency: Number(__ENV.PREFILL_CONCURRENCY || '0'),
    warmupCount: Number(__ENV.WARMUP_COUNT || '0'),
    maxTTFT: 500,
    maxTTFO: 750,
    maxTokenLatency: 50,
});

export default function() {
    const response = client.chat.completions.create({
        messages: [
            { role: 'user', content: 'Explain quantum computing in one paragraph.' },
        ],
        max_tokens: 200,
        temperature: 0.7,
        stream: true,
    });

    check(response, {
        'completion successful': (r) => r.status === 200,
        'has response content': (r) => r.choices?.[0]?.message?.content?.length > 0,
    });

    sleep(1);
}
