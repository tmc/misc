import llm from 'k6/x/llm-perf';
import {check} from 'k6';
export const options={vus:1,iterations:1,thresholds:{checks:['rate==1'],llm_successes:['count==6'],llm_failures:['count==1']}};
const client=new llm.Client({baseURL:__ENV.ENDPOINT_URL,apiKey:'fixture',strictMeasurement:true,timeout:'2s'});
const file=(name,filename,contentType)=>({name,filename,contentType,base64:'YQ=='});
export default function(){
 for(const format of ['text','json']){
  const r=client.request({path:'/audio/'+format,protocol:'audio-transcription',multipart:{fields:{model:'fixture',response_format:format},files:[file('file','audio.wav','audio/wav')]}});
  check(r,{['transcript '+format]:r=>!(r instanceof Error)&&r.choices[0].message.content==='spoken words'&&r.measurement.output_tokens.value===null});
 }
 const image=client.request({path:'/images/edits',protocol:'image',multipart:{fields:{model:'fixture',prompt:'edit'},files:[file('image','image.png','image/png')]}});
 check(image,{'image upload':r=>!(r instanceof Error)&&r.body.data[0].b64_json==='YQ=='&&r.measurement.output_tokens.value===null});
 const streamed=client.request({path:'/images/generations',protocol:'image',stream:true,body:{model:'fixture',prompt:'draw',stream:true}});
 check(streamed,{'image SSE':r=>!(r instanceof Error)&&r.body.events.length===2&&r.measurement.output_tokens.value===3&&r.measurement.ttft===null&&r.measurement.itl===null});
 const retrieval=client.request({path:'/retrieval',protocol:'retrieval',body:{input:[{type:'image_url',url:'data:image/png;base64,YQ=='}]}});
 check(retrieval,{'retrieval units':r=>!(r instanceof Error)&&r.body.data[0].embedding.length===2&&r.measurement.output_tokens.value===null});
 const video=client.request({path:'/videos',protocol:'video',body:{model:'fixture',prompt:'draw',seconds:4},poll:{interval:'10ms',maxPolls:3,download:true}});
 check(video,{'video lifecycle':r=>!(r instanceof Error)&&r.body.status==='completed'&&r.body.content_receipt.bytes===5&&r.measurement.output_tokens.value===null});
 const cancelled=client.request({path:'/slow-videos',protocol:'video',body:{prompt:'draw'},poll:{interval:'1s',maxPolls:3},cancelAfter:'30ms'});
 check(cancelled,{'video cancellation':r=>r instanceof Error&&r.measurement.outcome==='cancelled'});
}
