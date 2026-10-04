# wasmbind

Generate browser WebAssembly bindings for explicitly annotated Go packages.
Requires Go 1.27 or later. The generator uses Go and `go/packages`; Node is
needed only for integration tests.

```sh
go install github.com/tmc/misc/wasmbind/cmd/wasmbind@latest
wasmbind check ./webapi
wasmbind generate -go-out ./cmd/webbind -out ./web/generated ./webapi
wasmbind build -go-out ./cmd/webbind -out ./web/generated ./webapi
```

`generate` emits a standalone js/wasm main, an ES module, TypeScript declarations,
an API reference, and a manifest. `build` also emits Wasm and the selected compiler's local
runtime. The generated main must be inside the consumer module, outside the
input package. Dependencies must already be recorded and cached; the tool
uses `-mod=readonly` and does not update the consumer's module files.

```go
//wasmbind:export echo
func Echo(data []byte) []byte { return append([]byte(nil), data...) }
```

```js
import { load } from './generated/bindings.js';
const mod = await load(new URL('./generated/module.wasm', import.meta.url));
try {
  console.log(await mod.echo(new Uint8Array([1, 2, 3])));
} finally {
  await mod.dispose();
}
```

Exports always return Promises. A first `context.Context` parameter becomes an
optional final `{signal: AbortSignal}` argument. Byte buffers are copied before
the call returns its Promise. Resources use opaque instance-bound wrappers and
explicit `dispose()`. Method calls on one receiver run in admission order;
resource arguments are held for their lifetime but application concurrency
rules still apply. A resource can opt into overlapping ordinary method calls
with `//wasmbind:concurrent`. Use this only for implementations that support
concurrent calls; stream read/write and event iterator lanes keep their ordering.

Use `//wasmbind:record` for copied structs and `//wasmbind:resource Name` for
opaque pointer types. Each resource needs one `//wasmbind:close` method with no
arguments and either no result or `error`. See [SPEC.md](SPEC.md) for conversions,
limits, ownership, cancellation, and error codes.

Generator 0.3.0 adds explicit API naming and contracts. Opt in through source
annotations:

```go
// Package webapi exposes a browser API.
//wasmbind:naming camelCase
package webapi

//wasmbind:enum EventKind
//wasmbind:validate
type EventKind uint8
const Message EventKind = 1

//wasmbind:record
type EndpointInfo struct { URL string; Kind EventKind }

//wasmbind:export
//wasmbind:nonnull
func NewEndpointInfo() *EndpointInfo {
  return &EndpointInfo{URL: "https://example.com", Kind: Message}
}
```

