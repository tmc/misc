---
name: plan9-fs-api-design
description: Design APIs as Plan 9/Wanix synthetic filesystems with a source-backed tree, per-file semantics, rc examples, and supported/fails-closed boundary.
allowed-tools:
  - Bash
  - Read
  - Write
  - Edit
  - Glob
  - Grep
---

# plan9-fs-api-design

Design an API from the namespace outward. The output is a rigorous
`<service>(4)`-style Markdown document plus a txtar evidence bundle.

The design oracle is a NotebookLM notebook seeded with Plan 9 papers,
Plan 9 manual pages, Wanix source/docs, and a freshly mirrored source
corpus for the target API. The notebook is useful only when the source
set is right; source curation and probe gating are part of the skill.

## Portability Contract

This skill is intended to run from a Codex skill install, a Claude Code
personal skill install, or this repository. It requires:

- `bash`, `awk`, `sed`, `grep`, `curl`;
- `nlm` authenticated to NotebookLM;
- `html2md` for HTML mirroring;
- `txtar` for final bundle extraction;
- optional `jq` for issue-list cleanup in ad hoc source mirroring.

Always resolve the skill directory before running bundled scripts:

```bash
SKILL_DIR="${PLAN9_FS_SKILL_DIR:-}"
if [ -z "$SKILL_DIR" ]; then
  for d in \
    "$PWD/skills/plan9-fs-api-design" \
    "$PWD/.claude/skills/plan9-fs-api-design" \
    "${CODEX_HOME:-$HOME/.codex}/skills/plan9-fs-api-design" \
    "$HOME/.claude/skills/plan9-fs-api-design"; do
    if [ -f "$d/SKILL.md" ]; then SKILL_DIR="$d"; break; fi
  done
fi
[ -n "$SKILL_DIR" ] || {
  echo "set PLAN9_FS_SKILL_DIR to the plan9-fs-api-design skill directory" >&2
  exit 1
}
```

If installed in another custom location, set `PLAN9_FS_SKILL_DIR`
explicitly.

## Operating Contract

- Start from the filesystem tree, not language bindings.
- Treat `$WORK/sources/MANIFEST.md`, `$WORK/source-brief.md`, and probe
  `00-source-audit` as the run contract.
- Cite primary sources for API facts. The manifest and brief are routing
  maps, not semantic authority.
- Keep current upstream names in the target design. Omit name-history and
  compatibility material unless the user explicitly asks for it.
- Let probe outputs define the final contract. Do not maintain separate
  per-run grep lists.
- Reject weak synthesis. If probe 09 regresses from the authoritative
  tree, repair the final document deterministically and tighten the
  prompt that allowed the regression.

## Inputs

The user may provide one or more of:

- URL to a spec, README, tutorial, or issue tracker.
- Local path to a library or repo.
- Free-form API description.
- Desired service root such as `/llm` or `/audio`.

If the API identity or root is ambiguous, ask one concise clarifying
question before syncing sources.

Use a short kebab slug for the run:

```bash
ARG="$1"
SLUG=$(echo "$ARG" | sed -E 's,https?://,,; s,[^A-Za-z0-9]+,-,g' | tr A-Z a-z | sed 's/-$//')
WORK="${PLAN9_FS_HOME:-$HOME/.plan9-fs-designs}/$SLUG"
mkdir -p "$WORK"/{sources,probes,out}
echo "$ARG" > "$WORK/input.txt"
echo "unknown" > "$WORK/input-shape.txt"
```

Do not clobber an existing `$WORK`. Re-runs are incremental unless the
user asks for a refresh.

## Workflow

### 1. Mirror the Source Corpus

Actively discover the upstream corpus; do not stop at the first URL.
Use all source classes that apply:

- Normative spec, IDL, RFC, package docs, or upstream README.
- Prose-heavy reference: MDN, vendor docs, tutorial, examples, or design
  notes.
- Change signal: issue tracker snapshot, release notes, changelog, or a
  documented absence of one.
- Polyfill or reference implementation when available.
- Local docs/examples for path inputs.

Prefer `curl -sL URL | html2md` for HTML. Mirror IDL or source files raw.
When a local path is the input, copy only docs, examples, IDL, and
package docs into `$WORK/sources/`.

Write:

- `$WORK/sources/MANIFEST.md` with source path, origin URL/path, fetch
  date, role, and caveat.
- `$WORK/source-brief.md`, a short routing map:

```markdown
# Source brief for <slug>

## API identity
- canonical API name:
- chosen service root, if user-specified:
- current interface/type names:

## Source authority
| source | role | use for | caveat |

## Lifecycle facts to preserve
- creation-time options:
- runtime operations:
- streaming/events:
- callbacks/tools:
- abort/destroy:
- typed/binary inputs:
- unsupported/fails-closed state:
```

Before seeding the notebook, show the manifest summary to the user if the
corpus is missing any of: normative source, prose source, or change
signal.

### 2. Bootstrap and Sync Notebook Sources

Resolve or create the shared notebook:

```bash
NB=$("$SKILL_DIR/scripts/bootstrap-notebook.sh")
```

Use `--force` only when the user explicitly asks for a fresh notebook.
If forced creation hits a NotebookLM account limit, do not delete
notebooks. Reuse an existing notebook with fresh run-scoped source names,
set `SOURCE_MATCH` to the new slug plus `^plan9-|^wanix:`, open a fresh
conversation, and record that reuse caveat in the report.

