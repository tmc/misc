# Relay-only go-iroh browser bindings

This independent example module pins `github.com/tmc/go-iroh v0.3.0`.
`webapi` exposes endpoint, connection, and stream resources with explicit
lifetimes. `Stream.Exchange` uses four-byte big-endian length prefixes and a
one-MiB frame limit. Cancellation aborts the stream; retries need a new stream.
`Endpoint.FetchBlob` verifies a BLAKE3-addressed blob with the iroh blob protocol.

From this directory, with wasmbind installed:

```sh
wasmbind build -go-out ./generated -out ./web ./webapi
go run ./cmd/relay -web ./web -store ~/tmp/wasmbind/goiroh-example/store
```

The relay prints its local URL, peer ID, and blob hash. Serve the generated files
from that origin, then:

```js
import {load} from './bindings.js';
const mod = await load(new URL('./module.wasm', import.meta.url));
const options = {signal: AbortSignal.timeout(30000)};
const endpoint = await mod.bind(relayURL, options);
let connection, stream;
try {
  connection = await endpoint.connect(peerID, 'wasmbind-echo/1', options);
  stream = await connection.open(options);
  const echoed = await stream.exchange(new Uint8Array([1,2,3]), options);
  const blob = await endpoint.fetchBlob(peerID, blobHash, options);
  console.log(echoed, blob);
} finally {
  if (stream) await stream.dispose();
  if (connection) await connection.dispose();
  await endpoint.dispose();
  await mod.dispose();
}
```

Run `npm run test:goiroh` from the wasmbind root to build the generated client,
start a hermetic Go relay and native peer, and check 64-KiB stream echo plus a
70-KiB verified blob in Chromium, Firefox, and WebKit. The test needs installed
Playwright browsers and cached Go dependencies. It fails if a browser is missing.

This tests browser-to-native Go communication through one local WebSocket relay.
It makes no claim about direct UDP, arbitrary remote relays, Rust interoperability,
or unrestricted browser networking.
