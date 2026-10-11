import llm from 'k6/x/llm-perf';

export const options = {vus: 1, iterations: 1, thresholds: {
  llm_work_accepted: ['count==1'], llm_work_rejected: ['count==1'],
  llm_work_unavailable: ['count==1'], llm_successes: ['count==3'],
  llm_work_good_requests: ['count==1'], llm_work_acceptance: ['rate>0.333', 'rate<0.334'],
}};

export function setup() {
  llm.beginMeasurement({runId: 'work-qualification-smoke', maxRecords: 20});
}

export default function () {
  const client = new llm.Client({baseURL: __ENV.BASE_URL, model: __ENV.MODEL || 'fixture', maxConnsPerHost: 16});
  for (const test of [
    {work: {expectedModel: __ENV.MODEL || 'fixture', allowedFinishReasons: ['stop'], minOutputTokens: 1, maxOutputTokens: 32, requireExactUsage: true}, expected: 'accepted'},
    {work: {expectedModel: '__deliberately_wrong_model__'}, expected: 'rejected'},
    {work: {tokenIDsDigest: '0'.repeat(64)}, expected: 'unavailable'},
  ]) {
    const request = {
      messages: [{role: 'user', content: 'Say hello.'}],
      max_tokens: 32,
      work: test.work,
    };
    const response = test.expected === 'rejected' ? client.request({
      path: '/chat/completions', protocol: 'openai-chat', body: {
        model: __ENV.MODEL || 'fixture', maxConnsPerHost: 16, messages: request.messages, max_tokens: 32,
      }, work: test.work,
    }) : client.chat.completions.create(request);
    if (response.measurement.outcome !== 'success' || response.measurement.work.status !== test.expected) {
      throw new Error(`unexpected qualification: ${JSON.stringify(response.measurement)}`);
    }
  }
}

export function teardown() { llm.endMeasurement(); }

export function handleSummary(data) {
  const run = llm.measurementSummary();
  if (run.successes !== 3 || run.work_qualified !== 3 || run.work_accepted !== 1 ||
      run.work_rejected !== 1 || run.work_unavailable !== 1 || run.work_good_requests !== 1 ||
      data.metrics.llm_work_acceptance.values.rate !== 1 / 3) {
    throw new Error(`unexpected qualification populations: ${JSON.stringify(run)}`);
  }
  return {[__ENV.RUN_FILE || 'work-qualification-run.json']: JSON.stringify(run)};
}
