# Probe 0 — Source audit and run contract

Adopt the **source-cartographer** lens. State your blind spots.

Before designing the filesystem, audit whether the notebook contains the
right sources for this run. Use the uploaded source manifest and source
brief if present, then verify against the actual notebook sources. The
manifest is a map, not authority for API semantics.

Emit exactly these sections:

## Source roles

Markdown table:

| role | source title | authority for | freshness / caveat |

Required roles for a serious design:

- normative API definition: spec, IDL, RFC, upstream package docs, or
  equivalent;
- prose-heavy reference: vendor docs, MDN, README, tutorial, or examples;
- currentness signal: issue tracker, release notes, changelog, recent spec
  issue list, or explicit statement that none was found;
- Plan 9 precedent: manual page, paper, or source tree;
- Wanix precedent: source, docs, or explicit statement that it is missing.

## API identity

One short paragraph naming:

- the API being designed;
- the service root proposed by the sources or by the user's framing, if
  any;
- current canonical interface/type names;
- any unrelated source families that appear in the scoped source set.

Use current upstream names only. Do not include older-name or migration
history unless the sources expose a live naming conflict that would block
the design.

## Lifecycle facts to preserve

Bullet every source-backed lifecycle fact that downstream probes must not
lose: creation-time options, runtime operations, streaming behavior,
callback/tool surfaces, abort/destroy behavior, context/accounting, typed
binary inputs, and unsupported/fails-closed states. Cite primary sources.

## Source gaps

Bulleted list. Use `none found` only when the source set truly satisfies
the required roles above.

## Decision

End with one line:

`Decision: PROCEED` or `Decision: BLOCKED - <short reason>`

Choose BLOCKED when a normative API definition is missing, when no
non-trivial Plan 9 precedent source is available, or when the sources
identify conflicting current APIs and the user has not selected one. Do
not draft a tree in this probe.
