import { check } from 'k6';
import { Trend } from 'k6/metrics';
import { Client, evaluateRuntimeTemplate } from 'k6/x/llm-perf';

const evaluator = {
  python: __ENV.TEMPLATE_PYTHON, pythonSHA256: __ENV.TEMPLATE_PYTHON_SHA256,
  script: __ENV.TEMPLATE_SCRIPT, scriptSHA256: __ENV.TEMPLATE_SCRIPT_SHA256,
};
const renderTime = new Trend('template_render_ms', true);
const selectionTime = new Trend('template_selection_ms', true);
const client = new Client({ baseURL: __ENV.BASE_URL, strictMeasurement: true });
export const options = { vus: 1, iterations: 2, thresholds: { checks: ['rate==1'] } };

function evaluate(request) {
  const result = evaluateRuntimeTemplate({ ...evaluator, request });
  if (result instanceof Error) throw result;
  return result;
}

export default function () {
  const rendered = evaluate({
    schema_version: 1, operation: 'render',
    template: '{"model":{{ model|tojson }},"input":{{ texts|tojson }}}',
    variables: { model: 'fixture', texts: [`hello-${__ITER}`] },
    extra: {}, turn_extra: {},
  });
  renderTime.add(rendered.elapsed_ms);
  const response = client.request({
    path: '/v1/embeddings', protocol: 'openai-embeddings', body: rendered.result.value,
  });
  if (response instanceof Error) throw response;
  // Selection is application data, not additional token or TTFT evidence.
  const selected = evaluate({
    schema_version: 1, operation: 'select', expression: 'data[0].embedding', response: response.body,
  });
  selectionTime.add(selected.elapsed_ms);
  check(selected, { 'runtime selected embedding': r => r.result.value.length === 2 });
  for (const request of [
    { schema_version: 1, operation: 'select', expression: 'missing', response: {} },
    { schema_version: 1, operation: 'render', template: '{"x":{{ cycler.__init__.__globals__.os.name|tojson }}}', variables: {}, extra: {}, turn_extra: {} },
  ]) {
    check(evaluateRuntimeTemplate({ ...evaluator, request }), { 'invalid runtime input rejected': r => r instanceof Error });
  }
}
