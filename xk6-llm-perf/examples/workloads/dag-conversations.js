// Four bounded requests. Use an owned endpoint and retain its independent logs.
// http.asyncRequest supplies overlap; the extension Client remains synchronous.
import http from 'k6/http';
import { check } from 'k6';
import { dagConversations } from '../lib/workload.js';

export const options = {
    scenarios: { dag: { executor: 'shared-iterations', vus: 1, iterations: 1, maxDuration: '20s' } },
    thresholds: { checks: ['rate==1'], http_req_failed: ['rate==0'] },
};
const endpoint = __ENV.DAG_ENDPOINT_URL;
if (!endpoint) throw new Error('DAG_ENDPOINT_URL required');
const m = content => ({ role: 'user', content });
const graph = [
    { session_id: 'rounds', orchestrator: true, spawns: ['parent'], rounds: 2, think_time_ms: 3 },
    { session_id: 'parent', pre_session_spawns: ['child'], turns: [
        { messages: [m('parent')], delay: 5 },
    ] },
    { session_id: 'child', turns: [{ messages: [m('fresh child')], delay: 2 }] },
];
let receipt;
export default async function () {
    const attempts = [];
    const results = await dagConversations(graph, async (request, identity) => {
        const attempt = { ...identity, dispatchedAt: Date.now(), outcome: 'pending' };
        attempts.push(attempt);
        try {
            const response = await http.asyncRequest('POST', endpoint + '/chat/completions', JSON.stringify({ ...request, model: 'fixture', stream: false }), {
                headers: { 'Content-Type': 'application/json', 'X-Session-Id': identity.sessionId, 'X-Turn-Index': String(identity.turnIndex), 'X-Instance-Id': identity.instanceId },
                timeout: '5s', tags: { name: 'dag-turn' },
            });
            if (response.status !== 200) throw new Error('DAG HTTP status ' + response.status);
            const body = response.json();
            attempt.outcome = 'success';
            return body.choices[0].message;
        } catch (error) { attempt.outcome = 'failure'; attempt.error = String(error); throw error; }
        finally { attempt.finishedAt = Date.now(); }
    });
    receipt = { schemaVersion: 1, transport: 'k6/http.asyncRequest', extensionMeasurement: false, results: Object.fromEntries(results), attempts };
    check(receipt, {
        'all sessions successful': r => Object.values(r.results).every(s => s.outcome === 'success'),
        'four attempts retained': r => r.attempts.length === 4,
        'five instances retained': r => Object.keys(r.results).length === 5,
        'coordinator sends no request': r => Object.values(r.results).find(s => s.sessionId === 'rounds').turns.every(t => t.noRequest),
    });
}
export function handleSummary(data) {
    return { [__ENV.DAG_RECEIPT || 'dag-receipt.json']: JSON.stringify({ receipt, metrics: data.metrics }, null, 2) };
}
