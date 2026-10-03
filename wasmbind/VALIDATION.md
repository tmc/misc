# Local qualification

The implementation targets Go 1.27. The local qualification used Go 1.27.0
(darwin/arm64), TypeScript 6.0.3, Playwright 1.61.1, and:

| Engine | Version | General fixture | Relay fixture |
| --- | --- | --- | --- |
| Chromium | 149.0.7827.55 | 89 assertions | 65,536-byte echo; 71,680-byte blob |
| Firefox | 151.0 | 89 assertions | 65,536-byte echo; 71,680-byte blob |
| WebKit | 26.5 | 89 assertions | 65,536-byte echo; 71,680-byte blob |

The general fixture exercises all scalar widths, exact bigint extremes,
float overflow, record null/missing/unknown fields, throwing getters, malformed
strings, cycles, detached byte buffers, input/output copy isolation, collection
limits, and array lengths. It also covers application errors and panics, browser
HTTP yielding, pre-abort and abort/completion races, receiver queues, foreign
and forged resources, duplicate pointer returns, closed-pointer re-export,
close failures, repeated disposal, retained arguments, canceled queued calls,
late resource returns, rollback after collection conversion fails, module
teardown, and an application call that ignores cancellation.

Test-only companion code observes the live registry, retained calls, and
callback cleanup during repeated resource cycles. Abrupt `os.Exit` does not run
Go defers; its test-only observer is removed from JS after the exit rather than
counted as graceful callback release. Production wrappers reject after runtime
exit and drop their bridge reference.

Loader tests cover URL, Response, ArrayBuffer, and compiled Module inputs;
simultaneous instances with equal export names; early and unexpected runtime
exit; startup timeout; conflicting runtime identity; schema mismatch; and a
body-only mismatch with an unchanged schema. A dedicated Worker fixture runs in
each engine. Generated declarations pass TypeScript with `--noEmit --strict`.

The relay fixture uses `github.com/tmc/go-iroh v0.3.0`, a Go loopback WebSocket
relay, and a native Go peer. Actual generated endpoint/connection/stream APIs
exchange a 64-KiB framed message and fetch a verified 70-KiB BLAKE3 blob. Each
engine checks pre-abort and disposes all resources. These results cover that
relay-only Go-to-Go path; they do not qualify direct UDP, remote relays, or Rust
interoperability.

Native checks pass:

```sh
go test -race ./...
go vet ./...
go -C examples/goiroh test ./...
go -C examples/goiroh vet ./...
```

Native tests cover source-position diagnostics, aliases, specializations and
constraints, names, signatures, record graphs, tags, a golden manifest,
deterministic generation/builds, output ownership, stale artifact removal,
path checks, and preservation of an existing bundle after compiler failure.
CLI integration uses `rsc.io/script` txtar fixtures. Exported helpers and the
small echo package have runnable examples.

The repository-root workflow requires native checks, TypeScript, all three
browser engines, and the relay fixture. There is no observed hosted CI result
or release/tag in this change. Browser Window and dedicated Worker are the
qualified realms; Node, Deno, and Bun remain unqualified.

Machine-readable receipts, including generated artifact hashes, are kept at:

- `~/tmp/wasmbind/20261003-browser-tests/qualification.json`
- `~/tmp/wasmbind/20261003-goiroh-tests/qualification.json`

The scripts replace these receipts only after a complete successful run and
remove throwaway builds. No throughput, bundle-size, or zero-copy claim is made.
