#!/usr/bin/env bash
# bundle.sh — assemble the final txtar artifact for one design run.
# The synthesized document (probe 09 output) is the top entry; probe
# outputs follow as the evidence trail.
#
# Usage:
#   bundle.sh <slug>
#
# Output: $WORK/<slug>.txtar (also printed to stdout if --stdout is given)

set -euo pipefail

if [ "$#" -lt 1 ]; then
    echo "usage: bundle.sh <slug>" >&2
    exit 2
fi

SLUG="$1"
PLAN9_FS_HOME="${PLAN9_FS_HOME:-$HOME/.plan9-fs-designs}"
WORK="$PLAN9_FS_HOME/$SLUG"

[ -d "$WORK" ] || { echo "no work dir for $SLUG" >&2; exit 1; }
[ -f "$WORK/probes/09-synthesize-document.md" ] || {
    echo "no synthesized document; run probe 09 first" >&2
    exit 1
}

# Copy the synthesized document to a properly-named file
mkdir -p "$WORK/out"
cp "$WORK/probes/09-synthesize-document.md" "$WORK/out/${SLUG}(4).md"

bundle="$WORK/${SLUG}.txtar"
{
    echo "-- ${SLUG}(4).md --"
    cat "$WORK/out/${SLUG}(4).md"
    echo ""
    echo "-- input.txt --"
    cat "$WORK/input.txt"
    echo ""
    echo "-- input-shape.txt --"
    cat "$WORK/input-shape.txt"
    echo ""
    for f in source-brief.md; do
        [ -f "$WORK/$f" ] || continue
        echo "-- $f --"
        cat "$WORK/$f"
        echo ""
    done
    if [ -f "$WORK/sources/MANIFEST.md" ]; then
        echo "-- sources/MANIFEST.md --"
        cat "$WORK/sources/MANIFEST.md"
        echo ""
    fi
    for p in "$WORK"/probes/[0-9]*.md; do
        name=$(basename "$p")
        echo "-- probes/$name --"
        cat "$p"
        echo ""
    done
} > "$bundle"

echo "bundle: $bundle" >&2
echo "$bundle"
