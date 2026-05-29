#!/usr/bin/env bash
# sync.sh — resolve a NotebookLM notebook, sync source files into it, and
# classify the API shape in an isolated grounded call. Run this ONCE before the
# author/validate loop. Prints, on stdout:
#   notebook: <id>
#   shape: ... / durable-handle: ... / reason: ...   (the classify contract)
# so the caller can set PLAN9_FS_DESIGN_NOTEBOOK and inject the shape into the
# author prompt.
#
# Usage:
#   sync.sh <slug> <source-dir>
#
# Environment:
#   PLAN9_FS_DESIGN_SKILL_DIR   skill install path (overrides script-relative).
#   PLAN9_FS_DESIGN_HOME        work-tree root (default $HOME/.plan9-fs-designs).
#   PLAN9_FS_DESIGN_NOTEBOOK    reuse this notebook id (else cache, else create).
#   PLAN9_FS_DESIGN_SHAPE       skip the classify call; force this shape.

set -euo pipefail

if [ "$#" -lt 2 ]; then
    echo "usage: sync.sh <slug> <source-dir>" >&2
    exit 2
fi

SLUG="$1"; SRC_DIR="$2"
case "$SLUG" in
    ''|*[!a-z0-9-]*|-*|*-)
        echo "invalid slug '$SLUG': lowercase kebab-case" >&2; exit 2 ;;
esac
[ -d "$SRC_DIR" ] || { echo "source dir not found: $SRC_DIR" >&2; exit 2; }

SKILL_DIR="${PLAN9_FS_DESIGN_SKILL_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
CLASSIFY_FILE="$SKILL_DIR/prompts/classify.md"
command -v nlm >/dev/null 2>&1 || { echo "nlm not found on PATH" >&2; exit 127; }

HOME_DIR="${PLAN9_FS_DESIGN_HOME:-$HOME/.plan9-fs-designs}"
WORK="$HOME_DIR/$SLUG"
mkdir -p "$WORK"
CACHE="$HOME_DIR/.notebook-id"
SCOPE="^$SLUG:|^plan9-|^wanix:"

resolve_notebook() {
    if [ -n "${PLAN9_FS_DESIGN_NOTEBOOK:-}" ]; then echo "$PLAN9_FS_DESIGN_NOTEBOOK"; return; fi
    if [ -f "$CACHE" ]; then head -1 "$CACHE"; return; fi
    echo "Creating notebook plan9-fs-design..." >&2
    local log="$WORK/.create.out"
    if ! nlm notebook create 'plan9-fs-design' >"$log" 2>&1; then
        echo "nlm notebook create failed:" >&2; sed 's/^/  /' "$log" >&2; return 1
    fi
    grep -oE '[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}' "$log" | head -1
}

NB="$(resolve_notebook)"
[ -n "$NB" ] || { echo "could not resolve a notebook id" >&2; exit 1; }
printf '%s\n' "$NB" > "$CACHE.tmp" && mv "$CACHE.tmp" "$CACHE"
echo "Notebook: $NB" >&2

if [ -n "$(ls -A "$SRC_DIR" 2>/dev/null)" ]; then
    echo "Syncing sources from $SRC_DIR ..." >&2
    nlm source sync "$NB" "$SRC_DIR" --name "$SLUG: sources" >&2
    sleep 5
else
    echo "warning: no sources in $SRC_DIR" >&2
fi

# Classify the shape in isolation (or honor operator override).
SHAPE_FILE="$WORK/.shape-block"
UNKNOWN="shape: unknown — classify from the sources yourself (durable handle? if no, do NOT use clone/\$n)"
if [ -n "${PLAN9_FS_DESIGN_SHAPE:-}" ]; then
    printf 'shape: %s (operator-specified)\n' "$PLAN9_FS_DESIGN_SHAPE" > "$SHAPE_FILE"
elif [ -f "$CLASSIFY_FILE" ]; then
    echo "Classifying API shape (nlm call)..." >&2
    CPROMPT="$(sed -e "s,__SLUG__,$SLUG,g" "$CLASSIFY_FILE")"
    if nlm generate-chat --source-match "$SCOPE" "$NB" "$CPROMPT" \
            > "$WORK/.classify.raw" 2> "$WORK/classify.stderr"; then
        grep -iE '^(shape|durable-handle|reason):' "$WORK/.classify.raw" | head -3 > "$SHAPE_FILE"
    fi
    [ -s "$SHAPE_FILE" ] || printf '%s\n' "$UNKNOWN" > "$SHAPE_FILE"
else
    printf '%s\n' "$UNKNOWN" > "$SHAPE_FILE"
fi

printf 'notebook: %s\n' "$NB"
cat "$SHAPE_FILE"
