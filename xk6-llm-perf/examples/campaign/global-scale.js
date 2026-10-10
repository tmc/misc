// Admission caps are shared by all VUs in a worker's extension root; balanced
// worker quotas keep the global cap bounded without assuming mutable k6 VUs.
import llm from 'k6/x/llm-perf';
import http from 'k6/http';
import { sleep } from 'k6';
import { Counter } from 'k6/metrics';
const worker = Number(__ENV.SCALE_WORKER_ID);
const controlURL = __ENV.SCALE_CONTROL_URL;
if (!Number.isSafeInteger(worker) || worker < 0 || !controlURL) throw new Error('missing worker identity/control URL');
const receipts = new Counter('campaign_global_samples');
const epochs = new Counter('campaign_global_epochs');
export const options = {
    scenarios: { worker: { executor: 'per-vu-iterations', vus: Number(__ENV.SCALE_MAX_VUS), iterations: 1,
        maxDuration: __ENV.SCALE_RUN_DURATION, gracefulStop: '1s' } },
    thresholds: { campaign_global_epochs: ['count>0'] },
};
llm.setControlLimits('campaign-scale', { request: 1, prefill: 1 });
const client = new llm.Client({ baseURL: __ENV.ENDPOINT_URL,
    apiKey: __ENV.OPENAI_API_KEY, model: __ENV.MODEL || 'fixture',
    timeout: __ENV.REQUEST_TIMEOUT || '2s', strictMeasurement: true,
    controlGroup: 'campaign-scale' });
let sequence = 0, lastEpoch = 0;
function iterate() {
    const controlResponse = http.get(`${controlURL}/control?worker=${worker}`, { timeout: '1s' });
    if (controlResponse.status !== 200) throw new Error('control response failed');
    const control = controlResponse.json();
    if (control.stop) return false;
    if (control.active && lastEpoch !== control.epoch) { epochs.add(1); lastEpoch = control.epoch; }
    if (!control.active || Date.now() >= control.end || !control.request || !control.prefill) { sleep(0.01); return true; }
    llm.setControlLimits('campaign-scale', { request: control.request, prefill: control.prefill });
    const snapshot = llm.controlSnapshot('campaign-scale');
    const response = client.chat.completions.create({
        messages: [{ role: 'user', content: 'Return ok.' }], stream: true,
    });
    const measurement = response.measurement;
    if (!measurement || !measurement.attempted) return true;
    const result = http.post(`${controlURL}/sample?worker=${worker}`, JSON.stringify({
        id: `${worker}:${__VU}:${sequence++}`, worker, epoch: control.epoch,
        dispatched_at: measurement.dispatched_at, finished_at: measurement.finished_at,
        latency_ms: measurement.full_response_latency, outcome: measurement.outcome,
        requests: snapshot.requests, prefills: snapshot.prefills,
    }), { headers: { 'Content-Type': 'application/json' }, timeout: '1s' });
    if (result.status !== 202 && result.status !== 409 && result.status !== 429) throw new Error('sample receipt failed');
    receipts.add(1);
    return true;
}
export default function () { while (iterate()) {} }
