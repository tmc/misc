#!/usr/bin/env bash
# verify-output.sh — generic final gates for one plan9-fs-api-design run.
#
# Usage:
#   verify-output.sh <slug> [document]
#
# Checks are intentionally API-neutral. The final document is checked
# against the schema and, when present, explicit paths from probe 02b.

set -euo pipefail

if [ "$#" -lt 1 ]; then
    echo "usage: verify-output.sh <slug> [document]" >&2
    exit 2
fi

SLUG="$1"
PLAN9_FS_HOME="${PLAN9_FS_HOME:-$HOME/.plan9-fs-designs}"
WORK="$PLAN9_FS_HOME/$SLUG"

[ -d "$WORK" ] || { echo "no work dir for $SLUG" >&2; exit 1; }

DOC="${2:-}"
if [ -z "$DOC" ]; then
    DOC="$WORK/out/${SLUG}(4).md"
fi
[ -f "$DOC" ] || DOC="$WORK/probes/09-synthesize-document.md"
[ -f "$DOC" ] || { echo "no final document for $SLUG" >&2; exit 1; }

fail=0

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib-derive.sh
. "$HERE/lib-derive.sh"

contains() {
    # args: <literal> <file>
    grep -Fq "$1" "$2"
}

# tree_paths_from - literal-id absolute paths from a markdown tree, kept as a
# thin alias over the shared parser for the final-document comparison.
tree_paths_from() {
    tree_paths_raw "$1"
}

# contract_paths - the authoritative 02b tree with example ids normalized to
# the $id/$n template form used by the prose sections of the final document.
contract_paths() {
    tree_paths "$WORK/probes/02b-refine-decomposition.md"
}

echo "document: $DOC"

if grep -Eq '^# .+\([0-9]+\)' "$DOC"; then
    echo "ok heading: H1 manpage"
else
    echo "missing H1 manpage heading" >&2
    fail=1
fi

for h in \
    '## TL;DR' \
    '## Key Findings' \
    '## Details' \
    '### 1. Philosophy and design approach' \
    '### 2. Filesystem tree' \
    '### 3. Per-file / per-directory semantics' \
    '### 4. Feature-by-feature mapping table' \
    '### 5. Worked end-to-end examples' \
    '### 6. Notes on deviations from pure Plan 9 style' \
    '## Recommendations' \
    '## Caveats'
do
    if contains "$h" "$DOC"; then
        echo "ok heading: $h"
    else
        echo "missing heading: $h" >&2
        fail=1
    fi
done

if grep -Eq 'Supported[[:space:]]*\|[[:space:]]*Fails closed' "$DOC"; then
    echo "ok fails-closed table"
else
    echo "missing Supported | Fails closed table" >&2
    fail=1
fi

if [ -f "$WORK/probes/00-source-audit.md" ] &&
   grep -q '^Decision: BLOCKED' "$WORK/probes/00-source-audit.md"; then
    echo "source audit is BLOCKED" >&2
    fail=1
fi

# Contract-path gate. Every path in the authoritative 02b tree (with example
# ids normalized to $id/$n) must appear in the final document — either in its
# tree block or, for per-file leaf paths, as a section-3 semantics header. This
# catches a section 3 that silently dropped its $id/$n entries even when the
# tree block is intact.
paths=$(contract_paths || true)
final_tree=$(tree_paths "$DOC" || true)
# Section-3 headers in the doc, e.g. "#### `/llm/$id/ctl` - ...".
section3_paths=$(grep -Eo '^#### `[^`]+`' "$DOC" | sed -E 's/^#### `//; s/`$//' || true)
if [ -n "$paths" ]; then
    while IFS= read -r p; do
        [ -n "$p" ] || continue
        if printf '%s\n' "$final_tree" | grep -Fxq "$p" ||
           printf '%s\n' "$section3_paths" | grep -Fxq "$p" ||
           contains "$p" "$DOC"; then
            echo "ok contract path: $p"
        else
            echo "missing contract path from probe 02b: $p" >&2
            fail=1
        fi
    done <<EOF
$paths
EOF
else
    echo "warning: no explicit probe 02b paths to verify" >&2
fi

# Section-3 leaf-coverage gate. Each per-session ($id/$n) leaf file in the
# contract must have a section-3 semantics header. Section 3 legitimately
# templates dynamic child names to $NAME and groups sibling files as {a,b,c},
# so a leaf counts as covered when any section-3 header, after the same
# normalization, matches its directory family and basename. This turns the
# "section 3 skeleton only" regression (all $id files silently dropped) into a
# hard failure without flagging the templating the renderer intends.
fold() {
    # Put contract leaves and section-3 headers on the same footing:
    #  1. expand {a,b,c} grouped headers into one path per member;
    #  2. collapse dynamic child names under tools/ and in/ (the concrete
    #     getWeather, img0, aud0 and the templated $NAME / NAME alike) to a
    #     single <DYN> sentinel so a templated header covers concrete leaves.
    awk '
        {
            if (match($0, /\{[^}]+\}/)) {
                pre=substr($0, 1, RSTART-1)
                grp=substr($0, RSTART+1, RLENGTH-2)
                post=substr($0, RSTART+RLENGTH)
                nfld=split(grp, parts, ",")
                for (i=1;i<=nfld;i++) { g=parts[i]; gsub(/^[[:space:]]+|[[:space:]]+$/,"",g); print pre g post }
            } else print
        }
    ' |
    sed -E \
        -e 's#(/tools/)[^/]+(/|$)#\1<DYN>\2#' \
        -e 's#(/in/)[^/]+$#\1<DYN>#'
}

