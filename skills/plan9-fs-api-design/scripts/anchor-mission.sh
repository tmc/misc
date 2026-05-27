#!/usr/bin/env bash
# anchor-mission.sh — open a fresh chat conversation against the
# plan9-fs-api-design notebook with the mission anchor for one input API.
# Caches the conversation id under $WORK/conv-id.txt.
#
# Usage:
#   anchor-mission.sh <slug> <input-arg> [<input-shape>]
#
# Where <slug> is a short-kebab name, <input-arg> is the original CLI
# arg (URL/path/description), and <input-shape> defaults to 'unknown'.

set -euo pipefail

if [ "$#" -lt 2 ]; then
    echo "usage: anchor-mission.sh <slug> <input-arg> [<input-shape>]" >&2
    exit 2
fi

SLUG="$1"
INPUT="$2"
SHAPE="${3:-unknown}"

PLAN9_FS_HOME="${PLAN9_FS_HOME:-$HOME/.plan9-fs-designs}"
WORK="$PLAN9_FS_HOME/$SLUG"
SKILL_DIR="$(cd "$(dirname "$0")/.." && pwd)"

NB=$("$SKILL_DIR/scripts/bootstrap-notebook.sh" --print-id 2>/dev/null \
     || "$SKILL_DIR/scripts/bootstrap-notebook.sh")
NB=$(echo "$NB" | tail -1)

mkdir -p "$WORK"/{sources,probes,out}
echo "$INPUT" > "$WORK/input.txt"
echo "$SHAPE" > "$WORK/input-shape.txt"

{
    echo "Run anchor for $SLUG."
    echo
    echo "Input API: $INPUT"
    echo "Input shape: $SHAPE"
    echo
    echo "Use source-audit probe 00 as the run contract. If it conflicts"
    echo "with later probe memory, the source-audit facts and current"
    echo "primary sources win."
    if [ -f "$WORK/source-brief.md" ]; then
        echo "Source brief uploaded as a notebook source: $SLUG: source brief"
    fi
    if [ -f "$WORK/sources/MANIFEST.md" ]; then
        echo "Source manifest included in mirrored API sources."
    fi
} > "$WORK/anchor.txt"

# Re-set chat instructions every run — they can be overwritten by other
# notebooks or sessions, and the seed is the load-bearing lever.
echo "Re-setting chat instructions..." >&2
nlm chat instructions set "$NB" "$(cat "$SKILL_DIR/chat-instructions.md")"

PROMPT="$(cat "$WORK/anchor.txt")

Goal: produce a single manual-page-style design document per the bound
output schema (see your chat instructions), ready to land as
${SLUG}(4).md.

Confirm you have the chat instructions loaded by responding with:
(a) a one-line lens declaration,
(b) a one-line blind-spots statement,
(c) a one-paragraph restatement of the inviolable design rules in
    your own words.

I will then run the probes in numbered order (00-source-audit,
01-survey-api, 02-propose-tree, ...). Do not draft any section of the
document until the corresponding probe is sent."

nlm generate-chat "$NB" "$PROMPT" \
    2> "$WORK/anchor.stderr" > "$WORK/anchor.stdout"

CONV=$(grep -oE 'conv[a-zA-Z_-]*[: =]+[A-Za-z0-9_-]+' "$WORK/anchor.stderr" \
       | head -1 | sed -E 's/.*[: =]+//')

if [ -z "$CONV" ]; then
    echo "warning: could not extract conversation id from stderr" >&2
    echo "  inspect: $WORK/anchor.stderr" >&2
    echo "  and write the id manually to: $WORK/conv-id.txt" >&2
    exit 1
fi

echo "$CONV" > "$WORK/conv-id.txt"
echo "Conversation: $CONV" >&2
echo "Work dir: $WORK" >&2

# Sanity check the anchor response
if ! grep -qiE 'lens|panel|blind' "$WORK/anchor.stdout"; then
    echo "" >&2
    echo "WARNING: anchor response did not include a lens or panel declaration." >&2
    echo "The chat instructions may not have taken. Inspect:" >&2
    echo "  $WORK/anchor.stdout" >&2
    echo "and re-run if the response is generic." >&2
fi

echo "$CONV"
