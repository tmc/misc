# Probe 3 - Tree gate

Adopt the **tree reviewer** lens. State your blind spots.

Check the 02b tree for blockers before semantics. Keep the response
short. Report only defects that would make the final document wrong or
unusable. Do not report ergonomic alternatives such as preferring a
`clone` allocator over a usable `ctl` allocator.

For session-shaped APIs, the direct session frame is an invariant:
`/<service>/clone -> $id/{ctl,data,stream,status,event}`. Replace
`/<service>` with the root chosen in probe 02b. The presence of
`$id/data` and `$id/stream` is required, even when the tree also has
per-operation directories.

Treat the accepted probe-02b tree as the authority for optional families.
Do not require `model/params`, `model/event`, `prompt/`, `ctx/`, `in/`, or
`tools/` unless the accepted tree already includes that family and the family
is structurally incomplete.

Emit:

## Findings

Use 0-4 bullets. Each bullet is:

`- <verdict>: <path or omission> - <small repair>`

Verdicts: `VIOLATES`, `MISSING`, `HONEST`.

Check only:

- root has no unbacked global files;
- direct session frame exists;
- creation-time files freeze after `$id/ctl start`;
- optional operation scopes are complete when present;
- typed inputs are inside `$id/in/` and referenced relatively when present;
- callbacks/tools use `call` and `return` when present;
- names are current.

End with:

`Blocking verdict: PASS` or `Blocking verdict: REPAIR BEFORE SEMANTICS`
