export async function check(branded) {
 const {load,EventKind}=await import('./bindings.js');const m=await load(new URL('./module.wasm',import.meta.url));let passed=0;
 const assert=(ok,msg)=>{if(!ok)throw Error(msg);passed++};
 const rejected=async(fn,code,path)=>{try{await fn()}catch(e){assert(e.code===code,'code '+e.code+' '+e.message);if(path){assert(e.message.includes(path),'path '+e.message);if(code==='argument')assert(e.path===path,'structured path '+e.path);}return e}throw Error('expected rejection')};
 try {
 const out=await m.echo({required:'ok'});
 assert(out.enabled===true && out.big===18446744073709551615n,'literal defaults');
 assert(out.child.count===0 && out.child.label==='' && out.nullable===null && out.items.length===0,'omission is zero value');
 const explicit=await m.echo({required:'ok',enabled:false,big:0n,child:{},nullable:null});
 assert(explicit.enabled===false && explicit.big===0n && explicit.child.count===7 && explicit.child.label==='default','explicit zero and nested defaults');
 await rejected(()=>m.echo({}),'argument','options.required');
 await rejected(()=>m.echo({required:'ok',surprise:1}),'argument','options.surprise');
 await rejected(()=>m.echo({required:'ok',enabled:undefined}),'argument','options.enabled');
 await rejected(()=>m.echo({required:'ok',items:[{count:'bad'}]}),'argument','options.items[0].count');
 await rejected(()=>m.echo({required:'ok',child:null}),'argument','options.child');
 const getter={required:'ok',items:[Object.defineProperty({},'count',{enumerable:true,get(){throw Error('private')}})]};const inspected=await rejected(()=>m.echo(getter),'argument','options.items[0].count');assert(!inspected.message.includes('private'),'inspection redacted');
 assert(await m.slicesAreNil({}),'omitted slices retain Go nil');
 assert(await m.slicesAreEmpty({bytes:new Uint8Array(),ints:[],child:{bytes:new Uint8Array(),ints:[]}}),'supplied empty slices retain nonnil');
 await rejected(()=>m.slicesAreNil({bytes:null}),'argument','slices.bytes');
 const event=await m.eventEcho({kind:EventKind.message});assert(event.count===9 && event.child.count===0,'union optional defaults');
 const ready=await m.envelopeEcho({});assert(ready.event.kind===EventKind.ready && Object.keys(ready.event).length===1,'omitted union is Go zero');
 await rejected(()=>m.eventEcho({kind:EventKind.ready,count:3}),'argument','event.count');
 let allocated=false;const original=globalThis.Array;
 globalThis.Array=new Proxy(original,{construct(target,args,newTarget){if(args[0]===1000001)allocated=true;return Reflect.construct(target,args,newTarget)}});
 try {await rejected(()=>m.largeInput({}),'limit','large.zeros');assert(!allocated,'limit charged before zero allocation');}finally{globalThis.Array=original;}
 if(branded) {
 assert(await m.parsePeerID('valid')==='valid','validated constructor');
 await rejected(()=>m.parsePeerID(''),'go-error','PeerID');
 await rejected(()=>m.accept({ids:['valid','']}),'go-error','peers.ids[1]');
 const panic=await rejected(()=>m.parsePeerID('panic'),'panic');assert(!panic.message.includes('secret'),'panic redaction');
 const ac=new AbortController();const pending=m.parsePeerID('slow',{signal:ac.signal});ac.abort();try{await pending;throw Error('expected abort')}catch(e){assert(e.name==='AbortError','abort helper')}
 }
 }finally{await m.dispose()}
 return passed;
}
