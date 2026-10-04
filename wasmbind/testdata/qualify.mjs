import {execFileSync} from 'node:child_process';
import {mkdir,mkdtemp,writeFile,readFile,rm} from 'node:fs/promises';
import {dirname,join} from 'node:path';
import {homedir} from 'node:os';
import {fileURLToPath} from 'node:url';
const root=dirname(dirname(fileURLToPath(import.meta.url)));
const task=join(homedir(),'tmp/wasmbind/20261004-idiomatic');await mkdir(task,{recursive:true});
const dir=await mkdtemp(join(task,'qualify-'));
const run=(tool,args)=>execFileSync(tool,args,{cwd:root,env:{...process.env,GOWORK:'off'},encoding:'utf8',stdio:'pipe'});
try{
 const cli=join(dir,'wasmbind');run('go',['build','-o',cli,'./cmd/wasmbind']);
 const pkg=join(dir,'package');await mkdir(pkg);
 await writeFile(join(pkg,'package.json'),JSON.stringify({name:'@wasmbind/lifecycle',version:'0.0.0',type:'module',exports:{'.':{types:'./index.d.ts',import:'./index.js'}}}));
 await writeFile(join(pkg,'index.d.ts'),'export function load(): Promise<{dispose(): Promise<void>}>;\n');
 await writeFile(join(pkg,'index.js'),'export async function load(){return {async dispose(){}}}\n');
 const fixtures=join(dir,'fixtures');await mkdir(fixtures);
 await writeFile(join(fixtures,'helper.ts'),"import {load} from '@wasmbind/lifecycle'; export async function example(){const m=await load();await m.dispose();}\n");
 await writeFile(join(fixtures,'consumer.ts'),"import {example} from './helper.js'; await example();\n");
 await writeFile(join(fixtures,'doc.ts'),"import {example} from './helper.js'; void example();\n");
 const args=out=>['qualify','-package',pkg,'-out',join(dir,out),'-tsc',join(root,'node_modules/.bin/tsc'),'-npm','npm','-node',process.execPath,'-playwright',join(root,'node_modules/playwright/index.mjs'),'-fixtures',fixtures,'-consumer',join(fixtures,'consumer.ts'),'-timeout','1000',join(fixtures,'doc.ts')];
 run(cli,args('valid'));
 const receipt=JSON.parse(await readFile(join(dir,'valid/qualification.json'),'utf8'));
 if(receipt.browsers.length!==3||receipt.fixtures.length!==2)throw Error('missing required observations');
 await writeFile(join(fixtures,'consumer.ts'),'const invalid: number="bad";\n');
 let typeRejected=false;try{run(cli,args('invalid-types'));}catch(e){typeRejected=String(e.stderr).includes('not assignable');}if(!typeRejected)throw Error('invalid consumer accepted');
 await writeFile(join(fixtures,'consumer.ts'),"import {example} from './helper.js'; await example();\n");
 await writeFile(join(pkg,'index.js'),'export function load(){return new Promise(()=>{});}\n');
 let deadlineRejected=false;const started=Date.now();try{run(cli,args('never-load'));}catch(e){deadlineRejected=String(e.stderr).includes('Window lifecycle timeout');}if(!deadlineRejected||Date.now()-started>30000)throw Error('unbounded Window lifecycle');
 for(const name of ['invalid-types','never-load']){let exists=false;try{await readFile(join(dir,name,'qualification.json'));exists=true;}catch{}if(exists)throw Error('failure produced successful receipt');}
 await writeFile(join(task,'qualify-regressions.json'),JSON.stringify({scopedRelativeImports:true,allRequiredBrowserRealms:true,invalidConsumerRejected:true,windowLifecycleDeadline:true,noFailureReceipts:true},null,2)+'\n');
 console.log('qualify regressions passed');
}finally{await rm(dir,{recursive:true,force:true});}
