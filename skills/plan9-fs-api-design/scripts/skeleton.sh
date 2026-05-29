#!/usr/bin/env bash
# skeleton.sh — optional pre-author step: if the sources contain a Web IDL
# definition, generate the deterministic Plan 9 service(4) skeleton with the
# webidl2plan9 tool. The skeleton fixes the ~70% the IDL fully settles (tree,
# per-file roles, ctl verbs, the feature-mapping table, fails-closed types) so
# the author cannot drift on it — the class of errors a grounded review keeps
# catching (invented verbs, wrong return types, mislocated surfaces). The
# author then fleshes the ~30% of judgment the IDL does not contain.
#
# Usage:
#   skeleton.sh <api-name> <service-root> [source-dir]
#
# Prints the skeleton manpage to stdout. Exits 0 with the skeleton, exit 1 if
# no Web IDL is found (the caller should fall back to authoring from scratch),
# or the tool's nonzero exit on a parse error.
#
# Environment:
#   PLAN9_FS_DESIGN_HOME       work-tree root (default $HOME/.plan9-fs-designs).
#   PLAN9_FS_DESIGN_WEBIDL_CMD invocation of the tool (default: go run the
#                              module). Set to an installed binary path to skip
#                              the Go toolchain, e.g. /usr/local/bin/webidl2plan9.

set -euo pipefail

if [ "$#" -lt 2 ]; then
    echo "usage: skeleton.sh <api-name> <service-root> [source-dir]" >&2
    exit 2
fi
NAME="$1"; ROOT="$2"
case "$ROOT" in /*) : ;; *) echo "service-root must start with '/': $ROOT" >&2; exit 2 ;; esac

SRC_DIR="${3:-}"
if [ -z "$SRC_DIR" ]; then
    echo "no source-dir given; cannot locate Web IDL" >&2
    exit 1
fi

# Find a Web IDL source: a .idl file, or a .bs/.html/.md carrying an
# <xmp class=idl> / <pre class=idl> block (Bikeshed specs).
IDL_FILE=""
while IFS= read -r f; do
    case "$f" in
        *.idl) IDL_FILE="$f"; break ;;
        *) if grep -qE '<(xmp|pre)[^>]*class="?idl"?' "$f" 2>/dev/null; then IDL_FILE="$f"; break; fi ;;
    esac
done < <(find "$SRC_DIR" -type f \( -name '*.idl' -o -name '*.bs' -o -name '*.html' -o -name '*.md' \) 2>/dev/null | sort)

if [ -z "$IDL_FILE" ]; then
    echo "no Web IDL found in $SRC_DIR (.idl or <xmp class=idl> block); author from scratch" >&2
    exit 1
fi
echo "Web IDL: $IDL_FILE" >&2

# Resolve the tool invocation: explicit override, then an installed binary on
# PATH. webidl2plan9 is its own Go module; install it once with
#   go install github.com/tmc/misc/webidl2plan9/cmd/webidl2plan9@latest
# (or `cd .../webidl2plan9 && go install ./cmd/webidl2plan9`). Set
# PLAN9_FS_DESIGN_WEBIDL_CMD to run it some other way.
if [ -n "${PLAN9_FS_DESIGN_WEBIDL_CMD:-}" ]; then
    # shellcheck disable=SC2086  # intentional word-splitting of the command
    set -- $PLAN9_FS_DESIGN_WEBIDL_CMD
elif command -v webidl2plan9 >/dev/null 2>&1; then
    set -- webidl2plan9
else
    echo "webidl2plan9 not on PATH; install it (go install github.com/tmc/misc/webidl2plan9/cmd/webidl2plan9@latest) or set PLAN9_FS_DESIGN_WEBIDL_CMD. Author from scratch for now." >&2
    exit 1
fi

"$@" -name "$NAME" -root "$ROOT" "$IDL_FILE"
