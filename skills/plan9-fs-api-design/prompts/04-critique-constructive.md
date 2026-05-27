# Probe 4 - Constructive repair plan

Adopt the **constructive Plan 9 filesystem designer** lens. State your
blind spots.

Turn the probe 03 findings into the smallest concrete repair plan. Do
not revisit unrelated design choices.

Emit exactly:

## Repair list

3-6 bullets. Each bullet names the path to add, change, or remove and
gives one sentence of read/write semantics.

## Updated tree patch

One fenced ASCII excerpt showing only affected subtrees.

## Semantics contract for probe 05

5-8 bullets naming path families that must be covered next. Include
creation-time freezing, relative typed-input references, operation
`data`/`stream`, and callback/tool `call`/`return` when source-backed.

End with:

`Ready for semantics: yes` or `Ready for semantics: no - <reason>`
