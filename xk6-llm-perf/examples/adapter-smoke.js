import llm from 'k6/x/llm-perf';
import {check} from 'k6';
export const options={vus:1,iterations:1,thresholds:{checks:['rate==1'],llm_successes:['count==10'],llm_failures:['count==0']}};
const client=new llm.Client({baseURL:__ENV.ENDPOINT_URL,strictMeasurement:true,model:'fixture'});
export default function(){
 for(const protocol of ['openai-completions','openai-responses','anthropic']){
  for(const stream of [false,true]){
   const path=protocol==='openai-completions'?'/completions':protocol==='openai-responses'?'/responses':'/messages';
   const body = protocol==='openai-completions'?{model:'fixture',stream,prompt:'fixture'}:protocol==='openai-responses'?{model:'fixture',stream,input:'fixture'}:{model:'fixture',stream,max_tokens:3,messages:[{role:'user',content:'fixture'}]};
   const r=client.request({path,protocol,body});
   check(r,{[`${protocol} ${stream}`]:r=>!(r instanceof Error)&&r.choices[0].message.content==='hello'&&r.measurement.input_tokens.value===2&&r.measurement.output_tokens.value===3});
  }
 }
 for(const stream of [false,true]){
  const r=client.request({path:stream?'/generate_stream':'/generate',protocol:'tgi',stream,body:{inputs:'fixture',parameters:{max_new_tokens:3}}});
  check(r,{[`TGI ${stream}`]:r=>!(r instanceof Error)&&r.choices[0].message.content==='hello'&&r.measurement.input_tokens.value===null&&r.measurement.output_tokens.value===3});
 }
 for(const path of ['/embeddings','/rankings']){
  const r=client.request({path,protocol:'json',body:{model:'fixture',input:'fixture'}});
  check(r,{[path]:r=>!(r instanceof Error)&&r.body.object==='fixture'&&r.measurement.output_tokens.value===null&&r.measurement.last_content_latency===null});
 }
}