Sync the API corpus as run-scoped sources:

```bash
nlm source sync "$NB" "$WORK/sources" --name "$SLUG: mirrored API sources"
[ -f "$WORK/source-brief.md" ] && \
  nlm source add --name "$SLUG: source brief" "$NB" "$WORK/source-brief.md"
```

Uploads should be sequential. Wait briefly after large uploads so
NotebookLM indexing catches up.

### 3. Anchor the Run

Reset chat instructions and open a fresh conversation:

```bash
"$SKILL_DIR/scripts/anchor-mission.sh" \
  "$SLUG" "$(cat "$WORK/input.txt")" "$(cat "$WORK/input-shape.txt")"
```

The helper writes `$WORK/anchor.txt`, `$WORK/anchor.stdout`,
`$WORK/anchor.stderr`, and `$WORK/conv-id.txt`. If the anchor response
does not declare a lens or blind spots, reset the instructions and rerun
the anchor.

### 4. Run Probes

```bash
"$SKILL_DIR/scripts/run-probes.sh" "$SLUG"
```

The runner is incremental, retrying, and source-scoped by default to:
`^$SLUG:|^plan9-|^wanix:`. Override with `SOURCE_MATCH=...` only when
debugging source-list hygiene.

Probe map:

| probe | purpose |
|---|---|
| 00-source-audit | Source roles, API identity, lifecycle facts, proceed/block |
| 01-survey-api | Current-name surface inventory and lifecycle notes |
| 02-propose-tree | Panel-proposed namespace tree |
| 02b-refine-decomposition | Authoritative lifecycle-decomposed tree |
| 03-critique-adversarial | Deterministic tree gate for blocking shape defects |
| 04-critique-constructive | Deterministic repair plan and semantics contract |
| 05a-flesh-out-section3 | Deterministic per-file semantics from probe 02b |
| 05b-feature-mapping | Deterministic feature mapping table from probe 01 |
| 06-worked-examples | Deterministic rc transcripts from probe 02b |
| 07-philosophy-and-deviations | Deterministic philosophy and fails-closed boundary |
| 08-recommendations-and-caveats | Deterministic implementation plan and caveats |
| 09-synthesize-document | Deterministic final manpage assembler |
| 10-meta-critique | Prompt/instruction critique; periodic only |

If probe 00 says `Decision: BLOCKED`, stop and fix the corpus or get an
explicit user override. If probes 03/04 identify a rule leak in
`chat-instructions.md` or a prompt, edit the skill before continuing.
Probes 03 through 09 are generated locally by default once the source
audit, API inventory, and authoritative tree exist. This keeps final
quality tied to the accepted tree and avoids empty-response cascades in
long mechanical probes. Set the matching `DERIVE_*` variable to `0`
only when explicitly testing NotebookLM behavior for that step
(`DERIVE_03`, `DERIVE_04`, `DERIVE_05A`, `DERIVE_05B`, `DERIVE_06`,
`DERIVE_07`, `DERIVE_08`, or `DERIVE_09`).

### 5. Verify, Bundle, Finalize

Run the generic final gates:

```bash
"$SKILL_DIR/scripts/verify-output.sh" "$SLUG"
```

Then bundle the evidence and publish the user-facing manpage:

```bash
"$SKILL_DIR/scripts/bundle.sh" "$SLUG"
"$SKILL_DIR/scripts/finalize.sh" "$SLUG"
```

Outputs:

- `$WORK/<service>(4).md`
- `$WORK/<service>.txtar`
- `${PLAN9_FS_CATALOG:-$HOME/plan9-fs-specs}/<service>(4).md`
- `${PLAN9_FS_CATALOG:-$HOME/plan9-fs-specs}/<service>/`

The catalog copy is the artifact to compare against holdouts or golden
ideals.

## Review Standard

Before calling a run good, inspect:

- Probe 00 source roles and decision.
- Probe 01 raw surface count and omissions.
- Probe 02b tree against the user's desired namespace frame.
- Probe 03/04 blocking findings and whether the final repaired them.
- Probe 09 for drift from 02b.
- `verify-output.sh` output.

The right final answer is allowed to be hand-repaired from the verified
probe material. The goal is the artifact's design quality, not preserving
a bad generated draft.

## Failure Modes

- Wrong source titles can exclude the API corpus from the runner's
  default source scope. Keep uploaded API sources named `$SLUG: ...`.
- Long prompt context can make NotebookLM return empty responses; prefer
  lean probes with selected prior context.
- Mechanical mapping probes can be derived from structured earlier probes.
  Prefer deterministic local derivation over asking NotebookLM to restate
  a table it has already produced.
- Probe 09 may synthesize a weaker tree than probe 02b. Reject it.
- If the notebook response loses lenses, reset `chat-instructions.md`.
- Do not add a per-call auth file unless the API itself requires
  per-call authentication; Wanix capability possession is authority.

## Meta-Critique

Run probe 10 only after a real dogfood run or when output quality drops:

```bash
"$SKILL_DIR/scripts/run-probes.sh" "$SLUG" '10-*.md'
```

Apply concrete prompt/instruction edits, rerun the affected probes, and
rerun verification.
