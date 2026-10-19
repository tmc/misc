// Replays converted inputs.json sessions without rewriting authored payloads.
import execution from 'k6/execution';
import { SharedArray } from 'k6/data';
import { check } from 'k6';
import { Counter } from 'k6/metrics';
import llm from 'k6/x/llm-perf';

const sessions = new SharedArray('raw sessions', () => {
    const rows = JSON.parse(open(__ENV.SESSIONS_FILE));
    if (!Array.isArray(rows) || !rows.length || rows.length > 10000) throw new Error('invalid sessions');
    return rows;
});
const vus = Number(__ENV.VUS || '1');
if (!Number.isInteger(vus) || vus < 1 || vus > 64) throw new Error('VUS must be 1..64');
if (!__ENV.ENDPOINT_URL) throw new Error('ENDPOINT_URL is required');
const failures = new Counter('raw_session_failures');
const turns = new Counter('raw_session_turns');
export const options = {
    scenarios: { sessions: { executor: 'shared-iterations', vus, iterations: sessions.length, maxDuration: '5m' } },
    thresholds: { checks: ['rate==1'], raw_session_failures: ['count==0'], raw_session_turns: ['count>0'] },
};
const client = new llm.Client({ baseURL: __ENV.ENDPOINT_URL, apiKey: __ENV.API_KEY, timeout: '30s' });

export default function () {
    const session = sessions[execution.scenario.iterationInTest];
    // A session stays on this VU. Bodies are already complete API requests;
    // responses never become context for later authored turns.
    for (const turn of session.turns) {
        const response = client.request(turn.request);
        const ok = !(response instanceof Error) && response.status >= 200 && response.status < 300;
        turns.add(1);
        failures.add(ok ? 0 : 1);
        check(response, { 'authored turn succeeds': () => ok });
        // Do not erase failures or skip remaining authored turns.
        console.log(JSON.stringify({ session_id: session.session_id, turn_id: turn.id,
            source_sha256: session.source_sha256, outcome: ok ? 'success' : 'failure',
            status: response.status || 0, measurement: response.measurement || null }));
    }
}
