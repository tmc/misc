import llm from 'k6/x/llm-perf';
import { check, sleep } from 'k6';
import { textSummary } from 'https://jslib.k6.io/k6-summary/0.0.2/index.js';

export const options = {
    vus: 1,
    duration: '30s',
    thresholds: {
        'llm_ttft': ['p95<500'],
        'llm_token_latency': ['avg<50'],
        'llm_tokens_per_second': ['value>10'],
    },
};

const outputConfig = {
    directory: 'outputs',
    format: 'json',
    includeMetadata: true,
    includeTokens: true,
    includeTimings: true,
    har: {
        enabled: true,
        includeResponses: true,
        outputDir: 'outputs/har',
        filePattern: 'request-%d.har'
    }
};

const client = new llm.Client({
    baseURL: 'http://localhost:8080/v1',
    storage: outputConfig,
    httpTimeout: '30s',
});

export default function() {
    const params = {
        prompt: "Explain quantum computing",
        maxTokens: 200,
        temperature: 0.7,
        topP: 0.9,
        frequencyPenalty: 0.0,
        presencePenalty: 0.0,
        stopSequences: ["\n\n"],
        streaming: true,
    };

    const response = client.complete(params);

    check(response, {
        'completion successful': (r) => r.status === 200,
        'time to first token < 500ms': (r) => r.metrics.timeToFirstToken < 500,
        'token throughput > 10 tokens/sec': (r) => r.metrics.tokensPerSecond > 10,
    });

    sleep(1);
}

