# Resource lifetimes and source adapters

Resources and modules implement `dispose()` and the exported `asyncDispose`
symbol. When the runtime has `Symbol.asyncDispose`, TypeScript consumers can use
`await using`:

```ts
await using module = await load();
await using connection = await module.connect(id, "chat");
```

The declarations reference `esnext.disposable`. The generator does not install
or modify `Symbol.asyncDispose`. In runtimes without it, use `.dispose()` or
`resource[asyncDispose]()`. Downlevel TypeScript's `await using` helper requires
that the application provide `Symbol.asyncDispose`; the generated symbol alone
does not satisfy that helper. The fallback symbol is
`Symbol.for("Symbol.asyncDispose")`.

## Explicit parent ownership

An exported resource method can opt into ownership of returned resources:

```go
//wasmbind:export openStream
//wasmbind:owned
//wasmbind:nonnull
func (c *Connection) OpenStream(ctx context.Context) (*Stream, error) {
    // ...
}
```

Ownership also applies to slices and arrays of resource pointers. The bridge
records actual resource identities, including reused pointers, rather than
inferring a type-wide hierarchy. A child has at most one parent. Returning the
same child to that parent preserves identity. Assigning a second parent or
creating an ownership cycle rejects with `code: "contract"`.

Disposing a parent immediately invalidates its children, cancels their active
calls, and starts their interruption hooks. It waits for actual call leases,
then closes descendants before closing the parent. Errors from child closure
are included in the parent's disposal error. A child can be disposed early;
parent disposal does not close it twice. Resource-producing calls racing with
parent disposal cannot publish live children under the closed parent.

Ownership is explicit. A borrowed reference such as a subscription's sender
must not be annotated as owned unless closing the subscription really owns
that sender. A close function must not wait for its own resource's active call
leases; the bridge already waits for those leases before invoking close.

## Generate reusable Go adapters

Run before checking or building an API that imports the companion package:

```sh
wasmbind adapters -out webapi/bindadapter
```

The command emits a standard-library-only `bindadapter` package in the consumer
module. It adds no dependency on the wasmbind module. A content hash protects
hand edits; matching generated files can be refreshed by newer generators.
The emitted source should be committed alongside the wrapper source.

A pointer resource delegates its annotated hooks to an adapter:

```go
//wasmbind:resource ReceiveStream
type ReceiveStream struct { reader *bindadapter.Reader }

//wasmbind:read
func (s *ReceiveStream) Read(ctx context.Context, p []byte) (int, error) {
    return s.reader.Read(ctx, p)
}

//wasmbind:cancelread
func (s *ReceiveStream) CancelRead() error { return s.reader.Interrupt() }

//wasmbind:close
func (s *ReceiveStream) Close() error { return s.reader.Close() }
```

`ReadCloser` and `WriteCloser` adapt standard Go interfaces. `NewReader` and
`NewWriter` accept context-aware callbacks and separate interruption hooks.
`Writer.Finish` finishes successfully without subsequently interrupting it.
Each direction permits one active operation; overlapping native callers receive
`ErrBusy`. Cancellation permanently interrupts that direction. The read adapter
preserves bytes returned with an error. The write adapter leaves partial-write
retrying to the bridge.

`NewEvents[T]` adapts a context-aware pull without adding a queue.
`FromChannel[T]` receives from the caller's channel without adding a queue.
`FromSeq[T]` and `FromSeq2[T]` run a producer through an explicit capacity;
capacity zero uses rendezvous. A full queue blocks the producer. No events are
dropped. `Close` interrupts and joins a generated sequence producer. A sequence
error terminates iteration. Values are not copied by these Go adapters;
producers must not mutate yielded values afterward.

Interruption callbacks must safely run alongside an active operation and must
unblock it. For sequences they must unblock any source work outside `yield`;
sequences must stop when `yield` returns false. The bridge cannot force a Go
function that ignores interruption to return. Do not run cleanup requiring the
producer to finish inside its interruption callback; put that work after the
adapter's `Close` returns.
