#!/usr/bin/env bash
# run-probes.sh — execute the numbered probe set against an anchored
# conversation. Idempotent: skips probes that already have non-empty
# output files. Sleeps between probes for indexing + rate-limit buffer.
#
# Usage:
#   run-probes.sh <slug> [probe-glob]
#
# probe-glob defaults to '0[0-9]*-*.md' (source audit plus probes 01-09,
# including lettered probes such as 05a/05b). Pass '10-*.md' to run only
# the meta-critique probe; pass '0[0-9]*-*.md 10-*.md' to include the
# meta-critique in the same run.
#
# Three failure modes the runner guards against:
#
# 1. Lost conversation context. NotebookLM conversations do not always
#    present earlier probe answers to later turns. Starting at
#    $PRIOR_CONTEXT_FROM, the runner therefore prepends a small,
#    probe-specific set of prior outputs. The chat conversation is still
#    used for continuity, but correctness no longer depends on hidden
#    chat memory.
#
# 2. Empty-output cascade. Probes occasionally return empty (transient
#    nlm errors, indexing lag). Empty mid-pipeline probes silently let
#    probe 09 synthesize against a regressed tree. The runner re-fires
#    on short output (< MIN_BYTES) up to MAX_RETRIES, then fails loud.
#
# 3. Cross-API contamination. NotebookLM notebooks can hold old sources.
#    Every probe is source-scoped by default to this run's `$SLUG: ...`
#    sources plus stable Plan 9/Wanix sources. Override SOURCE_MATCH only
#    when debugging source-list hygiene.

set -euo pipefail

if [ "$#" -lt 1 ]; then
    echo "usage: run-probes.sh <slug> [probe-glob]" >&2
    exit 2
fi

SLUG="$1"
GLOB="${2:-0[0-9]*-*.md}"

PLAN9_FS_HOME="${PLAN9_FS_HOME:-$HOME/.plan9-fs-designs}"
WORK="$PLAN9_FS_HOME/$SLUG"
SKILL_DIR="$(cd "$(dirname "$0")/.." && pwd)"

[ -d "$WORK" ] || { echo "no work dir for $SLUG; run anchor-mission.sh first" >&2; exit 1; }
[ -f "$WORK/conv-id.txt" ] || [ "${NO_CONVERSATION:-0}" = "1" ] || {
    echo "no conv-id.txt; run anchor-mission.sh first" >&2; exit 1; }

NB=$("$SKILL_DIR/scripts/bootstrap-notebook.sh" --print-id)
CONV=""
[ -f "$WORK/conv-id.txt" ] && CONV=$(cat "$WORK/conv-id.txt")

MIN_BYTES=400
MAX_RETRIES=3
RETRY_SLEEP_SECONDS="${PLAN9_FS_RETRY_SLEEP:-30}"
PROBE_SLEEP_SECONDS="${PLAN9_FS_PROBE_SLEEP:-10}"
PRIOR_CONTEXT_FROM="${PRIOR_CONTEXT_FROM:-01}"

SOURCE_MATCH="${SOURCE_MATCH:-^${SLUG}:|^plan9-|^wanix:}"
if [ -n "$SOURCE_MATCH" ] && [ "$SOURCE_MATCH" != "off" ]; then
    echo "scoping probes to --source-match '$SOURCE_MATCH'" >&2
fi

prior_context_names() {
    # args: <current-probe-name>
    # Keep this deliberately small. NotebookLM can return empty responses
    # when we stuff every prior probe into later turns.
    case "$1" in
        01-*) echo "" ;;
        02-*) echo "" ;;
        02b-*) echo "" ;;
        03-*) echo "02b-refine-decomposition" ;;
        04-*) echo "03-critique-adversarial" ;;
        05a-*) echo "02b-refine-decomposition" ;;
        05b-*) echo "01-survey-api" ;;
        06-*) echo "02b-refine-decomposition" ;;
        07-*) echo "02b-refine-decomposition 04-critique-constructive" ;;
        08-*) echo "04-critique-constructive" ;;
        09-*) echo "00-source-audit 02b-refine-decomposition 05a-flesh-out-section3 05b-feature-mapping 06-worked-examples 07-philosophy-and-deviations 08-recommendations-and-caveats" ;;
        *) echo "" ;;
    esac
}

