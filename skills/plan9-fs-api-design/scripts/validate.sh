#!/usr/bin/env bash
# validate.sh — one grounded NotebookLM validation pass over a Claude-authored
# draft manpage. NotebookLM, scoped to the synced sources, audits the draft for
# spec-fidelity and Plan 9 idiom and returns a structured verdict + fix-list.
# It does NOT rewrite the design — Claude repairs from the fix-list and re-runs.
#
# Usage:
#   validate.sh <slug> <service-root> <draft.md>
#
# Environment:
#   PLAN9_FS_DESIGN_SKILL_DIR   skill install path (overrides script-relative).
#   PLAN9_FS_DESIGN_HOME        work-tree root (default $HOME/.plan9-fs-designs).
#   PLAN9_FS_DESIGN_NOTEBOOK    notebook id holding the synced sources (required
#                               here; sync once with sync.sh before validating).
#   PLAN9_FS_DESIGN_VALIDATE_TRIALS  trials per pass (default 3); categorical
#                               gate on blockers, advisory scores worst-of-N.
#   PLAN9_FS_DESIGN_PRIME       prepend prompts/preamble.md persona (default 1).

set -euo pipefail

if [ "$#" -lt 3 ]; then
    echo "usage: validate.sh <slug> <service-root> <draft.md>" >&2
    exit 2
fi

SLUG="$1"; ROOT="$2"; DRAFT="$3"
[ -f "$DRAFT" ] || { echo "draft not found: $DRAFT" >&2; exit 2; }

SKILL_DIR="${PLAN9_FS_DESIGN_SKILL_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
VALIDATE_FILE="$SKILL_DIR/prompts/validate.md"
PREAMBLE_FILE="$SKILL_DIR/prompts/preamble.md"
[ -f "$VALIDATE_FILE" ] || { echo "missing prompt: $VALIDATE_FILE" >&2; exit 1; }
command -v nlm >/dev/null 2>&1 || { echo "nlm not found on PATH" >&2; exit 127; }

NB="${PLAN9_FS_DESIGN_NOTEBOOK:-}"
[ -n "$NB" ] || { echo "PLAN9_FS_DESIGN_NOTEBOOK must be set (sync sources first)" >&2; exit 2; }

HOME_DIR="${PLAN9_FS_DESIGN_HOME:-$HOME/.plan9-fs-designs}"
WORK="$HOME_DIR/$SLUG"
mkdir -p "$WORK"
RETRIES="${PLAN9_FS_DESIGN_RETRIES:-2}"
MIN_BYTES="${PLAN9_FS_DESIGN_MIN_BYTES:-200}"
# NotebookLM verdicts are high-variance: identical calls flip between PASS and
# REVISE and surface different real findings each run. So validate N times and
# aggregate conservatively — worst score across trials, unioned fix-lists. A
# single trial misses real defects.
TRIALS="${PLAN9_FS_DESIGN_VALIDATE_TRIALS:-3}"
# Persona preamble (prompts/preamble.md): a dynamic-role frame that the A/B
# showed produces the sharpest structural critiques. On by default; set
# PLAN9_FS_DESIGN_PRIME=0 to drop it.
PRIME="${PLAN9_FS_DESIGN_PRIME:-1}"
# Scope the review call to the API sources AND the just-uploaded candidate.
SCOPE="^$SLUG:|^plan9-|^wanix:"
CAND_NAME="$SLUG: candidate"

# The draft must be a SOURCE, not prompt text: NotebookLM returns an empty
# response when the prompt runs long (a multi-KB manpage inlined into the
# prompt reliably triggers this). Upload the draft as a source and reference it
# by name in a short prompt, then re-sync it each pass so the review sees the
# current draft.
#
# NotebookLM ingestion DROPS fenced/preformatted code (verified: a ```rc``` block
# and an indented block both vanish from the indexed source while surrounding
# prose survives). So the reviewer never sees the worked-example transcripts;
# example-coherence is therefore checked locally by the author, not here, and
# validate.md tells the reviewer not to fault missing code. We upload the draft
# verbatim anyway — stripping the fences would not make the code visible — but
# the reviewer is scoped to the prose it CAN read.
UPLOAD="$WORK/.candidate.upload.md"
cp "$DRAFT" "$UPLOAD"
echo "Uploading draft as source '$CAND_NAME'..." >&2
if ! nlm source sync "$NB" "$UPLOAD" --name "$CAND_NAME" >&2; then
    echo "failed to upload draft as a source" >&2
    exit 1
fi
sleep 5  # indexing buffer

# Short prompt: optional persona preamble, then interpolate root/threshold and
# point the reviewer at the candidate source by name (no inlined manpage).
VPROMPT="$(sed -e "s,__ROOT__,$ROOT,g" "$VALIDATE_FILE")"
if [ "$PRIME" = "1" ] && [ -f "$PREAMBLE_FILE" ]; then
    VPROMPT="$(cat "$PREAMBLE_FILE")
$VPROMPT"
fi
VPROMPT="$VPROMPT

