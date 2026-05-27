package webkithost

// NativeBridgeScript exposes host-backed Wanix devices to pages loaded in the
// WebKit view. It is injected for both bundled and external Wanix pages.
const NativeBridgeScript = `
(() => {
  if (window.__wanixMacOSFS) return;
  window.__wanixCloneSchemas = __WANIX_CLONE_SCHEMAS__;
  window.__wanixInitialMacOSFS = __WANIX_INITIAL_FS__;
  const post = (msg) => window.webkit.messageHandlers.wanix.postMessage(JSON.stringify(msg));
  let nextNativeFSID = 0;
  const nativeFSPending = new Map();
  window.__wanixNativeFSReply = (id, reply) => {
    const pending = nativeFSPending.get(id);
    if (!pending) return;
    nativeFSPending.delete(id);
    if (!reply || !reply.ok) {
      pending.reject(new Error(reply && reply.error || "native fs call failed"));
      return;
    }
    pending.resolve(reply.value);
  };
  const nativeFS = (method, params = {}) => new Promise((resolve, reject) => {
    const id = String(++nextNativeFSID);
    nativeFSPending.set(id, {resolve, reject});
    post({type: "native.fs", id, method, params});
  });
  const bytesToBase64 = (data) => {
    if (typeof data === "string") data = new TextEncoder().encode(data);
    let binary = "";
    for (let i = 0; i < data.length; i++) binary += String.fromCharCode(data[i]);
    return btoa(binary);
  };
  const base64ToBytes = (encoded) => {
    const binary = atob(encoded || "");
    const bytes = new Uint8Array(binary.length);
    for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
    return bytes;
  };
  const clean = (name) => {
    name = (name || "").replace(/^\/+|\/+$/g, "");
    return name === "." ? "" : name;
  };
  const dirname = (name) => {
    name = clean(name);
    const i = name.lastIndexOf("/");
    return i < 0 ? "" : name.slice(0, i);
  };
  const basename = (name) => {
    name = clean(name);
    const i = name.lastIndexOf("/");
    return i < 0 ? name : name.slice(i + 1);
  };
  const macFiles = new Map([["status", "api macos\nstatus ok\n"]]);
  const macDirs = new Map([["", [{name: "status", dir: false}]]]);
  const rememberEntry = (name, dir) => {
    name = clean(name);
    const parent = dirname(name);
    const base = basename(name);
    if (!macDirs.has(parent)) macDirs.set(parent, []);
    const entries = macDirs.get(parent);
    if (base && !entries.some((entry) => entry.name === base)) entries.push({name: base, dir});
    if (dir && !macDirs.has(name)) macDirs.set(name, []);
  };
  const cacheText = (name, text) => {
    name = clean(name);
    macFiles.set(name, String(text));
    rememberEntry(name, false);
  };
  const cacheBytes = (name, data) => {
    name = clean(name);
    macFiles.set(name, data instanceof Uint8Array ? new Uint8Array(data) : data);
    rememberEntry(name, false);
  };
  for (const [name, entries] of Object.entries(window.__wanixInitialMacOSFS.dirs || {})) {
    macDirs.set(clean(name), entries);
  }
  for (const [name, encoded] of Object.entries(window.__wanixInitialMacOSFS.files || {})) {
    cacheBytes(name, base64ToBytes(encoded));
  }
  let globalCloneNext = 0;
  const cloneFiles = (svc) => window.__wanixCloneSchemas[svc] || {};
  const createClone = (svc) => {
    const id = String(++globalCloneNext);
    rememberEntry(svc + "/" + id, true);
    for (const [name, text] of Object.entries(cloneFiles(svc))) {
      cacheText(svc + "/" + id + "/" + name, text);
    }
    return id + "\n";
  };
  const hydrateMacOSFS = async (name = "", depth = 3) => {
    name = clean(name);
    const entries = await nativeFS("readDir", {path: name || "."});
    const merged = macDirs.get(name) || [];
    for (const entry of entries) {
      if (!merged.some((old) => old.name === entry.name)) merged.push(entry);
    }
    macDirs.set(name, merged);
    for (const entry of entries) {
      const child = clean(name ? name + "/" + entry.name : entry.name);
      rememberEntry(child, !!entry.dir);
      if (entry.dir) {
        if (depth > 0) await hydrateMacOSFS(child, depth - 1);
      } else {
        try {
          cacheText(child, new TextDecoder().decode(base64ToBytes(await nativeFS("readFile", {path: child}))));
        } catch (_) {
        }
      }
    }
  };
  const refreshCachedFile = async (name) => {
    try {
      cacheBytes(name, base64ToBytes(await nativeFS("readFile", {path: name})));
    } catch (_) {
    }
  };
  const refreshCachedDir = async (name) => {
    name = clean(name);
    try {
      const entries = await nativeFS("readDir", {path: name || "."});
      macDirs.set(name, entries);
      for (const entry of entries) {
        const child = clean(name ? name + "/" + entry.name : entry.name);
        rememberEntry(child, !!entry.dir);
        if (!entry.dir) await refreshCachedFile(child);
      }
    } catch (_) {
    }
  };
  const refreshAfterWrite = async (name) => {
    await refreshCachedFile(name);
    const parent = dirname(name);
    await refreshCachedDir(parent);
  };
  window.__wanixHydrateMacOSFS = hydrateMacOSFS;
  window.__wanixMacOSFS = {
    _updateCache: (name, encoded) => {
      cacheBytes(name, base64ToBytes(encoded));
    },
    _appendCache: (name, encoded) => {
      const cleanName = clean(name);
      const chunk = base64ToBytes(encoded);
      let old = macFiles.get(cleanName);
      if (!Array.isArray(old)) {
        old = old ? [old instanceof Uint8Array ? old : new TextEncoder().encode(String(old))] : [];
        macFiles.set(cleanName, old);
      }
      old.push(chunk);
      rememberEntry(cleanName, false);
    },
    _set: async (name, value) => {
      cacheText(name, value);
      await nativeFS("writeFile", {path: name, data: bytesToBase64(String(value))});
    },
    isDir: (name) => {
      name = clean(name);
      return macDirs.has(name);
    },
    readDir: (name) => {
      name = clean(name);
      return macDirs.get(name) || [];
    },
    readFile: (name) => {
      name = clean(name);
      if (name.endsWith("/clone")) {
        const svc = name.slice(0, -"/clone".length);
        return createClone(svc);
      }
      let value = macFiles.get(name);
      if (Array.isArray(value)) {
        const total = value.reduce((n, chunk) => n + chunk.length, 0);
        const flat = new Uint8Array(total);
        let offset = 0;
        for (const chunk of value) {
          flat.set(chunk, offset);
          offset += chunk.length;
        }
        value = flat;
        macFiles.set(name, value);
      }
      if (value instanceof Uint8Array) {
        let out = "";
        for (let i = 0; i < value.length; i += 4096) {
          out += String.fromCharCode.apply(null, value.subarray(i, i + 4096));
        }
        return out;
      }
      return value || "";
    },
    writeFile: (name, data) => {
      if (typeof data === "string") {
        cacheText(name, data);
      } else {
        cacheBytes(name, data);
      }
      nativeFS("writeFile", {path: name, data: bytesToBase64(data)}).then(() => refreshAfterWrite(name));
      return "ok";
    }
  };
})();
`

