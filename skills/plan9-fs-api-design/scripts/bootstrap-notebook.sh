#!/usr/bin/env bash
# bootstrap-notebook.sh — create the plan9-fs-api-design notebook and seed
# it with the canonical Plan 9 corpus + Wanix source. Idempotent: if a
# notebook id already exists in the cache or user auto-memory, reuse it.
#
# Usage:
#   bootstrap-notebook.sh                # auto-resolve or create
#   bootstrap-notebook.sh --force        # always create a new notebook
#   bootstrap-notebook.sh --print-id     # print the resolved id and exit
#
# Output: prints the resolved notebook id on stdout. Caches it at
# $PLAN9_FS_HOME/.notebook-id and (after success) hints to save it to
# user auto-memory.

set -euo pipefail

PLAN9_FS_HOME="${PLAN9_FS_HOME:-$HOME/.plan9-fs-designs}"
CACHE="$PLAN9_FS_HOME/.notebook-id"
CODEX_HOME="${CODEX_HOME:-$HOME/.codex}"
MEM="$CODEX_HOME/memory/reference_plan9_fs_api_design_notebook.md"
SKILL_DIR="$(cd "$(dirname "$0")/.." && pwd)"

force=0
print_only=0
for arg in "$@"; do
    case "$arg" in
        --force) force=1 ;;
        --print-id) print_only=1 ;;
        *) echo "unknown arg: $arg" >&2; exit 2 ;;
    esac
done

resolve_existing() {
    if [ -f "$MEM" ]; then
        grep -oE 'notebook_id=[A-Za-z0-9_-]+' "$MEM" 2>/dev/null \
            | head -1 | sed 's/notebook_id=//'
    elif [ -f "$CACHE" ]; then
        cat "$CACHE"
    fi
}

if [ "$force" -eq 0 ]; then
    nb=$(resolve_existing || true)
    if [ -n "${nb:-}" ]; then
        if [ "$print_only" -eq 1 ]; then
            echo "$nb"
            exit 0
        fi
        echo "Reusing existing notebook: $nb" >&2
        echo "$nb"
        exit 0
    fi
fi

[ "$print_only" -eq 1 ] && { echo "no cached notebook id" >&2; exit 1; }

mkdir -p "$PLAN9_FS_HOME"

echo "Creating notebook..." >&2
NB=$(nlm notebook create 'plan9-fs-api-design' 2>&1 \
     | grep -oE '[0-9a-f-]{36}' | head -1)

if [ -z "$NB" ]; then
    echo "failed to parse notebook id from nlm notebook create output" >&2
    exit 1
fi

echo "Created notebook: $NB" >&2
echo "$NB" > "$CACHE"

PLAN9_ROOT="${PLAN9_ROOT:-}"

mirror_url_to_md() {
    # args: <url> <out.md>
    local url="$1" out="$2" tmp="$2.tmp"
    if ! command -v html2md >/dev/null 2>&1; then
        echo "warning: html2md not found; cannot mirror $url" >&2
        return 1
    fi
    rm -f "$tmp"
    if curl -fsSL "$url" | html2md > "$tmp" && [ -s "$tmp" ]; then
        mv "$tmp" "$out"
        return 0
    fi
    rm -f "$tmp"
    return 1
}

sync_catv_plan9_fallback() {
    local dir="$PLAN9_FS_HOME/plan9-cat-v-mirror"
    local ok=0
    mkdir -p "$dir"
    while read -r name url; do
        [ -n "$name" ] || continue
        if mirror_url_to_md "$url" "$dir/$name.md"; then
            ok=$((ok + 1))
        else
            echo "warning: failed to mirror $url" >&2
        fi
    done <<'EOF'
paper-9 https://doc.cat-v.org/plan_9/4th_edition/papers/9
paper-names https://doc.cat-v.org/plan_9/4th_edition/papers/names
paper-plumb https://doc.cat-v.org/plan_9/4th_edition/papers/plumb
paper-net https://doc.cat-v.org/plan_9/4th_edition/papers/net
paper-acme https://doc.cat-v.org/plan_9/4th_edition/papers/acme/
man4-intro https://man.cat-v.org/plan_9/4/0intro
man4-srv https://man.cat-v.org/plan_9/4/srv
man4-webfs https://man.cat-v.org/plan_9/4/webfs
man4-acme https://man.cat-v.org/plan_9/4/acme
EOF
    if [ "$ok" -gt 0 ]; then
        nlm source sync "$NB" "$dir" \
            --name 'plan9-grounding: cat-v papers + man4 mirror' \
            || echo "warning: plan9 cat-v mirror sync failed" >&2
    else
        echo "warning: no cat-v Plan 9 sources mirrored" >&2
    fi
}

