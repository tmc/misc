package webkithost

// NativeBridgeScript exposes host-backed Wanix devices to pages loaded in the
// WebKit view. It is injected for both bundled and external Wanix pages.
const NativeBridgeScript = `
(() => {
  if (window.__wanixMacOSFS) return;
  const post = (msg) => window.webkit.messageHandlers.wanix.postMessage(JSON.stringify(msg));
  const nativeCall = (method, params = {}) => {
    post({type: "native.call", method, params});
    return "ok";
  };
  const dirs = new Set(["", "app", "window", "window/0", "pasteboard", "dialog", "dialog/open", "alert"]);
  const files = new Set([
    "status",
    "app/name", "app/ctl",
    "window/0/title", "window/0/ctl",
    "pasteboard/text",
    "dialog/open/ctl", "dialog/open/result",
    "alert/title", "alert/message", "alert/style", "alert/buttons", "alert/result", "alert/show"
  ]);
  const state = {
    "status": JSON.stringify({api: "macos", ok: true}) + "\n",
    "app/name": "Wanix\n",
    "pasteboard/text": "",
    "dialog/open/result": "",
    "alert/title": "Wanix\n",
    "alert/message": "",
    "alert/style": "informational\n",
    "alert/buttons": "OK\n",
    "alert/result": ""
  };
  const children = (prefix) => {
    const out = new Map();
    for (const dir of dirs) {
      if (dir === prefix) continue;
      if (prefix && !dir.startsWith(prefix + "/")) continue;
      const rest = prefix ? dir.slice(prefix.length + 1) : dir;
      if (rest && !rest.includes("/")) out.set(rest, {name: rest, dir: true});
    }
    for (const file of files) {
      if (prefix && !file.startsWith(prefix + "/")) continue;
      const rest = prefix ? file.slice(prefix.length + 1) : file;
      if (rest && !rest.includes("/")) out.set(rest, {name: rest, dir: false});
    }
    return [...out.values()];
  };
  window.__wanixMacOSFS = {
    _set: (name, value) => {
      if (!files.has(name)) throw new Error("not found: " + name);
      state[name] = String(value);
    },
    isDir: (name) => dirs.has(name),
    readDir: (name) => children(name),
    readFile: (name) => {
      if (!files.has(name)) throw new Error("not found: " + name);
      return state[name] || "";
    },
    writeFile: (name, data) => {
      const text = String(data).trimEnd();
      switch (name) {
      case "app/name":
        state[name] = text + "\n";
        return nativeCall("app.name", {value: text});
      case "app/ctl":
        return nativeCall("app." + text);
      case "window/0/title":
        state[name] = text + "\n";
        return nativeCall("window.0.title", {value: text});
      case "window/0/ctl":
        return nativeCall("window.0.ctl", {command: text});
      case "pasteboard/text":
        state[name] = text;
        return nativeCall("pasteboard.text", {value: text});
      case "dialog/open/ctl":
        state["dialog/open/result"] = "";
        return nativeCall("dialog.open", {json: text || "{}"});
      case "alert/title":
      case "alert/message":
      case "alert/style":
      case "alert/buttons":
        state[name] = text + "\n";
        return "ok";
      case "alert/show":
        return nativeCall("alert.show", {
          title: (state["alert/title"] || "").trim(),
          message: (state["alert/message"] || "").trim(),
          style: (state["alert/style"] || "").trim(),
          buttons: (state["alert/buttons"] || "").trim()
        });
      default:
        throw new Error("permission denied: " + name);
      }
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
    <wanix-bind dst="mnt/macos" src="#macos"></wanix-bind>
    <wanix-bind type="fetch" dst="rc.wasm" src="./rc.wasm"></wanix-bind>
    <wanix-task id="rc" cmd="rc.wasm" type="gojs" wd="web" term start></wanix-task>
    <wanix-term path="#task/rc/term"></wanix-term>
  </wanix-system>
  <script type="module">
    const post = (msg) => window.webkit.messageHandlers.wanix.postMessage(JSON.stringify(msg));
    window.addEventListener("error", (event) => post({type: "error", message: String(event.message || event.error)}));
    window.addEventListener("unhandledrejection", (event) => post({type: "error", message: String(event.reason)}));
    import("./wanix.js").then(async () => {
      const system = document.getElementById("system");
      window.wanixHost = { system };
      await system._ready;
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
    }).catch((err) => post({type: "error", message: String(err && (err.stack || err.message) || err)}));
  </script>
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
