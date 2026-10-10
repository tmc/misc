import llm from 'k6/x/llm-perf';
import { check } from 'k6';

const warmupSeconds = Number(__ENV.WARMUP_SECONDS || '5');
const measurementSeconds = Number(__ENV.MEASUREMENT_SECONDS || '30');
const cooldownSeconds = Number(__ENV.COOLDOWN_SECONDS || '1');
if (!Number.isFinite(warmupSeconds) || warmupSeconds < 1 ||
    !Number.isFinite(measurementSeconds) || measurementSeconds < 1 ||
    !Number.isFinite(cooldownSeconds) || cooldownSeconds < 1) {
    throw new Error('phase durations must be finite and at least one second');
}
const vus = Number(__ENV.VUS || '1');
export const options = {
    scenarios: {
        warmup: { executor: 'constant-vus', vus, duration: `${warmupSeconds}s`, gracefulStop: '0s', exec: 'warmup' },
        cooldown: { executor: 'constant-vus', vus, startTime: `${warmupSeconds + measurementSeconds}s`, duration: `${cooldownSeconds}s`, gracefulStop: '5s', exec: 'cooldown' },
        measurement: { executor: 'constant-vus', vus, startTime: `${warmupSeconds}s`, duration: `${measurementSeconds}s`, gracefulStop: '5s', exec: 'measure' },
    },
    thresholds: {
        'checks{scenario:measurement}': ['rate==1'],
        llm_slo_compliance: ['rate>=0.95'],
        llm_failures: ['count==0'],
        llm_successes: ['count>0'],
    },
};

function client(phase) {
    return new llm.Client({
        baseURL: __ENV.ENDPOINT_URL,
        apiKey: __ENV.OPENAI_API_KEY,
        model: __ENV.MODEL || 'fixture', phase,
        strictMeasurement: true, timeout: __ENV.TIMEOUT || '30s',
        maxTTFT: Number(__ENV.MAX_TTFT || '2000'),
        maxTTFO: Number(__ENV.MAX_TTFO || '2500'),
        rttSource: __ENV.RTT_SOURCE || 'unspecified',
        networkRTT: Number(__ENV.NETWORK_RTT || '0'),
    });
}
const warmupClient = client('warmup');
const measuredClient = client('measurement');
const cooldownClient = client('cooldown');

export function setup() {
    llm.beginMeasurement({
        runId: __ENV.RUN_ID || 'local', maxRecords: Number(__ENV.MAX_RECORDS || '10000'),
        traceHash: __ENV.TRACE_HASH || '', seed: Number(__ENV.SEED || '0'), workload: 'phased-load-test',
        startDelay: `${warmupSeconds}s`, duration: `${measurementSeconds}s`,
    });
}
function complete(c) {
    const response = c.chat.completions.create({ messages: [{ role: 'user', content: 'Explain HTTP keep-alive.' }], stream: true, max_tokens: 128 });
    check(response, { successful: (r) => !(r instanceof Error) && r.status === 200 });
}
export function warmup() { complete(warmupClient); }
export function measure() { complete(measuredClient); }
export function cooldown() { complete(cooldownClient); }
export function teardown() { llm.endMeasurement(); }
export function handleSummary(data) {
    const run = llm.measurementSummary();
    const records = run.records;
    return {
        'outputs/run.json': JSON.stringify(run, null, 2),
        'outputs/records.jsonl': records.map((r) => JSON.stringify(r)).join('\n') + '\n',
        'outputs/k6-summary.json': JSON.stringify(data, null, 2),
        'outputs/workload.json': JSON.stringify({ options, warmup_seconds: warmupSeconds, measurement_seconds: measurementSeconds, cooldown_seconds: cooldownSeconds, graceful_stop_seconds: 5 }, null, 2),
    };
}