split_path_cells() {
    # Split Section 4 "file(s)" cells on commas that are not inside brace
    # groups. Emits only filesystem-looking entries; N/A and fails-closed
    # cells are deliberately ignored.
    awk '
        function emit(s) {
            gsub(/^[[:space:]]+|[[:space:]]+$/, "", s)
            if (s ~ /^\//)
                print s
        }
        {
            gsub(/`/, "", $0)
            part=""
            depth=0
            for (i=1; i<=length($0); i++) {
                c=substr($0, i, 1)
                if (c == "{")
                    depth++
                else if (c == "}" && depth > 0)
                    depth--
                if (c == "," && depth == 0) {
                    emit(part)
                    part=""
                    next
                }
                part=part c
            }
            emit(part)
        }
    '
}
covered=$(printf '%s\n' "$section3_paths" | fold | sort -u)
leafs=$(printf '%s\n' "$paths" | grep -E '/\$(id|n)/' || true)
if [ -n "$leafs" ]; then
    while IFS= read -r p; do
        [ -n "$p" ] || continue
        # only check files that are leaves (no child path under them)
        printf '%s\n' "$paths" | grep -q "^$p/" && continue
        pf=$(printf '%s\n' "$p" | fold)
        if printf '%s\n' "$covered" | grep -Fxq "$pf" || contains "$p" "$DOC"; then
            : # covered by a direct, templated, or grouped section-3 header
        else
            echo "section 3 missing semantics for contract leaf: $p" >&2
            fail=1
        fi
    done <<EOF
$leafs
EOF
fi

# Section-4 table sanity. The final mapping table is generated from probe 01,
# so it can drift from the accepted section-3 semantics if a surface key is
# mis-normalized. Keep this small and API-neutral: accounting-like rows must
# point at accounting paths when those paths exist, and generated boilerplate
# should stay rare enough to deserve human attention.
section4_rows=$(awk '
    /^### 4\./ { in4=1; next }
    /^### / && in4 { in4=0 }
    in4 && /^\|/ && $0 !~ /API surface/ && $0 !~ /^\|[[:space:]:-]+\|/ { print }
' "$DOC" || true)

if [ -n "$section4_rows" ]; then
    row_count=$(printf '%s\n' "$section4_rows" | grep -c '^|' || true)
    generic_count=$(printf '%s\n' "$section4_rows" |
        grep -Ec 'Tagged state, scalar limit, usage, or parameter value\.|Current configured value when readable; otherwise N/A\.' || true)
    if [ "${row_count:-0}" -gt 0 ] &&
       [ $((generic_count * 4)) -gt "$row_count" ]; then
        echo "warning: section 4 has $generic_count generic cells across $row_count rows" >&2
    fi

    bad_accounting=$(printf '%s\n' "$section4_rows" | awk -F'|' '
        {
            label=tolower($2)
            path=tolower($3)
            if (label ~ /measure(input|context)?usage/) {
                if (path !~ /\/ctx\/measure/) print
                next
            }
            if (label ~ /(inputusage|contextusage)/) {
                if (path !~ /\/ctx\/usage/) print
                next
            }
            if (label ~ /(inputquota|contextwindow)/ && path !~ /\/ctx\/window/) print
        }
    ' || true)
    if [ -n "$bad_accounting" ]; then
        echo "section 4 accounting rows contradict section 3 accounting paths:" >&2
        printf '%s\n' "$bad_accounting" >&2
        fail=1
    fi

    contract_folded=$(printf '%s\n' "$paths" | fold | fold | fold | sort -u)
    section4_missing_paths=$(
        printf '%s\n' "$section4_rows" |
        awk -F'|' '{ print $3 }' |
        split_path_cells |
        fold | fold | fold |
        sort -u |
        while IFS= read -r p; do
            [ -n "$p" ] || continue
            [ "${p#/}" != "$p" ] || continue
            if printf '%s\n' "$contract_folded" | grep -Fxq "$p"; then
                continue
            fi
            printf '%s\n' "$p"
        done
    )
    if [ -n "$section4_missing_paths" ]; then
        echo "section 4 maps API surfaces to paths absent from the accepted tree:" >&2
        printf '%s\n' "$section4_missing_paths" >&2
        fail=1
    fi
fi

if [ "$fail" -ne 0 ]; then
    echo "verify-output: FAIL" >&2
    exit 1
fi

echo "verify-output: PASS"
