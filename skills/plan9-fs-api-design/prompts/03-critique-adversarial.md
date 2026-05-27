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

Emit:

## Findings

Use 0-4 bullets. Each bullet is:

`- <verdict>: <path or omission> - <small repair>`

Verdicts: `VIOLATES`, `MISSING`, `HONEST`.

Check only:

- root has no unbacked global files;
- direct session frame exists;
- creation-time files freeze after `$id/ctl start`;
- operation constraints live under `$id/prompt/$n/`;
- typed inputs are inside `$id/in/` and referenced relatively;
- tools use `call` and `return`;
- names are current.

End with:

`Blocking verdict: PASS` or `Blocking verdict: REPAIR BEFORE SEMANTICS`
