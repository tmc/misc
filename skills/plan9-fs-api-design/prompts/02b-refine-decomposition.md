# Probe 2b — Authoritative Decomposed Tree

Adopt the **namespace-outward architect** lens with a brief
**webfs(4) pragmatist** pass. State blind spots.

Emit only `## 2. Filesystem tree`, short layout notes, and one compact
API mapping paragraph. Use the source brief and sources, not prior-run
memory. Preserve the user/source-chosen root.

Hard namespace contract:

- Root: `/<service>/{clone,$id/}`. Add `model/` only when the source API
  has model/provider/device surfaces such as availability, params, or
  download/progress. Do not add root `ctl` or root `status` unless the
  source API has source-backed global control or status beyond model
  scope.
- Direct Plan 9 frame: `/<service>/clone -> $id/{ctl,data,stream,status,event}`.
  These five per-session files are mandatory for session-shaped APIs.
- Session scope starts from `$id/{ctl,data,stream,status,event}`. Add
  `$id/{availability,opts,initial,clone}` only when those are
  source-backed features. Creation-time files are draft-state and freeze
  after `$id/ctl start`. The system prompt is represented as the first
  object in `initial`, not as a separate sibling file.
- Service/provider scope: when source-backed, use the relevant separate
  files from `model/{availability,params,event,ctl}`. Include only the
  children the source API actually supports. Do not replace these with one
  generic `model/status`.
- Context/accounting scope: when source-backed, use
  `$id/ctx/{window,usage,measure}` for context limit, current usage, and
  measurement. If measurement needs both input and output, make
  `measure/` a directory with separate `body` input and `usage` output
  files.
- Typed input scope: when source-backed, use `$id/in/{ctl,img0,aud0}`.
  Prompt bodies reference these by relative paths such as `@in/img0`;
  never host paths.
- Operation scope: when the API has independent per-call operations, use
  `$id/prompt/{clone,$n/{ctl,body,constraint,data,stream,status}}`.
  Per-call constraints stay with the prompt operation. Prompt operation
  `ctl` supports `start`, `append`, and `abort`; do not put large
  prompt-message JSON in session `ctl`.
- Callback/tool scope: when the API has tools or callbacks, use
  `$id/tools/{ctl,$NAME/...}` and include the source-backed per-tool
  files: for example `ctl`, `title`, `description`, `schema`,
  `annotations`, `exposed_to`, `call`, `return`, `status`, or
  `interact/{call,return}` when the source exposes them. Do not invent
  tool metadata files that the source does not support.
- Current names only. The ideal tree must not discuss compatibility names
  or migration support.

Final self-check before emitting: every path in the fenced tree is an
actual filesystem path with operational semantics. Do not put
"Not applicable", "N/A", or omitted feature families in the tree. Explain
omissions in the short layout notes or fails-closed prose only.
