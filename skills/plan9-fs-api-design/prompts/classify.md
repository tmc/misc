From the notebook sources, classify the shape of the API named "__SLUG__".
Answer ONLY the questions below — do not design a filesystem, draw a tree, or
mention Plan 9. Cite a source for the durable-handle answer.

1. Does a caller obtain a DURABLE HANDLE — a live object that persists across
   multiple calls and carries mutable state between them (a socket, a decoder,
   a model session, a connection)? Or is every operation a self-contained
   one-shot call with no handle reused across calls?
   - A `CryptoKey`, a file descriptor you reuse, or an opened port = durable handle.
   - `hash(bytes) -> digest`, `geocode(addr) -> latlng` = one-shot, NO durable handle.

2. Pick exactly ONE shape:
   - `instance-connection` — callers mint durable, stateful handles that open,
     carry state, maybe stream, and close. (Only if Q1 = durable handle.)
   - `request-response` — stateless one-shot calls: inputs in, result out, no
     reused handle. (Q1 = one-shot.)
   - `resource-tree` — the API is mainly addressable nouns/records/keys with
     CRUD-ish access; state lives in named resources, not in a session.

3. If there ARE persistent objects but they are RESOURCES reused across many
   one-shot calls (e.g. a key used by many encrypt calls), say so: the resource
   is a `resource-tree` node, and the operations on it are `request-response` —
   NOT a session/connection. Name the dominant shape for the tree.

Emit exactly these three lines, nothing else:

```
shape: <instance-connection | request-response | resource-tree | mixed:<dominant>>
durable-handle: <yes — name it | no>
reason: <one sentence, cite a source>
```
