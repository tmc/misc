const abiTags = { null: 1, bool: 2, float: 3, string: 4, bytes: 5, array: 6, record: 7, resource: 8, int: 9, uint: 10 };
function abiTag(t) {
  if (!t) return 1;
  if (t.kind === "pointer") return abiTag(t.elem);
  if (["int", "int8", "int16", "int32", "int64"].includes(t.kind)) return 9;
  if (t.kind.startsWith("uint")) return 10;
  if (t.kind.startsWith("float")) return 3;
  return abiTags[t.kind === "slice" ? "array" : t.kind];
}
function abiJoin(parts) {
  const n = parts.reduce((n2, p) => n2 + p.length, 0);
  if (n > schema.abi.maxBufferBytes) fail("limit", "ABI frame exceeds limit");
  const out = new Uint8Array(n);
  let offset = 0;
  for (const p of parts) {
    out.set(p, offset);
    offset += p.length;
  }
  return out;
}
function abiField(id, tag, data) {
  const out = new Uint8Array(12 + data.length), v = new DataView(out.buffer);
  v.setUint32(0, id, true);
  v.setUint32(4, tag, true);
  v.setUint32(8, data.length, true);
  out.set(data, 12);
  return out;
}
function abiEncode(t, value) {
  if (!t || value === null) return { tag: 1, data: new Uint8Array() };
  if (t.kind === "pointer") return abiEncode(t.elem, value);
  const tag = abiTag(t);
  let data;
  switch (tag) {
    case 2:
      data = new Uint8Array([value ? 1 : 0]);
      break;
    case 4:
      data = new TextEncoder().encode(value);
      break;
    case 5:
      data = new Uint8Array(value);
      break;
    case 6:
      data = abiJoin(value.map((v2, j) => {
        const w = abiEncode(t.elem, v2);
        return abiField(j + 1, w.tag, w.data);
      }));
      break;
    case 7:
      data = abiJoin(records.get(t.name).fields.map((f, j) => {
        const w = abiEncode(f.type, value[f.name]);
        return abiField(j + 1, w.tag, w.data);
      }));
      break;
    case 8:
      data = new Uint8Array(4);
      new DataView(data.buffer).setUint32(0, value, true);
      break;
    default:
      data = new Uint8Array(8);
      const v = new DataView(data.buffer);
      if (tag === 3) v.setFloat64(0, value, true);
      else if (tag === 9) v.setBigInt64(0, BigInt(value), true);
      else v.setBigUint64(0, BigInt(value), true);
  }
  return { tag, data };
}
function abiFields(data) {
  const fields = /* @__PURE__ */ new Map();
  let offset = 0;
  while (offset < data.length) {
    if (data.length - offset < 12) fail("abi-mismatch", "truncated ABI field");
    const v = new DataView(data.buffer, data.byteOffset + offset, 12), id = v.getUint32(0, true), tag = v.getUint32(4, true), n = v.getUint32(8, true);
    if (!id || tag < 1 || tag > 10 || fields.has(id) || n > data.length - offset - 12) fail("abi-mismatch", "invalid ABI field");
    fields.set(id, { tag, data: data.slice(offset + 12, offset + 12 + n) });
    offset += 12 + n;
  }
  return fields;
}
function abiDecode(t, w) {
  if (!w) fail("abi-mismatch", "missing ABI field");
  if (w.tag === 1 && w.data.length === 0 && (!t || t.kind === "pointer" || t.kind === "resource")) return null;
  if (!t) fail("abi-mismatch", "unexpected ABI value");
  if (t.kind === "pointer") return abiDecode(t.elem, w);
  if (w.tag !== abiTag(t)) fail("abi-mismatch", "wrong ABI type tag");
  const data = w.data, v = new DataView(data.buffer, data.byteOffset, data.length);
  switch (w.tag) {
    case 2:
      if (data.length !== 1 || data[0] > 1) fail("abi-mismatch", "invalid bool");
      return !!data[0];
    case 4:
      return new TextDecoder("utf-8", { fatal: true }).decode(data);
    case 5:
      return new Uint8Array(data);
    case 6: {
      const fs = abiFields(data), values = [];
      if (t.kind === "array" && fs.size !== t.length) fail("abi-mismatch", "wrong array length");
      for (let j = 0; j < fs.size; j++) values.push(abiDecode(t.elem, fs.get(j + 1)));
      return values;
    }
    case 7: {
      const fs = abiFields(data), out = /* @__PURE__ */ Object.create(null), decl = records.get(t.name).fields;
      if (fs.size !== decl.length) fail("abi-mismatch", "wrong record fields");
      for (let j = 0; j < decl.length; j++) out[decl[j].name] = abiDecode(decl[j].type, fs.get(j + 1));
      return out;
    }
    case 8:
      if (data.length !== 4) fail("abi-mismatch", "wrong resource size");
      return v.getUint32(0, true);
    default:
      if (data.length !== 8) fail("abi-mismatch", "wrong scalar size");
      if (w.tag === 3) return v.getFloat64(0, true);
      const n = w.tag === 9 ? v.getBigInt64(0, true) : v.getBigUint64(0, true);
      return t.kind === "int64" || t.kind === "uint64" ? n.toString() : Number(n);
  }
}
function abiFrame(op, fields) {
  const payload = abiJoin(fields), out = new Uint8Array(16 + payload.length), v = new DataView(out.buffer);
  out.set([87, 66, 78, 68]);
  v.setUint32(4, 1, true);
  v.setUint32(8, out.length, true);
  v.setUint32(12, op, true);
  out.set(payload, 16);
  return out;
}
async function loadABI(source, limits, Go) {
  const go = new Go(), requests = /* @__PURE__ */ new Map();
  let exports, live, failed = false, disposed = false, timer;
  const statusCodes = ["", "pending", "argument", "limit", "closed", "unknown-request", "consumed", "invalid-allocation", "abi-mismatch"];
  function operation(name, ...args) {
    const value = exports[name](...args) >>> 0, status = exports.wb_status() >>> 0;
    if (status > 8) {
      failed = true;
      live?.exit("unknown ABI status");
      fail("abi-mismatch", "unknown ABI status");
    }
    return { value, status };
  }
  function checked(name, ...args) {
    const result = operation(name, ...args);
    if (result.status) fail(statusCodes[result.status], `ABI ${name}: ${statusCodes[result.status]}`);
    return result.value;
  }
  function submit(op, fields) {
    const frame = abiFrame(op, fields), ptr = checked("wb_alloc", frame.length);
    let id;
    try {
      const memory = exports.mem.buffer;
      if (ptr > memory.byteLength - 4 - frame.length) fail("abi-mismatch", "allocation outside memory");
      new Uint8Array(memory, ptr + 4, frame.length).set(frame);
      id = checked("wb_submit", ptr, frame.length);
    } finally {
      checked("wb_free", ptr);
    }
    return id;
  }
  function poll(id) {
    const result = operation("wb_poll", id);
    if (result.status === 1) return null;
    if (result.status) fail(statusCodes[result.status], "ABI poll failed");
    const ptr = result.value;
    try {
      const memory = exports.mem.buffer;
      if (ptr > memory.byteLength - 4) fail("abi-mismatch", "response outside memory");
      const n = new DataView(memory, ptr, 4).getUint32(0, true);
      if (n < 16 || n > schema.abi.maxBufferBytes || ptr + 4 > memory.byteLength - n) fail("abi-mismatch", "invalid response length");
      const frame = new Uint8Array(memory, ptr + 4, n).slice(), v = new DataView(frame.buffer);
      if (frame[0] !== 87 || frame[1] !== 66 || frame[2] !== 78 || frame[3] !== 68 || v.getUint32(4, true) !== 1 || v.getUint32(8, true) !== n) fail("abi-mismatch", "invalid response frame");
      return { op: v.getUint32(12, true), fields: abiFields(frame.slice(16)) };
    } finally {
      checked("wb_free", ptr);
    }
  }
  function notified(id) {
    setTimeout(() => {
      if (failed || disposed) return;
      const request = requests.get(id);
      if (!request) return;
      try {
        const response = poll(id);
        if (!response) return;
        if (response.op !== request.op || response.fields.size !== 3) fail("abi-mismatch", "wrong ABI response");
        requests.delete(id);
        const code = abiDecode({ kind: "string" }, response.fields.get(1)), message = abiDecode({ kind: "string" }, response.fields.get(2));
        const wire = code ? null : abiDecode(request.result, response.fields.get(3));
        live.complete(request.jsID, code, message, wire);
        if (request.op === 4294967281) disposed = true;
      } catch {
        failed = true;
        live.exit("invalid ABI completion");
      }
    }, 0);
  }
  go.importObject.wasmbind = { notify: (id) => notified(id >>> 0) };
  go.env = { ...go.env, WASMBIND_LIMITS: JSON.stringify(limits) };
  const start = async () => {
    let instance;
    if (source instanceof URL) source = await fetch(source);
    if (source instanceof Response) ({ instance } = await WebAssembly.instantiate(await source.arrayBuffer(), go.importObject));
    else if (source instanceof WebAssembly.Module) instance = await WebAssembly.instantiate(source, go.importObject);
    else ({ instance } = await WebAssembly.instantiate(source, go.importObject));
    if (failed) throw new WasmBindError("runtime-exit", "startup timed out");
    exports = instance.exports;
    const required = ["wb_version", "wb_alloc", "wb_free", "wb_submit", "wb_cancel", "wb_poll", "wb_status", "wb_shutdown"];
    if (required.some((k) => typeof exports[k] !== "function")) fail("abi-mismatch", "missing ABI protocol exports");
    const running = go.run(instance);
    if (go.exited) fail("runtime-exit", "runtime exited before readiness");
    if (exports.wb_version() !== 1) fail("abi-mismatch", "wrong ABI version");
    Promise.resolve(running).then(() => {
      if (!disposed) {
        failed = true;
        live?.exit("runtime exited unexpectedly");
      }
    }, () => {
      failed = true;
      live?.exit("runtime failed unexpectedly");
    });
    const fields = [schema.fingerprint, artifactIdentity].map((s, j) => {
      const w = abiEncode({ kind: "string" }, s);
      return abiField(j + 1, w.tag, w.data);
    });
    const handshake = submit(0, fields), response = poll(handshake);
    if (!response || response.op !== 0 || abiDecode({ kind: "string" }, response.fields.get(1))) fail("abi-mismatch", "handshake failed");
    const bridge = { dispatch(jsID, method, receiver, args) {
      try {
        const c = schema.calls[method], values = [abiEncode({ kind: "uint32" }, receiver), ...c.params.map((p, j) => abiEncode(p.type, args[j]))];
        const id = submit(method + 1, values.map((w, j) => abiField(j + 1, w.tag, w.data)));
        requests.set(id, { jsID, op: method + 1, result: c.result });
        return null;
      } catch (e) {
        return { code: e.code ?? "argument", message: e.message };
      }
    }, cancel(jsID) {
      for (const [id, r] of requests) if (r.jsID === jsID) {
        const status = operation("wb_cancel", id);
        if (status.status !== 0 && status.status !== 6) fail("abi-mismatch", "cancel failed");
        return;
      }
    }, disposeResource(jsID, handle) {
      try {
        const w = abiEncode({ kind: "uint32" }, handle), id = submit(4294967280, [abiField(1, w.tag, w.data)]);
        requests.set(id, { jsID, op: 4294967280, result: null });
      } catch (e) {
        live.complete(jsID, e.code ?? "closed", e.message, null);
      }
    }, shutdown(jsID) {
      try {
        const id = submit(4294967281, []);
        requests.set(id, { jsID, op: 4294967281, result: null });
        checked("wb_shutdown");
      } catch (e) {
        live.complete(jsID, e.code ?? "closed", e.message, null);
      }
    } };
    live = makeModule(bridge, limits, /* @__PURE__ */ new Map());
    return live.module;
  };
  const timeout = new Promise((_, reject) => {
    timer = setTimeout(() => {
      failed = true;
      if (exports) {
        try {
          operation("wb_shutdown");
        } catch {
        }
      }
      reject(new WasmBindError("runtime-exit", "startup timed out; use a dedicated Worker for hard termination"));
    }, limits.startupTimeoutMs);
  });
  try {
    return await Promise.race([start(), timeout]);
  } catch (e) {
    failed = true;
    if (exports) {
      try {
        operation("wb_shutdown");
      } catch {
      }
    }
    throw e instanceof WasmBindError ? e : new WasmBindError("runtime-exit", "ABI startup failed");
  } finally {
    clearTimeout(timer);
  }
}