You are the exacting but fair reviewer. The design under review is the
source named \"$CAND_NAME\". Judge THAT source against the other (API
specification) sources in the notebook."

# One trial: a generate-chat call that retries until it returns a parseable
# verdict block, or fails closed. Writes the raw verdict to $1; returns nonzero
# only on infrastructure failure (never a silent non-verdict).
one_trial() {
    local out="$1" attempt=0
    while :; do
        attempt=$((attempt + 1))
        if nlm generate-chat --source-match "$SCOPE" --citations tail "$NB" "$VPROMPT" \
                > "$out" 2> "$WORK/validate.stderr"; then
            local bytes; bytes=$(wc -c < "$out" | tr -d ' ')
            # Fail closed: empty/too-short is an NLM infrastructure failure, NOT
            # a REVISE verdict. Require a parseable 'verdict:' line.
            if [ "$bytes" -ge "$MIN_BYTES" ] && grep -qiE '^verdict:' "$out"; then
                return 0
            fi
            echo "  trial returned no usable verdict ($bytes bytes, no 'verdict:' line)" >&2
        else
            echo "  nlm generate-chat failed:" >&2; sed 's/^/    /' "$WORK/validate.stderr" >&2
        fi
        [ "$attempt" -le "$RETRIES" ] && { echo "  retrying trial..." >&2; sleep 10; continue; }
        return 1
    done
}

# Run TRIALS trials and aggregate. The GATE is categorical: PASS iff NO trial
# reported a blocker-severity finding (a hallucinated/wrong-shape/misstated
# surface). Highs and lows do not fail the design — they are the punch-list.
# The 0-10 dimension scores are ADVISORY (worst-of-N, for trend-watching only)
# and do NOT decide the verdict. Fixes are unioned across trials because NLM
# verdicts vary run-to-run and each trial catches different real defects.
DIMS="spec-fidelity shape-fit completeness plan9-idiom"
declare -A worst
for d in $DIMS; do worst[$d]=10; done
FIXES="$WORK/.validate.fixes"
: > "$FIXES"
ok_trials=0
any_blocker=0
trials_with_blocker=0
for t in $(seq 1 "$TRIALS"); do
    [ "$t" -gt 1 ] && sleep 3  # space calls so back-to-back trials aren't throttled
    echo "Validating (trial $t/$TRIALS, prime=$PRIME)..." >&2
    raw="$WORK/.validate.$t.raw"
    if ! one_trial "$raw"; then
        echo "  trial $t produced no verdict; skipping it" >&2
        continue
    fi
    ok_trials=$((ok_trials + 1))
    # Advisory: track the worst score per dimension.
    for d in $DIMS; do
        s=$(grep -iE "^$d:" "$raw" | head -1 | grep -oE '[0-9]+' | head -1)
        [ -n "$s" ] && [ "$s" -lt "${worst[$d]}" ] && worst[$d]=$s
    done
    # Collect this trial's fix bullets (lines after 'fixes:' up to the fence).
    awk 'tolower($0) ~ /^fixes:/ {f=1; next} f && /^```/ {f=0} f && /^- / {print}' "$raw" >> "$FIXES"
    # Gate: did THIS trial flag any blocker? (a [blocker] fix tag, or a
    # blockers: N>0 count line). Union across trials — any blocker fails.
    tb=0
    grep -qiE '^- *\[blocker\]' "$raw" && tb=1
    bc=$(grep -iE '^blockers:' "$raw" | head -1 | grep -oE '[0-9]+' | head -1)
    [ -n "$bc" ] && [ "$bc" -gt 0 ] && tb=1
    if [ "$tb" -eq 1 ]; then any_blocker=1; trials_with_blocker=$((trials_with_blocker + 1)); fi
done

if [ "$ok_trials" -eq 0 ]; then
    echo "validation produced no verdict in $TRIALS trials (NLM infrastructure failure, not a design verdict)" >&2
    exit 4
fi

# Categorical gate: any blocker in any trial → REVISE.
if [ "$any_blocker" -eq 0 ]; then verdict=PASS; else verdict=REVISE; fi
# Advisory overall = lowest dimension across trials (diagnostic only).
overall=10
for d in $DIMS; do [ "${worst[$d]}" -lt "$overall" ] && overall=${worst[$d]}; done

{
    echo "verdict: $verdict"
    echo "blockers: $trials_with_blocker/$ok_trials trials flagged a blocker"
    echo "advisory-score: $overall (lowest dimension, worst-of-N; not a gate)"
    for d in $DIMS; do echo "$d: ${worst[$d]} (advisory)"; done
    echo "trials: $ok_trials/$TRIALS (categorical gate on blockers; fixes unioned)"
    echo "fixes:"
    # Union the trials' fix bullets, dropping exact-duplicate lines but keeping
    # first-seen order (NLM phrases the same defect near-identically each trial).
    if [ -s "$FIXES" ]; then awk '!seen[$0]++' "$FIXES"; else echo "- none"; fi
} | tee "$WORK/validate.out"
