async function check(host) {
  const { load, Kind, OpenKind, Boundary } = await import("./bindings.js");
  let passed = 0;
  const assert = (ok, message) => {
    if (!ok) throw Error(message);
    passed++;
  };
  const reject = async (fn, code) => {
    try {
      await fn();
    } catch (e) {
      assert(e.code === code, `${code}: ${e.code} ${e.message}`);
      return;
    }
    throw Error("expected " + code);
  };
  assert(Object.isFrozen(Kind) && Kind.first === Kind.alias && Kind.last === 255, "enum object");
  assert(Boundary.min === -(1n << 63n) && Boundary.max === (1n << 63n) - 1n, "bigint constants");
  const source = new URL("./module.wasm", import.meta.url);
  const m = await load(source);
  try {
    const r = await m.httpResult(false);
    assert(r.httpStatus === 200 && r.url === "https://example.com" && r.Exact === "override" && r.kind === Kind.first, "camelCase and field override");
    await reject(() => m.httpResult(true), "contract");
    await reject(() => m.resource(true), "contract");
    const resource = await m.resource(false);
    await resource.dispose();
    assert(resource !== null, "nonnull resource");
    assert(await m.kind(Kind.alias) === Kind.first, "enum alias");
    await reject(() => m.kind(2), "argument");
    await reject(() => m.invalidKind(), "argument");
    assert(await m.openKind(73) === 73 && OpenKind.known === 1, "open enum");
    assert(await m.boundary(Boundary.min) === Boundary.min && await m.boundary(Boundary.max) === Boundary.max, "bigint values");
    if (host) {
      assert(await m.identifier("id") === "id", "scalar");
      await reject(() => m.identifier(""), "go-error");
      await reject(() => m.invalidIdentifier(), "go-error");
      assert(await m.fetchHost("id") === "default:id", "Go default factory");
      assert(await m.emptyDefaults() === 1, "empty interface default factory");
    }
  } finally {
    await m.dispose();
  }
  if (host) {
    await reject(() => load(source, { imports: { Host: {} } }), "argument");
    await reject(() => load(source, { imports: { Host: null } }), "argument");
    let reads = 0;
    const imports = { Empty: {}, get Host() {
      reads++;
      return { fetch(identifier, { signal }) {
        assert(identifier === "id" && !signal.aborted, "host args");
        return "override:" + identifier;
      } };
    } };
    const override = await load(source, { imports });
    try {
      assert(await override.fetchHost("id") === "override:id", "explicit import override");
      assert(reads === 1, "capture interface once");
      assert(await override.emptyDefaults() === 0, "provided empty interface skips factory");
    } finally {
      await override.dispose();
    }
  }
  return passed;
}
export {
  check
};
