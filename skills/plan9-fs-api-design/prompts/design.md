Reinterpret the API described in the synced source files as a Plan 9 / Wanix
synthetic filesystem rooted at `__ROOT__`. Slug: __SLUG__. You (Claude) are the
architect — read the sources directly and design from them. Work ONLY from the
sources; cite every load-bearing claim. If the sources lack a normative API
definition, emit only `Decision: BLOCKED - <reason>` and stop.

__FEEDBACK_BLOCK__

Open with two lines, then go straight to the design (no input summary):

> Lens: namespace-outward architect — <error you are hunting>.
> Blind spots: <one clause>.

The API shape has ALREADY been classified (in a separate analysis):

__SHAPE_BLOCK__

Honor that shape — do NOT re-classify. It dictates the tree pattern:
- `instance-connection` → a `clone` allocator + numbered `$n/` dirs, one per durable handle.
- `request-response` → NO `clone`, NO `$n/` session dirs, NO per-call context allocation. Use a `req`/`resp` (or `call`/`return`) file pair per operation, or one operation directory per distinct call type. Each operation is one-shot.
- `resource-tree` → NO `clone`. Directories/files named for the resources (e.g. a directory per key/record); operations on a resource are one-shot `req`/`resp` files beside it, not a session.
- `mixed` → use the dominant shape's pattern; fold the secondary in where the sources demand.

Restate the locked shape in the first line of TL;DR. If the shape is request-response or resource-tree, the word `clone` and `$n` MUST NOT appear in your tree. A persistent object that is REUSED across many one-shot calls (a key, a record) is a resource-tree node, never a session.

Then emit a manpage with these sections, starting at the H1, no outer code fence:

`# __ROOT__(4) — A Plan 9 / Wanix Synthetic Filesystem for <API name>`

## TL;DR — API shape, the tree, the one idiom that carries it.
## Filesystem tree — one fenced tree. Include only the file roles the API needs from: `ctl` (control verbs), `data` (complete payload), `req`/`resp`, `stream` (incremental output), `status` (readable state), `event` (async push), input files. Service-wide state under a named scope, not loose root files.
## Per-file semantics — per file: path, read, write, the source-backed upstream op or field it maps to (cite). A `ctl` verb table (verb, args, effect, error) for each `ctl`.
## Feature mapping — table: upstream surface → path/op → notes. Every source surface appears or is listed fails-closed.
## Worked examples — 2-3 rc transcripts on the API's real main path, in its own shape, using real tree paths.
## Supported / fails-closed boundary — two lists; name every unrepresentable surface and why. Do not paper over gaps.
## Caveats — divergences from pure Plan 9 style, uncited assumptions, what to verify against the live API.

Rules: one role per file (split control/payload/stream/state/events when the API distinguishes them; not every role need exist). Clone is per-open (instance shapes only). Creation-time options freeze after a started instance starts (omit for stateless APIs). Binary inputs are relative files (`@in/<name>`), never host paths. Callbacks/tools/push → event streams or `call`/`return` pairs (only if present). Capability/mount is the security boundary; no auth files unless the API needs per-call auth. Current upstream names only. Tagged text for scalars; JSON only where the API already exposes structured records. Do NOT import vocabulary from a different kind of API (no `model/`, `prompt/`, `tools/`, `@in/img0`, `baudRate`, etc. unless THIS API's sources define them). Terse, factual manpage register.
