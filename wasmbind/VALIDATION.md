# Local qualification

The implementation targets Go 1.27. The local qualification used Go 1.27.0
(darwin/arm64), TypeScript 6.0.3, Playwright 1.61.1, and:

| Engine | Version | General fixture | Relay fixture |
| --- | --- | --- | --- |
| Chromium | 149.0.7827.55 | 90 assertions | 65,536-byte echo; 71,680-byte blob |
| Firefox | 151.0 | 90 assertions | 65,536-byte echo; 71,680-byte blob |
| WebKit | 26.5 | 90 assertions | 65,536-byte echo; 71,680-byte blob |

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
exchange a 64-KiB duplex stream message and fetch a verified 70-KiB BLAKE3 blob. Each
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

## Extension candidate matrix

| Backend and features | Window | Dedicated Worker | Toolchain |
| --- | --- | --- | --- |
| syscalljs, base | 90 assertions per engine | copied bytes and disposal | Go 1.27.0 |
| syscalljs, streams/events/imports/errors together | 44-assertion extension fixture | stream/event/import exercise | Go 1.27.0 |
| syscalljs, go-iroh extensions | duplex echo, blocked cancellation, gossip early break, typed validation error, imported HTTP, verified blob | same network fixture | Go 1.27.0 |
| abi, base, experimental | 90 shared assertions and 29 direct protocol assertions per engine | copied bytes and disposal | Go 1.27.0 and 1.27.1 |
| abi with any extension | rejected | rejected | all |
| pack, base | installed ES module and esbuild consumer | bundled Worker using matching copied assets | Go 1.27.0 |

All three pinned engines are required in these rows. Extension declarations pass
strict ES2022 DOM/DOM.Iterable and WebWorker fixtures with skipLibCheck=false,
including readonly/opaque-resource/import-result negative tests. Error tests
include wrapped/joined matches, bounded cycles, malformed data fallback, and
private-brand guards. Imports test captured callbacks, HTTP, thrown and malformed
results, ignored abort, never-settling promises, initialization failure, and
same-instance reentrancy. Streams test final EOF/error chunks, no prefetch,
short/zero writes, invalid counts, no progress, half-close, and blocked abort.
Events test concurrent next, return, throw, early break, and stored terminal errors.

The direct protocol fixture checks integer exports, host notifications, strict
frames and handshake, allocation starts and sizes, interior/double frees, the
aggregate buffer cap, memory growth, pending/canceled calls, consumed requests,
and shutdown. The shared fixture covers bigint extremes, resource identity,
conversion rollback, runtime exit, and loader mismatch. No ABI extension or
performance claim follows from these results. Unlisted combinations and browsers
are unqualified.

Firefox's Playwright 1.61.1 driver asserts when tracking Worker WebSockets without
corresponding tracked HTTP requests. Its relay Worker fixture therefore runs in
the same pinned native Firefox executable, with a loopback HTTP completion
callback; it executes the same assertions and is required to finish. This is a
test-driver accommodation, not an engine skip. The runtime adapter also clears
Go 1.27's scheduled wakeups on abrupt exit, using the matching compiler runtime's
internal timer map; compiler/runtime upgrades need renewed qualification.

Packaging qualifies npm 12.0.1 dry-run lists, local installation, TypeScript
conditional exports, browser default URLs, and esbuild 0.25.12 with explicit
Wasm/runtime asset handling. The new go-iroh-js package uses the streams/events/errors
syscalljs backend and includes dependency module-root licenses and Go's license.
Its source and generated package remain local candidates.

Additional receipts:

- `~/tmp/wasmbind/20261003-syscalljs-browser-tests/qualification.json`
- `~/tmp/wasmbind/20261003-extension-tests/qualification.json`
- `~/tmp/wasmbind/20261003-abi-browser-tests/qualification.json`
- `~/tmp/wasmbind/20261003-abi-go1.27.1-browser-tests/qualification.json`
- `~/tmp/wasmbind/20261003-package-tests/qualification.json`
- `~/tmp/go-iroh-js/20261003-package-tests/qualification.json`
- `~/tmp/wasmbind/20261004-syscalljs-browser-tests/qualification.json`
- `~/tmp/wasmbind/20261004-abi-browser-tests/qualification.json`
- `~/tmp/wasmbind/20261004-abi-go1.27.1-browser-tests/qualification.json`
- `~/tmp/wasmbind/20261004-goiroh-tests/qualification.json`
- `~/tmp/go-iroh-js/20261004-package-tests/qualification.json`

## Explicit API contracts

`npm run test:policies` qualifies naming, named scalars/enums, non-null results,
and optional Go import defaults. Chromium, Firefox, and WebKit each passed
33 syscalljs assertions in both Window and Worker, and 11 ABI assertions in
each realm. Separate strict TypeScript 6.0.3 DOM and WebWorker fixtures verify
constructor non-nullability, named enum types, field naming, and optional imports.
The browser checks include acronym/tag behavior, aliases, signed bigint extremes,
input/output validation, open enums, omitted defaults, explicit overrides,
partial/null host rejection, and capturing an interface getter once.
Go validation tests additionally cover invalid UTF-8 enum constants, imprecise
number constants, missing validators/factories, collisions, and unsupported ABI
Go validators. Receipt:
`~/tmp/wasmbind/20261003-api-improvements/qualification.json`.

The packaging fixture installs the actual npm tarball, resolves the typed
Node-only helper, overwrites stale assets with byte-identical matching files,
and loads the esbuild bundle with its default URL in Window and Worker.

Scalar-validator scheduling regression: on core revision `02b6d70c`, a yielding
input validator allowed AbortSignal cancellation to arrive before Go invocation
admission, so the canceled export still executed its target. A plain 20ms sleep
alone completed normally; the negative control establishes lost cancellation,
not a general deadlock. Custom input validators now run in the admitted
invocation coroutine before the target. Channel-gated checks verify cancellation
skips the target and later validators, retains a resource lease until validation
returns, and then releases it. Sleeping input/output/import-result validators,
yielding validation errors, and redacted validator panics are qualified in all
three engines and both Window and Worker. Negative-control receipt:
`~/tmp/wasmbind/20261003-api-improvements/validator-negative-control.json`.
