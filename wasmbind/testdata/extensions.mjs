import { chromium, firefox, webkit } from "playwright";
import { createServer } from "node:http";
import { readFile, writeFile, mkdir, mkdtemp, cp, rm } from "node:fs/promises";
import { execFileSync } from "node:child_process";
import { join, dirname } from "node:path";
import { homedir } from "node:os";
import { fileURLToPath } from "node:url";
const root = dirname(dirname(fileURLToPath(import.meta.url))), task = join(homedir(), "tmp/wasmbind", "20261003-extension-tests");
await mkdir(task, { recursive: true });
const dir = await mkdtemp(join(task, "run-"));
const run = (cmd, args, cwd = dir) => execFileSync(cmd, args, { cwd, env: { ...process.env, GOWORK: "off" }, stdio: "pipe" }).toString();
let server;
try {
  run("go", ["build", "-o", join(dir, "wasmbind"), "./cmd/wasmbind"], root);
  await mkdir(join(dir, "api"));
  await cp(join(root, "testdata/extensions/api.go"), join(dir, "api/api.go"));
  await writeFile(join(dir, "go.mod"), "module example.com/extensions\n\ngo 1.27.0\n");
  run(join(dir, "wasmbind"), ["build", "-features", "streams,events,imports,errors", "-go-out", "./cmd/bind", "-out", "./web", "./api"]);
  const fixture = `import {load,Stream,Subscription,NotFound,isNotFound} from './bindings.js';
async function check(){const m=await load(new URL('module.wasm',import.meta.url),{imports:{Host:{Lookup:async (key,options)=>{options.signal.aborted;return new Uint8Array()}}}});const s=await m.stream('');if(s){const r:ReadableStream<Uint8Array>=s.readable;const w:WritableStream<Uint8Array>=s.writable;
// @ts-expect-error stream view is readonly
s.readable=r;
// @ts-expect-error opaque resource cannot be constructed
const fake:Stream={readable:r,writable:w,dispose:async()=>{}};
await s.dispose();}const events=await m.subscribe('');if(events){for await(const event of events){const text:string=event;break;}}
try{await m.fail('wrapped')}catch(e:unknown){if(isNotFound(e)){const k:string=e.data.Key;
// @ts-expect-error metadata is readonly
e.domain='fake';}}
// @ts-expect-error import result must match the interface
await load(new URL('module.wasm',import.meta.url),{imports:{Host:{Lookup:()=>42}}});await m.dispose();}
`;
  await writeFile(join(dir, "web/check.ts"), fixture);
  for (const lib of ["es2022,dom,dom.iterable", "es2022,webworker"]) run(process.execPath, [join(root, "node_modules/typescript/bin/tsc"), "--noEmit", "--strict", "--skipLibCheck", "false", "--target", "es2022", "--module", "esnext", "--moduleResolution", "bundler", "--lib", lib, join(dir, "web/check.ts")]);
  const body = await readFile(join(dir, "web/module.wasm"));
  server = createServer(async (req, res) => {
    if (req.url === "/") {
      res.end("<!doctype html><title>extensions</title>");
      return;
    }
    if (req.url === "/http") {
      res.end("HTTP");
      return;
    }
    try {
      if (!/^\/web\/[\w.-]+$/.test(req.url)) throw Error();
      res.setHeader("Content-Type", req.url.endsWith(".wasm") ? "application/wasm" : "text/javascript");
      res.end(await readFile(join(dir, req.url)));
    } catch {
      res.writeHead(404).end();
    }
  });
  await new Promise((r) => server.listen(0, "127.0.0.1", r));
  const base = `http://127.0.0.1:${server.address().port}`;
  await writeFile(join(dir, "web/worker.js"), `import {load} from './bindings.js';self.onmessage=async()=>{try{const m=await load(new URL('./module.wasm',import.meta.url),{imports:{Host:{Lookup:()=>new Uint8Array([7])}}});const s=await m.stream('');const r=await s.readable.getReader().read();const events=await m.subscribe('');for await(const v of events)break;await s.dispose();await m.dispose();postMessage({value:Array.from(r.value)});}catch(e){postMessage({error:e.message})}};`);
  const receipts = { compiler: JSON.parse(await readFile(join(dir, "web/bindings.json"))).compiler, typescript: JSON.parse(await readFile(join(root, "node_modules/typescript/package.json"))).version, browsers: [] };
  for (const [name, type] of Object.entries({ chromium, firefox, webkit })) {
    const browser = await type.launch({ headless: true });
    try {
      const page = await browser.newPage();
      page.on("pageerror", (e) => console.error(name, e));
      await page.goto(base);
      const result = await page.evaluate(async () => {
        const { load, NotFound, isNotFound } = await import("/web/bindings.js");
        let passed = 0;
        const assert = (v, msg) => {
          if (!v) throw Error(msg);
          passed++;
        };
        const reject = async (fn, code) => {
          try {
            await fn();
          } catch (e2) {
            assert(e2.code === code || e2.name === code, `expected ${code}, got ${e2.code}/${e2.name}: ${e2.message}`);
            return e2;
          }
          throw Error("expected rejection " + code);
        };
        const url = () => new URL("/web/module.wasm", location.href);
        await reject(() => load(url(), {}), "argument");
        await reject(() => load(url(), { imports: { Host: { Lookup() {
          throw Error("init failed");
        } } } }), "go-error");
        let m;
        const imports = { Host: { Lookup: async (key, { signal }) => {
          if (key === "init") return new Uint8Array();
          if (key === "throw") throw Error("host failed");
          if (key === "wrong") return 7;
          if (key === "never") return new Promise(() => {
          });
          if (key === "reenter") {
            await reject(() => m.fail("plain"), "reentrant-call");
            return new Uint8Array([1]);
          }
          return new Uint8Array(await (await fetch("/http", { signal })).arrayBuffer());
        } } };
        m = await load(url(), { imports });
        imports.Host.Lookup = () => {
          throw Error("mutated");
        };
        assert(new TextDecoder().decode(await m.lookup("http")) === "HTTP", "captured HTTP import");
        assert((await m.lookup("reenter"))[0] === 1, "reentrancy");
        await reject(() => m.lookup("throw"), "go-error");
        await reject(() => m.lookup("wrong"), "go-error");
        const ac = new AbortController();
        const pending = m.lookup("never", { signal: ac.signal });
        await new Promise((r) => setTimeout(r, 10));
        ac.abort();
        await reject(() => pending, "AbortError");
        assert((await m.lookup("http")).length === 4, "ignored cancellation releases admission");
        const error = await reject(() => m.fail("wrapped"), "go-error");
        assert(error instanceof NotFound && isNotFound(error) && error.data.Key === "key" && error.message === "lookup: missing key" && !("Private" in error.data), "typed wrapped error");
        assert(!isNotFound({ ...error }), "guard private branding");
        assert(isNotFound(await reject(() => m.fail("joined"), "go-error")), "joined typed error");
        assert(!isNotFound(await reject(() => m.fail("invalid"), "go-error")), "invalid domain fields fallback");
        assert(!isNotFound(await reject(() => m.fail("cycle"), "go-error")), "cycle bounded");
        const s = await m.stream("");
        assert(s.readable === s.readable && s.writable === s.writable, "stable views");
        await new Promise((r) => setTimeout(r, 10));
        assert((await m.streamStats(s))[0] === 0, "no prefetch");
        const reader = s.readable.getReader();
        const final = await reader.read();
        assert(new TextDecoder().decode(final.value) === "final" && !final.done, "final bytes");
        assert((await reader.read()).done, "EOF");
        const writer = s.writable.getWriter();
        await writer.write(new Uint8Array([1, 2, 3, 4, 5]));
        assert((await m.streamStats(s))[1] === 5, "positive short writes");
        await writer.write(new Uint8Array());
        await writer.close();
        await s.dispose();
        const e = await m.stream("error"), er = e.readable.getReader();
        assert(new TextDecoder().decode((await er.read()).value) === "last", "final bytes before error");
        await reject(() => er.read(), "go-error");
        await e.dispose();
        const typed = await m.stream("typed"), tr = typed.readable.getReader();
        assert(new TextDecoder().decode((await tr.read()).value) === "typed", "typed final chunk");
        assert(isNotFound(await reject(() => tr.read(), "go-error")), "typed stream error");
        await typed.dispose();
        const idle = await m.stream("idle");
        await reject(() => idle.readable.getReader().read(), "go-error");
        await idle.dispose();
        const invalid = await m.stream("invalid");
        await reject(() => invalid.readable.getReader().read(), "protocol");
        await invalid.dispose();
        const zero = await m.stream("zero");
        await reject(() => zero.writable.getWriter().write(new Uint8Array([1])), "go-error");
        await zero.dispose();
        const finish = await m.stream("finish-error");
        await reject(() => finish.writable.getWriter().close(), "go-error");
        await finish.dispose();
        const blocked = await m.stream("blocked"), br = blocked.readable.getReader(), bw = blocked.writable.getWriter();
        const reading = br.read(), writing = bw.write(new Uint8Array([1]));
        const handled = reject(() => writing, "AbortError");
        await new Promise((r) => setTimeout(r, 10));
        await Promise.race([bw.abort(), new Promise((_, reject2) => setTimeout(() => reject2(Error("blocked abort timeout")), 2e3))]);
        await handled;
        await br.cancel();
        assert((await reading).done, "read interrupted independently");
        await blocked.dispose();
        const events = await m.subscribe("");
        assert(events[Symbol.asyncIterator]() === events, "same iterator");
        const values = [];
        for await (const value of events) {
          values.push(value);
          break;
        }
        assert(values.length === 1 && (await events.next()).done, "for await break cleanup");
        const sub = await m.subscribe("blocked");
        const next = sub.next();
        await reject(() => sub.next(), "busy");
        await sub.return();
        assert((await next).done, "return cancels pending next");
        const bad = await m.subscribe("error");
        const stored = await reject(() => bad.next(), "go-error");
        assert(await reject(() => bad.next(), "go-error") === stored, "stored terminal error");
        const thrown = await m.subscribe("");
        const reason = Error("consumer");
        try {
          await thrown.throw(reason);
        } catch (e2) {
          assert(e2 === reason, "throw preserves reason");
        }
        const worker = await new Promise((resolve, reject2) => {
          const w = new Worker("/web/worker.js", { type: "module" });
          w.onmessage = (e2) => {
            w.terminate();
            resolve(e2.data);
          };
          w.onerror = reject2;
          w.postMessage({});
        });
        assert(worker.value?.join(",") === "102,105,110,97,108", "Worker extensions");
        const ending = m.lookup("never");
        const ended = reject(() => ending, "AbortError");
        await new Promise((r) => setTimeout(r, 10));
        await m.dispose();
        await ended;
        return { passed };
      });
      receipts.browsers.push({ name, version: browser.version(), ...result });
      console.log(`${name}: ${result.passed} extension assertions passed`);
    } finally {
      await browser.close();
    }
  }
  await writeFile(join(task, "qualification.json"), JSON.stringify(receipts, null, 2) + "\n");
  console.log("Qualification receipt: " + join(task, "qualification.json"));
} finally {
  if (server) await new Promise((r) => server.close(r));
  if (process.env.WASMBIND_KEEP_TEST_ARTIFACTS) console.log("Kept " + dir);
  else await rm(dir, { recursive: true, force: true });
}
