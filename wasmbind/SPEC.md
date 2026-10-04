# wasmbind

Status: implemented with Go 1.27. Local browser qualification and its limits are
recorded in VALIDATION.md; a hosted CI run has not yet been observed.

## Purpose

`github.com/tmc/misc/wasmbind` generates JavaScript bindings for Go packages
compiled to browser WebAssembly. Given an explicitly annotated Go API, it
produces Go adapters, an ES module, TypeScript declarations, and a binding
manifest. A build command also produces the Wasm binary and matching Go
runtime support file.

The developer writes ordinary Go functions and methods. JavaScript callers
receive typed functions, Promises, Uint8Array values, and objects with explicit
lifetimes. They do not write syscall/js adapters or manage Go callback handles.

The tool is written in Go, uses the standard Go compiler, and runs without
Node, npm, Rust, or a network connection when dependencies are already cached.
The intended install command is:

```sh
go install github.com/tmc/misc/wasmbind/cmd/wasmbind@latest
```

This is the Go counterpart to wasm-bindgen's binding-generation role, not a
replacement compiler. Version one targets `GOOS=js GOARCH=wasm`. TinyGo,
WASI, arbitrary JavaScript imports, WebIDL, and automatic npm publishing are
outside its scope.

## Platform basis

Go's [WebAssembly guide](https://go.dev/wiki/WebAssembly) describes the browser
target and its runtime support. [syscall/js](https://pkg.go.dev/syscall/js)
provides host values, callbacks, and byte-copy operations. Its callback
documentation requires blocking work to start a separate goroutine and requires
callbacks to be released when no longer callable. The generated bridge must
observe these rules.

[go:wasmexport](https://go.dev/doc/go1.24) exposes functions through the Wasm
ABI, but does not itself define JavaScript object, Promise, or ownership
semantics. Version one uses syscall/js; a direct ABI backend is a separate
future design. The generated consumer API must not depend on that backend.

## Example

A consumer creates a narrow wrapper package around its existing library:

```go
package webapi

import "context"

// Digest computes the digest of data.
//wasmbind:export digest
func Digest(data []byte) (string, error) { /* implementation */ }

// Client owns a connection to the service.
//wasmbind:resource Client
type Client struct { /* implementation */ }

// Dial connects to the service.
//wasmbind:export dial
func Dial(ctx context.Context, address string) (*Client, error) { /* implementation */ }

// Send sends data and returns a response.
//wasmbind:export send
func (c *Client) Send(ctx context.Context, data []byte) ([]byte, error) { /* implementation */ }

// Close releases the connection.
//wasmbind:close
func (c *Client) Close() error { /* implementation */ }
```

These snippets illustrate signatures, not compilable implementations.

```sh
wasmbind generate -go-out ./cmd/webbind -out ./web/generated ./webapi
wasmbind build -go-out ./cmd/webbind -out ./web/generated ./webapi
```

```js
import { load } from './generated/bindings.js';

const mod = await load(new URL('./generated/module.wasm', import.meta.url));
const client = await mod.dial('service-address');
try {
  const response = await client.send(new Uint8Array([1, 2, 3]));
  console.log(response);
} finally {
  await client.dispose();
  await mod.dispose();
}
```

The generated declarations include:

```ts
export interface CallOptions { signal?: AbortSignal }
export interface Client {
  send(data: Uint8Array, options?: CallOptions): Promise<Uint8Array>;
  dispose(): Promise<void>;
}
export interface Module {
  digest(data: Uint8Array): Promise<string>;
  dial(address: string, options?: CallOptions): Promise<Client>;
  dispose(): Promise<void>;
}
export function load(
  source: URL | Response | BufferSource | WebAssembly.Module,
  options?: LoadOptions,
): Promise<Module>;
```

## Input contract

Accept exactly one non-main package per invocation. Load and type-check it for
js/wasm using go/packages. Respect the selected toolchain, module replacements,
workspace, build tags, and package visibility rules. Do not select all exported
declarations implicitly.

Recognize directives in the declaration's doc comment:

| Directive | Meaning |
| --- | --- |
| `//wasmbind:export name` | Export a function or resource method under the exact JS name |
| `//wasmbind:record` | Export a named struct as copied data |
| `//wasmbind:resource Name` | Export pointer instances through an opaque resource wrapper |
| `//wasmbind:close` | Designate the resource's disposal method |
| `//wasmbind:nonnull` | Require an exported pointer result to be non-null on success |
| `//wasmbind:scalar Name` | Preserve a public defined basic type as a named TypeScript alias |
| `//wasmbind:enum Name` | Export typed constants as a frozen object and preserve their named value type |
| `//wasmbind:validate` | Restrict an enum to its exported typed constant values |
| `//wasmbind:validate Function` | Validate a scalar using a public Go `func(T) error` |

Names must be valid JavaScript identifiers. Reject duplicate names and reserved
API names (`load`, `dispose`, `then`, `constructor`, and `__proto__`) in their
respective scopes. Defaults preserve names and never overload them. Go doc
comments become generated API documentation; directives are omitted.

Exports must be public Go declarations. Resource methods require a pointer
receiver on an annotated resource type. Record types have public data fields;
private fields, embedded fields, recursive record graphs, and contradictory
directives are rejected. Record fields use their Go names unless a
`wasmbind:"name"` tag supplies a valid identifier. The `wasmbind:"-"` tag
explicitly excludes a field. JSON tags have no binding meaning.

A package doc comment may opt into `//wasmbind:naming camelCase`. The policy
changes record fields, unnamed exports (`//wasmbind:export`), imported method
names, and enum members. Acronyms retain word boundaries: `HTTPServer` becomes
`httpServer` and `URL` becomes `url`. An identifier consisting entirely of
an acronym and a trailing plural `s` is lowered as a whole: `ALPNs` becomes
`alpns`, `URLs` becomes `urls`, and `IDs` becomes `ids`. Type names retain their explicit spelling.
Named export directives and field tags are exact overrides. Import methods may
also use `//wasmbind:export name` in their method doc comment. Apply collision
and reserved-name checks after naming; never append suffixes to API names.

`//wasmbind:nonnull` supplements export on functions and resource methods with a
top-level supported pointer or resource result. An error remains an error;
`(nil,nil)` or a nil-only result rejects with bridge code `contract`. Defaults
remain nullable. The contract changes only the top-level result, not arguments
or nested pointers, and does not manufacture resource handles.

Annotated scalar and enum types must have a supported basic underlying type.
Enums support string, bool, and integer values. Collect only exported constants
with exactly that defined Go type, retain aliases, and reject enums without any
such constants. The frozen JS object contains each constant under its selected
name. Unvalidated enums remain open primitive aliases in TypeScript; only
validated enums use a finite union. Validate inputs and outputs on the Go side;
unknown enum values reject with `argument`. Deduplicate Go switch cases for
constant aliases. Reject invalid UTF-8 string constants and `int`/`uint`
constants not exactly representable as JS numbers. `int64`/`uint64` enum objects
and declarations use bigint literals, including the signed minimum.

A scalar validator must be a public nongeneric `func(T) error` in the input
package, where T is exactly the annotated scalar. It checks both decoded inputs
and returned values. Decode and copy inputs during dispatch, but defer custom
Go input validators until the invocation is admitted, after resource leases and
receiver-lane waits. Run them in the invocation goroutine before the target,
checking cancellation before and after validation; a canceled invocation must
not run later validators or the target. Validators may yield. A validator that
ignores cancellation can delay release of the invocation's leases, like an
application call. Validate imported results directly in their existing Go
coroutine, and returned values in the export's coroutine.
Validation errors retain Go error semantics; panics use
the bridge's panic handling. The ABI backend rejects Go scalar validators:
application code cannot execute during ABI submission. Pure enum validation,
named scalar aliases without validators, naming, and non-null contracts work
with both backends.

All exported calls are asynchronous. An optional first context.Context parameter
is supplied by the bridge and is absent from the JS argument list. Only these
return forms are accepted: no results, T, error, or (T, error). Reject variadic
functions, unspecialized generic declarations, callbacks, channels, arbitrary interfaces,
unsafe pointers, and unsupported signatures with source-position diagnostics.
Type aliases follow their underlying supported type.

Go 1.27 generic functions and methods are supported through explicit concrete
specializations: `//wasmbind:export name[Type,...]`. Repeat the directive with
different JS names to export multiple specializations. Type arguments are
resolved in the declaration's Go scope and checked against its constraints.
Every instantiated parameter and result must satisfy the value contract below.
Generic record and resource declarations remain unsupported. TypeScript exposes
concrete specialized methods; it does not infer Go type arguments at runtime.

## Value contract

| Go type | JavaScript type and conversion |
| --- | --- |
| bool | boolean, exact type required |
| string | string; inputs containing unpaired UTF-16 surrogates are rejected |
| int, uint, int8 through int32, uint8 through uint32 | number; require integer and target-specific range |
| int64, uint64 | bigint; require range; convert without passing through number |
| float32, float64 | number; require finite values; reject float32 overflow |
| []byte | Uint8Array; copy on entry and exit |
| []T, except []byte | Array of supported copied values |
| [N]T | Array with exact length; byte arrays also use Array in version one |
| Annotated record | Plain object with required, explicitly named fields |
| *Annotated record | Record or null |
| *Annotated resource | Instance-bound opaque wrapper or null |

Maps and other pointers are unsupported in version one. Named public scalar types
follow their scalar representation. Go wasm uses 64-bit int and uint. Returned
int and uint values that cannot be represented exactly as a JS number are
rejected; lossless integer APIs should use int64 or uint64. A nil slice maps to an empty array or empty
Uint8Array; only supported pointer types use null. Undefined is not a substitute
for null. Record decoding rejects missing required and unknown enumerable own fields.

Converters enforce types on both sides; TypeScript declarations are not runtime
validation. Reject malformed records, cycles, detached buffers, and invalid
resource wrappers before invoking application code. Records are copied, not
identity-preserving. Getters and proxies may throw during inspection; report a
conversion error. Construct output records without inherited setters.

No returned typed array aliases Go memory. Mutating an argument after a call
returns its Promise cannot affect the Go call. Copy and validate inputs in the
dispatch callback before scheduling work. Use CopyBytesToGo and CopyBytesToJS
for byte buffers after checking their accepted type.

## Calls, errors, and cancellation

The JS wrapper always returns a Promise, including for argument errors and
closed resources. The Go dispatch callback must return promptly after input
conversion and admission. It must not wait for I/O or run application code.
Application calls execute in goroutines and settle their Promises through a
shared completion bridge. Async does not mean CPU parallelism or protection
against a long CPU-bound Go call blocking the browser thread.

Bridge-generated failures use WasmBindError with a stable code: `argument`,
`closed`, `foreign-resource`, `limit`, `abi-mismatch`, `runtime-exit`, or `panic`.
Go errors reject with code `go-error` and the exact Error() message. Do not
serialize arbitrary error internals or invent typed domain errors. Cancellation
rejects with an Error whose name is `AbortError`. Recover application panics at
the invocation boundary and return a generic message; do not expose stacks or
panic values. Fatal runtime errors and panics in unrelated goroutines are
runtime failures, not recoverable invocation errors.

For context-taking exports, the last JS parameter is an optional CallOptions.
Create a context derived from the module lifetime. A pre-aborted signal prevents
invocation. A later abort cancels the context and settles the Promise once;
completion and cancellation use a single state transition. Remove abort
listeners after settlement. Cancellation is cooperative and cannot terminate a
goroutine. Retain invocation-owned resources until the Go call actually returns,
even when its Promise has already rejected. Exports without context do not
accept CallOptions.

Module closure cancels every call. Late completions after cancellation are
discarded; a resource returned by such a completion must still be disposed.
Define and test one owner for every converted resource before transferring it
to a settled Promise. Failed output conversion rolls back newly created handles.

## Resource ownership

Each module instance owns its registry. JavaScript never sees Go pointers or
chooses registry IDs. Use monotonically allocated handles without reuse during
an instance's lifetime and a private JS brand associated with the module.
Reject forged, closed, and foreign-instance wrappers. Do not expose a public
numeric-handle constructor.

One live Go pointer has one registry entry and one JS wrapper in its instance.
Returning it again returns that wrapper. A disposed pointer cannot be re-exported
within that instance; report a closed-resource error. Registry entries retain
the Go object until disposal completes. Garbage collection is not the ownership
protocol; no correctness requirement may depend on FinalizationRegistry.

A resource must have exactly one annotated close method, taking no arguments
and returning either nothing or error. It is exposed as dispose(), not as an
ordinary exported method. Explicit disposal is idempotent and returns the same
settlement to repeated callers, including a disposal error. Once disposal
starts, reject new calls, cancel active contexts involving that resource, wait
for admitted calls holding it to return, invoke close once, then drop the entry.
Serialize receiver-method calls in admission order; calls on different resources
may overlap. Passing a resource as an argument holds a lifetime reference but
does not implicitly serialize access; document that application concurrency
rules still apply. Waiting receiver calls check cancellation before invocation.

A resource may opt into overlapping ordinary exported methods with
`//wasmbind:concurrent` alongside its resource directive. Its implementation
must support concurrent calls. Admission still holds lifetime references and
disposal still cancels and waits for calls. Stream read/write and event lanes
retain their existing ordering. The directive takes no arguments.

Module.dispose() closes admission, cancels calls, waits for retained calls,
disposes remaining resources in reverse creation order, releases js.Func values,
and allows the generated Go main to return. It returns a Promise and aggregates
close failures. A call that ignores cancellation can delay disposal indefinitely;
the tool must not release callbacks or pointers still in use to enforce a timeout.
Applications needing hard termination must run the module in a dedicated Worker.

## Runtime loading and isolation

Build copies wasm_exec.js from the selected compiler's GOROOT. Record the exact
Go version and hash in the manifest; do not fetch an unrelated runtime release.
Generate an ES-module adapter that loads this local support file and captures
the Go constructor. Multiple instances using the same runtime version are
supported. Loading a conflicting runtime version in one JS realm fails clearly;
the tool must not overwrite another active runtime silently.

load() creates a fresh Go runtime and a unique registration token. Pass the token
through go.env to generated main. Main registers against that token in a private
bootstrap table and announces readiness with a schema fingerprint. Remove the
bootstrap entry after readiness or failure. The steady-state API is the returned
Module object, not a global application name. The generated Go main waits on an
explicit lifetime channel rather than an uninterruptible select{}.

LoadOptions specifies startupTimeoutMs, maxPendingCalls, maxResources,
maxValueBytes, and maxValueDepth. Defaults are 30 seconds, 256 calls, 1024
resources, 16 MiB of copied payload per call (inputs and outputs each), and depth
64. Byte accounting includes UTF-8 strings and recursively contained byte data.
Also cap aggregate collection elements at 1,000,000 per call. Reject before
allocation where possible. These are bridge limits, not a sandbox for Go code.

Use instantiateStreaming only for a Response with a suitable content type;
otherwise use arrayBuffer and instantiate. Never retry a consumed Response.
Reject startup on timeout, registration mismatch, or early go.run exit. A
timeout cannot force-stop an already executing runtime; clean bootstrap state,
request cooperative shutdown, and document the dedicated-Worker escape hatch.
After readiness, observe go.run: an unexpected exit rejects pending calls,
marks all wrappers closed, and prevents future calls.

Version one qualifies browser Window and dedicated Worker realms. Node, Deno,
and Bun require separate tests before being advertised. No eval, external CDN,
inline script, or network-loaded runtime is required. Binding generation does
not remove browser restrictions on sockets, files, or HTTP cross-origin access.

## CLI and generated files

```
wasmbind generate [-tags list] -go-out dir -out dir package
wasmbind build [-tags list] -go-out dir -out dir package
wasmbind check [-tags list] package
```

check validates the API and emits diagnostics without writing files. generate
writes a standalone main package to go-out, and bindings.js, bindings.d.ts,
bindings.json, and API.md to out. The API reference and readable declarations
share the validated schema, including field/import-method documentation and
identifier-aware generated-name substitutions. Consolidate load options in one
interface and preserve source parameter names with safe local disambiguation. build performs generation, runs go build with js/wasm,
and adds module.wasm, wasm_exec.js, and its ES-module runtime adapter to out.
The generated main must reside inside the consumer's module and outside the
input package; reject paths that violate internal-package visibility or create
an import cycle. The caller's module remains responsible for dependencies.

Do not edit source declarations, go.mod, or go.sum. Do not run go mod tidy or
install tools implicitly. Build with module updates disabled; report missing
dependencies as actionable errors. Generated source contains a standard
generated-file header and js/wasm build constraints. File ordering, IDs, source
maps to declaration positions, and schema fingerprints are deterministic.
Do not include timestamps or absolute paths in schema output.

The manifest contains schema version, generator version, package import path,
selected declarations, conversion rules, limits, and a schema fingerprint.
Build adds compiler identity, effective tags, and artifact hashes. Separate the
schema fingerprint from artifact hashes to avoid circular hashing. The JS and
Go adapters embed the same fingerprint and check it before exposing exports.
Build also embeds a deterministic identity of compiler version, tags, generated
bridge source, consumer companion files, and selected application/dependency
sources. The loader checks this identity so a body-only application change
cannot silently mix with an older JS bundle, even when the schema is identical.

Only replace tool-owned outputs whose prior manifest and generated markers
match. Refuse to overwrite user files. Validate and stage a complete output set
before replacement; publish the manifest last as the completion marker. A
failed command must not leave a plausible successful bundle. Detect mismatched
JS/Wasm files at load time even after an interrupted replacement. Provide no
promise of cross-directory atomic filesystem replacement.

Diagnostics use file:line:column, name the declaration and unsupported type,
and suggest a narrow wrapper when appropriate. Exit codes: 0 success,
1 validation/generation/build failure, 2 CLI usage. Diagnostics go to stderr;
successful commands print the manifest path to stdout. Honor signals and forward
cancellation to compiler subprocesses. Persistent scratch files belong in a
task-specific directory under ~/tmp/wasmbind, never directly in ~/tmp.

## Implementation structure

Use an independent module under misc/wasmbind, with cmd/wasmbind for the CLI,
internal packages for loading, schema validation, and generation, and a small
js/wasm bridge package if generated consumers require shared helpers. Keep that
runtime surface versioned alongside the generator. Stdlib first; x/tools is
acceptable for go/packages. Emit typed converters rather than runtime reflection.
Consumers must not need x/tools at runtime.

Tests of exported Go helpers include runnable examples. CLI integration tests
use rsc.io/script txtar fixtures. Documentation follows Go doc-comment rules.

## Delivery and acceptance

1. Define the schema and validation pass. Golden fixtures cover valid APIs,
   aliases, name collisions, build tags, unsupported types, and stable output.
2. Generate scalar, record, array, and byte conversions and async adapters.
   Test exact integer bounds, bigint round trips, null, malformed strings,
   nested limits, and buffer-copy isolation in a real browser.
3. Add loader, cancellation, and instance lifecycle. A Go function awaiting
   browser HTTP must complete without callback deadlock. Test pre-abort,
   abort/completion races, early exit, startup timeout, wrong manifests, and
   simultaneous instances with equal export names.
4. Add resources. Test cross-instance and forged objects, duplicate returns,
   close errors, double disposal, in-flight calls, late resource returns,
   canceled queued methods, and module teardown. Use test-only registry counters
   to verify cleanup after repeated create/call/dispose cycles.
5. Qualify generated declarations with tsc --noEmit and actual JS calls. Run
   browser integration fixtures in Chromium, Firefox, and WebKit, including a
   Worker fixture. Node tooling is a test dependency, not a generator dependency.
6. Add a go-iroh wrapper example that generates endpoint/connection/stream APIs
   with AbortSignal cancellation and explicit disposal. Prove browser-to-native
   relay stream echo and a blob fetch against a hermetic Go relay. Keep
   transport support claims bounded to that tested path.

CI must fail when a required browser or TypeScript compiler is unavailable;
release qualification cannot count skipped integration tests as passes. Report
toolchain/browser versions and generated artifact hashes. Native unit tests
alone do not qualify a generated browser API.

## Deferred extensions

The contracts and qualification gates for these extensions are specified in
[EXTENSIONS.md](EXTENSIONS.md). They are opt-in work beyond the base release.

ReadableStream/WritableStream adapters with backpressure, AsyncIterable event
subscriptions, Go-to-JS imported functions, typed domain errors, npm metadata,
and a direct Wasm ABI backend need separate contracts. In particular, do not
infer streaming APIs from io.Reader or channels in version one: cancellation,
partial reads, EOF, ownership, and backpressure require explicit semantics.

There is no initial throughput, bundle-size, zero-copy, or Rust-equivalence
claim. Measure call overhead, byte-copy cost, startup time, and bundle size
against hand-written syscall/js adapters before making performance claims.

### Optional input fields and constant defaults

A copied record field may use `wasmbind:"name,optional"`. The name may be
empty to use the package naming policy. Optional applies to input records only;
outputs contain every declared field. The generated `RecordInput` type describes
input records, including nested inputs. `Record` remains the complete output type.

An omitted optional field uses the Go zero value. `wasmbinddefault:"7"` supplies
an explicit Go constant expression for an optional basic field. The generator
checks type, range, UTF-8, and JavaScript number representability. String literals
must be quoted inside the tag, for example `wasmbinddefault:"\"auto\""`.
Defaults never execute application code. Supplying zero, false, an empty string,
or null (for a nullable field) overrides a default. Explicit undefined is an
invalid supplied value. Missing a nested record uses its entire Go zero value;
supplying `{}` applies that nested record's optional field defaults.

### Validated scalar brands

`//wasmbind:brand` may accompany `//wasmbind:scalar ID` and
`//wasmbind:validate ValidateID`. It emits a distinct TypeScript scalar brand and
an asynchronous `module.parseID(value, options?)` constructor. The constructor
accepts the underlying primitive, invokes the Go validator in an admitted
coroutine, and returns the validated value. It preserves the spelling and value;
it does not normalize them. Constructor names participate in export collision
checks. Brands do not alter the runtime wire representation and are not a
security boundary: every received scalar still undergoes runtime validation.
Go validators, including brand constructors, currently require syscalljs.

### Validation paths

Copied argument failures include paths such as `options.items[2].count`.
JavaScript inspection failures expose the safe message `value inspection failed`,
with the field path when known. Missing and unknown fields identify the offending
field. Integer ranges and fixed array lengths identify their required constraint.
Deferred Go scalar validation captures the argument path before decoding proceeds.
Existing bridge error codes and redacted application panic behavior remain intact.

### Consumer qualification

`qualify` requires an explicit local package directory, new output directory,
TypeScript, npm, Node and Playwright executables/module, a consumer TypeScript
fixture, and TypeScript documentation fixtures. It never installs tools. It
packs and installs the local package offline with scripts disabled, checks all
fixtures under strict DOM and WebWorker libraries, and observes package load and
idempotent disposal in all three Playwright browsers in Window and Worker.
Successful receipts identify fixture and package hashes, tool versions and
observed checks. Optional application hooks are labeled caller-provided, not
independent browser observations. Existing output directories are preserved.

Cancellation parameters are named `callOptions`. Go parameter names are kept
unless they collide with a JavaScript reserved word or the generated parameter.
Union conversions validate only the discriminant and active payload. Inactive
fields retain full wire layout with Go zeros without domain validation; runtime
projection removes them from consumer output.