This emits `mod.newEndpointInfo(): Promise<EndpointInfo>` and a frozen
`EventKind.message` constant. Nil success rejects with code `contract`.
`//wasmbind:scalar Name` preserves a domain alias; add
`//wasmbind:validate ValidateName` for a Go `func(T) error` validator.
Unvalidated enums accept future primitive values. CamelCase naming handles
acronyms, respects explicit exports/tags, and diagnoses collisions. Defaults
remain unchanged. Generated `API.md` and formatted `.d.ts` files retain member
documentation. See [the input contract](SPEC.md#input-contract) for details.

Go 1.27 generic functions and methods require explicit concrete specializations:

```go
//wasmbind:export echoInt[int]
//wasmbind:export echoString[string]
func (c *Client) Echo[T ~int | ~string](v T) T { return v }
```

This generates `client.echoInt(number)` and `client.echoString(string)`.
Constraint violations and unsupported instantiated types are validation errors.
Generic resource and record types are not accepted. There is no runtime type
argument inference or automatic expansion of a constraint's type set.

Go's wasm target uses 64-bit `int` and `uint`. Their JS representation is
`number`; returned integers that cannot be represented exactly are rejected.
Use `int64` and `uint64` for lossless `bigint` APIs.

The loader supports browser Windows and dedicated Workers. Cancellation is
cooperative. Disposal waits for admitted calls to return; a call that ignores
cancellation can delay it indefinitely. A startup timeout cleans registration
state but cannot interrupt executing Go code. Use a dedicated Worker and
terminate it when hard termination is required. Node, Deno, and Bun are not
qualified targets.

Outputs carry generated markers and artifact hashes. Modified or unrecognized
files are never overwritten. Publication stages all files and writes the
manifest last. A schema fingerprint and deterministic build identity detect
mismatched JS/Wasm, including different application code with the same API.
File replacement across two directories is not atomic. Manifests contain no
timestamps or absolute source paths.

Run the example from this checkout:

```sh
go run ./cmd/wasmbind build -go-out ./examples/echo/cmd/webbind -out ./examples/echo/web ./examples/echo
```

The [go-iroh example](examples/goiroh/README.md) provides relay-only endpoint,
connection, duplex stream, gossip subscription, and verified blob-fetch APIs.

For qualification:

```sh
go test -race ./...
go vet ./...
npm ci
npx --no-install playwright install --with-deps chromium firefox webkit
npm run test:browser
npm run test:goiroh
```

Integration commands require every browser and TypeScript compiler; missing
tools fail the run. Tests exercise Windows and Workers, typed conversions,
HTTP callback yielding, cancellation races, resource cleanup, runtime failure,
loader mismatches, and disposal. The go-iroh suite uses a hermetic loopback relay
and native Go peer. Receipts record compiler/browser versions and generated
hashes under `~/tmp/wasmbind/<date>-browser-tests/` and
`~/tmp/wasmbind/<date>-goiroh-tests/`; temporary builds are removed.

The repository-root `.github/workflows/wasmbind.yml` runs these gates on changes
to this package. Local passing tests do not establish a hosted CI run,
release readiness, or transport support beyond the tested relay path.

Enable the opt-in [extensions](EXTENSIONS.md) explicitly:

```sh
wasmbind build -features streams,events,imports,errors -go-out ./cmd/webbind -out ./web/generated ./webapi
wasmbind build -backend abi -go-out ./cmd/webbind -out ./web/generated ./webapi
wasmbind pack -bundle ./web/generated -out ./npm-package -name @scope/name -version 0.1.0 -license-file ./LICENSE
```

Stream resources expose readonly platform stream views with separate read/write
lanes and interruption hooks. Event resources implement AsyncIterableIterator;
early loop exit disposes them. Imported functions are supplied and captured at
load, with bounded pending calls and cooperative cancellation. Annotated domain
errors produce privately branded Error subclasses and guards, including terminal
read errors after their final chunk. Resource declarations have opaque brands.

The direct ABI backend is experimental and supports the base API only; it rejects
all feature combinations. It uses integer exports and bounded WBND protocol-one
frames, with schema/build handshakes, rooted allocation ownership, and deferred
notification/poll completion. The matching Go runtime remains required. No
performance or zero-copy improvement is claimed. Packaging writes a local ES
module directory with explicit licensing; optional `-readme-file` supplies its
README. It never publishes. The package includes its generated `API.md` and a
Node-only asset helper. For an ES module browser bundle, copy the matching assets
beside the output so the default `load()` works:

```js
// build.mjs
import {build} from 'esbuild';
import {copyAssets} from '@scope/name/copy-assets';
await build({entryPoints: ['app.js'], outfile: 'public/app.js',
  bundle: true, format: 'esm', platform: 'browser', target: 'es2022'});
await copyAssets('public');
```

`copyAssets` creates the directory and replaces `module.wasm`, `runtime.js`, and
`wasm_exec.js`; choose a directory owned by the build. Each package needs its own
output directory when using its default Wasm URL. Packages with required imports
call `load(undefined, {imports})`. The packaged `BUNDLING.md` contains the same
recipe and loading constraints.

Additional gates are `npm run test:policies`, `npm run test:extensions`, `npm run test:abi`, and
`npm run test:pack`. The minimum tested TypeScript version is 6.0.3 and is recorded
in manifests. The relay example now uses stream views, gossip iteration, typed
errors, and imported HTTP. [Local qualification](VALIDATION.md) records the tested
combinations and remaining release limits.

Copied record inputs can omit fields tagged `wasmbind:",optional"`. Add
`wasmbinddefault:"7"` to provide a checked constant default. Generated input types
are distinct from complete output records, and validation errors identify nested
argument paths. A validated scalar can opt into `//wasmbind:brand` to receive a
TypeScript brand and an asynchronous `module.parseType(value)` constructor.
See [the value contracts](SPEC.md#optional-input-fields-and-constant-defaults).

Generate a navigable reference and check TypeScript consumers with a local compiler:

```sh
wasmbind docs -bundle ./web/generated -out ./docs/site
wasmbind check-examples -tsc ./node_modules/.bin/tsc examples/basic.ts
wasmbind adapters -out ./webapi/bindadapter
```

JavaScript and `.d.ts` remain the package outputs. See
[LIFETIME.md](LIFETIME.md) for explicit parent ownership, async disposal, and
standard-library stream/event source adapters, and
[EXTENSIONS.md](EXTENSIONS.md#discriminated-records-and-api-placement) for event unions and API organization.

`wasmbind qualify` checks the installed npm package with local tools. Supply one
consumer and one or more TypeScript documentation fixtures that import the npm
package by name. Without `-fixtures`, each file must be self-contained. Use
`-fixtures ./fixtures` to copy a scoped tree and preserve relative sibling imports;
all input fixtures must be inside that tree. Dependency directories and Git
metadata are excluded. No tools are downloaded.

```sh
wasmbind qualify -package ./dist -out ./qualification \
  -tsc ./node_modules/.bin/tsc -npm npm -node node \
  -playwright ./node_modules/playwright/index.mjs \
  -consumer ./consumer.ts ./docs-example.ts
```

The output directory must not exist. Qualification packs the local directory,
installs its archive offline with lifecycle scripts disabled, type-checks all
fixtures with strict DOM and Worker libraries, and loads and disposes the
installed package in Chromium, Firefox and WebKit Window and dedicated Worker
contexts. Missing browser executables fail the command. `qualification.json`
records package and fixture hashes, tool versions and observed lifecycle checks.
The generated runner, archive, consumer workspace and logs stay in the owned
output directory for inspection. The copied fixture tree is hashed before
compilation. `-timeout 30000` bounds each browser lifecycle operation in
milliseconds; missing tools and timeout failures do not produce a passing
receipt. Use `-hook ./application-tests.mjs` for a local
Node application test; its successful exit is recorded separately and does not
stand in for the observed browser lifecycle checks.

Generated calls use `callOptions` for cancellation. A Go argument named `options`
keeps that name. In a discriminated union, only active fields undergo domain
validation. Inactive wire fields contain genuine Go zeros; their original Go
values are discarded before conversion and the JavaScript output omits them.
