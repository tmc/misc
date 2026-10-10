import llm, { Client } from 'k6/x/llm-perf';
import { check } from 'k6';

export const options = {
    vus: 1,
    iterations: 1,
    thresholds: {
        checks: ['rate==1'],
        llm_requests: ['count==2'],
        llm_completion_tokens: ['count>0'],
        http_req_failed: ['rate==0'],
        'llm_requests{fixture:offline}': ['count==2'],
    },
    tags: { fixture: 'offline' },
};
const client = new Client({ baseURL: __ENV.ENDPOINT_URL, model: 'fixture', timeout: '2s' });
export default function () {
    check(llm, { 'named and default constructors match': (m) => m.Client === Client });
    for (const stream of [false, true]) {
        const response = client.chat.completions.create({
            messages: [{ role: 'user', content: 'hello' }], stream,
        });
        check(response, {
            'status is 200': (r) => r.status === 200,
            'content reaches JS': (r) => r.choices[0].message.content === 'hello',
        });
    }
}
