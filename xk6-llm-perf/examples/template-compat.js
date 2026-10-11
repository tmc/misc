import { check } from 'k6';
import { Client } from 'k6/x/llm-perf';

// Generate this artifact with scripts/template-compat.py before starting k6.
const rendered = JSON.parse(open(__ENV.TEMPLATE_FILE));
if (rendered.schema_version !== 1 || rendered.operation !== 'render') {
  throw new Error('invalid rendered template artifact');
}
const client = new Client({ baseURL: __ENV.BASE_URL, strictMeasurement: true });
export const options = { vus: 1, iterations: 1, thresholds: { checks: ['rate==1'] } };

export default function () {
  const response = client.request({
    path: __ENV.REQUEST_PATH || '/custom',
    protocol: __ENV.RESPONSE_PROTOCOL || 'json',
    body: rendered.value,
  });
  if (response instanceof Error) throw response;
  check(response, { 'request succeeded': r => r.status === 200 });
}
