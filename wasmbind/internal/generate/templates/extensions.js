function boundedMessage(error) {
  let message;
  try {
    message = typeof error?.message === "string" ? error.message : String(error);
  } catch {
    message = "host function failed";
  }
  message = message.slice(0, 4096);
  while (new TextEncoder().encode(message).length > 4096) message = message.slice(0, -1);
  return message;
}
function captureImports(value) {
  const imports = /* @__PURE__ */ new Map();
  imports.provided = new Set();
  for (const d of schema.declarations.filter((d2) => d2.kind === "imports")) {
    const host = value?.[d.name];
    if (host === undefined && d.optional) continue;
    if (!host || typeof host !== "object") fail("argument", "missing import interface " + d.name);
    imports.provided.add(d.name);
    for (const c of d.importMethods ?? []) {
      const fn = host[c.name];
      if (typeof fn !== "function") fail("argument", "missing import method " + d.name + "." + c.name);
      imports.set(d.name + "." + c.name, fn.bind(host));
    }
  }
  return imports;
}
function makeViews(w, d, limits, call, isClosed) {
  const methods = d.methods ?? {}, closers = [];
  const invoke = (op, args = []) => call(methods[op], args);
  if (methods.read) {
    let terminal, finished = false, controller, active;
    const read = new ReadableStream({
      start(c) {
        controller = c;
      },
      async pull(c) {
        if (terminal) {
          c.error(terminal);
          finished = true;
          return;
        }
        if (isClosed()) {
          c.error(new WasmBindError("closed", "resource is closed"));
          finished = true;
          return;
        }
        try {
          for (let retries = 0; ; retries++) {
            active = invoke("read", [limits.streamChunkBytes]);
            const result = await active;
            active = null;
            if (finished) return;
            if (result.data.length) c.enqueue(result.data);
            if (result.eof) {
              c.close();
              finished = true;
              return;
            }
            if (result.message) {
              terminal = result.error ?? new WasmBindError("go-error", result.message);
              if (!result.data.length) {
                c.error(terminal);
                finished = true;
              }
              return;
            }
            if (result.data.length) return;
            if (retries >= 99) throw new WasmBindError("go-error", "multiple Read calls return no data or error");
            await new Promise((resolve) => setTimeout(resolve, 0));
          }
        } catch (e) {
          if (!finished) c.error(e);
          finished = true;
        }
      },
      async cancel() {
        if (finished) return;
        finished = true;
        const operation = invoke("cancelread");
        await Promise.all([operation, active?.catch(() => {
        })]);
      }
    }, { highWaterMark: 0 });
    Object.defineProperty(w, "readable", { value: read, enumerable: true });
    closers.push(() => {
      if (!finished) {
        finished = true;
        controller.error(new WasmBindError("closed", "resource is closed"));
      }
    });
  }
  if (methods.write) {
    let finished = false, controller, active, interruption;
    const write = new WritableStream({
      start(c) {
        controller = c;
        c.signal.addEventListener("abort", () => {
          if (!finished) {
            interruption = invoke("cancelwrite");
            interruption.catch(() => {
            });
          }
        }, { once: true });
      },
      async write(data) {
        if (isClosed()) throw new WasmBindError("closed", "resource is closed");
        if (!(data instanceof Uint8Array)) throw new WasmBindError("argument", "expected Uint8Array");
        if (!data.length) return;
        active = invoke("write", [data]);
        try {
          await active;
        } finally {
          active = null;
        }
      },
      async close() {
        await invoke("finish");
        finished = true;
      },
      async abort() {
        if (finished) return;
        finished = true;
        const operation = interruption ?? invoke("cancelwrite");
        await Promise.all([operation, active?.catch(() => {
        })]);
      }
    }, new ByteLengthQueuingStrategy({ highWaterMark: limits.streamHighWaterBytes }));
    Object.defineProperty(w, "writable", { value: write, enumerable: true });
    closers.push(() => {
      if (!finished) {
        finished = true;
        controller.error(new WasmBindError("closed", "resource is closed"));
      }
    });
  }
  if (methods.next) {
    let busy = false, terminal, finished = false, returning;
    w[Symbol.asyncIterator] = () => w;
    w.next = async () => {
      if (terminal) throw terminal;
      if (finished) return { done: true, value: void 0 };
      if (busy) throw new WasmBindError("busy", "one next call is already pending");
      busy = true;
      try {
        const value = await invoke("next");
        if (finished) return { done: true, value: void 0 };
        if (value.done) {
          finished = true;
          await w.dispose();
        }
        return value;
      } catch (e) {
        if (finished && returning) return { done: true, value: void 0 };
        terminal = e;
        finished = true;
        try {
          await w.dispose();
        } catch (cleanup) {
          try {
            e.cause = cleanup;
          } catch {
          }
        }
        throw e;
      } finally {
        busy = false;
      }
    };
    w.return = () => returning ??= (async () => {
      finished = true;
      await w.dispose();
      return { done: true, value: void 0 };
    })();
    w.throw = async (reason) => {
      try {
        await w.return();
      } catch (cleanup) {
        if (reason && typeof reason === "object") {
          try {
            reason.cause = cleanup;
          } catch {
          }
        }
      }
      throw reason;
    };
    closers.push(() => {
      finished = true;
    });
  }
  return { close() {
    for (const close of closers) close();
  } };
}
