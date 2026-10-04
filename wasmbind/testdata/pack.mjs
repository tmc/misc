import { chromium, firefox, webkit } from "playwright";
import { build } from "esbuild";
import { createServer } from "node:http";
import { readFile, writeFile, mkdir, mkdtemp, rm } from "node:fs/promises";
import { execFileSync } from "node:child_process";
import { join, dirname } from "node:path";
import { homedir } from "node:os";
import { fileURLToPath } from "node:url";
const root = dirname(dirname(fileURLToPath(import.meta.url))), task = join(homedir(), "tmp/wasmbind", "20261003-package-tests");
await mkdir(task, { recursive: true });
const dir = await mkdtemp(join(task, "run-"));
let server;
const run = (cmd, args, cwd = dir) => execFileSync(cmd, args, { cwd, env: { ...process.env, GOWORK: "off" }, stdio: "pipe" }).toString();
try {
  run("go", ["build", "-o", join(dir, "wasmbind"), "./cmd/wasmbind"], root);
  await mkdir(join(dir, "api"));
  await writeFile(join(dir, "go.mod"), "module example.com/pack\n\ngo 1.27.0\n");
  await writeFile(join(dir, "api/api.go"), "package api\n//wasmbind:export echo\nfunc Echo(s string)string{return s}\n");
  await writeFile(join(dir, "license.txt"), "Explicit test license\n");
  run(join(dir, "wasmbind"), ["build", "-go-out", "./cmd/bind", "-out", "./web", "./api"]);
  run(join(dir, "wasmbind"), ["pack", "-bundle", "./web", "-out", "./package", "-name", "@fixture/echo", "-version", "1.0.0", "-license-file", "./license.txt"]);
  const listing = JSON.parse(run("npm", ["pack", "--dry-run", "--json", "--ignore-scripts"], join(dir, "package")));
  const dry = Array.isArray(listing) ? listing[0] : Object.values(listing)[0];
  const files = dry.files.map((f) => f.path).sort();
  if (files.some((f) => /main.go|package-manifest|license.txt|cmd\/|node_modules/.test(f)) || !files.includes("module.wasm")) throw Error("package allowlist failed: " + files);
  const packed = JSON.parse(run("npm", ["pack", "--json", "--ignore-scripts"], join(dir, "package")));
  const archive = (Array.isArray(packed) ? packed[0] : Object.values(packed)[0]).filename;
  await mkdir(join(dir, "consumer"));
  await writeFile(join(dir, "consumer/package.json"), JSON.stringify({ private: true, type: "module" }));
  run("npm", ["install", "--ignore-scripts", "--no-audit", "--no-fund", join(dir, "package", archive)], join(dir, "consumer"));
  const source = `import {load} from '@fixture/echo';export async function check(){const m=await load();try{const value=await m.echo('packaged');if(value!=='packaged')throw Error('echo mismatch');return value;}finally{await m.dispose()}};export async function defaults(){const m=await load();await m.dispose();}`;
  await writeFile(join(dir, "consumer/check.js"), source);
  await writeFile(join(dir, "consumer/check.ts"), `import {load,Module} from '@fixture/echo';import {copyAssets} from '@fixture/echo/copy-assets';const copy: (directory:string|URL)=>Promise<void> = copyAssets;async function check(){const m:Module=await load();const s:string=await m.echo('types');await m.dispose();}`);
  run(process.execPath, [join(root, "node_modules/typescript/bin/tsc"), "--noEmit", "--strict", "--target", "es2022", "--module", "nodenext", "--moduleResolution", "nodenext", join(dir, "consumer/check.ts")]);
  await build({ entryPoints: [join(dir, "consumer/check.js")], outfile: join(dir, "consumer/bundled.js"), bundle: true, format: "esm", platform: "browser", target: "es2022" });
  await writeFile(join(dir, "consumer/assets.mjs"), `import {copyAssets} from '@fixture/echo/copy-assets';await copyAssets(new URL('./', import.meta.url));`);
  run(process.execPath, [join(dir, "consumer/assets.mjs")]);
  await writeFile(join(dir, "consumer/module.wasm"), "stale build");
  run(process.execPath, [join(dir, "consumer/assets.mjs")]);
  for (const name of ["module.wasm", "runtime.js", "wasm_exec.js"]) {
    if (!(await readFile(join(dir, "package", name))).equals(await readFile(join(dir, "consumer", name)))) throw Error("copied asset mismatch: " + name);
  }
  await writeFile(join(dir, "consumer/worker.mjs"), `import {check} from './bundled.js';check().then(value=>postMessage({value}),error=>postMessage({error:String(error)}));`);
  server = createServer(async (req, res) => {
    if (req.url === "/") {
      res.end("<!doctype html><title>package consumer</title>");
      return;
    }
    try {
      const path = req.url.split("?")[0];
      if (!/^\/(package|consumer)\/[\w.-]+$/.test(path)) throw Error();
      res.setHeader("Content-Type", path.endsWith(".wasm") ? "application/wasm" : "text/javascript");
      res.end(await readFile(join(dir, path)));
    } catch {
      res.writeHead(404).end();
    }
  });
  await new Promise((r) => server.listen(0, "127.0.0.1", r));
  const receipts = { npm: run("npm", ["--version"]).trim(), typescript: JSON.parse(await readFile(join(root, "node_modules/typescript/package.json"))).version, bundler: "esbuild 0.25.12", files, browsers: [] };
  for (const [name, type] of Object.entries({ chromium, firefox, webkit })) {
    const b = await type.launch({ headless: true });
    try {
      const p = await b.newPage();
      await p.goto(`http://127.0.0.1:${server.address().port}`);
      const value = await p.evaluate(async () => {
        const pkg = await import("/package/index.js");
        const m = await pkg.load();
        if (await m.echo("default") !== "default") throw Error("default URL failed");
        await m.dispose();
        const bundled = await import("/consumer/bundled.js");
        return bundled.check();
      });
      if (value !== "packaged") throw Error("bundled package failed");
      const workerValue = await p.evaluate(() => new Promise((resolve, reject) => {
        const worker = new Worker('/consumer/worker.mjs', {type: 'module'});
        const timer = setTimeout(() => {worker.terminate();reject(Error('bundled Worker timeout'));}, 45000);
        worker.onmessage = ({data}) => {clearTimeout(timer);worker.terminate();data.error ? reject(Error(data.error)) : resolve(data.value);};
        worker.onerror = (error) => {clearTimeout(timer);worker.terminate();reject(Error(error.message));};
      }));
      if (workerValue !== "packaged") throw Error("bundled Worker package failed");
      receipts.browsers.push({ name, version: b.version(), defaultURL: true, bundled: true, copiedAssets: true, worker: true });
      console.log(name + ": installed package and bundled consumer passed");
    } finally {
      await b.close();
    }
  }
  await writeFile(join(task, "qualification.json"), JSON.stringify(receipts, null, 2) + "\n");
  console.log("Qualification receipt: " + join(task, "qualification.json"));
} finally {
  if (server) await new Promise((r) => server.close(r));
  await rm(dir, { recursive: true, force: true });
}
