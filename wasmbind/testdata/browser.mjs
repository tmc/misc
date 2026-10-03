import { chromium, firefox, webkit } from 'playwright';
import { createServer } from 'node:http';
import { readFile, writeFile, mkdir, mkdtemp, cp, rm } from 'node:fs/promises';
import { execFileSync } from 'node:child_process';
import { join, dirname } from 'node:path';
import { homedir } from 'node:os';
import { fileURLToPath } from 'node:url';
import { createHash } from 'node:crypto';

const backend=process.env.WASMBIND_BACKEND??'syscalljs';
const root = dirname(dirname(fileURLToPath(import.meta.url)));
const task = join(homedir(), 'tmp/wasmbind', new Date().toISOString().slice(0, 10).replaceAll('-', '') + '-'+backend+(process.env.GOTOOLCHAIN?'-'+process.env.GOTOOLCHAIN:'')+'-browser-tests');
await mkdir(task, { recursive: true });
const dir = await mkdtemp(join(task, 'run-'));
const run = (bin, args, cwd = root) => execFileSync(bin, bin.endsWith('wasmbind')&&args[0]==='build'?[args[0],'-backend',backend,...args.slice(1)]:args, { cwd, env: { ...process.env, GOWORK: 'off' }, stdio: 'pipe' }).toString();
let server;
try {
  run('go', ['build', '-o', join(dir, 'wasmbind'), './cmd/wasmbind']);
  await mkdir(join(dir, 'api'));
  await cp(join(root, 'testdata/api/api.go'), join(dir, 'api/api.go'));
  await writeFile(join(dir, 'go.mod'), 'module example.com/browser\n\ngo 1.27.0\n');
  await mkdir(join(dir,'cmd/bind'),{recursive:true});
  await writeFile(join(dir,'cmd/bind/counters.go'), `//go:build js && wasm
package main
import ("os";"syscall/js";"example.com/browser/api")
func init(){observeBridge=func(b *runtimeBridge)func(){
 table:=js.Global().Get("__wasmbind_test_counters");if table.IsUndefined(){table=js.Global().Get("Object").Call("create",js.Null());js.Global().Set("__wasmbind_test_counters",table)}
 token:=os.Getenv("WASMBIND_TOKEN")
 f:=js.FuncOf(func(_ js.Value,args []js.Value)any{if len(args)>0{api.ReleaseIgnore()};return map[string]any{"ignoreStarted":api.IgnoreStarted(),"resources":len(b.resources),"calls":len(b.calls),"live":b.liveResources}})
 table.Set(token,f);return func(){table.Delete(token);f.Release()}
}}`);
  run(join(dir, 'wasmbind'), ['build', '-go-out', './cmd/bind' , '-out', './web', './api'], dir);
  const tsc = join(root, 'node_modules/typescript/bin/tsc');
  await writeFile(join(dir, 'web/check.ts'), `import {load, Client, Scalars} from './bindings.js';
async function check() {const m=await load(new URL('module.wasm',import.meta.url));const c: Client | null=await m.newClient(false);if(c) {const s:string=await c.echoString('x');const n:number=await c.echoInt(1);await c.dispose();} await m.dispose();}
`);
  run(process.execPath, [tsc, '--noEmit', '--strict', '--target', 'es2022', '--module', 'esnext', '--moduleResolution', 'bundler', join(dir, 'web/check.ts')]);
  const wasm = await readFile(join(dir, 'web/module.wasm'));
  const manifest = JSON.parse(await readFile(join(dir, 'web/bindings.json'), 'utf8'));
  // A body-only change keeps the schema but must change the build identity.
  const original = await readFile(join(dir, 'api/api.go'),'utf8');
  await writeFile(join(dir, 'api/api.go'), original.replace('func Bytes(v []byte) []byte { return v }','func Bytes(v []byte) []byte { return append(v, 9) }'));
  await mkdir(join(dir,'cmd/body'),{recursive:true});await cp(join(dir,'cmd/bind/counters.go'),join(dir,'cmd/body/counters.go'));
  run(join(dir, 'wasmbind'), ['build', '-go-out', './cmd/body', '-out', './body', './api'], dir);
  const bodyManifest=JSON.parse(await readFile(join(dir,'body/bindings.json'),'utf8'));
  if (bodyManifest.fingerprint!==manifest.fingerprint || bodyManifest.buildIdentity===manifest.buildIdentity) throw Error('build/schema identity separation failed');
  await writeFile(join(dir,'api/api.go'),original);
  // Wrong-schema Wasm is real compiled code, not a mocked bridge.
  await writeFile(join(dir, 'api/api.go'), (await readFile(join(dir, 'api/api.go'), 'utf8')).replace('wasmbind:export bytes', 'wasmbind:export wrongBytes'));
  run(join(dir, 'wasmbind'), ['build', '-go-out', './cmd/wrong', '-out', './wrong', './api'], dir);
  // Early exit before the generated main reaches registration.
  await writeFile(join(dir, 'api/api.go'), 'package api\nimport "os"\nfunc init(){os.Exit(0)}\n//wasmbind:export early\nfunc Early(){}\n');
  run(join(dir, 'wasmbind'), ['build', '-go-out', './cmd/early', '-out', './early', './api'], dir);
  await mkdir(join(dir, 'conflict'));
  await cp(join(dir, 'web/bindings.js'), join(dir, 'conflict/bindings.js'));
  await writeFile(join(dir, 'conflict/runtime.js'), (await readFile(join(dir, 'web/runtime.js'),'utf8')).replace('const identity = "', 'const identity = "conflicting-'));
  server = createServer(async (req, res) => {
    if (req.url === '/blob') { res.end(Buffer.from([1, 2, 3, 4])); return; }
    if (req.url === '/slow') { setTimeout(() => { res.setHeader('Content-Type', 'application/wasm'); res.end(wasm); }, 150); return; }
    if (req.url === '/') { res.setHeader('Content-Type', 'text/html'); res.end('<!doctype html><title>wasmbind tests</title>'); return; }
    try {
      const path = req.url.split('?')[0];
      if (!/^\/(web|wrong|early|conflict|body)\/[a-zA-Z0-9_.-]+$/.test(path)) { res.writeHead(404).end(); return; }
      const data = await readFile(join(dir, path));
      res.setHeader('Content-Type', path.endsWith('.wasm') ? 'application/wasm' : path.endsWith('.js') ? 'text/javascript' : 'application/json');
      res.end(data);
    } catch { res.writeHead(404).end(); }
  });
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
  const base = `http://127.0.0.1:${server.address().port}`;
  await writeFile(join(dir, 'web/worker.js'), `import {load} from './bindings.js';
self.onmessage=async()=>{try{const m=await load(new URL('./module.wasm',import.meta.url));const v=await m.bytes(new Uint8Array([9,8]));await m.dispose();postMessage({value:Array.from(v)});}catch(e){postMessage({error:e.message});}};`);
  if(backend==='abi')await cp(join(root,'testdata/abi-probe.js'),join(dir,'web/abi-probe.js'));
  const receipts = { backend,compiler: manifest.compiler, artifacts: manifest.artifacts, typescript: JSON.parse(await readFile(join(root, 'node_modules/typescript/package.json'), 'utf8')).version, browsers: [] };
  for (const [name, browserType] of Object.entries({ chromium, firefox, webkit })) {
    // Required browsers must launch: unavailable executables fail this suite.
    const browser = await browserType.launch({ headless: true });
    try {
      const page = await browser.newPage();
      page.on('pageerror',e=>console.log(name,e));await page.goto(base);
      const results = await page.evaluate(async () => {
        const { load, WasmBindError } = await import('/web/bindings.js');
        let passed = 0;
        const assert = (v, msg) => { if (!v) throw new Error(msg); passed++; };
        const rejects = async (fn, code) => {
          try { await fn(); } catch (e) { assert(e.code === code || e.name === code, `wanted ${code}, got ${e.name}/${e.code}: ${e.message}`); return e; }
          throw new Error(`expected ${code} rejection`);
        };
        const url = () => new URL('/web/module.wasm', location.href);
        const m = await load(url()), other = await load(url());
        const stats = Object.values(globalThis.__wasmbind_test_counters)[0];
        const input = new Uint8Array([1,2,3]); const p = m.bytes(input); input[0] = 99;
        const output = await p; assert(output[0] === 1, 'input copied before Promise return'); output[0] = 88;
        assert((await m.bytes(new Uint8Array([1])))[0] === 1, 'output isolated');
        const scalars = {Bool:true,Int:2**60,Uint:2**63,I8:-128,U8:255,I16:-32768,U16:65535,I32:-2147483648,U32:4294967295,I64:-(1n<<63n),U64:(1n<<64n)-1n,F32:1.25,F64:1e300,Text:'😀'};
        const round = await m.scalars(scalars); assert(round.I64 === scalars.I64 && round.U64 === scalars.U64 && round.Int === scalars.Int, 'scalar bounds');
        for (const [field,value] of [['I8',128],['U32',4294967296],['I64',1n<<63n],['U64',-1n],['Int',2**63],['Uint',2**64],['F32',1e100],['F64',Infinity],['Bool',0],['Text','\ud800']])
          await rejects(() => m.scalars({...scalars,[field]:value}), 'argument');
        assert(await m.payload(null) === null, 'null pointer');
        const rec = await m.payload({name:'ok',Values:[1,2],Bytes:new Uint8Array([3])});assert(Object.getPrototypeOf(rec) === null && rec.name === 'ok', 'safe record output');
        await rejects(() => m.payload(undefined), 'argument');
        await rejects(() => m.payload({name:'ok',Values:[],Bytes:new Uint8Array(),unknown:1}), 'argument');
        await rejects(() => m.payload({name:'ok',Values:[]}), 'argument');
        await rejects(() => m.payload({get name(){throw Error('secret')},Values:[],Bytes:new Uint8Array()}), 'argument');
        const cyc = [];cyc.push(cyc);await rejects(() => m.nested(cyc), 'argument');
        const detached = new Uint8Array(1);structuredClone(detached.buffer,{transfer:[detached.buffer]});await rejects(() => m.bytes(detached),'argument');
        await rejects(() => m.array([1]), 'argument');assert((await m.array([0,255]))[1]===255,'byte arrays');
        assert((await rejects(() => m.fail(),'go-error')).message==='exact Go failure','Go message');
        const pe=await rejects(() => m.panic(),'panic');assert(!pe.message.includes('private'),'panic redacted');
        assert((await m.fetch(new URL('/blob',location.href).href)).length===4,'HTTP yields callback');
        const pre=new AbortController();pre.abort();await rejects(()=>m.wait(100,{signal:pre.signal}),'AbortError');
        const ac=new AbortController();const waiting=m.wait(100,{signal:ac.signal});ac.abort();await rejects(()=>waiting,'AbortError');
        const c=await m.newClient(false);assert(await c.same()===c,'one pointer one wrapper');
        assert(await c.echoInt(12)===12 && await c.echoString('generic')==='generic','generic methods');
        assert(await m.identityInt(7)===7,'generic function');
        const adds=await Promise.all([c.add(1),c.add(2),c.add(3)]);assert(adds.join(',')==='1,3,6','receiver admission order');
        await rejects(()=>m.use({}),'argument');await rejects(()=>other.use(c),'foreign-resource');
        const held=m.use(c);const d=c.dispose();assert(c.dispose()===d,'idempotent disposal Promise');await rejects(()=>held,'AbortError');await d;
        await rejects(()=>c.add(1),'closed');await rejects(()=>c.same(),'closed');
        const bad=await m.newClient(true);const badDispose=bad.dispose();const closeErr=await rejects(()=>badDispose,'go-error');assert(closeErr.message==='close failed','close failure');assert(bad.dispose()===badDispose,'same failed settlement');
        const lateAbort=new AbortController();const late=m.late(30,{signal:lateAbort.signal});await new Promise(r=>setTimeout(r,5));lateAbort.abort();await rejects(()=>late,'AbortError');
        for(let n=0;n<100&&(await m.counts())[0]!==0;n++)await new Promise(r=>setTimeout(r,20));assert((await m.counts())[0]===0,'late resource cleanup');
        const q=await m.newClient(false);const qa=new AbortController();const first=q.add(1);const queued=q.add(100,{signal:qa.signal});qa.abort();await rejects(()=>queued,'AbortError');assert(await first===1 && await q.add(1)===2,'canceled queued method never invokes');await q.dispose();
        for(let i=0;i<20;i++){const c=await m.newClient(false);await c.echoInt(i);await c.dispose();}assert((await m.counts())[0]===0,'resource cycle cleanup');assert(stats().resources===0 && stats().calls===0 && stats().live===0,'test-only registry counters cleaned');
        const bounded=await load(url(),{maxResources:1,maxValueBytes:8,maxValueDepth:1,maxPendingCalls:1});
        await rejects(()=>bounded.bytes(new Uint8Array(9)),'limit');await rejects(()=>bounded.huge(9),'limit');await rejects(()=>bounded.nested([['x']]),'limit');
        const one=await bounded.newClient(false);await rejects(()=>bounded.newClient(false),'limit');await rejects(()=>bounded.clients(3),'limit');await rejects(()=>bounded.duplicateClients(),'limit');await one.dispose();const two=await bounded.newClient(false);await two.dispose();assert((await bounded.counts())[0]===0,'failed resource conversion rolls back all pointers');
        const pending=bounded.wait(30);await rejects(()=>bounded.wait(30),'limit');await pending;await bounded.dispose();
        await rejects(()=>load(new URL('/wrong/module.wasm',location.href)),'abi-mismatch');await rejects(()=>load(new URL('/body/module.wasm',location.href)),'abi-mismatch');
        await rejects(()=>load(new URL('/early/module.wasm',location.href)),'runtime-exit');
        const conflict=await import('/conflict/bindings.js');await rejects(()=>conflict.load(url()),'abi-mismatch');
        const saved=await m.saved();await saved.dispose();await rejects(()=>m.saved(),'closed');
        const beforeDying=new Set(Object.keys(globalThis.__wasmbind_test_counters));const dying=await load(url());const dyingToken=Object.keys(globalThis.__wasmbind_test_counters).find(k=>!beforeDying.has(k));const during=dying.wait(100);const dyingCall=dying.exit();await rejects(()=>during,'runtime-exit');await rejects(()=>dyingCall,'runtime-exit');await rejects(()=>dying.bytes(new Uint8Array()),'closed');await rejects(()=>dying.dispose(),'runtime-exit');delete globalThis.__wasmbind_test_counters[dyingToken];
        const beforeIgnoring=new Set(Object.keys(globalThis.__wasmbind_test_counters));const ignoring=await load(url());const ignoringStats=globalThis.__wasmbind_test_counters[Object.keys(globalThis.__wasmbind_test_counters).find(k=>!beforeIgnoring.has(k))];const ignored=ignoring.ignore(-1);for(let n=0;n<2000&&!ignoringStats().ignoreStarted;n++)await new Promise(r=>setTimeout(r,1));assert(ignoringStats().ignoreStarted,'uncancellable call started');const ignoredHandled=rejects(()=>ignored,'AbortError');let disposed=false;const disposeIgnore=ignoring.dispose().then(()=>disposed=true);await ignoredHandled;assert(!disposed&&ignoringStats().calls===1,'disposal waits for uncancellable application call');ignoringStats('release');await disposeIgnore;
        for(let i=0;i<12;i++){const race=new AbortController();const promise=m.wait(1,{signal:race.signal});setTimeout(()=>race.abort(),i%3);try{assert(await promise==='done','completion wins once')}catch(e){assert(e.name==='AbortError','abort wins once')}}
        const before=performance.now();await rejects(()=>load(new URL('/slow',location.href),{startupTimeoutMs:10}),'runtime-exit');assert(performance.now()-before<130,'timeout rejects promptly');
        const response=await fetch(url());const responseMod=await load(response);await responseMod.dispose();
        const bytes=await (await fetch(url())).arrayBuffer();const bufferMod=await load(bytes);await bufferMod.dispose();
        const compiled=await WebAssembly.compile(bytes);const compiledMod=await load(compiled);await compiledMod.dispose();
        const workerResult=await new Promise((resolve,reject)=>{const w=new Worker('/web/worker.js',{type:'module'});w.onmessage=e=>{w.terminate();resolve(e.data)};w.onerror=reject;w.postMessage({});});assert(workerResult.value?.join(',')==='9,8','dedicated Worker');
        const teardown=await m.newClient(true);const disposing=m.dispose();assert(m.dispose()===disposing,'module disposal idempotence');await rejects(()=>disposing,'go-error');await rejects(()=>m.bytes(new Uint8Array()),'closed');await rejects(()=>teardown.same(),'closed');
        await other.dispose();
        assert(Object.keys(globalThis.__wasmbind_bootstrap_v1__??{}).length===0,'bootstrap cleaned');assert(Object.keys(globalThis.__wasmbind_test_counters).length===0,'test-only callbacks released');
        return {passed};
      });
      if(backend==='abi')results.protocol=await page.evaluate(async()=>{const {probe}=await import('/web/abi-probe.js');return probe();});
      receipts.browsers.push({ name, version: browser.version(), ...results });
      console.log(`${name} ${browser.version()}: ${results.passed} assertions passed`);
    } finally { await browser.close(); }
  }
  await writeFile(join(task, 'qualification.json'), JSON.stringify(receipts, null, 2)+'\n');
  console.log(`Qualification receipt: ${join(task, 'qualification.json')}`);
} finally {
  if (server) await new Promise(resolve => server.close(resolve));
  if (process.env.WASMBIND_KEEP_TEST_ARTIFACTS) console.log(`Kept artifacts: ${dir}`);
  else await rm(dir, {recursive:true,force:true});
}
