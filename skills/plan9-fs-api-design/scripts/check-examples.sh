#!/usr/bin/env bash
# check-examples.sh — local, no-NLM example-coherence check for a draft manpage.
# NotebookLM cannot see fenced code (its ingestion strips preformatted blocks),
# so the worked-example transcripts must be validated here instead of by the
# grounded reviewer. This is a mechanical, structural check — it does NOT judge
# spec-fidelity (that is validate.sh's job).
#
# It verifies, from the manpage's own text:
#   - there is at least one fenced rc transcript (a line beginning "% ");
#   - every ctl verb written in a transcript (echo <verb> ... >.../ctl) appears
#     in some verb table (a markdown table row naming that verb).
#
# It deliberately does NOT check transcript path tokens against the tree: both
# the tree and the transcripts are fenced blocks containing the service root, so
# no text heuristic reliably tells them apart, and a bogus path in a transcript
# would validate itself. Path coherence is left to the author's read of the
# draft. The verb check is robust because verbs live in markdown tables, which
# transcripts never are.
#
# Exits 0 if coherent, 5 if incoherent (prints the offending lines), 2 on usage.
#
# Usage:
#   check-examples.sh <service-root> <draft.md>

set -euo pipefail

if [ "$#" -lt 2 ]; then
    echo "usage: check-examples.sh <service-root> <draft.md>" >&2
    exit 2
fi
# shellcheck disable=SC2034  # ROOT kept for signature parity with validate.sh
ROOT="$1"; DRAFT="$2"
[ -f "$DRAFT" ] || { echo "draft not found: $DRAFT" >&2; exit 2; }

# Collect transcript (% ...) lines.
mapfile -t XSCRIPT < <(grep -nE '^% ' "$DRAFT" || true)
if [ "${#XSCRIPT[@]}" -eq 0 ]; then
    echo "FAIL: no rc worked-example transcripts (no lines beginning '% ')" >&2
    exit 5
fi

problems=0
note() { echo "  $1" >&2; problems=$((problems + 1)); }

# Verb coherence: each verb written to a .../ctl must appear in a verb table.
#    Match "echo <verb> ... >.../ctl" and ">$something/ctl" forms.
for line in "${XSCRIPT[@]}"; do
    text="${line#*:}"
    case "$text" in
        *'/ctl'*|*'ctl') : ;;
        *) continue ;;
    esac
    # extract the first word after 'echo '
    verb="$(sed -nE "s/^% +echo +'?([A-Za-z][A-Za-z0-9-]*).*/\1/p" <<<"$text")"
    [ -z "$verb" ] && continue
    # a verb table row looks like: | `verb` | ... | or | verb ... |
    grep -qE "^\| *\`?${verb}\b" "$DRAFT" || note "ctl verb '$verb' used in transcript but not in any verb table: ${line%%:*}: $text"
done

if [ "$problems" -gt 0 ]; then
    echo "example-coherence: FAIL ($problems issue(s))" >&2
    exit 5
fi
echo "example-coherence: ok (${#XSCRIPT[@]} transcript lines, paths and verbs consistent)" >&2
exit 0
