import llm from 'k6/x/llm-perf';
import { check } from 'k6';
import { Counter } from 'k6/metrics';

const provenance = {
  reported: new Counter('dashboard_output_reported'),
  estimated: new Counter('dashboard_output_estimated'),
  client_declared: new Counter('dashboard_output_client'),
  unavailable: new Counter('dashboard_output_unavailable'),
};
const ttft = Number(__ENV.MAX_TTFT_MS || 1000);
const ttfo = Number(__ENV.MAX_TTFO_MS || 2000);
const tokenLatency = Number(__ENV.MAX_TOKEN_LATENCY_MS || 100);
export const options = {
  scenarios: {
    arrivals: {
      executor: 'constant-arrival-rate',
      rate: Number(__ENV.RATE || 2), timeUnit: '1s',
      duration: __ENV.DURATION || '60s',
      preAllocatedVUs: Number(__ENV.VUS || 4),
      maxVUs: Number(__ENV.MAX_VUS || 16),
    },
  },
  thresholds: {
    checks: ['rate==1'], llm_successes: ['count>0'],
    llm_failures: ['count==0'], dropped_iterations: ['count==0'],
    llm_slo_compliance: ['rate>=0.95'],
  },
};
const client = new llm.Client({
  baseURL: __ENV.ENDPOINT_URL || 'http://127.0.0.1:8088/v1',
  apiKey: __ENV.OPENAI_API_KEY || 'local-key',
  model: __ENV.MODEL || 'fixture', timeout: __ENV.TIMEOUT || '15s',
  strictMeasurement: true,
  maxTTFT: ttft, maxTTFO: ttfo, maxTokenLatency: tokenLatency,
});
export default function () {
  const response = client.chat.completions.create({
    messages: [{ role: 'user', content: __ENV.PROMPT || 'Count from one to four.' }],
    stream: true, max_tokens: Number(__ENV.MAX_TOKENS || 32), temperature: 0,
  });
  check(response, { 'successful completion': r => !(r instanceof Error) && r.measurement?.outcome === 'success' });
  const source = response.measurement?.output_tokens?.source;
  (provenance[source] || provenance.unavailable).add(1);
}
export function handleSummary(data) {
  return { [__ENV.SUMMARY_FILE || 'dashboard-summary.json']: JSON.stringify(data, null, 2) };
}