echo "Seeding Plan 9 papers..." >&2
if [ -n "$PLAN9_ROOT" ] && [ -d "$PLAN9_ROOT/sys/doc" ]; then
    # Foundational papers in troff .ms form. nlm source sync bundles
    # them into one txtar source for indexing efficiency.
    nlm source sync "$NB" \
        "$PLAN9_ROOT/sys/doc/9.ms" \
        "$PLAN9_ROOT/sys/doc/names.ms" \
        "$PLAN9_ROOT/sys/doc/plumb.ms" \
        "$PLAN9_ROOT/sys/doc/auth.ms" \
        "$PLAN9_ROOT/sys/doc/comp.ms" \
        "$PLAN9_ROOT/sys/doc/net/net.ms" \
        "$PLAN9_ROOT/sys/doc/acme/acme.ms" \
        --name 'plan9-papers: 9 names plumb auth comp net acme (.ms)' \
        || echo "warning: plan9-papers sync failed" >&2
else
    if [ -n "$PLAN9_ROOT" ]; then
        echo "warning: $PLAN9_ROOT/sys/doc not found; falling back to cat-v local mirror" >&2
    else
        echo "warning: PLAN9_ROOT not set; falling back to cat-v local mirror" >&2
    fi
    sync_catv_plan9_fallback
fi
sleep 15

echo "Seeding Plan 9 manual section 4 (file servers — load-bearing)..." >&2
if [ -n "$PLAN9_ROOT" ] && [ -d "$PLAN9_ROOT/sys/man/4" ]; then
    # All of section 4 is file servers — exactly our design corpus.
    nlm source sync "$NB" "$PLAN9_ROOT/sys/man/4" \
        --name 'plan9-man4: file servers (all)' \
        || echo "warning: plan9-man4 sync failed" >&2
fi
sleep 10

echo "Seeding Plan 9 manual sections 1-3 (shell + syscalls + libs)..." >&2
if [ -n "$PLAN9_ROOT" ] && [ -d "$PLAN9_ROOT/sys/man" ]; then
    # Section 1: bind(1), rc(1), 9p(1)
    # Section 2: intro(2), 9p(2), namespace(2)
    # Section 3: proc(3), srv(3), draw(3)
    nlm source sync "$NB" \
        "$PLAN9_ROOT/sys/man/1/bind" \
        "$PLAN9_ROOT/sys/man/1/rc" \
        "$PLAN9_ROOT/sys/man/2/0intro" \
        "$PLAN9_ROOT/sys/man/2/9p" \
        "$PLAN9_ROOT/sys/man/3/0intro" \
        "$PLAN9_ROOT/sys/man/3/proc" \
        "$PLAN9_ROOT/sys/man/3/srv" \
        "$PLAN9_ROOT/sys/man/3/draw" \
        --name 'plan9-man1-3: bind rc intro 9p proc srv draw' \
        || echo "warning: plan9-man1-3 sync failed" >&2
fi
sleep 10

echo "Seeding Plan 9 reference implementations (acme/plumb/webfs/srv)..." >&2
if [ -n "$PLAN9_ROOT" ] && [ -d "$PLAN9_ROOT/sys/src/cmd" ]; then
    # The actual code that defines the patterns the chat-instructions
    # rest on. Bundled as one source per service.
    for svc in acme plumb webfs aux/srv; do
        if [ -d "$PLAN9_ROOT/sys/src/cmd/$svc" ]; then
            nlm source sync "$NB" "$PLAN9_ROOT/sys/src/cmd/$svc" \
                --name "plan9-src: cmd/$svc" \
                || echo "warning: plan9-src $svc sync failed" >&2
            sleep 5
        fi
    done
fi
sleep 10

echo "Seeding Wanix source..." >&2
WANIX_ROOT="${WANIX_ROOT:-}"
if [ -n "$WANIX_ROOT" ] && [ -d "$WANIX_ROOT" ]; then
    nlm source sync "$NB" \
        "$WANIX_ROOT/fs" "$WANIX_ROOT/api" "$WANIX_ROOT/examples" \
        --name 'wanix: fs/api/examples' \
        || echo "warning: wanix source sync failed" >&2
else
    if [ -n "$WANIX_ROOT" ]; then
        echo "warning: $WANIX_ROOT not found; skipping Wanix source seed" >&2
    else
        echo "warning: WANIX_ROOT not set; skipping Wanix source seed" >&2
    fi
fi
sleep 15

echo "Setting chat instructions..." >&2
nlm chat instructions set "$NB" "$(cat "$SKILL_DIR/chat-instructions.md")"

echo "" >&2
echo "Notebook ready: $NB" >&2
echo "" >&2
echo "Save to user auto-memory by writing:" >&2
echo "  $MEM" >&2
echo "  containing the line: notebook_id=$NB" >&2
echo "" >&2

echo "$NB"