prior_context_body() {
    # args: <path>
    # NotebookLM sometimes returns an empty response when prior generated
    # trees are embedded with box-drawing glyphs. Keep prior context plain
    # ASCII; generated outputs can still use richer formatting.
    sed \
        -e 's/├/+/g' \
        -e 's/└/+/g' \
        -e 's/│/|/g' \
        -e 's/←/<-/g' \
        "$1"
}

check_probe_verdict() {
    # args: <probe-name> <output-file>
    #
    # Probe 03 is the adversarial repair gate for the accepted tree. Later
    # probes are deterministic from that tree, so continuing after a blocking
    # verdict only turns a known-bad contract into polished prose.
    local name="$1" out="$2"
    case "$name" in
        03-critique-adversarial)
            if grep -q '^Blocking verdict: REPAIR BEFORE SEMANTICS' "$out"; then
                if [ "${ALLOW_TREE_REPAIR:-0}" = "1" ]; then
                    echo "warning: $name requests repair; continuing because ALLOW_TREE_REPAIR=1" >&2
                else
                    echo "BLOCKED by $name; inspect $out before continuing." >&2
                    exit 4
                fi
            fi
            ;;
    esac
}

check_prior_verdicts() {
    # args: <current-probe-name>
    # A targeted resume at 04-* or later must not skip past an already-known
    # bad tree. If probe 03 is absent, let the selected run proceed.
    local name="$1" f
    case "$name" in
        04-*|05*-*|06-*|07-*|08-*|09-*)
            f="$WORK/probes/03-critique-adversarial.md"
            [ -s "$f" ] && check_probe_verdict "03-critique-adversarial" "$f"
            ;;
    esac
}

run_one() {
    # args: <prompt-file> <out-file>
    #
    # Prompt assembly: optional anchor (from $WORK/anchor.txt) prepended
    # to the probe body, passed as positional arg. `nlm generate-chat`
    # has no stdin form for the prompt — only --source-ids accepts '-'
    # as a stdin sentinel. The prompt arg is strictly positional. macOS
    # ARG_MAX is ~256KB so even the largest probe (~6KB anchored) fits
    # by two orders of magnitude. Earlier "stdin form" lore was wrong:
    # the trailing '-' was being sent as a literal one-char prompt and
    # the model was responding from conversation context + chat
    # instructions only, which masked the bug.
    local P="$1" out="$2"
    local current
    current=$(basename "$P" .md)
    local ANCHOR=""
    if [ -f "$WORK/anchor.txt" ]; then
        ANCHOR="$(cat "$WORK/anchor.txt")"$'\n\n---\n\n'
    fi
    local PRIOR=""
    local prior_name f
    if [[ ! "$current" < "$PRIOR_CONTEXT_FROM" ]]; then
        for prior_name in $(prior_context_names "$current"); do
            f="$WORK/probes/$prior_name.md"
            [ -s "$f" ] || continue
            PRIOR+=$'\n\nPrior probe output '"$prior_name"$':\n\n'
            PRIOR+="$(prior_context_body "$f")"
        done
        if [ -n "$PRIOR" ]; then
            PRIOR=$'\n\nPrior context for this run follows. Use it as local context; if it conflicts with the task, the task wins.'"$PRIOR"
        fi
    fi
    local PROMPT="${ANCHOR}$(cat "$P")${PRIOR}"
    # stderr captured to .stderr sibling — post-run grep for
    # 'Generating response for:' proves the prompt reached the server.
    local args=(generate-chat)
    if [ "${NO_CONVERSATION:-0}" != "1" ] && [ -n "$CONV" ]; then
        args+=(--conversation "$CONV")
    fi
    if [ -n "$SOURCE_MATCH" ] && [ "$SOURCE_MATCH" != "off" ]; then
        args+=(--source-match "$SOURCE_MATCH")
    fi
    nlm "${args[@]}" "$NB" "$PROMPT" > "$out" 2> "$out.stderr"
}

