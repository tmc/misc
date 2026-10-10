import { Client, renderRequestTemplate } from 'k6/x/llm-perf';

import { Counter } from 'k6/metrics';
const passed = new Counter('fixture_passed');
export const options = { vus: 1, iterations: 1, thresholds: { fixture_passed: ['count==1'], llm_successes: ['count==12'] } };
const client = new Client({ baseURL: __ENV.BASE_URL, model: 'fixture', strictMeasurement: true });

export default function () {
  const cases = [
    ['chat-embeddings', { model: 'fixture', messages: [{ role: 'user', content: [{ type: 'text', text: 'hello' }, { type: 'image_url', image_url: { url: 'data:image/png;base64,aGVsbG8=' } }] }] }],
    ['nim-embeddings', { model: 'fixture', texts: ['hello'], images: ['data:image/png;base64,aGVsbG8='] }],
    ['nv-embedqa', { texts: ['hello'] }],
    ['openai-embeddings', { model: 'fixture', texts: ['hello'] }],
    ['cohere-rankings', { model: 'fixture', query: 'hello', passages: ['world'] }],
    ['nim-rankings', { model: 'fixture', query: 'hello', passages: [{ text: 'world' }] }],
    ['hf-tei-rankings', { query: 'hello', passages: ['world'] }],
    ['nim-image-retrieval', { images: [{ type: 'image_url', url: 'data:image/png;base64,aGVsbG8=' }] }],
  ];
  for (const [protocol, variables] of cases) {
    const rendered = renderRequestTemplate({ name: protocol, variables });
    if (rendered.provenance.schemaVersion !== 1 || rendered.provenance.payloadSHA256.length !== 64) throw new Error('template receipt missing');
    const response = client.request({ path: '/' + protocol, protocol, body: rendered.body });
    if (response instanceof Error) throw response;
    if (response.status !== 200 || !response.body || (response.choices || []).length !== 0) throw new Error('non-generation contract failed');
    if (response.measurement.output_tokens.source !== 'unavailable') throw new Error('invented output tokens');
  }
  for (const [path, input] of [['single-token', [1, 2]], ['batch-token', [[1, 2], [3]]]]) {
    const response = client.request({ path: '/' + path, protocol: 'openai-embeddings', body: { model: 'fixture', input, encoding_format: 'base64' } });
    if (response instanceof Error) throw response;
    if (response.body.data[0].embedding !== 'AACAPwAAAEA=' || response.measurement.output_tokens.source !== 'unavailable') throw new Error('base64/token input contract');
  }
  const rag = renderRequestTemplate({ name: 'solido-rag', variables: { model: 'fixture', queries: ['hello'] } });
  for (const stream of [false, true]) {
    const response = client.request({ path: stream ? '/solido-rag-stream' : '/solido-rag', protocol: 'solido-rag', body: rag.body, stream });
    if (response instanceof Error) throw response;
    if (response.choices[0].message.content !== 'hello world' || !response.sources || response.sources[0].document !== 'fixture.pdf') throw new Error('SOLIDO content or sources lost');
    if (response.measurement.output_tokens.value !== 3 || response.measurement.output_tokens.source !== 'reported' || (stream && (response.measurement.ttft === null || response.measurement.ttfo === null))) throw new Error('SOLIDO generation measurements');
  }
  const authored = renderRequestTemplate({ body: { input: { $var: 'texts' }, model: { $var: 'model' }, literal: '{{ literal }}' }, variables: { texts: ['hello'], model: 'fixture' } });
  if (authored.body.literal !== '{{ literal }}') throw new Error('literal interpreted');
  let rejected = false;
  try { renderRequestTemplate({ body: { input: { $var: 'missing' } }, variables: {} }); } catch (e) { rejected = true; }
  if (!rejected) throw new Error('missing template variable accepted');
  passed.add(1);
}