// BootstrapHTML loads Wanix from the asset server and reports startup state
// through the WebKit script-message bridge.
const BootstrapHTML = `<!doctype html>
<html>
<head>
  <meta charset="utf-8">
  <title>Wanix</title>
  <style>
    html, body, wanix-system {
      margin: 0;
      width: 100%;
      height: 100%;
      background: #101010;
      color: #f5f5f5;
    }
    wanix-system {
      display: block;
    }
    wanix-term {
      display: block;
      width: 100%;
      height: 100%;
    }
  </style>
</head>
<body>
  <wanix-system id="system" wasm="./wanix.debug.wasm" debug>
    <wanix-bind dst="task" src="#task"></wanix-bind>
    <wanix-bind dst="term" src="#term"></wanix-bind>
    <wanix-bind dst="web" src="#web"></wanix-bind>
    <wanix-bind dst="js" src="#js"></wanix-bind>
    <wanix-bind dst="tmp" src="#ramfs"></wanix-bind>
    <wanix-bind dst="macos" src="#macos"></wanix-bind>
    <wanix-bind dst="mnt/macos" src="#macos"></wanix-bind>
    <wanix-bind type="fetch" dst="rc.wasm" src="./rc.wasm"></wanix-bind>
    <wanix-task id="rc" cmd="rc.wasm" type="gojs" wd="web" term start></wanix-task>
    <wanix-term path="#task/rc/term"></wanix-term>
  </wanix-system>
  <script>
    window.wanixHostControllerStarted = true;
    const post = (msg) => window.webkit.messageHandlers.wanix.postMessage(JSON.stringify(msg));
    const bootTimer = setTimeout(() => post({type: "error", message: "wanix bootstrap timeout"}), 20000);
    const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
    async function waitForRuntimeHooks(system) {
      const deadline = Date.now() + 20000;
      while (Date.now() < deadline) {
        const setup = String(system._setupNamespace);
        const open = String(system._openPort);
        if (!setup.includes("wasm not ready") && !open.includes("wasm not ready")) {
          return;
        }
        await sleep(25);
      }
      throw new Error("wanix runtime hooks timeout");
    }
    async function ensureNamespace(system) {
      window.wanixHostEnsureNamespaceStarted = true;
      await waitForRuntimeHooks(system);
      window.wanixHostRuntimeHooksReady = true;
      if (system.isReady) {
        return;
      }
      await system._setupNamespace("1", "", system.querySelectorAll(":scope > wanix-bind"));
      system.isReady = true;
      await window.__wanixHydrateMacOSFS("", 3);
      window.wanixHostNamespaceEnsured = true;
    }
    const postError = (err) => {
      const message = String(err && (err.stack || err.message) || err);
      if (message.includes("Go program has already exited")) return;
      post({type: "error", message});
    };
    window.addEventListener("error", (event) => postError(event.error || event.message));
    window.addEventListener("unhandledrejection", (event) => postError(event.reason));
    (async () => {
      await customElements.whenDefined("wanix-system");
      const system = document.getElementById("system");
      window.wanixHost = { system };
      if (!system) {
        post({type: "error", message: "wanix-system element missing"});
        return;
      }
      const ready = new Promise((resolve, reject) => {
        if (system.isReady) {
          resolve();
          return;
        }
        system.addEventListener("ready", resolve, {once: true});
        system.addEventListener("error", (event) => reject(event.detail && event.detail.error || event.error || "wanix-system error"), {once: true});
      });
      const fallbackReady = ensureNamespace(system);
      const readyTimer = setTimeout(() => {
        post({
          type: "error",
          message: "wanix-system ready event timeout",
          value: {
            tagName: system.tagName,
            childCount: system.childElementCount,
            wasm: system.getAttribute("wasm"),
            debug: system.hasAttribute("debug"),
            isReady: !!system.isReady,
            readyType: typeof system._ready
          }
        });
      }, 20000);
      await Promise.race([ready, fallbackReady]);
      await window.__wanixHydrateMacOSFS("", 3);
      clearTimeout(readyTimer);
      clearTimeout(bootTimer);
      post({
        type: "ready",
        capabilities: {
          wasm: typeof WebAssembly !== "undefined",
          worker: typeof Worker !== "undefined",
          messageChannel: typeof MessageChannel !== "undefined",
          blob: typeof Blob !== "undefined",
          fetch: typeof fetch !== "undefined",
          sharedArrayBuffer: typeof SharedArrayBuffer !== "undefined",
          crossOriginIsolated: !!globalThis.crossOriginIsolated
        }
      });
    })().catch(postError);
  </script>
  <script type="module" src="./wanix.min.js"></script>
</body>
</html>
`

// CapabilityProbe is a small deterministic script for the first self-test.
const CapabilityProbe = `(() => JSON.stringify({
  wasm: typeof WebAssembly !== "undefined",
  worker: typeof Worker !== "undefined",
  messageChannel: typeof MessageChannel !== "undefined",
  blob: typeof Blob !== "undefined",
  fetch: typeof fetch !== "undefined",
  sharedArrayBuffer: typeof SharedArrayBuffer !== "undefined",
  crossOriginIsolated: !!globalThis.crossOriginIsolated,
  system: !!document.getElementById("system")
}))()`
