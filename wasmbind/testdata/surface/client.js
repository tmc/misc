export async function check() {
 const {load,EventKind,BigKind}=await import('./bindings.js');
 let passed=0;
 const assert=(ok,msg)=>{if(!ok)throw Error(msg);passed++};
 const reject=async f=>{try{await f()}catch(e){assert(e.code==='argument','argument rejection '+e.code);return}throw Error('expected rejection')};
 const m=await load(new URL('./module.wasm',import.meta.url));
 try {
 assert(EventKind.message===1&&EventKind.lagged===2,'member names');
 assert(!m.echo&&typeof m.events.echo==='function','namespace');
 const message=await m.events.echo({kind:EventKind.message,content:new Uint8Array([42])});
 assert(message.content[0]===42&&!('dropped'in message),'message shape');
 const lagged=await m.events.echo({kind:EventKind.lagged,dropped:5n});
 assert(lagged.dropped===5n&&!('content'in lagged),'lagged shape');
 await reject(()=>m.events.echo({kind:EventKind.message,content:new Uint8Array(),dropped:1n}));
 await reject(()=>m.events.echo({kind:EventKind.message}));
 await reject(()=>m.events.echo({kind:99,content:new Uint8Array()}));
 const projected=await m.events.projected();
 assert(projected.content.length===5&&!('dropped'in projected),'inactive Go fields projected');
 // Invalid Go enum outputs reject before union projection.
 try{await m.events.invalid();throw Error('expected invalid output')}catch(e){assert(e.code==='argument','output enum rejection '+e.code)}
 assert(await m.events.inactiveZero({kind:EventKind.lagged,dropped:1n}),'inactive slice is nil in Go');
 const big=await m.events.big({kind:BigKind.max,text:'precise'});
 assert(big.kind===BigKind.max&&big.text==='precise','bigint discriminator precision');
 const zero=await m.events.big({kind:BigKind.zero});
 assert(zero.kind===0n&&!('text' in zero),'empty variant');
 let reads=0;const stable=await m.events.echo({get kind(){reads++;return reads===1?EventKind.message:EventKind.lagged},content:new Uint8Array([7])});
 assert(reads===1&&stable.kind===EventKind.message&&stable.content[0]===7,'discriminant captured once');
 const session=await m.Session.create();await session.dispose();passed++;
 }finally{await m.dispose()}
 return passed;
}
