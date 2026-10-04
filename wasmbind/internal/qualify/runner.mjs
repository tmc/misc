import {readFile, writeFile, mkdir, cp, realpath, stat, readdir} from 'node:fs/promises';
import {execFileSync} from 'node:child_process';
import {createServer} from 'node:http';
import {createHash} from 'node:crypto';
import {join, resolve, relative, sep} from 'node:path';
import {pathToFileURL} from 'node:url';
const o=JSON.parse(await readFile(process.argv[2],'utf8'));
const run=(tool,args,cwd=o.out)=>execFileSync(tool,args,{cwd,encoding:'utf8',stdio:'pipe'});
let packageJSON=JSON.parse(await readFile(join(o.package,'package.json'),'utf8'));
if(!packageJSON.name)throw Error('package has no name');
const packed=JSON.parse(run(o.npm,['pack',o.package,'--json','--ignore-scripts','--offline']));
const archives=Array.isArray(packed)?packed:Object.values(packed);
if(archives.length!==1)throw Error('expected one package archive');
const archive=join(o.out,archives[0].filename);
const consumer=join(o.out,'consumer');await mkdir(consumer);
await writeFile(join(consumer,'package.json'),JSON.stringify({private:true,type:'module'}));
run(o.npm,['install','--offline','--ignore-scripts','--no-audit','--no-fund','--package-lock=false',archive],consumer);
const installed=await realpath(join(consumer,'node_modules',packageJSON.name));
packageJSON=JSON.parse(await readFile(join(installed,'package.json'),'utf8'));
const files=[],fixtures=[];
if(o.fixtures)await cp(o.fixtures,join(consumer,'fixtures'),{recursive:true,filter:p=>!['node_modules','.git'].includes(p.split(sep).at(-1))});
for(const [i,file] of [o.consumer,...o.examples].entries()){
 let dest;
 if(o.fixtures){const name=relative(o.fixtures,file);if(name.split(sep).includes('..')||name==='')throw Error('fixture outside scoped tree');dest=join(consumer,'fixtures',name);}
 else{dest=join(consumer,`fixture-${i}.ts`);await cp(file,dest);}
 files.push(dest);fixtures.push({path:file,compiledPath:relative(o.out,dest),sha256:createHash('sha256').update(await readFile(dest)).digest('hex')});
}
const fixtureTree=[];
if(o.fixtures){
 const snapshot=join(consumer,'fixtures');
 const scan=async dir=>{for(const entry of (await readdir(dir,{withFileTypes:true})).sort((a,b)=>a.name.localeCompare(b.name))){const file=join(dir,entry.name);if(entry.isDirectory())await scan(file);else if(entry.isFile())fixtureTree.push({path:relative(snapshot,file),sha256:createHash('sha256').update(await readFile(file)).digest('hex')});else throw Error('unsupported fixture entry');}};
 await scan(snapshot);
}
const checks=[];
for(const lib of ['ES2022,DOM','ES2022,WebWorker']){
 run(o.tsc,['--strict','--exactOptionalPropertyTypes','--noEmit','--target','ES2022','--module','NodeNext','--moduleResolution','NodeNext','--lib',lib,...files],o.out);
 checks.push({lib,consumer:1,documentationExamples:o.examples.length});
}
const exports=packageJSON.exports;
const entry=typeof exports==='string'?exports:exports?.['.']?.import??exports?.['.']?.default??packageJSON.module??packageJSON.main;
if(typeof entry!=='string'||!entry.startsWith('./'))throw Error('expected explicit local ESM package entrypoint');
const entryURL='/package/'+entry.slice(2);
const lifecycle=`const m=await import(${JSON.stringify(entryURL)});const api=await m.load();if(typeof api.dispose!=='function')throw Error('missing dispose');await api.dispose();await api.dispose();return {loaded:true,disposed:true,idempotentDispose:true};`;
const server=createServer(async(req,res)=>{
 try{
  const pathname=new URL(req.url,'http://localhost').pathname;
  if(pathname==='/'){res.setHeader('Content-Type','text/html');res.end('<!doctype html><title>wasmbind qualify</title>');return;}
  if(pathname==='/worker.mjs'){res.setHeader('Content-Type','text/javascript');res.end(`(async()=>{${lifecycle}})().then(value=>postMessage({value}),error=>postMessage({error:error.message}));`);return;}
  if(!pathname.startsWith('/package/'))throw Error('unknown route');
  const file=await realpath(resolve(installed,decodeURIComponent(pathname.slice(9))));
  if(relative(installed,file).split(sep).includes('..')||!(await stat(file)).isFile())throw Error('outside package');
  res.setHeader('Content-Type',file.endsWith('.wasm')?'application/wasm':file.endsWith('.js')?'text/javascript':'application/octet-stream');
  res.end(await readFile(file));
 }catch{res.writeHead(404).end();}
});
await new Promise(r=>server.listen(0,'127.0.0.1',r));
const browsers=[];
try{
 const playwright=await import(pathToFileURL(o.playwright).href);
 for(const name of ['chromium','firefox','webkit']){
  if(!playwright[name])throw Error('missing browser target '+name);
  const browser=await playwright[name].launch({headless:true});
  try{
   const page=await browser.newPage();page.setDefaultTimeout(30000);
   await page.goto('http://127.0.0.1:'+server.address().port);
   const window=await page.evaluate(new Function('return Promise.race([(async()=>{'+lifecycle+'})(),new Promise((_,reject)=>setTimeout(()=>reject(Error(\"Window lifecycle timeout\")),'+o.timeoutMs+'))])'));
   const worker=await page.evaluate(timeout=>new Promise((resolve,reject)=>{
    const w=new Worker('/worker.mjs',{type:'module'});
    const timer=setTimeout(()=>{w.terminate();reject(Error('Worker lifecycle timeout'));},timeout);
    w.onmessage=e=>{clearTimeout(timer);w.terminate();e.data.error?reject(Error(e.data.error)):resolve(e.data.value);};
    w.onerror=e=>{clearTimeout(timer);w.terminate();reject(Error(e.message));};
   }),o.timeoutMs);
   browsers.push({name,version:browser.version(),window,worker});
  }finally{await browser.close();}
 }
}finally{await new Promise(r=>server.close(r));}
let hook;
if(o.hook){run(o.node,[o.hook],consumer);hook={path:o.hook,exitCode:0,kind:'caller-provided application hook',independentlyObserved:false};}
const sha256=async path=>createHash('sha256').update(await readFile(path)).digest('hex');

const receipt={package:packageJSON.name,version:packageJSON.version,archiveSHA256:await sha256(archive),installation:{offline:true,ignoreScripts:true},tools:{node:process.version,npm:run(o.npm,['--version']).trim(),typescript:run(o.tsc,['--version']).trim()},checks,fixtures,...(o.fixtures?{fixtureTree}:{}),browsers,...(hook?{hook}:{})};
await writeFile(join(o.out,'qualification.json'),JSON.stringify(receipt,null,2)+'\n');
console.log('qualified '+packageJSON.name);
