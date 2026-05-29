# webidl2plan9

Generate a Plan 9 / Wanix `service(4)` synthetic-filesystem manpage skeleton
from a Web IDL definition.

```
go run ./cmd/webidl2plan9 -name "Chrome Prompt API" -root /llm spec.bs > llm.md
```

Input may be raw `.idl` or a Bikeshed `.bs`/HTML file with an
`<xmp class=idl>` / `<pre class=idl>` block (the IDL is extracted automatically).

## Why

Designing an API as a Plan 9 filesystem (see the `plan9-fs-api-design` skill)
splits into two parts:

- **~70% is deterministic bookkeeping the WebIDL fully settles** — which
  interfaces and dictionaries become scopes, which attributes are readable
  status files, which methods become `ctl` verbs or `clone` allocators, file
  permissions from `readonly`, deprecated members, the value types that can't
  cross a file boundary, and the complete feature-mapping table.
- **~30% is design judgment the IDL does not contain** — the namespace idiom,
  the draft/start/freeze lifecycle, callback-as-pipe representation, and the
  shape call (instance-connection vs request-response vs resource-tree).

A hand- or LLM-written design reliably drifts on the deterministic 70%: in
testing, a grounded reviewer flagged a Chrome Prompt API design for inventing
`ctl` verbs absent from the IDL, mapping a per-session callback to a global
event stream, and misstating a method's return type in the mapping table. Every
one of those is a place the author left the IDL. This tool emits that 70%
mechanically — so it *cannot* invent surfaces — and leaves the 30% of judgment
clearly marked for an author or LLM to complete.

## What it derives

| WebIDL construct | filesystem mapping |
|---|---|
| `static M() : Promise<ThisInterface>` | `clone` allocator → instance-connection shape |
| interface | session `$id/` scope (or resource dir) |
| `readonly attribute` | read-only status/value file |
| operation | `ctl` verb (return type → role) or sub-allocator |
| `Promise<double>` etc. return | numeric read; `ReadableStream` → stream; `undefined` → pure verb |
| dictionary member | staged draft file (frozen after start) |
| `callback` | `call`/`return` pipe pair |
| `enum` | legal token set of a status file |
| `... includes DestroyableModel` | `destroy` ctl verb |
| `object` / `EventHandler` / `AbortSignal` | fails closed / event file / ctl verb |
| union typedef | crossable members → byte slots; host-object members fail closed |
| `**DEPRECATED**` / `**EXPERIMENTAL**` comments | member flagged for fail-closed/symlink |

## What it does NOT do (left to the author/LLM)

- Worked `rc` examples and prose rationale.
- The final shape decision (it emits a heuristic shape, marked as such — verify
  it against the API's real lifecycle).
- Wanix capability prose, recommendations, and source citations.

It is a prototype: per-file semantics for non-instance-connection shapes are
stubbed, and static-returned interfaces (e.g. `params()`) are not yet rerooted
under their static path. The intent is to feed the deterministic skeleton into
the design loop, not to replace the reviewer.
