import llm from 'k6/x/llm-perf';
import exec from 'k6/execution';
import { check, sleep } from 'k6';
import { sha256 } from 'k6/crypto';

// execution-segment partitions k6's global iteration counter across workers.
const rawRows = open('./rows.json');
const rows = JSON.parse(rawRows);
if (!Array.isArray(rows) || rows.length < 2 || rows.length > 100000 || new Set(rows.map(r=>r.id)).size !== rows.length) throw new Error('bounded unique rows required');
const assetHash = 'sha256:'+sha256(open('./worker.js')+'\x00'+rawRows,'hex');
if (__ENV.TRACE_HASH !== assetHash) throw new Error('portable worker assets differ from declared trace hash');
const epoch = Number(__ENV.WINDOW_START_MS);
const duration = Number(__ENV.WINDOW_DURATION_MS || 3000);
const worker = __ENV.WORKER_ID || __ENV.POD_NAME;
if (!worker || !Number.isFinite(epoch) || !Number.isFinite(duration) || duration <= 0 || !__ENV.TRACE_HASH) throw new Error('worker identity, common window, and asset hash required');
export const options = { thresholds:{checks:['rate==1']}, scenarios: { shared: { executor:'shared-iterations', vus:2, iterations:rows.length, maxDuration:'1m' } } };
const client = new llm.Client({baseURL:__ENV.ENDPOINT_URL, model:'fixture', strictMeasurement:true});
export function setup() {
 llm.beginMeasurement({runId:__ENV.RUN_ID || 'distributed', workerId:worker, maxRecords:rows.length, startedAt:epoch, duration:duration+'ms', workload:'distributed-rows-v1', traceHash:__ENV.TRACE_HASH, seed:1});
}
export default function () {
 const delay = epoch+2-Date.now(); if (delay > 0) sleep(delay/1000);
 const row = rows[exec.scenario.iterationInTest];
 if (!row) throw new Error('iteration outside portable asset');
 if (!check(Date.now(), {'arrival before shared window end':now=>now<epoch+duration})) throw new Error('worker missed shared window');
 const r = client.chat.completions.create({messages:[{role:'user', content:row.id}], stream:false});
 check(r, {'response inside shared window':r=>!(r instanceof Error) && r.measurement.dispatched_at>=epoch && r.measurement.finished_at<=epoch+duration});
 if (r instanceof Error) throw r;
 if (r.measurement.dispatched_at < epoch || r.measurement.finished_at > epoch+duration) throw new Error('request outside shared window: '+JSON.stringify({epoch,duration,now:Date.now(),measurement:r.measurement}));
}
export function teardown() { const delay=epoch+duration+2-Date.now(); if (delay>0) sleep(delay/1000); llm.endMeasurement(); }
export function handleSummary() { return {[__ENV.RUN_FILE || ('/results/'+worker+'.json')]:JSON.stringify(llm.measurementSummary())}; }
