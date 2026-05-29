---
name: plan9-fs-api-design
description: Design an API as a Plan 9/Wanix synthetic filesystem — Claude authors the shape-classified design from the sources, NotebookLM validates the prose against those sources and Plan 9 idiom, a local check covers the rc examples (NLM can't see code), and Claude repairs until it passes. Emits a <service>(4) manpage.
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
`<service>(4)`-style Markdown manpage: a shape-classified tree, per-file
semantics, rc worked examples, and a supported/fails-closed boundary.

**Author–validate–repair loop.** Claude is the architect and NotebookLM is the
grounded reviewer — the division of labor that testing showed each is best at.
Claude reads the sources and writes the design; NotebookLM, scoped to those
same sources, audits the draft for spec-fidelity and Plan 9 idiom and returns a
verdict plus a cited fix-list (it does *not* rewrite); Claude repairs from the
fix-list; repeat until NotebookLM signs off or the iteration cap is hit. In
head-to-head testing Claude-direct produced more spec-faithful designs than
NotebookLM-as-author, while NotebookLM-as-reviewer reliably caught real missed
and misstated surfaces — so this split plays to both strengths.

For a one-shot NotebookLM-authored design (no loop), `scripts/design.sh` is
still here as a fallback. For the older 13-probe pipeline see the
`plan9-fs-api-design-legacy` skill.

## Requirements

- `bash`, `curl`;
- `nlm` authenticated to NotebookLM;
- `html2md` for HTML mirroring (optional; raw mirror works without it).

## Configuration

| variable | default | purpose |
|---|---|---|
| `PLAN9_FS_DESIGN_SKILL_DIR` | script-relative | skill install path |
| `PLAN9_FS_DESIGN_HOME` | `$HOME/.plan9-fs-designs` | per-run work tree root |
| `PLAN9_FS_DESIGN_NOTEBOOK` | (none) | notebook id holding the synced sources; `sync.sh` prints the one it resolved/created |
| `PLAN9_FS_DESIGN_PASS_THRESHOLD` | `8` | per-dimension PASS floor for `validate.sh` |
| `PLAN9_FS_DESIGN_VALIDATE_TRIALS` | `3` | `validate.sh` trials per pass; scored worst-of-N, fixes unioned |
| `PLAN9_FS_DESIGN_PRIME` | `1` | prepend `prompts/preamble.md` (dynamic-persona frame); `0` to drop it |
| `PLAN9_FS_DESIGN_SHAPE` | (none) | skip the classify call and force a shape: `instance-connection`, `request-response`, `resource-tree`, or `mixed:<dominant>` |
| `PLAN9_FS_DESIGN_MIN_BYTES` / `_RETRIES` | `600` / `2` | only used by the `design.sh` fallback |

## Workflow (the loop)

Drop spec/IDL/README/MDN files into a source directory first (mirror HTML with
`curl -sL <url> | html2md` if needed), then:

**1. Sync + classify (once).** Resolves a notebook, uploads the sources, and
classifies the API shape in an isolated grounded call (the two-step split is
load-bearing — a combined prompt mis-classifies stateless APIs like Web Crypto
as connection-shaped and forces a spurious `clone`/`$n` lifecycle). Capture the
notebook id and shape from its stdout:

```bash
SKILL_DIR="${PLAN9_FS_DESIGN_SKILL_DIR:-$HOME/.claude/skills/plan9-fs-api-design}"
"$SKILL_DIR/scripts/sync.sh" <slug> <source-dir>
# -> notebook: <id>
#    shape: ...  / durable-handle: ...  / reason: ...
export PLAN9_FS_DESIGN_NOTEBOOK=<id>
```

**2. Author (Claude).** Read the synced source files and `prompts/design.md`.
Splice the classified shape block in at `__SHAPE_BLOCK__`, drop the
`__FEEDBACK_BLOCK__` line on the first pass, interpolate `__SLUG__`/`__ROOT__`,
and follow the prompt to write the manpage to `$WORK/<service>(4).md`. Ground
every load-bearing claim in the sources; do not import vocabulary from a
different kind of API.

**3. Validate.** Two complementary checks, because the grounded reviewer is
blind to one thing:

```bash
"$SKILL_DIR/scripts/check-examples.sh" <service-root> "$WORK/<service>(4).md"   # local, no NLM
"$SKILL_DIR/scripts/validate.sh"      <slug> <service-root> "$WORK/<service>(4).md"  # NotebookLM
```

- `validate.sh` uploads the draft as a source and has NotebookLM, scoped to the
  spec, score four **prose** dimensions — spec-fidelity, shape-fit,
  completeness, plan9-idiom — and emit a cited `fixes:` punch-list. It reviews,
  it does not rewrite. It fails closed (exit 4) if NLM returns no parseable
  verdict, rather than emitting a silent REVISE. Because NLM verdicts are
  high-variance, it runs `PLAN9_FS_DESIGN_VALIDATE_TRIALS` trials (default 3),
  takes the **worst** dimension score, and **unions** the fix-lists — a single
  call misses real defects that the union catches. The call is primed with
  `prompts/preamble.md` (a dynamic-persona frame) by default; in a powered A/B
  the persona's wording was load-bearing — an "exacting but fair, score
  calibrated separately from the fix-list" reviewer scored 9.4/100%-PASS vs
  8.3/60% with no preamble, while a "hostile auditor" frame scored *worse*
  (7.8/30%). Set `PLAN9_FS_DESIGN_PRIME=0` to drop it.
- `check-examples.sh` covers **example-coherence locally** (exit 5 on failure):
  it verifies the draft has rc transcripts and that every `ctl` verb a
  transcript writes appears in a verb table. This is *not* in the NLM rubric
  because **NotebookLM ingestion strips fenced/preformatted code** — the
  reviewer never sees the transcripts, so it cannot judge them (it would
  perpetually report "missing examples"). Verified: a fenced block and an
  indented block both vanish from an indexed source while surrounding prose
  survives. Keep code coherence a local check; keep the NLM call on prose.

**4. Repair (Claude) and loop.** If `validate.sh` says `verdict: REVISE` (or
`check-examples.sh` fails), re-author the manpage addressing each fix — re-run
`prompts/design.md` with the verdict's `fixes:` list spliced in at
`__FEEDBACK_BLOCK__` — then go back to step 3. Stop when `validate.sh` is
`verdict: PASS` (every dimension ≥ `PLAN9_FS_DESIGN_PASS_THRESHOLD`) **and**
`check-examples.sh` passes, or after a few iterations; report the last verdict
either way.

The fixes are concrete and source-cited (e.g. "design conflates the mutable
builder and the immutable compiled graph — the spec keeps them as separate
objects; split them"), so each repair is a targeted edit, not a rewrite. In
testing on WebNN this converged to a clean `PASS` (10/10 on all four NLM
dimensions) with example-coherence verified locally.

### One-shot fallback

`scripts/design.sh [--verify] <slug> <service-root> [source-dir]` runs the old
single NotebookLM-authored path (sync → classify → design, optional self-critique
`--verify`). Use it when you don't want the loop; the author–validate loop above
produced more spec-faithful designs in testing.

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
