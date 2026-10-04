import { chromium, firefox, webkit } from "playwright";
import { createServer } from "node:http";
import { readFile, writeFile, mkdir, mkdtemp, cp, rm } from "node:fs/promises";
import { execFileSync } from "node:child_process";
import { join, dirname } from "node:path";
import { homedir } from "node:os";
import { fileURLToPath } from "node:url";
const root = dirname(dirname(fileURLToPath(import.meta.url)));
const task = join(homedir(), "tmp/wasmbind/20261004-ergonomics-lifetime");
await mkdir(task, { recursive: true });
const dir = await mkdtemp(join(task, "run-"));
const manifests = {};
const run = (cmd, args, cwd = root) => execFileSync(cmd, args, { cwd, env: { ...process.env, GOWORK: "off" }, stdio: "pipe" }).toString();
let server;
try {
  run("go", ["build", "-o", join(dir, "wasmbind"), "./cmd/wasmbind"]);
  for (const backend of ["syscalljs", "abi"]) {
    const module = join(dir, backend);
    await mkdir(join(module, "api"), { recursive: true });
    await writeFile(join(module, "go.mod"), "module example.com/lifetime\n\ngo 1.27.0\n");
    await cp(join(root, "testdata/lifetime/api.go"), join(module, "api/api.go"));
    run(join(dir,"wasmbind"),["adapters","-out",join(module,"api/bindadapter")]);
    if (backend === "syscalljs") await cp(join(root, "testdata/lifetime/events.go"), join(module, "api/events.go"));
    run(join(dir, "wasmbind"), ["build", "-backend", backend, ...backend === "syscalljs" ? ["-features", "streams,events"] : [], "-go-out", "./cmd/bind", "-out", "./web", "./api"], module);
    manifests[backend] = JSON.parse(await readFile(join(module, "web/bindings.json"), "utf8"));
    await cp(join(root, "testdata/lifetime/client.js"), join(module, "web/client.js"));
    await writeFile(join(module, "web/worker.js"), `import {check} from './client.js';check(${backend === "syscalljs"}).then(passed=>postMessage({passed}),e=>postMessage({error:e.message}));`);
    const fixture = `import {load,Node,asyncDispose} from './bindings.js';
async function test(){await using m=await load(new URL('./module.wasm',import.meta.url));await using p=await m.node('p');const child:Node=await p.child('c');await child[asyncDispose]();}`;
    await writeFile(join(module, "web/check.ts"), fixture);
    for (const lib of ["es2022,dom,dom.iterable", "es2022,webworker"]) run(process.execPath, [join(root, "node_modules/typescript/bin/tsc"), "--noEmit", "--strict", "--skipLibCheck", "false", "--target", "es2022", "--module", "esnext", "--moduleResolution", "bundler", "--lib", lib, join(module, "web/check.ts")]);
  }
  server = createServer(async (req, res) => {
    if (req.url === "/") {
      res.end("<!doctype html><title>API policies</title>");
      return;
    }
    try {
      if (!/^\/(syscalljs|abi)\/web\/[\w.-]+$/.test(req.url)) throw Error("path");
      res.setHeader("Content-Type", req.url.endsWith(".wasm") ? "application/wasm" : "text/javascript");
      res.end(await readFile(join(dir, req.url)));
    } catch {
      res.writeHead(404).end();
    }
  });
  await new Promise((r) => server.listen(0, "127.0.0.1", r));
  const receipts = { typescript: JSON.parse(await readFile(join(root, "node_modules/typescript/package.json"), "utf8")).version, backends: Object.fromEntries(Object.entries(manifests).map(([backend, m]) => [backend, { compiler: m.compiler, fingerprint: m.fingerprint, buildIdentity: m.buildIdentity, artifacts: m.artifacts }])), browsers: [] };
  for (const [name, type] of Object.entries({ chromium, firefox, webkit })) {
    const browser = await type.launch({ headless: true });
    try {
      const page = await browser.newPage();
      await page.goto(`http://127.0.0.1:${server.address().port}`);
      const results = await page.evaluate(async () => {
        const result = {};
        for (const backend of ["syscalljs", "abi"]) {
          const { check } = await import(`/${backend}/web/client.js`);
          const window = await check(backend === "syscalljs");
          const worker = await new Promise((resolve, reject) => {
            const w = new Worker(`/${backend}/web/worker.js`, { type: "module" });
            const timer = setTimeout(() => {
              w.terminate();
              reject(Error("Worker timeout"));
            }, 3e4);
            w.onmessage = (e) => {
              clearTimeout(timer);
              w.terminate();
              if (e.data.error) reject(Error(e.data.error));
              else resolve(e.data.passed);
            };
            w.onerror = (e) => {
              clearTimeout(timer);
              w.terminate();
              reject(Error(e.message));
            };
          });
          result[backend] = { window, worker };
        }
        return result;
      });
      receipts.browsers.push({ name, version: browser.version(), ...results });
      console.log(name, results);
    } finally {
      await browser.close();
    }
  }
  await writeFile(join(task, "qualification.json"), JSON.stringify(receipts, null, 2) + "\n");
} finally {
  await new Promise((r) => server ? server.close(r) : r());
  await rm(dir, { recursive: true, force: true });
}
