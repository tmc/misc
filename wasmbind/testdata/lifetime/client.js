export async function check(features) {
 const original=Symbol.asyncDispose;
 const {load,asyncDispose}=await import('./bindings.js');const m=await load(new URL('./module.wasm',import.meta.url));let passed=0;
 const assert=(ok,message)=>{if(!ok)throw Error(message);passed++};
 const rejects=async(fn,code)=>{try{await fn()}catch(e){assert(e.code===code,'expected '+code+' got '+e.code+' '+e.message);return}throw Error('expected rejection '+code)};
 try {
  assert(Symbol.asyncDispose===original,'global Symbol unchanged');
  assert(asyncDispose===(Symbol.asyncDispose??Symbol.for('Symbol.asyncDispose')),'dispose symbol fallback');
  assert(m[asyncDispose]===m.dispose,'module disposal symbol');
  await m.reset();const p=await m.node('p');const c=await p.child('c');const g=await c.child('g');
  assert(c===await p.child('unused') && c===await c.alias(),'alias identity');assert(c[asyncDispose]===c.dispose,'resource disposal symbol');
  const other=await m.node('o');await rejects(()=>other.adopt(c),'contract');await rejects(()=>g.adopt(p),'contract');
  await p[asyncDispose]();assert(await m.sequence()==='gcp','child before parent');await rejects(()=>g.name(),'closed');await rejects(()=>c.name(),'closed');
  await other.dispose();await m.reset();const many=await m.node('p');const children=await many.children();await many.dispose();assert(await m.closed()===3,'collection children disposed');for(const child of children)await rejects(()=>child.name(),'closed');
  await m.reset();const early=await m.node('p');const gone=await early.child('c');await gone.dispose();await early.dispose();assert(await m.sequence()==='cp','early child closed once');
  await m.reset();const failure=await m.node('p');await failure.child('!');await rejects(()=>failure.dispose(),'go-error');assert(await m.sequence()==='!p','parent closes after child error');
  await m.reset();await m.prepareLate();const lateOwner=await m.node('p');const late=lateOwner.late();late.catch(()=>{});for(let tries=0;!await m.lateStarted();tries++){if(tries>100)throw Error('late not admitted');await new Promise(r=>setTimeout(r,1))}const disposeLate=lateOwner.dispose();await m.releaseLate();await disposeLate;try{await late;throw Error('expected late abort')}catch(e){assert(e.name==='AbortError','late result canceled')};assert(await m.sequence()==='lp','unpublished late child closes before parent');
  await m.reset();const leased=await m.node('p');const child=await leased.child('c');const wait=child.wait();wait.catch(()=>{});await leased.dispose();try{await wait;throw Error('expected abort')}catch(e){assert(e.name==='AbortError','child lease canceled')};assert(await m.sequence()==='cp','lease released before parent close');
  if(features){await m.reset();const n=await m.node('p');const events=await n.events();const next=events.next();next.catch(()=>{});for(let tries=0;!await events.started();tries++){if(tries>100)throw Error('next not admitted');await new Promise(r=>setTimeout(r,1))}await n.dispose();try{await next;throw Error('expected event abort')}catch(e){assert(e.name==='AbortError','blocked Next interrupted')};assert(await m.sequence()==='ep','event closed before parent');
   await m.reset();const owner=await m.node('p');const stream=await owner.stream();const writer=stream.writable.getWriter();const reader=stream.readable.getReader();const reading=reader.read();await writer.write(new Uint8Array([1,2,3]));assert((await reading).value.join(',')==='1,2,3','adapter duplex transfer');await writer.close();assert((await reader.read()).done,'adapter writer finish EOF');reader.releaseLock();writer.releaseLock();await owner.dispose();assert(await m.sequence()==='sp','adapter child disposed');
   const blockedOwner=await m.node('p');const blocked=await blockedOwner.stream();const read=blocked.readable.getReader().read();read.catch(()=>{});await blockedOwner.dispose();await rejects(()=>read,'closed');
  }
 } finally {await m.dispose()}
 return passed;
}
