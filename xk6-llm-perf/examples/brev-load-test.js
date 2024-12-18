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
    // change these to all be trends:
    thresholds: {
        'llm_ttft': ['p(95)<2000'],
        'llm_token_latency': ['avg<100'],
        'llm_completion_time': ['p(95)<10000'],
        'llm_tokens_per_second': ['p(95)>5'],
        'llm_request_duration': ['p(95)<10000'],
        'llm_errors': ['count<10'],
    },
};

export default function() {
    const client = new llm.Client({
        baseURL: __ENV.ENDPOINT_URL || 'https://localhost:9000/v1',
        isStreaming: __ENV.STREAM_MODE === 'true',
        httpTimeout: '30s',
    });

    try {
        const response = client.complete({
            model: "meta-llama/Llama-3.3-70B-Instruct",
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
            'has response content': (r) => r.text && r.text.length > 0,
        });

        sleep(Math.random() * 2 + 1);
    } catch (error) {
        console.error('Request failed:', error);
    }
}

export function handleSummary(data) {
    return {
        'outputs/summary.json': JSON.stringify(data),
        stdout: textSummary(data, { indent: ' ', enableColors: true }),
    };
}

