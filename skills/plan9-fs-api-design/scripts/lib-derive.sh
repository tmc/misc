#!/usr/bin/env bash
# lib-derive.sh - shared helpers for the deterministic probe renderers and
# the final verifier. Source it; do not execute it.
#
#   . "$SKILL_DIR/scripts/lib-derive.sh"
#
# All helpers are API-neutral: they parse the authoritative probe 02b tree
# and expose path queries against it. Keeping this in one file means a parser
# fix (or recovery) happens in exactly one place.

# normalize_ids - rewrite the example session id and prompt id emitted by
# probe 02b (e.g. /llm/1, /llm/1/prompt/1) to the template tokens $id and $n,
# so path queries match structurally regardless of the concrete numbering the
# panel chose. Generic: the session dir is root's single non-clone direct
# child; the prompt dir is the single non-clone direct child of
# <root>/<sid>/prompt. Reads literal paths on stdin, writes normalized paths.
normalize_ids() {
    local paths root sid pid
    paths=$(cat)
    root=$(printf '%s\n' "$paths" | awk 'NR == 1 { print; exit }')
    [ -n "$root" ] || { printf '%s\n' "$paths"; return; }
    sid=$(printf '%s\n' "$paths" | sed -n "s#^$root/\([^/]*\)\$#\1#p" | grep -vx clone | head -1)
    if [ -z "$sid" ]; then printf '%s\n' "$paths"; return; fi
    pid=$(printf '%s\n' "$paths" | sed -n "s#^$root/$sid/prompt/\([^/]*\)\$#\1#p" | grep -vx clone | head -1)
    if [ -n "$pid" ]; then
        printf '%s\n' "$paths" | sed -E \
            -e "s#^$root/$sid/prompt/$pid(/|\$)#$root/\$id/prompt/\$n\1#" \
            -e "s#^$root/$sid(/|\$)#$root/\$id\1#"
    else
        printf '%s\n' "$paths" | sed -E \
            -e "s#^$root/$sid(/|\$)#$root/\$id\1#"
    fi
}

# tree_paths_raw - parse the first fenced tree under a "2. Filesystem tree"
# heading in the named file and emit absolute paths with literal example ids.
# Args: <markdown-file>
tree_paths_raw() {
    local f="$1"
    [ -f "$f" ] || return 0
    awk '
        /^#+ 2\. Filesystem tree/ { in2=1; next }
        in2 && /^```/ { fence = !fence; next }
        in2 && fence { print }
        in2 && !fence && /^#+ / { in2=0 }
    ' "$f" |
    sed \
        -e 's/├──/+--/g' \
        -e 's/└──/+--/g' \
        -e 's/|--/+--/g' \
        -e 's/`--/+--/g' \
        -e 's/\\--/+--/g' \
        -e 's/│/|/g' \
        -e 's/←/<-/g' |
    awk '
        {
            line=$0
            sub(/[[:space:]]*<-.*/, "", line)
        }
        line ~ /^\/[^[:space:]]+/ {
            root=$1
            sub(/\/$/, "", root)
            stack[0]=root
            print root
            next
        }
        {
            pos=index(line, "+--")
            if (pos == 0 || root == "")
                next
            prefix=substr(line, 1, pos-1)
            depth=int(length(prefix)/4)+1
            rest=substr(line, pos+3)
            sub(/^[[:space:]]+/, "", rest)
            name=rest
            sub(/[[:space:]].*/, "", name)
            sub(/\/$/, "", name)
            if (name == "")
                next
            parent=stack[depth-1]
            if (parent == "")
                next
            path=parent "/" name
            print path
            stack[depth]=path
            for (i=depth+1; i<32; i++)
                delete stack[i]
        }
    ' |
    sort -u
}

# tree_paths - parse the tree in the named file and normalize example ids to
# $id / $n. Args: <markdown-file>
tree_paths() {
    tree_paths_raw "$1" | normalize_ids
}

# service_root - the root path of the tree (e.g. /llm). Args: <markdown-file>
service_root() {
    tree_paths_raw "$1" | awk 'NR == 1 { print; exit }'
}

# Path queries below operate on a paths list supplied via the PATHS variable,
# set once per renderer with:  PATHS=$(tree_paths "$tree")
has_path() {
    printf '%s\n' "$PATHS" | grep -Fxq "$1"
}

has_glob() {
    local pat="$1" p
    printf '%s\n' "$PATHS" | while IFS= read -r p; do
        case "$p" in
            $pat) echo yes; return 0 ;;
        esac
    done | grep -q yes
}

first_child() {
    # args: <parent-path> <fallback> - first leaf child name of parent that is
    # not ctl/clone/$n.
    local parent="$1" fallback="$2"
    printf '%s\n' "$PATHS" |
    awk -v p="$parent/" -v fallback="$fallback" '
        index($0, p) == 1 {
            rest=substr($0, length(p)+1)
            if (rest !~ /\// && rest != "ctl" && rest != "clone" && rest != "$n") {
                print rest
                found=1
                exit
            }
        }
        END { if (!found) print fallback }
    '
}

# api_name - best-effort human name for the target API, from the source audit,
# then the source brief, then the run input. Args: <work-dir>
api_name() {
    local work="$1" api=""
    api=$(
        sed -n 's/^The target is the \([^,(]*\).*/\1/p' "$work/probes/00-source-audit.md" 2>/dev/null |
        head -1 | sed 's/^[[:space:]]*//; s/[[:space:]]*$//'
    )
    if [ -z "$api" ] && [ -f "$work/source-brief.md" ]; then
        api=$(sed -n 's/^- canonical API name:[[:space:]]*//p' "$work/source-brief.md" |
            head -1 | sed 's/^[[:space:]]*//; s/[[:space:]]*$//')
    fi
    if [ -z "$api" ]; then
        api=$(sed -n '1p' "$work/input.txt" 2>/dev/null |
            sed -E 's/.* of the ([^;]+).*/\1/; s/,.*//; s/;.*//; s/^[[:space:]]*//; s/[[:space:]]*$//')
    fi
    [ -n "$api" ] || api="target API"
    printf '%s\n' "$api"
}
