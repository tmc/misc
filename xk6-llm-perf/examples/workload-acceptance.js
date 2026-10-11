import llm from 'k6/x/llm-perf';
import {check} from 'k6';
import {importDataset, distributionReceipt, sequences, realize, verifiedCounter, assetReceipt, cacheSynthesis, trace, replayArrival, conversation, dag} from './lib/workload.js';
import {controlHook, router} from './lib/control.js';
const tokenReceipt = JSON.parse(open(__ENV.TOKEN_RECEIPT));
const corpus = '[{"id":"one","prompt":"hello world"}]';
export const options = {vus:1, iterations:1, thresholds:{checks:['rate==1'],llm_successes:['count==5']}};
const clients = [new llm.Client({baseURL:__ENV.ENDPOINT_URL, model:'fixture'}),new llm.Client({baseURL:__ENV.SECOND_ENDPOINT_URL, model:'fixture'})];
export function setup() {
    const count = verifiedCounter([tokenReceipt],tokenReceipt.tokenizer_sha256);
    if (count('hello world')!==11) throw new Error('offline count mismatch');
    let missing=false;try{count('changed');}catch(e){missing=true;}if(!missing)throw new Error('modified payload accepted');
    const pairs=[{input:11,output:4,weight:70},{input:11,output:8,weight:30}];
    const targets=sequences(pairs,100,42);
    if(JSON.stringify(targets)!==JSON.stringify(sequences(pairs,100,42)))throw new Error('sequence replay mismatch');
    const realized=realize(targets,['hello world'],count,{assetHash:tokenReceipt.tokenizer_sha256,tokenizer:tokenReceipt.measurement.tokenizer});
    if(distributionReceipt(realized).meanRealizedInputTokens!==11 || Object.values(distributionReceipt(realized).pairCounts).reduce((a,b)=>a+b,0)!==100)throw new Error('distribution receipt mismatch');
    if(realized.some(r=>r.realizedInputTokens!==11))throw new Error('budget mismatch');
    let refused=false;try{realize([{inputTokens:12,outputBudget:1}],['hello world'],count,{assetHash:tokenReceipt.tokenizer_sha256,tokenizer:'fixture'});}catch(e){refused=true;}if(!refused)throw new Error('unrealizable target accepted');
    const shared=cacheSynthesis(['hello '],['world'],count,{assetHash:tokenReceipt.tokenizer_sha256});
    if(shared[0].inputTokens!==11 || shared[0].cachePolicy!=='shared-prefix')throw new Error('prefix synthesis mismatch');
    const collision=cacheSynthesis(['hello '],['world'],count,{assetHash:tokenReceipt.tokenizer_sha256,id:'forged',prompt:'changed',inputTokens:999,prefixGroup:'forged',cachePolicy:'forged',realizedInputTokens:999});
    if(collision[0].id!=='0:0'||collision[0].prompt!=='hello world'||collision[0].inputTokens!==11||collision[0].prefixGroup!=='0'||collision[0].cachePolicy!=='shared-prefix'||'realizedInputTokens' in collision[0])throw new Error('provenance overwrote synthesis');
    for(const budget of [1000001,1e9,Number.MAX_SAFE_INTEGER+1,1.5,Infinity,0]){
        let denied=false;try{realize([{inputTokens:11,outputBudget:budget}],['hello world'],count,{assetHash:tokenReceipt.tokenizer_sha256,tokenizer:'fixture'});}catch(e){denied=true;}if(!denied)throw new Error('invalid output budget accepted');
    }
    if(importDataset(corpus).length!==1 || importDataset('{"prompt":"hello"}\n','jsonl').length!==1 || assetReceipt(corpus).sha256.length!==64)throw new Error('dataset import mismatch');
    const receipt=replayArrival(trace([{id:'late',offset:0}])[0],Date.now()-1000,1,()=>{throw new Error('dropped work dispatched');});
    if(receipt.outcome!=='dropped'||!receipt.dropped)throw new Error('drop accounting mismatch');
    const busted=cacheSynthesis(['hello '],['world','world'],text=>text.length,{assetHash:tokenReceipt.tokenizer_sha256},true);
    if(busted[0].prefixGroup===busted[1].prefixGroup || busted[0].prompt===busted[1].prompt)throw new Error('cache-bust collision');
    let invalid=false;try{sequences([{input:1,output:1,weight:-1}],1,42);}catch(e){invalid=true;}if(!invalid)throw new Error('negative weight accepted');
    let cycleCalls=0;try{dag([{id:'a',dependsOn:['a']}],()=>{cycleCalls++;});throw new Error('cycle accepted');}catch(e){if(cycleCalls||!String(e).includes('cycle'))throw e;}
    const r1=router([{chat:{completions:{create:()=>new Error('failure')}}}], 'random',42);
    let calls=0;const r2=router([{chat:{completions:{create:()=>{calls++;return new Error('failure');}}}}]);
    if(!(r1({}).result instanceof Error)||!(r2({}).result instanceof Error)||calls!==1)throw new Error('routing replayed failure');
    let hookFailure=false;try{controlHook(__ENV.ENDPOINT_URL,'/reject');}catch(e){hookFailure=true;}if(!hookFailure)throw new Error('failed hook accepted');
    controlHook(__ENV.ENDPOINT_URL,'/reset_prefix_cache');
    controlHook(__ENV.ENDPOINT_URL,'/start_profile');
}
export default function(){
    const route=router(clients);
    const request={messages:[{role:'user',content:'route'}],stream:false};
    const a=route(request),b=route(request);
    check(a,{'routing rotates exactly once':()=>a.endpointIndex===0&&b.endpointIndex===1});
    const turns=conversation(clients[0],[{prompt:'turn-one',gap:0.01},{prompt:'turn-two',gap:0.01}],[{role:'system',content:'context'}]);
    check(turns,{'conversation keeps context':r=>r.length===2&&!(r[1] instanceof Error)});
    const results=dag([{id:'child',dependsOn:['parent']},{id:'parent',dependsOn:[]}],(node,parents)=>node.id==='parent'?clients[1].chat.completions.create(request):parents.get('parent').value.choices[0].message.content);
    check(results,{'DAG passes parent output':r=>r.get('child').value==='hello'});
}
export function teardown(){controlHook(__ENV.ENDPOINT_URL,'/stop_profile');}
