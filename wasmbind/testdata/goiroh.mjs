import { chromium, firefox, webkit } from "playwright";
import { nativeWorker } from "./native-worker.mjs";
import { mkdir, mkdtemp, readFile, writeFile, cp, rm } from "node:fs/promises";
import { spawn, execFileSync } from "node:child_process";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { homedir } from "node:os";
const root = dirname(dirname(fileURLToPath(import.meta.url)));
const task = join(homedir(), "tmp/wasmbind", (/* @__PURE__ */ new Date()).toISOString().slice(0, 10).replaceAll("-", "") + "-goiroh-tests");
await mkdir(task, { recursive: true });
const dir = await mkdtemp(join(task, "run-"));
const run = (bin, args, cwd = root) => execFileSync(bin, args, { cwd, env: { ...process.env, GOWORK: "off" }, stdio: "pipe" }).toString();
let relay;
try {
  await cp(join(root, "examples/goiroh"), join(dir, "example"), { recursive: true, filter: (p) => !p.includes("/generated/") && !p.includes("/web/") });
  const example = join(dir, "example");
  run("go", ["build", "-o", join(dir, "wasmbind"), "./cmd/wasmbind"]);
  run(join(dir, "wasmbind"), ["build", "-features", "streams,events,imports,errors", "-go-out", "./generated", "-out", "./web", "./webapi"], example);
  run("go", ["build", "-mod=readonly", "-o", join(dir, "relay"), "./cmd/relay"], example);
  await writeFile(join(example, "web/index.html"), "<!doctype html><title>go-iroh binding test</title>");
  relay = spawn(join(dir, "relay"), ["-web", join(example, "web"), "-store", join(dir, "store")], { stdio: ["ignore", "pipe", "pipe"] });
  let stderr = "";
  relay.stderr.on("data", (v) => stderr += v);
  const info = await new Promise((resolve, reject) => {
    let text = "";
    const timer = setTimeout(() => reject(Error("relay startup timeout: " + stderr)), 3e4);
    relay.stdout.on("data", (v) => {
      text += v;
      if (text.includes("\n")) {
        clearTimeout(timer);
        try {
          resolve(JSON.parse(text.split("\n")[0]));
        } catch (e) {
          reject(e);
        }
      }
    });
    relay.on("exit", (code) => {
      clearTimeout(timer);
      reject(Error("relay exited " + code + ": " + stderr));
    });
  });
  const manifest = JSON.parse(await readFile(join(example, "web/bindings.json"), "utf8"));
  run(process.execPath, [join(root, "node_modules/typescript/bin/tsc"), "--noEmit", "--strict", "--target", "es2022", join(example, "web/bindings.d.ts")]);
  const receipt = { compiler: manifest.compiler, dependency: "github.com/tmc/go-iroh v0.3.0", artifacts: manifest.artifacts, browsers: [] };
  const exercise = async (info2) => {
    const { load, isWrapperError } = await import("/bindings.js");
    const mod = await load(new URL("/module.wasm", location.href), { imports: { Host: { Fetch: async (url, { signal }) => new Uint8Array(await (await fetch(url, { signal })).arrayBuffer()) } } });
    let endpoint, connection, stream;
    const controller = new AbortController();
    const deadline = setTimeout(() => controller.abort(), 3e4);
    const opts = { signal: controller.signal };
    try {
      endpoint = await mod.bind(info2.relay, opts);
      connection = await endpoint.connect(info2.peer, "wasmbind-raw/1", opts);
      stream = await connection.open(opts);
      const data = Uint8Array.from({ length: 64 * 1024 }, (_, i) => i * 31 + 7);
      const reader = stream.readable.getReader(), writer = stream.writable.getWriter();
      const chunks = [];
      let length = 0;
      const receiving = (async () => {
        while (length < data.length) {
          const part = await reader.read();
          if (part.done) throw Error("early stream EOF");
          chunks.push(part.value);
          length += part.value.length;
        }
      })();
      await writer.write(data);
      await receiving;
      await writer.close();
      const echo = new Uint8Array(length);
      let offset = 0;
      for (const part of chunks) {
        echo.set(part, offset);
        offset += part.length;
      }
      if (echo.length !== data.length || echo.some((v, i) => v !== data[i])) throw Error("relay echo mismatch");
      await stream.dispose();
      stream = await connection.open(opts);
      const blocked = stream.readable.getReader();
      const waiting = blocked.read();
      await new Promise((r) => setTimeout(r, 20));
      await blocked.cancel();
      if (!(await waiting).done) throw Error("blocked receive not canceled");
      await stream.dispose();
      stream = null;
      await connection.dispose();
      connection = null;
      const topic = new Uint8Array(32);
      topic[0] = 1;
      const subscription = await endpoint.subscribe(info2.peer, topic, opts);
      let gossip = false;
      for await (const event of subscription) {
        if (new TextDecoder().decode(event.Content) === "relay-gossip") {
          gossip = true;
          break;
        }
      }
      if (!gossip || (await subscription.next()).done !== true) throw Error("gossip iterator cleanup failed");
      try {
        await endpoint.connect("invalid", "wasmbind-raw/1", opts);
        throw Error("missing typed error");
      } catch (e) {
        if (!isWrapperError(e) || e.data.Operation !== "connect") throw e;
      }
      if (new TextDecoder().decode(await mod.fetchFixture(new URL("/fixture", location.href).href, opts)) !== "browser-import") throw Error("imported fetch failed");
      const blob = await endpoint.fetchBlob(info2.peer, info2.hash, opts);
      if (blob.length !== 70 * 1024 || blob.some((v, i) => v !== (i * 17 + 11 & 255))) throw Error("verified blob mismatch");
      const pre = new AbortController();
      pre.abort();
      try {
        await endpoint.connect(info2.peer, "wasmbind-echo/1", { signal: pre.signal });
        throw Error("pre-abort invoked");
      } catch (e) {
        if (e.name !== "AbortError") throw e;
      }
      return { echoBytes: echo.length, blobBytes: blob.length, preAbort: true, streams: true, gossip: true, typedError: true, importedFetch: true };
    } finally {
      clearTimeout(deadline);
      if (stream) await stream.dispose();
      if (connection) await connection.dispose();
      if (endpoint) await endpoint.dispose();
      await mod.dispose();
    }
  };
  await writeFile(join(example, "web/worker.js"), `self.onmessage=async({data})=>{try{const result=await (${exercise})(data);postMessage({result})}catch(e){postMessage({error:e.message})}};`);
  for (const [name, type] of Object.entries({ chromium, firefox, webkit })) {
    const browser = await type.launch({ headless: true });
    try {
      const page = await browser.newPage();
      await page.goto(info.url);
      const result = await page.evaluate(exercise, info);
      const worker = name === "firefox" ? await nativeWorker(type, info, join(example, "web"), dir) : await page.evaluate((info2) => new Promise((resolve, reject) => {
        const w = new Worker("/worker.js", { type: "module" });
        const timer = setTimeout(() => {
          w.terminate();
          reject(Error("Worker timeout"));
        }, 6e4);
        w.onmessage = (e) => {
          clearTimeout(timer);
          w.terminate();
          if (e.data.error) reject(Error(e.data.error));
          else resolve(e.data.result);
        };
        w.onerror = (e) => {
          clearTimeout(timer);
          w.terminate();
          reject(Error(e.message));
        };
        w.postMessage(info2);
      }), info);
      receipt.browsers.push({ name, version: browser.version(), ...result, worker });
      console.log(`${name}: relay echo ${result.echoBytes} bytes, blob ${result.blobBytes} bytes`);
    } finally {
      await browser.close();
    }
  }
  await writeFile(join(task, "qualification.json"), JSON.stringify(receipt, null, 2) + "\n");
  console.log(`Qualification receipt: ${join(task, "qualification.json")}`);
} finally {
  if (relay && relay.exitCode === null && relay.signalCode === null) {
    const exited = new Promise((resolve) => relay.once("exit", resolve));
    relay.kill("SIGTERM");
    await exited;
  }
  await rm(dir, { recursive: true, force: true });
}
