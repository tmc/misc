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
and a manifest. `build` also emits Wasm and the selected compiler's local
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
rules still apply.

Use `//wasmbind:record` for copied structs and `//wasmbind:resource Name` for
opaque pointer types. Each resource needs one `//wasmbind:close` method with no
arguments and either no result or `error`. See [SPEC.md](SPEC.md) for conversions,
limits, ownership, cancellation, and error codes.

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
connection, framed stream, and verified blob-fetch APIs.

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
