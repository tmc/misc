import llm from 'k6/x/llm-perf';
import { check, sleep } from 'k6';
import { textSummary } from 'https://jslib.k6.io/k6-summary/0.0.2/index.js';

export const options = {
    scenarios: {
        ramping_load: {
            executor: 'ramping-vus',
            startVUs: 1,
            stages: [
                { duration: '2m', target: 5 },
                { duration: '3m', target: 5 },
                { duration: '2m', target: 10 },
                { duration: '3m', target: 10 },
            ],
        },
    },
    thresholds: {
        'llm_ttft': ['p(95)<2000'],
        'llm_ttfo': ['p(95)<2500'],
        'llm_token_latency': ['avg<100'],
        'llm_inter_chunk_latency': ['p(95)<250'],
        'llm_request_latency': ['p(95)<5000'],
        'llm_good_request': ['rate>0.95'],
        'llm_tokens_per_second': ['value>5'],
        'llm_completion_tokens': ['count>0'],
        'llm_errors': ['count<10'],
    },
};

const config = {
    apiKey: __ENV.OPENAI_API_KEY,
    baseURL: __ENV.ENDPOINT_URL || 'https://api.openai.com/v1',
    baseURLs: (__ENV.ENDPOINT_URLS || '').split(',').filter(Boolean),
    model: __ENV.MODEL || 'gpt-4',
    timeout: __ENV.TIMEOUT || '30s',
    networkRTT: Number(__ENV.NETWORK_RTT || '0'),
    tokenMultiplier: Number(__ENV.TOKEN_MULTIPLIER || '1.33'),
    prefillConcurrency: Number(__ENV.PREFILL_CONCURRENCY || '0'),
    warmupCount: Number(__ENV.WARMUP_COUNT || '0'),
    maxTTFT: Number(__ENV.MAX_TTFT || '2000'),
    maxTTFO: Number(__ENV.MAX_TTFO || '2500'),
    maxTokenLatency: Number(__ENV.MAX_TOKEN_LATENCY || '100'),
};

const client = new llm.Client(config);

export default function() {
    try {
        const response = client.chat.completions.create({
            messages: [
                {
                    role: "user",
                    content: "How tough was Roger Federer in his prime at tennis"
                }
            ],
            temperature: 0.7,
            max_tokens: 1000,
            stream: __ENV.STREAM_MODE === 'true',
        });

        check(response, {
            'completion successful': (r) => r.status === 200,
            'has response content': (r) => r.choices?.[0]?.message?.content?.length > 0,
        });

        const delay = Math.random() * 2 + 1;
        sleep(delay);
    } catch (error) {
        console.error('Request failed:', error);
    }
}

export function handleSummary(data) {
    return {
        'outputs/summary.json': JSON.stringify(data, null, 2),
        stdout: textSummary(data, { indent: ' ', enableColors: true }),
    };
}
