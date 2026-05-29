#!/usr/bin/env bash
# design.sh — one-shot Plan 9 / Wanix filesystem design via a single
# NotebookLM call. Resolves a notebook, syncs sources, fires one prompt,
# writes the manpage. The entire orchestration lives here.
#
# Usage:
#   design.sh [--verify] <slug> <service-root> [source-dir]
#
# Flags:
#   --verify    After the design call, fire a second self-critique call that
#               checks the manpage against the sources for hallucinated
#               surfaces, missing surfaces, and overloaded files, and emits a
#               repaired manpage. Doubles the nlm cost; use for high-stakes
#               designs. Off by default (single call).
#
# Environment:
#   PLAN9_FS_DESIGN_SKILL_DIR   skill install path (overrides script-relative).
#   PLAN9_FS_DESIGN_HOME        work-tree root (default $HOME/.plan9-fs-designs).
#   PLAN9_FS_DESIGN_NOTEBOOK    reuse this notebook id (else cache, else create).
#   PLAN9_FS_DESIGN_MIN_BYTES   min acceptable design size before retry (default 600).
#   PLAN9_FS_DESIGN_RETRIES     design-call retries on empty/short output (default 2).
#   PLAN9_FS_DESIGN_SHAPE       skip the classify call; force this shape
#                               (instance-connection | request-response | resource-tree | mixed:<x>).

set -euo pipefail

VERIFY=0
while [ "$#" -gt 0 ]; do
    case "$1" in
        --verify) VERIFY=1; shift ;;
        --) shift; break ;;
        -*) echo "unknown flag: $1" >&2; exit 2 ;;
        *) break ;;
    esac
done

if [ "$#" -lt 2 ]; then
    echo "usage: design.sh [--verify] <slug> <service-root> [source-dir]" >&2
    exit 2
fi

SLUG="$1"
ROOT="$2"
case "$SLUG" in
    ''|*[!a-z0-9-]*|-*|*-)
        echo "invalid slug '$SLUG': lowercase kebab-case ([a-z0-9-], no leading/trailing dash)" >&2
        exit 2 ;;
