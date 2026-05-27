# Probe 5a - Section 3 Semantics

Adopt the **9P protocol implementer** lens. State your blind spots.

Write `### 3. Per-file / per-directory semantics` for the authoritative
02b tree. Keep it compact but complete.

For each important path family, include:

- a `#### /path - summary (mode)` heading;
- read semantics;
- write semantics;
- error strings;
- a verb table for `ctl` files.

Required coverage for session-shaped APIs, replacing `/<service>` with
the root chosen in probe 02b:

- `/<service>/clone`;
- `/<service>/model/{availability,params,event,ctl}` when present in 02b;
- `/<service>/$id/{ctl,status,availability,opts,initial,data,stream,event,clone}`;
- `/<service>/$id/ctx/{window,usage,measure/body,measure/usage}`;
- `/<service>/$id/in/{ctl,img0,aud0}`;
- `/<service>/$id/prompt/{clone,$n/{ctl,body,constraint,data,stream,status}}`;
- `/<service>/$id/tools/{ctl,$NAME/{description,schema,call,return,status}}`.

If 02b omits a listed path family, follow 02b and do not invent it. Do
not include root `ctl` or root `status` unless 02b includes them.

State that creation-time files freeze after `$id/ctl start`; late writes
return `Estarted`. Operation input files carry input, `data` is complete
output, and `stream` is incremental output. Callback/tool `call` and
`return` are matched by call id. Binary references are relative
(`@in/img0`), never host paths.
