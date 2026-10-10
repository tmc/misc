import llm from 'k6/x/llm-perf';
import exec from 'k6/execution';
import { check } from 'k6';
import { arrivals, dataset, sample, prefixPrompt, waitArrival, dag } from './lib/workload.js';
const rows = dataset(Array.from({length:7}, (_,i) => ({prompt:`row-${i}`})));
const offsets = arrivals(7, 200, 'constant', 42);
export const options = { scenarios: { replay: {executor:'shared-iterations', vus:3, iterations:7, maxDuration:'10s'} }, thresholds: { checks:['rate==1'], llm_successes:['count==7'], llm_failures:['count==0'] } };
const client = new llm.Client({baseURL:__ENV.ENDPOINT_URL,model:'fixture',strictMeasurement:true,requestConcurrency:2,prefillConcurrency:1});
export function setup() {
 if (JSON.stringify(arrivals(100,20,'gamma',42,2)) !== JSON.stringify(arrivals(100,20,'gamma',42,2))) throw new Error('seed replay mismatch');
 if (JSON.stringify(sample(rows,7,42)) !== JSON.stringify(sample(rows,7,42))) throw new Error('sample replay mismatch');
 if (prefixPrompt('prefix:',' suffix',100) !== 'prefix: suffix') throw new Error('prefix mismatch');
 const got=dag([{id:'b',dependsOn:['a']},{id:'a',dependsOn:[]},{id:'c',dependsOn:['b']}],node => {if(node.id==='b')throw new Error('fixture');return node.id;});
 if (got.get('a').outcome!=='success'||got.get('b').outcome!=='failure'||got.get('c').outcome!=='skipped')throw new Error('DAG policy mismatch');
 let calls=0;
 try { dag([{id:'a',dependsOn:['a']},{id:'b',dependsOn:[]}],()=>{calls++;});throw new Error('cycle accepted'); } catch(error) { if (!String(error).includes('cycle')||calls) throw error; }
 llm.beginMeasurement({runId:'absolute-fixture',maxRecords:20,seed:42,workload:'seven rows, constant absolute arrival'});
 return {epoch:Date.now()+20};
}
export default function(data) {
 const index=exec.scenario.iterationInTest;
 const arrival=waitArrival(data.epoch,offsets[index],10000);
 if (arrival.dropped) throw new Error('fixture arrival dropped');
 const result=client.chat.completions.create({messages:[{role:'user',content:rows[index].prompt}],stream:true,scheduledAt:arrival.scheduledAt});
 check(result,{'scheduled request':r=>!(r instanceof Error)&&typeof r.measurement.arrival_lag==='number'&&r.measurement.arrival_lag>=0});
}
export function teardown() {
 const run=llm.endMeasurement();
 if(run.attempts!==7||run.records.length!==7)throw new Error('absolute replay accounting mismatch');
}
