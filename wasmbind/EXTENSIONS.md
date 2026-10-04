# wasmbind extensions

Status: implemented as a local candidate. Companion to [SPEC.md](SPEC.md).
[VALIDATION.md](VALIDATION.md) records the qualified feature combinations and
realms. The direct ABI backend remains experimental and rejects every extension
combination. No publication or hosted CI result is implied.

## Scope and compatibility

This specification covers every item deferred by the base spec: browser stream
adapters, asynchronous event subscriptions, imported JavaScript functions, typed
domain errors, npm packaging metadata, and a direct Wasm ABI backend. It also
defines how TypeScript declarations evolve with those extensions.

Base conversion, instance isolation, cancellation, output ownership, and explicit
disposal contracts continue to apply unless a section explicitly extends them.
The base API remains usable without enabling any extension. No directive is
silently inferred from an unannotated method, interface, or channel.

Add `-features` to check, generate, and build. It accepts a comma-separated list
of `streams`, `events`, `imports`, and `errors`. Packaging uses a separate pack
command; the ABI uses a separate backend flag. Record selected features,
backend, and protocol versions in the manifest and schema fingerprint. Unknown
features or directives requiring a disabled feature are generation errors.
Do not fall back to another backend or drop unsupported exports silently.

## TypeScript is part of the binding contract

Keep generated JavaScript as the runnable artifact and bindings.d.ts as a
mandatory companion. TypeScript generation is included in the base release,
not postponed to an extension. Generating or loading bindings requires no
TypeScript compiler, npm, or transpilation step. Do not generate a second .ts
runtime implementation.

Declarations must describe the exact runtime API: Promise returns, bigint,
Uint8Array, nullable records, resource wrappers, disposal, and optional
AbortSignal. Extension declarations add ReadableStream, WritableStream,
AsyncIterableIterator, import interfaces, and error classes as applicable.
Use readonly properties for generated stream views and error metadata; do not
claim copied records are immutable unless the runtime actually freezes them.

Resource types are opaque interfaces with an unexported unique-symbol brand in
the declaration file. This prevents accidental structural construction; runtime
instance checks remain authoritative. Do not use `any` to bypass unsupported
types. Rejection types cannot be encoded by Promise<T>; provide exported error
classes and type guards, not fictitious Promise<T, E> declarations.

Qualify declarations with a pinned TypeScript compiler and record its version.
Initial supported configuration is ES2022 with DOM and DOM.Iterable libraries;
qualify dedicated Workers in a separate ES2022/WebWorker fixture. Use strict
mode and skipLibCheck=false, with positive usage and @ts-expect-error negative
fixtures. State the minimum tested compiler version in release metadata. Adding
language syntax that raises that minimum is an intentional compatibility change.

Test ordinary JavaScript callers too. Declaration correctness must not conceal
missing runtime validation or make TypeScript a prerequisite for consumers.

## Browser byte streams

### Declaration and generated surface

With streams enabled, resources may mark methods:

```go
//wasmbind:read
func (s *Stream) Read(ctx context.Context, p []byte) (int, error)

//wasmbind:write
func (s *Stream) Write(ctx context.Context, p []byte) (int, error)

//wasmbind:finish
func (s *Stream) Finish(ctx context.Context) error
```

These signatures are illustrative declarations. Each resource has at most one
of each directive. A read or write direction also requires an interruption hook:

```go
//wasmbind:cancelread
func (s *Stream) CancelRead() error

//wasmbind:cancelwrite
func (s *Stream) CancelWrite() error
```

