# Relay-only go-iroh browser bindings

This example pins `github.com/tmc/go-iroh v0.3.0` and uses all four wasmbind
extensions: duplex byte streams, gossip event iteration, imported HTTP, and
public wrapper errors. Endpoints, connections, streams, and subscriptions have
explicit lifetimes. Dispose children before parents. Read and write cancellation
are independent; closing a writer half-closes the stream.

```sh
wasmbind build -features streams,events,imports,errors -go-out ./generated -out ./web ./webapi
go run ./cmd/relay -web ./web -store ~/tmp/wasmbind/goiroh-example/store
```

```js
import {load} from './bindings.js';
const mod = await load(new URL('./module.wasm', import.meta.url), {imports: {Host: {
  Fetch: async (url, {signal}) => new Uint8Array(await (await fetch(url, {signal})).arrayBuffer()),
}}});
const endpoint = await mod.bind(relayURL);
const connection = await endpoint.connect(peerID, 'wasmbind-raw/1');
const stream = await connection.open();
try {
  const reader = stream.readable.getReader(), writer = stream.writable.getWriter();
  const receiving = reader.read();
  await writer.write(new Uint8Array([1,2,3]));
  console.log((await receiving).value);
  await writer.close();
} finally {
  await stream.dispose(); await connection.dispose();
  await endpoint.dispose(); await mod.dispose();
}
```

`Endpoint.fetchBlob` verifies a BLAKE3-addressed blob. `Endpoint.subscribe` joins
a 32-byte topic and yields copied events; breaking a for-await loop disposes the
subscription. Its upstream queue holds 128 events and reports loss with Lagged
events; the bridge adds no queue. `isWrapperError` recognizes copied public
validation errors. `mod.fetchFixture` invokes the supplied browser HTTP callback.

`npm run test:goiroh` checks 64-KiB duplex echo, cancellation of a blocked read,
gossip early break, typed errors, imported HTTP, and a verified 70-KiB blob in
Window and Worker realms in all three browsers. Firefox's pinned native executable
runs the Worker fixture due to a Playwright Worker WebSocket tracking failure.

The standalone `github.com/tmc/go-iroh-js` source module builds these bindings into
`@tmc/go-iroh-js`, including Wasm, declarations, runtime assets, and notices.
Local relay tests do not qualify direct UDP, remote relays, Rust interoperability,
or Node execution. Packaging does not publish anything.
