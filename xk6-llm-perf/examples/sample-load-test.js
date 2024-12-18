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
        'llm_token_latency': ['avg<100'],
        'llm_tokens_per_second': ['value>5'],
        'llm_total_tokens': ['count>0'],
        'llm_errors': ['count<10'],
    },
};

const config = {
    apiKey: __ENV.OPENAI_API_KEY || 'default-key',
    baseURL: __ENV.ENDPOINT_URL || 'https://api.openai.com/v1',
    model: __ENV.MODEL || 'gpt-4',
    timeout: __ENV.TIMEOUT || '30s',
};

console.log(`Starting test with config: ${JSON.stringify(config, null, 2)}`);

// Create client once, outside the default function
const client = new llm.Client(config);
console.log('Client created successfully');

export default function() {
    try {
        console.log('Starting request...');
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
            'completion successful': (r) => {
                console.log(`Response status: ${r.status}`);
                return r.status === 200;
            },
            'has response content': (r) => {
                const hasContent = r.choices && r.choices.length > 0 && r.choices[0].message.content.length > 0;
                console.log(`Has content: ${hasContent}`);
                if (hasContent) {
                    console.log(`First few chars of response: ${r.choices[0].message.content.substring(0, 50)}...`);
                }
                return hasContent;
            },
        });

        // Add a small random delay between requests
        const delay = Math.random() * 2 + 1;
        console.log(`Sleeping for ${delay} seconds...`);
        sleep(delay);
    } catch (error) {
        console.error('Request failed:', error);
        if (error.stack) {
            console.error('Stack trace:', error.stack);
        }
    }
}

export function handleSummary(data) {
    // Ensure outputs directory exists
    if (__ENV.CI !== 'true') {
        try {
            const fs = require('fs');
            if (!fs.existsSync('outputs')) {
                console.log('Creating outputs directory...');
                fs.mkdirSync('outputs');
            }
        } catch (error) {
            console.error('Failed to create outputs directory:', error);
        }
    }

    const summary = {
        'outputs/summary.json': JSON.stringify(data, null, 2),
        stdout: textSummary(data, { indent: ' ', enableColors: true }),
    };

    console.log('Test summary:', JSON.stringify(data, null, 2));
    return summary;
}