All methods must be exported. Reject directive/export combinations on the same
method. write requires finish; read requires cancelread; write requires
cancelwrite. The methods adapt application operations, rather than forcing
io.Reader/io.Writer implementations to grow contexts. An ordinary reader or
writer can be wrapped, but the wrapper must define interruption and ownership.
The behavior of partial reads and writes follows the
[Go io contracts](https://pkg.go.dev/io).

Generated wrappers expose stable readonly readable: ReadableStream<Uint8Array>
and/or writable: WritableStream<Uint8Array> properties. Construct these views
once per resource. They are not separate owners and do not replace dispose().
Native stream locks and piping follow the
[WHATWG Streams Standard](https://streams.spec.whatwg.org/).

### Reading and backpressure

Use a default readable stream with highWaterMark=0: start no read without
consumer demand. At most one Go read is active per resource. Default chunk size
is 64 KiB; LoadOptions.streamChunkBytes may reduce or increase it up to the
base maxValueBytes limit. Allocate a Go buffer and copy each delivered chunk.
Default readers are supported; BYOB readers and buffer loans are not included.

Require 0 <= n <= len(p). For n>0 with nil error, deliver bytes. For n>0 with
io.EOF, deliver bytes and close. For n>0 with another error, deliver bytes and
retain the terminal error until the next pull so the final chunk is observable.
For n=0 with io.EOF, close immediately. For n=0 with another error, error the
stream. Retry (0,nil) with cooperative scheduling, at most 100 consecutive
times, then fail with io.ErrNoProgress. Invalid n is a bridge protocol error.
No read occurs after a terminal state.

ReadableStream.cancel() cancels the direction's context and invokes CancelRead
once, promptly and independently of the read lane. Discard late bytes. Its
Promise waits for the read to exit and cancellation cleanup to complete. An
application hook that fails to interrupt a stuck read can delay cleanup; the
bridge cannot terminate it. Cancellation of reading does not close writing.
CancelRead errors reject cancellation but the direction stays terminal.

### Writing, completion, and duplex operation

Serialize writes within the write direction. The underlying sink copies the
Uint8Array when its write callback runs and resolves that callback's Promise
only after Go accepts every byte. Standard WritableStream.write() may queue
the caller's original object before that callback runs; document that callers
must keep it unchanged until their write Promise settles. Do not promise the
base exported-call snapshot timing for writes through a platform stream.

Require 0 <= n <= len(p). Retry a positive short write with nil error using
only the unwritten suffix. A zero-byte successful write of nonempty data fails
with io.ErrShortWrite. Any write error rejects the current write and errors the
direction; already accepted bytes are not rolled back. Empty writes succeed
without calling Go. writer.close() invokes Finish once after prior writes;
success marks only the write direction finished. Finish errors error that
direction. writer.abort() cancels and invokes CancelWrite once; queued writes
reject, and cleanup waits for the active operation to exit.

Use a byte-length queue strategy with a default high-water mark of 256 KiB,
configurable by LoadOptions.streamHighWaterBytes. This is a backpressure signal,
not a hard cap: a caller ignoring writer.ready can queue more data in JavaScript.
The bridge caps active copies and rejects any chunk exceeding maxValueBytes.
It cannot claim to bound the browser's entire WritableStream queue.

For annotated stream methods only, replace base receiver-wide serialization
with separate read and write lanes. One read and one write may overlap; finish
shares the write lane. Interruption hooks use neither lane and must be safe to
call during I/O. Reject resources mixing ordinary exported receiver methods
with stream directives; put controls in a separate wrapper resource. This avoids
implicit concurrency changes to existing methods.

Resource disposal invokes unfinished-direction interruption hooks before waiting
for I/O references, then follows the base close protocol. Hooks and close must
be idempotent at the application level; the bridge tracks its own once-only
invocations. Normal EOF or finished writing does not dispose the resource.

### Acceptance

Test final bytes with EOF and with non-EOF errors, short writes, invalid counts,
no progress, pipeTo backpressure, locked streams, abort during blocked I/O,
duplex echo, half-close, close errors, and full disposal. Count active read/write
operations and copied bytes under a slow consumer. Distinguish bridge memory
bounds from unbounded application or platform queues.

## Asynchronous event subscriptions

Use pull-based event resources instead of exporting channels:

```go
//wasmbind:resource Subscription
type Subscription struct { /* application state */ }

//wasmbind:next
func (s *Subscription) Next(ctx context.Context) (Event, bool, error)

//wasmbind:interrupt
func (s *Subscription) Interrupt() error

//wasmbind:close
func (s *Subscription) Close() error
```

Event is a supported copied value or annotated record. An annotated exported
constructor returns *Subscription through the base call contract. next requires
interrupt, and those methods cannot also be ordinary exports. The bool denotes
whether an event exists. (value,true,nil) delivers an event; (_,false,nil) ends
iteration; any error terminates it and rejects next(), ignoring the value.

The generated resource implements AsyncIterableIterator<Event>, dispose(),
next(), return(), and throw(). Symbol.asyncIterator returns the same instance;
there is one consumer, not implicit multicast. Allow at most one pending next:
a concurrent next rejects with code `busy`. No prefetch or bridge event queue
is allowed. Application buffering is outside bridge limits and must be described
by the wrapper. Do not silently drop events or convert channels to an unbounded
JS queue.

return() closes admission, cancels the active Next context, invokes Interrupt
once outside the receiver lane, awaits active work, disposes the subscription,
and resolves {done:true,value:undefined}; cleanup errors reject. throw(reason)
performs the same cleanup and rejects with reason; attach cleanup failures as
cause metadata without changing the original rejection reason when possible.
End-of-iteration and terminal errors automatically run disposal. Later next()
after clean completion resolves done=true; after terminal failure it rejects
with the stored failure. Explicit return takes precedence as a cancellation
request, but never delivers a late event. Define the race by one settlement
transition as in the base call protocol.

Constructors accept AbortSignal when they take context. A subscription returned
by a completed constructor has its own module-derived lifetime; its operation
contexts must not retain the completed constructor context. Consumers cancel it
through return()/dispose() or module disposal. Document this distinction in the
generated declaration comments.

Test for-await break, explicit return, thrown consumer errors, slow consumption,
simultaneous next, cancellation/completion races, terminal errors, module exit,
and repeated subscribe/dispose without retained listeners or goroutines.

## Go-to-JavaScript imports

### Schema and application binding

With imports enabled, annotate an exported interface:

```go
//wasmbind:imports Host
type Host interface {
    Lookup(ctx context.Context, key string) ([]byte, error)
}

//wasmbind:init
func Initialize(ctx context.Context, host Host) error
```

Exactly one init function is permitted; it accepts context.Context and all
annotated import interfaces in declaration order, and returns error. It is not
an exported JS call. Interface methods must be public and take context first,
followed by supported copied values, returning error or (T,error). Resources,
streams, events, variadics, embedded interfaces, and callbacks cannot cross the
import boundary in the first imports release. JS method names match Go
method names by default; package naming policy and explicit method export
overrides follow the base specification.

Generate an implementation of each interface and a TypeScript Imports type.
load(source, {imports:{Host:{Lookup: async (key, options) => ...}}}) supplies the
implementations. JS receives copied arguments plus {signal:AbortSignal}.
Synchronous returns are accepted and normalized through Promise.resolve.
Capture validated callable references at load; no dynamic global-name lookup
or later mutation of the imports object changes the bound implementation.
Missing required interfaces or methods reject before Go starts. A feature flag
with no annotated import interfaces does not require an init or imports option.

An interface may additionally declare `//wasmbind:optional DefaultHost`, where
`DefaultHost` is a public nongeneric `func() Host` in the input package. Omitting
the entire interface supplies that Go factory's result to init. Explicitly
supplied interfaces must still contain every callable method: null, partial
objects, and missing methods are errors. Capture interface presence and methods
once, including empty interfaces. A nil default result fails initialization;
a factory panic follows initialization panic handling. The factory runs only
when omitted, and no missing-method fallback occurs. `Imports` properties are
optional only for annotated optional groups; `LoadOptions.imports` is optional
only when no required groups exist. Initialization runs in
a goroutine; load resolves only after it succeeds. Initialization failure
cancels the module and performs cooperative cleanup.

### Cancellation, errors, and reentrancy

Allocate an invocation ID and AbortController for each outbound call. A canceled
Go context aborts that controller and returns ctx.Err() promptly. Do not wait
for a JS Promise that ignores its signal. Retain settlement handlers until the
JS Promise settles or module teardown invalidates them, but drop Go references
and bounded call-table entries when Go stops waiting. Use shared JS handlers
rather than one unreleasable js.Func pair per orphaned Promise. Late values are
discarded. Add maxPendingImports, default 256; reject admission at the limit.

Copy successful JS results before returning to Go. Convert throws and rejections
to a Go import error naming the interface and method, with a bounded string
message (4 KiB maximum), without transferring JS objects or stacks. Result
conversion failures are import errors. Cancellation is recognizable with
errors.Is(err, context.Canceled) or context.DeadlineExceeded.

Host code must not await a same-instance exported call from within an import.
Such calls reject with `reentrant-call` while any import in that instance awaits
settlement. This conservative rule also rejects unrelated same-instance calls
during that interval; document it. Cross-instance calls are allowed and remain
the application's responsibility. An import that needs same-instance behavior
must return and schedule a later task. Go must never wait for an imported
Promise inside a syscall/js dispatch callback.

Imports extend module disposal: close admission, cancel outbound contexts and
signals, invalidate late handlers, then release Go callbacks after active Go
calls return. Test fetch-based imports, synchronous throws, malformed results,
ignored abort, never-settling Promises, init failure, duplicate settlement,
teardown, and deterministic same-instance reentrancy rejection.

## Typed domain errors

With errors enabled, annotate exported struct error types:

```go
//wasmbind:error NotFound
type NotFoundError struct {
    Key string
}
func (e *NotFoundError) Error() string { /* application message */ }
```

Require *T to implement error. Use base record field validation and conversions
for explicitly exposed data; exclude fields only with wasmbind:"-". Nested
resources and errors are forbidden. Reject colliding class names and error
names matching bridge errors. The type's qualified Go identity becomes a stable
domain identifier recorded in the schema; renaming it changes the contract.

Generate an exported JS Error subclass with readonly domain and data properties,
name equal to its declared JS name, and the original Error() message. Export a
type guard that verifies private branding and domain identity. code remains
`go-error`, preserving generic handling. Brand is shared by the generated module
definition, so instances of the same artifact recognize each other's errors;
other artifacts need an explicit domain check rather than instanceof promises.

Match registered types in wrapped errors using the standard error unwrap tree,
preorder, first match. Define one deterministic declaration order for candidates
at a node. Limit traversal to 64 levels and 256 nodes; detect cycles. Do not
invoke custom As methods or serialize the full error chain. Unregistered types
retain the base generic error behavior. Field conversion failure falls back to
a generic go-error and does not disclose partially converted domain data.
Bridge cancellation remains AbortError even if the canceled operation returns
a typed domain error after cancellation has already won settlement.

TypeScript exposes classes and guards; it cannot statically type thrown values.
Test wrapped and joined errors, cycles, competing matches, malformed fields,
cross-instance class recognition, unregistered errors, and cancellation races.
Application authors choose the public error fields; the generator must not
expose unannotated internal error state.

## npm metadata and local packaging

Add:

```
wasmbind pack -bundle dir -out dir -name @scope/name -version x.y.z
```

pack validates a completed build manifest and artifact hashes, then writes a
local package directory. It never publishes, installs dependencies, runs npm,
or changes credentials. Use the
[npm package.json contract](https://docs.npmjs.com/cli/v11/configuring-npm/package-json/)
for name, version, files, type, and exports.

Generate type=module, root exports for import and types, and explicit asset
exports for module.wasm and bindings.json. Include runtime assets, README with
load examples and limitations, generated API.md, BUNDLING.md, manifest, declared
license, and provenance.
Require -license-file and copy its contents; do not invent a license or assume
the wrapper license covers bundled dependencies. Refuse scripts, private
credentials, incidental build files, absolute source paths, or unlisted assets.
Package allowlists and hash validation prevent inclusion of unrelated files.

The module entry point has no side effects that start Go or fetch Wasm. Users
call load() explicitly. Provide a default URL resolved with import.meta.url and
an explicit source override for bundlers. Do not promise every bundler copies
Wasm assets automatically. TypeScript conditional exports resolve to the same
API as direct .d.ts imports. CommonJS output is not included initially.

The package exports a typed Node-only `copyAssets(destination)` build helper.
It copies the matching module.wasm, runtime.js, and wasm_exec.js beside an ES
module browser bundle. It creates the destination and replaces those fixed
assets, so the destination must belong to the consumer build. Keep packages
with distinct Wasm assets in distinct output directories. BUNDLING.md gives an
esbuild ES2022 recipe using that helper; test the resulting default load in
Window and Worker against an installed npm tarball.

pack produces a directory, not an npm-compatible tarball. Users can run npm
pack on that directory themselves. Output obeys base ownership and manifest-last
replacement rules. Explicit -name, -version, and -license-file arguments are
required; do not derive publish identity from a Git tag or directory name.

Qualify npm pack --dry-run file lists, local installation, TypeScript module
resolution, browser asset loading, and at least one pinned bundler fixture.
Node execution remains separately qualified; shipping npm metadata is not a
claim that browser networking works in Node. Publishing remains a user action.

## Direct Wasm ABI backend

Add `-backend syscalljs|abi`, default syscalljs. The ABI backend is experimental
until it passes the same consumer tests. It still targets standard Go js/wasm
and includes the matching Go runtime; direct exports do not remove Go's runtime
or enable browser UDP. TinyGo and WASI require separate backend specifications.

Use only documented
[go:wasmexport/go:wasmimport rules](https://go.dev/cmd/compile).
First prove exported calls, runtime initialization, asynchronous completion,
host imports, and memory access on every supported compiler version. If the
compiler/runtime combination cannot support an operation, reject abi builds
requiring it. Do not infer support from successful compilation alone.

### Wire format and memory ownership

Define ABI protocol version 1 with these generated exports, all integer-only:

| Export | Purpose |
| --- | --- |
| wb_version() -> u32 | Protocol version |
| wb_alloc(size:u32) -> u32 | Allocate a bounded input buffer; zero means failure |
| wb_free(ptr:u32) | Free exactly one live bridge allocation |
| wb_submit(ptr:u32,len:u32) -> u32 | Validate and admit a request; return request ID or zero |
| wb_cancel(id:u32) | Request cancellation |
| wb_poll(id:u32) -> u32 | Return terminal response allocation address or zero if pending |
| wb_status() -> u32 | Read status from the immediately preceding ABI operation |
| wb_shutdown() | Begin cooperative module disposal |

Actual Go signatures use types accepted by the selected toolchain, with unsigned
range interpretation specified by the protocol. Zero request ID is reserved;
IDs do not repeat during an instance's lifetime; exhaustion returns limit.
Each operation except wb_version and wb_status sets an instance-local status:
0 success, 1 pending, 2 argument, 3 limit, 4 closed, 5 unknown-request,
6 consumed, 7 invalid-allocation, 8 abi-mismatch. wb_status does not clear it.
The loader reads status immediately before allowing another operation, without
awaiting or invoking application callbacks between those calls. Unknown status
codes fail the instance as abi-mismatch. There is no application execution in
submit. Poll transfers a terminal response allocation to the JS caller once;
a later poll returns zero with status consumed. The loader validates the schema
fingerprint through a reserved operation 0 handshake before admitting user calls.

Encode frames explicitly, little-endian: four ASCII magic bytes WBND, u32
protocol version, u32 total frame length, u32 operation ID, then fields. Each
field has u32 field ID, u32 type tag, u32 payload length, then payload bytes;
there is no alignment padding. Assign type tags in the protocol manifest, freeze
them for protocol version 1, and reject unknown tags, duplicate fields, trailing
bytes, and missing required fields. Length prefixes are u32; integers use fixed
width; strings are UTF-8 with strict validation; records use schema-assigned
field IDs; resource references carry instance-local handle IDs. Response frames
contain success, error, or cancellation status. Use a separate allocation header
with the payload length so JS can bound-check before decoding. Never serialize
Go structs, slice headers, interfaces, or raw resource pointers.

Keep each Go byte allocation rooted in a registry until free. wb_submit copies
and validates its frame before returning; JS frees the input after submit.
wb_poll returns a copied response owned by JS until free. Module teardown
reclaims remaining bridge allocations after consumers and operations are
quiescent. Validate pointers against registered allocation starts and sizes,
not merely linear-memory bounds; reject double free, interior pointers, and
foreign allocations. Cap total live ABI buffer bytes, default 32 MiB, in
addition to base value limits. Allocation failure is a normal bridge error.

JavaScript recreates DataView/Uint8Array views after every Wasm call that may
grow memory; it must not cache memory.buffer. No borrowed Go heap views escape
to consumers, and there is no zero-copy claim. Decode output into owned JS
values before freeing its frame.

### Asynchrony and backend parity

Submit schedules application work and returns. Completion uses a documented
host notification carrying only request ID; JS polls that ID on a later task
to avoid runtime reentrancy. Runtime delivery of that host import must be
qualified before enabling the backend. Notifications are hints; duplicate or
late notifications cannot settle a Promise twice. The public Promise,
AbortSignal, error, resource, and module-disposal contracts remain identical.

Future extension combinations must be explicitly listed as qualified in the
manifest. ABI stream chunks and event values use the same bounded frame protocol;
imports need a separately versioned outbound-request/completion protocol before
abi+imports is enabled. Initially reject unsupported combinations rather than
route part of an ABI bundle through an undocumented syscall/js fallback.

Tests cover allocation ownership, wrong frame versions, invalid lengths,
memory growth, bigint extremes, double consumption, cancellation, runtime exit,
notification reordering, and disposal. Run the same API fixtures against both
backends and compare values, errors, lifecycle events, and TypeScript surface.
Measure call latency, copies, allocation pressure, and bundle size against the
base backend before asserting any improvement.

## Delivery order and release evidence

| Slice | Prerequisite | Required evidence |
| --- | --- | --- |
| TypeScript baseline | Base schema | Strict positive/negative fixtures and JS callers |
| Streams | Base resources and cancellation | Slow-consumer, duplex, and interruption tests |
| Events | Base resources and cancellation | Pull-only iteration and cleanup tests |
| Imports | Base loader and call lifecycle | Promise, cancellation, initialization, reentrancy tests |
| Domain errors | Base conversions and errors | Deterministic unwrap and declaration tests |
| Packaging | Completed build bundles | File allowlist, hashes, local consumer fixture |
| ABI feasibility | Supported Go toolchains | Executable runtime/export/import/memory probes |
| ABI release | Feasibility and base API | Shared fixtures and explicit feature-combination matrix |

Run browser fixtures in Chromium, Firefox, and WebKit, with dedicated Worker
variants where the feature applies. Missing required tools fail qualification;
skips do not count. Record compiler, browser, generator, TypeScript, and bundle
identities. Maintain a release matrix by feature, backend, and runtime realm;
one green slice does not establish support for every combination.

For go-iroh, qualify duplex relay echo through stream views, cancellation of a
blocked receive, gossip consumption with for-await and early break, and typed
wrapper errors. Test imported browser functionality with hermetic HTTP fixtures.
Keep networking claims limited to relay paths actually exercised.

These extensions do not automatically introduce arbitrary maps, generic APIs,
transferable resource wrappers, persistent browser storage, automatic finalizer
cleanup, WebIDL generation, or publishing. Each needs its own contract if added.

## Protocol-one implementation details

The ABI manifest fixes tags null=1, bool=2, float=3, string=4, bytes=5,
array=6, record=7, resource=8, signed integer=9, unsigned integer=10. Integers
and floats use eight-byte payloads, resource handles four bytes, and bool one
byte. Narrow integer ranges remain enforced by their schema types. Arrays and
records contain nested TLV fields numbered from one by element index or field
order in the manifest. Null pointers/resources have tag 1 and no payload.

User call operation IDs are manifest call IDs plus one; operation zero is the
schema/build handshake. Request field 1 is the receiver handle represented as
uint32; arguments begin at field 2. Handshake fields 1 and 2 are fingerprint and
build identity strings. Response fields 1, 2, and 3 are code, message, and result.
Void and failed results use null. Control operations 0xfffffff0 and 0xfffffff1
admit resource disposal and module disposal respectively; the latter is followed
by wb_shutdown. Their fields are a uint32 handle and no fields respectively.

The allocation address points to a four-byte payload-length header; request
payload starts at ptr+4. wb_submit's length excludes that header. Response buffers
are owned copies rooted until free; pending response frames count toward the
32-MiB buffer cap. Consumed IDs use monotonic history without retained response
entries. A one-millisecond Go timer wakes work admitted through direct exports;
notifications trigger polling on a later browser task. This is an experimental
scheduling mechanism, with no latency or throughput guarantee. Shutdown releases
remaining allocations and responses after the disposal response is consumed.

The local pack command additionally accepts -readme-file for an explicit package
README. It validates browser artifact hashes; the external generated main.go hash
is retained as compiler-source provenance and is not a browser package asset.
