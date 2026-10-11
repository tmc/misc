import llm from 'k6/x/llm-perf';
import { check } from 'k6';
import { conversation } from '../lib/workload.js';

export const options = { vus: 1, iterations: 1, thresholds: { checks: ['rate==1'] } };

const client = new llm.Client({ baseURL: __ENV.ENDPOINT_URL, model: __ENV.MODEL });

export default function () {
    const results = conversation(client, [
        { prompt: 'Describe a B-tree in one sentence.', request: { max_tokens: 64, temperature: 0 } },
        { prompt: 'When would you use one?', request: { max_tokens: 96, temperature: 0 } },
    ], [{ role: 'system', content: 'Answer briefly.' }]);
    check(results, { 'both turns completed': rows => rows.length === 2 && rows.every(row => !(row instanceof Error)) });
}
