import llm from 'k6/x/llm-perf';
import { check } from 'k6';
export const options = { vus: 2, iterations: 6, thresholds: { checks: ['rate==1'], llm_successes: ['count==6'], llm_failures: ['count==0'] } };
const client = new llm.Client({ baseURL: __ENV.ENDPOINT_URL, model: 'fixture', strictMeasurement: true });
export function setup() { llm.beginMeasurement({ runId: 'binary-fixture', maxRecords: 20 }); }
export default function () {
 const r = client.chat.completions.create({ messages: [{role:'user', content:'fixture'}], stream: true });
 check(r, { 'canonical measurement': r => !(r instanceof Error) && r.measurement.outcome === 'success' && typeof r.measurement.ttft === 'number' && r.measurement.output_tokens.source === 'reported' });
}
export function teardown() {
 const run = llm.endMeasurement();
 if (run.attempts !== 6 || run.records.length !== 6 || run.failures !== 0) throw new Error('shared run accounting failed: ' + JSON.stringify(run));
}
export function handleSummary() { return { [__ENV.RUN_FILE]: JSON.stringify(llm.measurementSummary()) }; }
