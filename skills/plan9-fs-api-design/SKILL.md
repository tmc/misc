---
name: plan9-fs-api-design
description: Design an API as a Plan 9/Wanix synthetic filesystem in one source-grounded NotebookLM call — shape-classified tree, per-file semantics, rc examples, supported/fails-closed boundary. Optional --verify self-critique pass.
allowed-tools:
  - Bash
  - Read
  - Write
  - Edit
  - Glob
  - Grep
---

# plan9-fs-api-design

Design an API from the namespace outward and emit a single
`<service>(4)`-style Markdown manpage. One prompt produces the whole design
in one NotebookLM call, grounded in mirrored sources: classify the API shape
(instance/connection, request/response, resource tree), draw the tree, give
per-file semantics, rc worked examples, and a supported/fails-closed boundary.

The prompt picks filesystem vocabulary by what the API actually does, so a
stateless or pure-data API is not forced into a connection lifecycle. For an
older, heavier pipeline that splits the work across 13 gated probes, see the
`plan9-fs-api-design-legacy` skill — in head-to-head testing this single-call
design scored higher and avoided the template-bleed the probe chain produced,
so prefer it unless you specifically need per-stage audit gates.

## Requirements

- `bash`, `curl`;
- `nlm` authenticated to NotebookLM;
- `html2md` for HTML mirroring (optional; raw mirror works without it).

## Configuration

| variable | default | purpose |
|---|---|---|
| `PLAN9_FS_DESIGN_SKILL_DIR` | script-relative | skill install path |
| `PLAN9_FS_DESIGN_HOME` | `$HOME/.plan9-fs-designs` | per-run work tree root |
| `PLAN9_FS_DESIGN_NOTEBOOK` | (none) | reuse an existing notebook id; if unset, the script reuses a cached id or creates one |
| `PLAN9_FS_DESIGN_MIN_BYTES` | `600` | minimum design size before retry |
| `PLAN9_FS_DESIGN_RETRIES` | `2` | design-call retries on empty/short output |
| `PLAN9_FS_DESIGN_SHAPE` | (none) | skip the classify call and force a shape: `instance-connection`, `request-response`, `resource-tree`, or `mixed:<dominant>` |

## Workflow

One command does the whole run once sources exist:

```bash
SKILL_DIR="${PLAN9_FS_DESIGN_SKILL_DIR:-$HOME/.claude/skills/plan9-fs-api-design}"
"$SKILL_DIR/scripts/design.sh" [--verify] <slug> <service-root> [source-dir]
```

`design.sh`:

1. resolves or creates a NotebookLM notebook (cached in
   `$PLAN9_FS_DESIGN_HOME/.notebook-id`);
2. syncs the source directory as run-scoped sources;
3. **classifies the API shape in an isolated call** (instance-connection,
   request-response, resource-tree, or mixed) and locks the answer — this
   keeps the design call from defaulting every API to the `/net` connection
   idiom. Set `PLAN9_FS_DESIGN_SHAPE` to skip this call and force a shape;
4. fires the design prompt with the service root, slug, and locked shape
   interpolated, source-scoped to this run; retries on empty/short output
   and exits 3 if the source audit returns `Decision: BLOCKED`;
5. cleans citation-bracket artifacts and writes `$WORK/<service>(4).md`
   plus `$WORK/design.stderr` (and `$WORK/classify.stderr`).

The two-call classify→design split is load-bearing: a single combined prompt
reliably mis-classifies stateless APIs (e.g. Web Crypto) as connection-shaped
and forces a spurious `clone`/`$n` session lifecycle onto them. Classifying in
isolation first fixed that in testing.

`--verify` adds one self-critique call: the draft is fed back with the
sources and a hostile-reviewer prompt that hunts hallucinated/imported
surfaces, missing surfaces, overloaded files, and shape mismatch, then emits
a repaired manpage (the draft is kept as `<service>(4).pre-verify.md`). This
doubles the nlm cost; use it for high-stakes designs.

To mirror sources first, drop spec/IDL/README/MDN files into a directory:

```bash
WORK="${PLAN9_FS_DESIGN_HOME:-$HOME/.plan9-fs-designs}/<slug>"
mkdir -p "$WORK/sources"
curl -sL <spec-url> | html2md > "$WORK/sources/spec.md"
"$SKILL_DIR/scripts/design.sh" <slug> /<service> "$WORK/sources"
```

## Design discipline (folded into the prompt)

The prompt enforces these Plan 9 invariants. The shape-dependent ones are
applied only when the API has that shape — the prompt classifies the API
(instance/connection, request/response, resource tree) before drawing a tree,
so a stateless or pure-data API is not forced into a connection lifecycle.

- **Namespace first.** The tree is the artifact; do not wrap a language API.
- **Scopes or resources become directories.** Distinct upstream lifecycles
  *or resource hierarchies* → distinct filesystem scopes. An API with no
  session/operation lifecycle may have a single flat scope.
- **One role per file.** `ctl`, `status`, `data`, `stream`, `event`,
  `req`/`resp` never share responsibilities — and only the roles the API
  needs exist.
- **Clone allocation is per open** *(instance/connection shapes only)*. Open
  `clone`, read returns the child id. Stateless and resource designs have no
  `clone`.
- **Creation-time state freezes** after a started instance starts *(only for
  APIs with started instances)*.
- **Bytes cross as files**, referenced relatively, never as host paths.
- **Callbacks become file traffic** *(only if the API has callbacks/push)*.
- **Unsupported state fails closed**, named explicitly. No silent fallback.
- **Capability is access.** Mount/9P permissions are the boundary; no ad
  hoc auth files unless the source API requires per-call auth.
- **Every load-bearing API claim cites a mirrored source.** Do not import
  vocabulary from a different kind of API.

## Output

- `$WORK/<service>(4).md` — the manpage, starting at the H1.

There is no bundle, no catalog, no verification gate. Inspect the manpage
and hand-repair if needed; that is the intended workflow.
