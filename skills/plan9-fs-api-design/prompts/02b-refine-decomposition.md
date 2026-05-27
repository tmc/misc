# Probe 2b — Authoritative Decomposed Tree

Adopt the **namespace-outward architect** lens with a brief
**webfs(4) pragmatist** pass. State blind spots.

Emit only `## 2. Filesystem tree`, short layout notes, and one compact
API mapping paragraph. Use the source brief and sources, not prior-run
memory. Preserve the user/source-chosen root.

Hard namespace contract:

- Root: `/<service>/{clone,model/,$id/}`. Do not add root `ctl` or
  root `status` unless the source API has source-backed global control
  or status beyond the model scope.
- Direct Plan 9 frame: `/<service>/clone -> $id/{ctl,data,stream,status,event}`.
  These five per-session files are mandatory for session-shaped APIs.
- Service/provider scope: if the upstream has availability, params, or
  download/progress surfaces, use separate files
  `model/{availability,params,event,ctl}`. Do not replace these with
  one generic `model/status`.
- Session scope: `$id/{ctl,status,availability,opts,initial,data,stream,event,clone}`.
  Creation-time files are draft-state and freeze after `$id/ctl start`.
  The system prompt is represented as the first object in `initial`, not
  as a separate sibling file.
- Context/accounting scope: `$id/ctx/{window,usage,measure}` when the
  upstream exposes context limit, current usage, and measurement.
  If measurement needs both input and output, make `measure/` a
  directory with separate `body` input and `usage` output files.
- Typed input scope: `$id/in/{ctl,img0,aud0}`. Prompt bodies reference
  these by relative paths such as `@in/img0`; never host paths.
- Operation scope: `$id/prompt/{clone,$n/{ctl,body,constraint,data,stream,status}}`.
  Per-call constraints stay with the prompt operation. Prompt operation
  `ctl` supports `start`, `append`, and `abort`; do not put large
  prompt-message JSON in session `ctl`. Flags such as omitted constraint
  text belong in `constraint` metadata or `ctl`, not a separate sibling
  file unless upstream exposes such an object.
- Callback/tool scope:
  `$id/tools/{ctl,$NAME/{description,schema,call,return,status}}`.
  Registration is creation-time; `call`/`return` are runtime RPC pipes.
- Current names only. The ideal tree must not discuss compatibility names
  or migration support.

Final self-check before emitting: every path family above appears if the
source-backed feature exists. If you are tempted to omit one, put it in
a short "Not applicable" note with the source-backed reason.