esac
case "$ROOT" in
    /*) : ;;
    *) echo "invalid service-root '$ROOT': must start with '/' (e.g. /serial)" >&2; exit 2 ;;
esac

SKILL_DIR="${PLAN9_FS_DESIGN_SKILL_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
PROMPT_FILE="$SKILL_DIR/prompts/design.md"
CLASSIFY_FILE="$SKILL_DIR/prompts/classify.md"
VERIFY_FILE="$SKILL_DIR/prompts/verify.md"
[ -f "$PROMPT_FILE" ] || { echo "missing prompt: $PROMPT_FILE" >&2; exit 1; }

command -v nlm >/dev/null 2>&1 || { echo "nlm not found on PATH; install and authenticate it" >&2; exit 127; }

HOME_DIR="${PLAN9_FS_DESIGN_HOME:-$HOME/.plan9-fs-designs}"
WORK="$HOME_DIR/$SLUG"
SRC_DIR="${3:-$WORK/sources}"
mkdir -p "$WORK"
CACHE="$HOME_DIR/.notebook-id"
MIN_BYTES="${PLAN9_FS_DESIGN_MIN_BYTES:-600}"
RETRIES="${PLAN9_FS_DESIGN_RETRIES:-2}"
SCOPE="^$SLUG:|^plan9-|^wanix:"

# Strip NotebookLM citation-bracket artifacts the model sometimes emits, e.g.
# "[6]3, 6, 7]" or "[8]-10]" — collapse to a clean "[6, 7]" / "[8-10]".
clean_citations() {
    # Conservatively repair the two unambiguous NotebookLM bracket artifacts:
    #   "[8]-10]" -> "[8-10]" (split range) and "[4], 5]" -> "[4, 5]" (split list).
    # Genuinely garbled forms (e.g. "[6]3, 6, 7]") are left for the --verify
    # pass, whose prompt repairs citations; a greedier regex risks corrupting
    # valid markers, so we stay conservative here.
    sed -E \
        -e 's/\[([0-9]+)\](-[0-9]+)\]/[\1\2]/g' \
        -e 's/\[([0-9]+)\], ([0-9]+)\]/[\1, \2]/g'
}

# 1. Resolve notebook: explicit env > cache > create. Write cache atomically.
resolve_notebook() {
    if [ -n "${PLAN9_FS_DESIGN_NOTEBOOK:-}" ]; then echo "$PLAN9_FS_DESIGN_NOTEBOOK"; return; fi
    if [ -f "$CACHE" ]; then head -1 "$CACHE"; return; fi
    echo "Creating notebook plan9-fs-design..." >&2
    local log="$WORK/.create.out"
    if ! nlm notebook create 'plan9-fs-design' >"$log" 2>&1; then
        echo "nlm notebook create failed:" >&2
        sed 's/^/  /' "$log" >&2
        return 1
    fi
    grep -oE '[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}' "$log" | head -1
}

NB="$(resolve_notebook)"
[ -n "$NB" ] || { echo "could not resolve a notebook id" >&2; exit 1; }
printf '%s\n' "$NB" > "$CACHE.tmp" && mv "$CACHE.tmp" "$CACHE"
echo "Notebook: $NB" >&2

# 2. Sync run-scoped sources.
if [ -d "$SRC_DIR" ] && [ -n "$(ls -A "$SRC_DIR" 2>/dev/null)" ]; then
    echo "Syncing sources from $SRC_DIR ..." >&2
    nlm source sync "$NB" "$SRC_DIR" --name "$SLUG: sources" >&2
    sleep 5  # indexing buffer
else
    echo "warning: no sources in $SRC_DIR; design will be ungrounded" >&2
fi

# 3. Classify the API shape in an isolated call, so the design call cannot
#    drift toward the connection idiom while drawing a tree. The locked shape
#    is injected into the design prompt. Skip if PLAN9_FS_DESIGN_SHAPE is set.
SHAPE_FILE="$WORK/.shape-block"
UNKNOWN="shape: unknown — classify from the sources yourself (durable handle? if no, do NOT use clone/\$n)"
if [ -n "${PLAN9_FS_DESIGN_SHAPE:-}" ]; then
    printf 'shape: %s (operator-specified)\n' "$PLAN9_FS_DESIGN_SHAPE" > "$SHAPE_FILE"
    echo "Shape (operator-set): $PLAN9_FS_DESIGN_SHAPE" >&2
elif [ -f "$CLASSIFY_FILE" ]; then
    echo "Classifying API shape (nlm call)..." >&2
    CPROMPT="$(sed -e "s,__SLUG__,$SLUG,g" "$CLASSIFY_FILE")"
    if nlm generate-chat --source-match "$SCOPE" "$NB" "$CPROMPT" \
            > "$WORK/.classify.raw" 2> "$WORK/classify.stderr"; then
        # Keep only the three contract lines (shape/durable-handle/reason).
        grep -iE '^(shape|durable-handle|reason):' "$WORK/.classify.raw" | head -3 > "$SHAPE_FILE"
    fi
    if [ ! -s "$SHAPE_FILE" ]; then
        echo "warning: shape classification returned nothing usable; design will self-classify" >&2
        printf '%s\n' "$UNKNOWN" > "$SHAPE_FILE"
    else
        sed 's/^/  /' "$SHAPE_FILE" >&2
    fi
else
    printf '%s\n' "$UNKNOWN" > "$SHAPE_FILE"
fi

# 4. Build the design prompt with slug, root, and locked shape interpolated.
#    Splice the shape-block file in at the placeholder line (sed 'r' handles
#    multi-line content safely), then interpolate slug/root and drop the marker.
PROMPT="$(sed -e "/^__SHAPE_BLOCK__$/r $SHAPE_FILE" -e "/^__SHAPE_BLOCK__$/d" "$PROMPT_FILE" \
         | sed -e "s,__SLUG__,$SLUG,g" -e "s,__ROOT__,$ROOT,g")"

# 5. One source-scoped design call, retrying on empty/short output.
OUT="$WORK/${ROOT#/}(4).md"
attempt=0
while :; do
    attempt=$((attempt + 1))
    echo "Designing (nlm call, attempt $attempt)..." >&2
    if nlm generate-chat --source-match "$SCOPE" "$NB" "$PROMPT" \
            > "$WORK/.design.raw" 2> "$WORK/design.stderr"; then
        clean_citations < "$WORK/.design.raw" > "$OUT"
    else
        rc=$?
        echo "nlm generate-chat failed (exit $rc):" >&2
        sed 's/^/  /' "$WORK/design.stderr" >&2
        [ "$attempt" -le "$RETRIES" ] && { echo "retrying..." >&2; sleep 10; continue; }
        exit "$rc"
    fi

    bytes=$(wc -c < "$OUT" | tr -d ' ')
    if [ "$bytes" -lt "$MIN_BYTES" ]; then
        if grep -q '^Decision: BLOCKED' "$OUT"; then
            echo "Design BLOCKED by source audit:" >&2
            sed 's/^/  /' "$OUT" >&2
            exit 3
        fi
        echo "short output ($bytes < $MIN_BYTES bytes)" >&2
        [ "$attempt" -le "$RETRIES" ] && { echo "retrying..." >&2; sleep 10; continue; }
        echo "giving up after $attempt attempts; see $WORK/design.stderr" >&2
        exit 1
    fi
    break
done

# 6. Optional self-critique / repair pass.
if [ "$VERIFY" -eq 1 ]; then
    if [ ! -f "$VERIFY_FILE" ]; then
        echo "warning: --verify requested but $VERIFY_FILE missing; skipping" >&2
    else
        echo "Verifying (second nlm call)..." >&2
        VPROMPT="$(sed -e "s,__SLUG__,$SLUG,g" -e "s,__ROOT__,$ROOT,g" "$VERIFY_FILE")"
        VPROMPT="$VPROMPT

--- DRAFT MANPAGE TO REVIEW ---
$(cat "$OUT")"
        if nlm generate-chat --source-match "$SCOPE" "$NB" "$VPROMPT" \
                > "$WORK/.verify.raw" 2> "$WORK/verify.stderr"; then
            if [ -s "$WORK/.verify.raw" ] && \
               [ "$(wc -c < "$WORK/.verify.raw" | tr -d ' ')" -ge "$MIN_BYTES" ]; then
                cp "$OUT" "$WORK/${ROOT#/}(4).pre-verify.md"
                clean_citations < "$WORK/.verify.raw" > "$OUT"
                echo "Verified manpage replaces draft (draft saved as .pre-verify.md)" >&2
            else
                echo "warning: verify pass returned too little; keeping draft" >&2
            fi
        else
            echo "warning: verify call failed; keeping draft" >&2
        fi
    fi
fi

echo "Wrote $OUT ($(wc -c < "$OUT" | tr -d ' ') bytes)" >&2
echo "$OUT"
