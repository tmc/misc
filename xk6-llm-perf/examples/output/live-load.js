import llm from 'k6/x/llm-perf';
export const options = {vus: 1, iterations: 3, thresholds: {llm_successes: ['count==3']}};
const client = new llm.Client({baseURL: __ENV.ENDPOINT_URL, model: 'fixture', strictMeasurement: true});
export default function () {
  const response = client.chat.completions.create({messages: [{role: 'user', content: 'fixture'}], stream: false});
  if (!response.measurement) throw new Error('request did not produce a measurement');
  console.log(JSON.stringify(response.measurement));
}
export function handleSummary() {return {};}
