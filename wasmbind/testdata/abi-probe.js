async function probe() {
  const manifest = await (await fetch("/web/bindings.json")).json();
  const { getGo } = await import("/web/runtime.js");
  const Go = await getGo(), go = new Go();
  let exports, notify;
  const notifications = [];
  go.importObject.wasmbind = { notify: (id) => {
    notifications.push(id >>> 0);
    notify?.();
  } };
  go.env = { ...go.env, WASMBIND_LIMITS: JSON.stringify(manifest.limits) };
  const { instance } = await WebAssembly.instantiate(await (await fetch("/web/module.wasm")).arrayBuffer(), go.importObject);
  exports = instance.exports;
  const running = go.run(instance);
  let passed = 0;
  const assert = (value, message) => {
    if (!value) throw Error(message);
    passed++;
  };
  const call = (name, ...args) => {
    const value = exports[name](...args) >>> 0, status = exports.wb_status() >>> 0;
    return { value, status };
  };
  assert(exports.wb_version() === 1, "version");
  assert(call("wb_poll", 4294967295).status === 5, "unknown request");
  const ptr = call("wb_alloc", 16).value;
  assert(!!ptr, "allocation");
  assert(call("wb_free", ptr + 1).status === 7, "interior free");
  assert(call("wb_submit", ptr + 1, 15).status === 7, "interior submission");
  assert(call("wb_submit", ptr, 17).status === 2, "allocation size validation");
  call("wb_free", ptr);
  assert(call("wb_free", ptr).status === 7, "double free");
  const field = (id, tag, data) => {
    const out = new Uint8Array(12 + data.length), v = new DataView(out.buffer);
    v.setUint32(0, id, true);
    v.setUint32(4, tag, true);
    v.setUint32(8, data.length, true);
    out.set(data, 12);
    return out;
  };
  const frame = (op, fields2 = []) => {
    const n = 16 + fields2.reduce((n2, f) => n2 + f.length, 0), out = new Uint8Array(n), v = new DataView(out.buffer);
    out.set([87, 66, 78, 68]);
    v.setUint32(4, 1, true);
    v.setUint32(8, n, true);
    v.setUint32(12, op, true);
    let offset = 16;
    for (const f of fields2) {
      out.set(f, offset);
      offset += f.length;
    }
    return out;
  };
  const submit = (data) => {
    const ptr2 = call("wb_alloc", data.length).value;
    if (!ptr2) throw Error("allocation failed");
    new Uint8Array(exports.mem.buffer, ptr2 + 4, data.length).set(data);
    const result = call("wb_submit", ptr2, data.length);
    call("wb_free", ptr2);
    return result;
  };
  assert(submit(frame(1)).status === 8, "handshake required");
  const wrong = frame(0);
  new DataView(wrong.buffer).setUint32(4, 2, true);
  assert(submit(wrong).status === 8, "wrong version");
  const short = frame(0);
  new DataView(short.buffer).setUint32(8, 17, true);
  assert(submit(short).status === 2, "wrong length");
  const text = (s) => new TextEncoder().encode(s), fields = [field(1, 4, text(manifest.fingerprint)), field(2, 4, text(manifest.buildIdentity))];
  assert(submit(frame(0, [fields[0], fields[0]])).status === 2, "duplicate fields");
  assert(submit(frame(0, [field(1, 99, new Uint8Array())])).status === 2, "unknown tag");
  assert(submit(frame(0, [field(1, 4, text("wrong")), fields[1]])).status === 8, "wrong fingerprint");
  const handshake = submit(frame(0, fields));
  assert(handshake.status === 0 && handshake.value, "handshake");
  const response = call("wb_poll", handshake.value);
  assert(response.value && response.status === 0, "terminal poll");
  call("wb_free", response.value);
  assert(call("wb_poll", handshake.value).status === 6, "double consume");
  assert(call("wb_cancel", handshake.value).status === 6, "cancel consumed");
  const before = exports.mem.buffer;
  const allocations = [];
  for (let j = 0; j < 3; j++) {
    const result = call("wb_alloc", 8 << 20);
    assert(result.value && result.status === 0, "bounded large allocation");
    allocations.push(result.value);
  }
  assert(call("wb_alloc", 8 << 20).status === 3, "total buffer limit");
  assert(exports.mem.buffer !== before, "memory growth recreates buffer");
  for (const p of allocations) assert(call("wb_free", p).status === 0, "free after growth");
  const c = manifest.calls.find((c2) => c2.name === "wait"), u = new Uint8Array(8);
  new DataView(u.buffer).setBigUint64(0, 0n, true);
  const arg = new Uint8Array(8);
  new DataView(arg.buffer).setBigInt64(0, 1000n, true);
  const request = submit(frame(c.id + 1, [field(1, 10, u), field(2, 9, arg)]));
  assert(request.status === 0, "async admission");
  assert(call("wb_poll", request.value).status === 1, "pending");
  call("wb_cancel", request.value);
  const wait = async (id) => {
    for (let j = 0; j < 2e3; j++) {
      const result = call("wb_poll", id);
      if (result.status === 0) return result.value;
      if (result.status !== 1) throw Error("poll status " + result.status);
      await new Promise((r) => setTimeout(r, 1));
    }
    throw Error("notification timeout");
  };
  const canceled = await wait(request.value);
  assert(notifications.includes(request.value), "host completion notification");
  call("wb_free", canceled);
  const shutdown = submit(frame(4294967281));
  call("wb_shutdown");
  assert(call("wb_alloc", 1).status === 4, "closed admission");
  const closed = await wait(shutdown.value);
  call("wb_free", closed);
  await running;
  return { passed };
}
export {
  probe
};