shopt -s nullglob
for P in "$SKILL_DIR"/prompts/$GLOB; do
    name=$(basename "$P" .md)
    out="$WORK/probes/$name.md"
    check_prior_verdicts "$name"
    if [ -s "$out" ] && [ "$(wc -c < "$out")" -ge "$MIN_BYTES" ]; then
        echo "skip $name (already exists)" >&2
        if [ "$name" = "00-source-audit" ] && grep -q '^Decision: BLOCKED' "$out"; then
            echo "BLOCKED by existing source audit; inspect $out before continuing." >&2
            exit 3
        fi
        check_probe_verdict "$name" "$out"
        continue
    fi

    attempts=0
    until [ "$attempts" -ge "$MAX_RETRIES" ]; do
        attempts=$((attempts + 1))
        # Probes 03-09 are rendered deterministically from earlier verified
        # probe material by a single dispatcher. Each maps to a DERIVE_* gate
        # variable; set it to 0 to force the NotebookLM path for that probe.
        derive_gate=""
        case "$name" in
            03-critique-adversarial)        derive_gate="${DERIVE_03:-1}" ;;
            04-critique-constructive)       derive_gate="${DERIVE_04:-1}" ;;
            05a-flesh-out-section3)         derive_gate="${DERIVE_05A:-1}" ;;
            05b-feature-mapping)            derive_gate="${DERIVE_05B:-1}" ;;
            06-worked-examples)             derive_gate="${DERIVE_06:-1}" ;;
            07-philosophy-and-deviations)   derive_gate="${DERIVE_07:-1}" ;;
            08-recommendations-and-caveats) derive_gate="${DERIVE_08:-1}" ;;
            09-synthesize-document)         derive_gate="${DERIVE_09:-1}" ;;
        esac
        if [ -n "$derive_gate" ] && [ "$derive_gate" != "0" ]; then
            echo "running $name via deterministic renderer..." >&2
            if "$SKILL_DIR/scripts/derive.sh" "$SLUG" "$name" > "$out" 2> "$out.stderr"; then
                bytes=$(wc -c < "$out")
                echo "  $name produced $bytes bytes" >&2
                check_probe_verdict "$name" "$out"
                break
            fi
            echo "  $name deterministic renderer failed; falling back to NotebookLM" >&2
        fi

        echo "running $name (attempt $attempts)..." >&2
        if ! run_one "$P" "$out"; then
            echo "  $name nlm returned non-zero; treating as empty and retrying after 30s..." >&2
            mv "$out" "$out.nlmerr.$attempts" 2>/dev/null || true
            sleep "$RETRY_SLEEP_SECONDS"
            continue
        fi
        bytes=$(wc -c < "$out")

        if [ "$bytes" -lt "$MIN_BYTES" ]; then
            echo "  $name produced $bytes bytes (< $MIN_BYTES); waiting 30s and retrying..." >&2
            mv "$out" "$out.empty.$attempts"
            sleep "$RETRY_SLEEP_SECONDS"
            continue
        fi

        echo "  $name produced $bytes bytes" >&2
        if [ "$name" = "00-source-audit" ] && grep -q '^Decision: BLOCKED' "$out"; then
            echo "BLOCKED by source audit; inspect $out before continuing." >&2
            exit 3
        fi
        check_probe_verdict "$name" "$out"
        break
    done

    if [ ! -s "$out" ] || [ "$(wc -c < "$out")" -lt "$MIN_BYTES" ]; then
        echo "BLOCKED on $name: $MAX_RETRIES retries produced empty/short output." >&2
        echo "Inspect $WORK/probes/$name.empty.* and $WORK/probes/$name.nlmerr.*." >&2
        exit 2
    fi

    sleep "$PROBE_SLEEP_SECONDS"
done

echo "probes complete; outputs under $WORK/probes/" >&2
