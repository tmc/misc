import { createServer } from "node:http";
import { spawn } from "node:child_process";
import { mkdir, writeFile } from "node:fs/promises";
import { join } from "node:path";
async function nativeWorker(type, info, web, dir) {
  const profile = join(dir, "firefox-profile");
  await mkdir(profile);
  let resolveResult, rejectResult;
  const result = new Promise((resolve, reject) => {
    resolveResult = resolve;
    rejectResult = reject;
  });
  const server = createServer(async (req, res) => {
    if (req.method !== "POST" || req.url != "/result") {
      res.writeHead(404).end();
      return;
    }
    try {
      let body = "";
      for await (const chunk of req) {
        body += chunk;
        if (body.length > 65536) throw Error("oversized Worker result");
      }
      const data = JSON.parse(body);
      res.end();
      if (data.error) rejectResult(Error(data.error));
      else resolveResult(data.result);
    } catch (e) {
      rejectResult(e);
      res.writeHead(400).end();
    }
  });
  await new Promise((r) => server.listen(0, "127.0.0.1", r));
  const callback = `http://127.0.0.1:${server.address().port}/result`;
  await writeFile(join(web, "worker-test.html"), `<!doctype html><script type="module">const w=new Worker('/worker.js',{type:'module'});w.onmessage=e=>{w.terminate();fetch(${JSON.stringify(callback)},{method:'POST',mode:'no-cors',body:JSON.stringify(e.data)})};w.onerror=e=>fetch(${JSON.stringify(callback)},{method:'POST',mode:'no-cors',body:JSON.stringify({error:e.message})});w.postMessage(${JSON.stringify(info)});<\/script>`);
  let stderr = "";
  const child = spawn(type.executablePath(), ["--headless", "--no-remote", "--profile", profile, info.url + "/worker-test.html"], { stdio: ["ignore", "ignore", "pipe"] });
  child.stderr.on("data", (v) => stderr += v);
  const timer = setTimeout(() => rejectResult(Error("native Firefox Worker timeout: " + stderr)), 6e4);
  child.on("error", rejectResult);
  child.on("exit", (code) => rejectResult(Error("native Firefox exited " + code + ": " + stderr)));
  try {
    return await result;
  } finally {
    clearTimeout(timer);
    if (child.exitCode === null && child.signalCode === null) {
      const exited = new Promise((r) => child.once("exit", r));
      child.kill("SIGTERM");
      await exited;
    }
    await new Promise((r) => server.close(r));
  }
}
export {
  nativeWorker
};
