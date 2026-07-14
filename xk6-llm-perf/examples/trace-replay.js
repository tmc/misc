import llm from 'k6/x/llm-perf';
import { check, sleep } from 'k6';
import { SharedArray } from 'k6/data';

const trace = new SharedArray('trace', () => {
    const path = __ENV.TRACE_FILE || 'trace.json';
    return JSON.parse(open(path));
});

export const options = {
    scenarios: {
        trace_replay: {
            executor: 'shared-iterations',
            vus: Number(__ENV.VUS || '1'),
            iterations: trace.length,
            maxDuration: __ENV.MAX_DURATION || '30m',
        },
    },
    thresholds: {
        'llm_ttft': ['p(95)<2000'],
        'llm_ttfo': ['p(95)<2500'],
        'llm_good_request': ['rate>0.95'],
        'http_req_failed': ['rate<0.01'],
    },
};

const client = new llm.Client({
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
});

export default function() {
    const row = trace[__ITER % trace.length];
    const delay = Number(row.delay_seconds || row.delaySeconds || 0);
    if (delay > 0) {
        sleep(delay);
    }

    const response = client.chat.completions.create({
        model: row.model || undefined,
        messages: row.messages || [{ role: 'user', content: String(row.prompt || '') }],
        max_tokens: Number(row.max_tokens || row.maxTokens || 256),
        temperature: Number(row.temperature || 0),
        stream: row.stream !== false,
    });

    check(response, {
        'completion successful': (r) => r.status === 200,
        'has response content': (r) => r.choices?.[0]?.message?.content?.length > 0,
    });
}
