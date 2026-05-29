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
#   PLAN9_FS_DESIGN_PASS_THRESHOLD  per-dimension PASS floor (default 8).

set -euo pipefail

if [ "$#" -lt 3 ]; then
    echo "usage: validate.sh <slug> <service-root> <draft.md>" >&2
    exit 2
fi

SLUG="$1"; ROOT="$2"; DRAFT="$3"
[ -f "$DRAFT" ] || { echo "draft not found: $DRAFT" >&2; exit 2; }

SKILL_DIR="${PLAN9_FS_DESIGN_SKILL_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
VALIDATE_FILE="$SKILL_DIR/prompts/validate.md"
[ -f "$VALIDATE_FILE" ] || { echo "missing prompt: $VALIDATE_FILE" >&2; exit 1; }
command -v nlm >/dev/null 2>&1 || { echo "nlm not found on PATH" >&2; exit 127; }

NB="${PLAN9_FS_DESIGN_NOTEBOOK:-}"
[ -n "$NB" ] || { echo "PLAN9_FS_DESIGN_NOTEBOOK must be set (sync sources first)" >&2; exit 2; }

HOME_DIR="${PLAN9_FS_DESIGN_HOME:-$HOME/.plan9-fs-designs}"
WORK="$HOME_DIR/$SLUG"
mkdir -p "$WORK"
THRESHOLD="${PLAN9_FS_DESIGN_PASS_THRESHOLD:-8}"
RETRIES="${PLAN9_FS_DESIGN_RETRIES:-2}"
MIN_BYTES="${PLAN9_FS_DESIGN_MIN_BYTES:-200}"
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

# Short prompt: interpolate root/threshold, point the reviewer at the candidate
# source by name (no inlined manpage).
VPROMPT="$(sed -e "s,__ROOT__,$ROOT,g" -e "s,__PASS_THRESHOLD__,$THRESHOLD,g" "$VALIDATE_FILE")"
VPROMPT="$VPROMPT

The design under review is the source named \"$CAND_NAME\". Judge THAT source
against the other (API specification) sources in the notebook."

attempt=0
while :; do
    attempt=$((attempt + 1))
    echo "Validating draft against sources (nlm call $attempt, threshold $THRESHOLD)..." >&2
    if nlm generate-chat --source-match "$SCOPE" --citations tail "$NB" "$VPROMPT" \
            > "$WORK/.validate.raw" 2> "$WORK/validate.stderr"; then
        bytes=$(wc -c < "$WORK/.validate.raw" | tr -d ' ')
        # Fail closed: an empty / too-short response is an NLM infrastructure
        # failure, NOT a REVISE verdict. Retry, then abort — never emit a
        # silent non-verdict the loop would read as REVISE.
        if [ "$bytes" -ge "$MIN_BYTES" ] && grep -qiE '^verdict:' "$WORK/.validate.raw"; then
            break
        fi
        echo "validator returned no usable verdict ($bytes bytes, no 'verdict:' line)" >&2
        sed 's/^/  /' "$WORK/validate.stderr" >&2
    else
        echo "nlm generate-chat failed:" >&2
        sed 's/^/  /' "$WORK/validate.stderr" >&2
    fi
    if [ "$attempt" -le "$RETRIES" ]; then echo "retrying validation..." >&2; sleep 10; continue; fi
    echo "validation did not produce a verdict after $attempt attempts (NLM infrastructure failure, not a design verdict)" >&2
    exit 4
done

# Surface the verdict block to stdout.
cp "$WORK/.validate.raw" "$WORK/validate.out"
cat "$WORK/validate.out"
