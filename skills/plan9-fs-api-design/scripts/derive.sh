#!/usr/bin/env bash
# derive.sh - deterministic renderer for one plan9-fs-api-design probe.
#
# Usage:
#   derive.sh <slug> <probe>
#
# <probe> is the probe basename without extension, e.g. 05a-flesh-out-section3.
# Each renderer reads verified earlier-probe material from $WORK/probes and
# writes the probe body to stdout. Shared tree parsing lives in lib-derive.sh.

set -euo pipefail

if [ "$#" -lt 2 ]; then
    echo "usage: derive.sh <slug> <probe>" >&2
    exit 2
fi

SLUG="$1"
PROBE="$2"
PLAN9_FS_HOME="${PLAN9_FS_HOME:-$HOME/.plan9-fs-designs}"
WORK="$PLAN9_FS_HOME/$SLUG"

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib-derive.sh
. "$HERE/lib-derive.sh"

tree="$WORK/probes/02b-refine-decomposition.md"

need() {
    [ -f "$1" ] || { echo "missing required probe: $1" >&2; exit 1; }
}

require_tree() {
    need "$tree"
    PATHS=$(tree_paths "$tree")
    root=$(printf '%s\n' "$PATHS" | awk 'NR == 1 { print; exit }')
    [ -n "$root" ] || { echo "could not parse root from $tree" >&2; exit 1; }
}

# section - per-file semantics block used by render_05a.
section() {
    local path="$1" summary="$2" mode="$3" read_sem="$4" write_sem="$5" errors="$6" verbs="${7:-}"
    if [ "${TRACK_SECTIONS:-0}" = 1 ]; then
        mark_section_path "$path"
    fi
    echo "#### \`$path\` - $summary ($mode)"
    echo
    echo "Read: $read_sem"
    echo
    echo "Write: $write_sem"
    echo
    echo "Errors: $errors"
    if [ -n "$verbs" ]; then
        echo
        echo "| Verb | Effect | Failure |"
        echo "|---|---|---|"
        printf '%s\n' "$verbs"
    fi
    echo
}

expand_group_paths() {
    # Expand one brace group per pass. Callers that allow nested groups can
    # pipe through this twice.
    awk '
        {
            if (match($0, /\{[^}]+\}/)) {
                pre=substr($0, 1, RSTART-1)
                grp=substr($0, RSTART+1, RLENGTH-2)
                post=substr($0, RSTART+RLENGTH)
                nfld=split(grp, parts, ",")
                for (i=1; i<=nfld; i++) {
                    g=parts[i]
                    gsub(/^[[:space:]]+|[[:space:]]+$/, "", g)
                    print pre g post
                }
            } else print
        }
    '
}

template_path() {
    local p="$1"
    awk -v root="$root" '
        {
            p=$0
            sub("^" root "/\\$id/tools/[^/]+", root "/$id/tools/$NAME", p)
            sub("^" root "/\\$id/in/[^/]+$", root "/$id/in/NAME", p)
            print p
        }
    ' <<EOF
$p
EOF
}

mark_section_path() {
    local expanded p
    expanded=$(printf '%s\n' "$1" | expand_group_paths | expand_group_paths)
    while IFS= read -r p; do
        [ -n "$p" ] || continue
        p=$(template_path "$p")
        EMITTED_SECTIONS="${EMITTED_SECTIONS}${p}"$'\n'
    done <<EOF
$expanded
EOF
}

section_emitted() {
    local p
    p=$(template_path "$1")
    printf '%s\n' "$EMITTED_SECTIONS" | grep -Fxq "$p"
}

leaf_paths() {
    printf '%s\n' "$PATHS" | awk '
        NF {
            n++
            path[n]=$0
        }
        END {
            for (i=1; i<=n; i++) {
                leaf=1
                for (j=1; j<=n; j++) {
                    if (i != j && index(path[j], path[i] "/") == 1) {
                        leaf=0
                        break
                    }
                }
                if (leaf)
                    print path[i]
            }
        }
    '
}

generic_section() {
    local path="$1" base
    base="${path##*/}"
    case "$base" in
        ctl)
            section "$path" "control file" "0222" \
                "Rejected." \
                "Accepts verbs defined by the adjacent directory; unknown verbs fail closed." \
                "\`Ebadctl\` for unknown verbs; \`Estarted\` for late creation-time changes; \`Edestroyed\` after teardown."
            ;;
        clone)
            section "$path" "allocator" "0444" \
                "Each open allocates a new child directory; reading that open returns the relative id, then EOF." \
                "Rejected." \
                "\`Eperm\` on write; \`Ebusy\` when allocation is temporarily unavailable."
            ;;
        status)
            section "$path" "status" "0444" \
                "Returns the current state and optional diagnostic fields." \
                "Rejected." \
                "\`Eperm\` on write; \`Enodata\` before the state exists."
            ;;
        event)
            section "$path" "event stream" "0444" \
                "Blocks until an event is available, then returns one newline-delimited record." \
                "Rejected." \
                "\`Einterrupted\` when the read is canceled; \`Eperm\` on write."
            ;;
        call)
            section "$path" "call pipe" "0444" \
                "Blocks until a call record is available, then returns a call id plus payload." \
                "Rejected." \
                "\`Einterrupted\` when the read is canceled; \`Edestroyed\` after teardown."
            ;;
        return)
            section "$path" "return pipe" "0222" \
                "Rejected." \
                "Writes a call id plus result or error payload to resume the waiting operation." \
                "\`Enotfound\` for an unknown call id; \`Einval\` for malformed results; \`Eperm\` on read."
            ;;
        stream)
            section "$path" "incremental output" "0444" \
                "Blocks and returns incremental chunks until completion." \
                "Rejected." \
                "\`Enodata\` before output starts; \`Einterrupted\` when the read is canceled; \`Eperm\` on write."
            ;;
        data)
            section "$path" "complete payload" "0666" \
                "Returns the last complete payload associated with this file." \
                "Writes a complete payload when this file is an input surface; rejected when it is output-only." \
                "\`Einval\` for malformed payloads; \`Ebusy\` while an incompatible operation is active."
            ;;
        title|description|schema|annotations|exposed_to)
            section "$path" "${base//_/ }" "0666 before start, 0444 after start" \
                "Returns the staged metadata value." \
                "Stages the metadata value before the enclosing session or object starts." \
                "\`Estarted\` after start; \`Einval\` for malformed metadata."
            ;;
        *)
            section "$path" "${base//_/ }" "0666 before start, 0444 after start" \
                "Returns the staged value when readable." \
                "Stages the value before the enclosing session or operation starts." \
                "\`Estarted\` after start; \`Einval\` for malformed values; \`Eperm\` when the backing API exposes no write."
            ;;
    esac
}

emit_missing_sections() {
    local p tp
    while IFS= read -r p; do
        [ -n "$p" ] || continue
        tp=$(template_path "$p")
        section_emitted "$tp" && continue
        generic_section "$tp"
    done <<EOF
$(leaf_paths)
EOF
}

render_03() {
    require_tree
    local findings=""
    add_finding() { findings="${findings}- $1: $2 - $3"$'\n'; }

    has_path "$root/clone" || add_finding MISSING "$root/clone" "add the session allocator"
    has_path "$root/ctl" && add_finding VIOLATES "$root/ctl" "remove unbacked root control; keep service state under model/"
    has_path "$root/status" && add_finding VIOLATES "$root/status" "remove unbacked root status; keep service state under model/"

    local p
    for p in ctl data stream status event; do
        has_path "$root/\$id/$p" || add_finding MISSING "$root/\$id/$p" "preserve the direct session frame"
    done

    if has_path "$root/model"; then
        has_path "$root/model/availability" || add_finding MISSING "$root/model/availability" "surface service readiness"
        has_path "$root/model/params" || add_finding MISSING "$root/model/params" "surface service parameters"
        has_path "$root/model/event" || add_finding MISSING "$root/model/event" "use an event file for model progress"
    fi

    if has_path "$root/\$id/prompt"; then
        for p in clone '$n/ctl' '$n/body' '$n/data' '$n/stream' '$n/status'; do
            has_path "$root/\$id/prompt/$p" || add_finding MISSING "$root/\$id/prompt/$p" "complete the per-operation scope"
        done
    fi

    if has_path "$root/\$id/tools"; then
        has_glob "$root/\$id/tools/*/call" || add_finding MISSING "$root/\$id/tools/*/call" "add blocking tool-call pipe"
        has_glob "$root/\$id/tools/*/return" || add_finding MISSING "$root/\$id/tools/*/return" "add tool-result pipe"
    fi

    has_path "$root/\$id/in" && { has_path "$root/\$id/in/ctl" || add_finding MISSING "$root/\$id/in/ctl" "add typed-input allocator/control"; }

    echo "Lens: deterministic tree reviewer - checking namespace invariants before semantics."
    echo
    echo "Blind spots: this gate checks path shape only; source-level nuance remains in probes 00 and 01."
    echo
    echo "Checks performed: root clone allocator; no unbacked root ctl/status; direct session ctl/data/stream/status/event; model availability/params/event; prompt operation ctl/body/data/stream/status; tool call/return pipes; typed input control."
    echo
    echo "## Findings"
    echo
    if [ -n "$findings" ]; then
        printf '%s' "$findings"
        echo
        echo "Blocking verdict: REPAIR BEFORE SEMANTICS"
    else
        echo "- HONEST: $root/clone -> \$id/{ctl,data,stream,status,event} - direct session frame is present."
        echo
        echo "Blocking verdict: PASS"
    fi
}

render_04() {
    require_tree
    local gate="$WORK/probes/03-critique-adversarial.md"
    need "$gate"
    echo "Lens: deterministic Plan 9 repair planner - converting the tree gate into a semantics contract."
    echo
    echo "Blind spots: this repair plan is structural; detailed per-file behavior is left to probe 05a."
    echo
    echo "## Repair list"
    echo
    local ready
    if grep -q '^Blocking verdict: PASS' "$gate"; then
        echo "- No structural repairs. Use probe 02b as the authoritative tree."
        ready=yes
    else
        grep '^- \(VIOLATES\|MISSING\):' "$gate" | sed 's/^- /- Apply gate finding: /' || true
        ready="no - tree gate reported blocking findings"
    fi
    echo
    echo "## Updated tree patch"
    echo
    echo '```text'
    [ "$ready" = "yes" ] && echo "(no patch)" || echo "(see probe 03 findings)"
    echo '```'
    echo
    echo "## Semantics contract for probe 05"
    echo
    echo "- Preserve the direct session frame: $root/clone -> \$id/{ctl,data,stream,status,event}."
    echo "- Creation-time files such as opts, initial, and tool declarations freeze after \$id/ctl start."
    echo "- Model readiness, parameters, and progress events stay under $root/model/."
    echo "- Per-operation prompt body, constraint, data, stream, status, and ctl stay under \$id/prompt/\$n/."
    echo "- Context window, usage, and measurement stay under \$id/ctx/."
    echo "- Binary input references are relative to the service namespace, for example @in/img0."
    echo "- Tool callbacks use call and return pipe pairs matched by call id."
    echo
    [ "$ready" = "yes" ] && echo "Ready for semantics: yes" || echo "Ready for semantics: $ready"
}

render_05a() {
    require_tree
    TRACK_SECTIONS=1
    EMITTED_SECTIONS=""
    echo "Lens: deterministic 9P protocol implementer - deriving per-file semantics from the accepted tree."
    echo
    echo "Blind spots: this pass defines wire behavior from the namespace contract. Exact payload schemas still come from the source API inventory and the final mapping table."
    echo
    echo "### 3. Per-file / per-directory semantics"
    echo
    echo "General rule: directories are walkable only; files use whole-record reads and writes unless named as streams. Creation-time files freeze after \`\$id/ctl start\`; late writes return \`Estarted\`. Operation input files carry input, \`data\` is complete output, and \`stream\` is incremental output. Callback/tool \`call\` and \`return\` records are matched by call id. Binary references are relative, such as \`@in/img0\`, never host paths."
    echo

    if has_path "$root/clone"; then
        section "$root/clone" "session allocator" "0444" \
            "Each open allocates a new draft session directory; reading that open returns the relative id, then EOF." \
            "Rejected." \
            "\`Eperm\` on write; \`Ebusy\` when allocation is temporarily unavailable; \`Enomem\` when the backing API refuses a new session."
    fi

    if has_path "$root/model/availability" || has_path "$root/model/params" || has_path "$root/model/event" || has_path "$root/model/ctl"; then
        has_path "$root/model/availability" && section "$root/model/availability" "service availability" "0444" \
            "Returns the service-level readiness token exposed by the backing API." \
            "Rejected." \
            "\`Eio\` when the readiness probe fails; \`Eperm\` on write."
        has_path "$root/model/params" && section "$root/model/params" "service parameter limits" "0444" \
            "Returns stable default and maximum parameter values as newline-delimited key/value records." \
            "Rejected." \
            "\`Eio\` when parameters cannot be queried; \`Eperm\` on write."
        has_path "$root/model/event" && section "$root/model/event" "service event stream" "0444" \
            "Blocks until a model/service event is available, then returns one newline-delimited JSON record." \
            "Rejected." \
            "\`Einterrupted\` when the read is canceled; \`Eperm\` on write."
        has_path "$root/model/ctl" && section "$root/model/ctl" "service control" "0222" \
            "Rejected." \
            "Accepts implementation-supported service verbs. Unsupported verbs fail closed instead of inventing hidden policy." \
            "\`Ebadctl\` for unknown verbs; \`Ebusy\` for incompatible concurrent service work." \
            "| \`prepare\` | Ask the service to make the model ready if the source API exposes such an action. | \`Ebadctl\` if unsupported. |
| \`cancel\` | Cancel an outstanding service preparation action. | \`Enotstarted\` if nothing is pending. |"
    fi

    if has_path "$root/\$id/ctl"; then
        section "$root/\$id/ctl" "session lifecycle control" "0222" \
            "Rejected." \
            "Runs session lifecycle verbs. \`start\` consumes \`opts\`, \`initial\`, and registered tools; \`destroy\` tears down the backing session." \
            "\`Estarted\` for duplicate start or late creation-time mutation; \`Ebadctl\` for unknown verbs; \`Edestroyed\` after destroy." \
            "| \`start\` | Create the backing live session from the draft directory. | \`Estarted\` if already started; \`Einval\` for malformed options. |
| \`destroy\` | Release the backing session and make further operations fail. | \`Edestroyed\` if already destroyed. |"
    fi

    has_path "$root/\$id/status" && section "$root/\$id/status" "session status" "0444" \
        "Returns \`draft\`, \`ready\`, \`busy\`, \`error\`, or \`destroyed\` plus optional diagnostic fields." \
        "Rejected." \
        "\`Edestroyed\` after final teardown when the server chooses not to retain status; \`Eperm\` on write."

    has_path "$root/\$id/availability" && section "$root/\$id/availability" "session-specific availability" "0444" \
        "Returns readiness after applying this draft session's options and expected input/output shape." \
        "Rejected." \
        "\`Einval\` if draft options are malformed; \`Eperm\` on write."

    has_path "$root/\$id/opts" && section "$root/\$id/opts" "creation-time options" "0666 before start, 0444 after start" \
        "Returns the current creation options as JSON." \
        "Replaces or patches draft options before start." \
        "\`Estarted\` after start; \`Einval\` for unknown option names or invalid ranges."

    has_path "$root/\$id/initial" && section "$root/\$id/initial" "initial context" "0666 before start, 0444 after start" \
        "Returns the initial context payload staged for session creation." \
        "Writes the source API's initial message/context array before start." \
        "\`Estarted\` after start; \`Einval\` for malformed message payloads or invalid relative references."

    has_path "$root/\$id/data" && section "$root/\$id/data" "direct complete I/O" "0666" \
        "Returns the last complete direct-session result, when the source API exposes one." \
        "Writes a direct-session payload when the source API supports direct input on the session frame." \
        "\`Enotstarted\` before \`$root/\$id/ctl start\`; \`Ebusy\` while another direct operation is active; \`Einval\` for malformed payloads; \`Ebadctl\` if direct input is unsupported."

    has_path "$root/\$id/stream" && section "$root/\$id/stream" "direct incremental output" "0444" \
        "Blocks and returns chunks for the active direct-session stream, when one exists." \
        "Rejected." \
        "\`Enotstarted\` before start; \`Einterrupted\` when the read is canceled; \`Eperm\` on write."

    has_path "$root/\$id/event" && section "$root/\$id/event" "session event stream" "0444" \
        "Blocks until a session event is available, then returns one newline-delimited JSON record." \
        "Rejected." \
        "\`Edestroyed\` after destroy; \`Einterrupted\` when the read is canceled."

    has_path "$root/\$id/clone" && section "$root/\$id/clone" "session fork allocator" "0444" \
        "Each open clones the live session state; reading that open returns the new relative id, then EOF." \
        "Rejected." \
        "\`Enotstarted\` before the source session is live; \`Edestroyed\` after destroy; \`Eperm\` on write."

    if has_path "$root/\$id/ctx/window" || has_path "$root/\$id/ctx/usage" || has_path "$root/\$id/ctx/measure/body" || has_path "$root/\$id/ctx/measure/usage"; then
        has_path "$root/\$id/ctx/window" && section "$root/\$id/ctx/window" "context capacity" "0444" \
            "Returns the maximum context capacity for the live or draft session." \
            "Rejected." \
            "\`Eio\` if the backing API cannot report capacity; \`Eperm\` on write."
        has_path "$root/\$id/ctx/usage" && section "$root/\$id/ctx/usage" "current context usage" "0444" \
            "Returns the current consumed context units for the session." \
            "Rejected." \
            "\`Enotstarted\` if usage only exists for live sessions; \`Eperm\` on write."
        has_path "$root/\$id/ctx/measure/body" && section "$root/\$id/ctx/measure/body" "context measurement input" "0666" \
            "Returns the last measurement input payload." \
            "Writes a message payload to be measured without appending it to the session." \
            "\`Einval\` for malformed payloads; \`Ebusy\` while measurement is active."
        has_path "$root/\$id/ctx/measure/usage" && section "$root/\$id/ctx/measure/usage" "context measurement result" "0444" \
            "Returns the measured usage for the last \`measure/body\` write." \
            "Rejected." \
            "\`Enodata\` before a measurement completes; \`Eperm\` on write."
    fi

    if has_path "$root/\$id/in/ctl" || has_glob "$root/\$id/in/*"; then
        has_path "$root/\$id/in/ctl" && section "$root/\$id/in/ctl" "typed input allocator" "0222" \
            "Rejected." \
            "Allocates typed input slots such as image or audio blobs for later relative references." \
            "\`Ebadctl\` for unknown input classes; \`Estarted\` if the implementation freezes inputs at operation start." \
            "| \`new TYPE\` | Allocate the next input slot for TYPE and return/enable its name. | \`Ebadctl\` for unsupported TYPE. |
| \`remove NAME\` | Drop an unused staged input slot. | \`Enotfound\` if NAME does not exist. |"
        section "$root/\$id/in/NAME" "staged typed input bytes" "0666 before operation start" \
            "Returns staged bytes when the implementation makes slots readable; otherwise rejected." \
            "Writes the raw bytes for a staged typed input. Prompts refer to the slot with relative names such as \`@in/img0\`." \
            "\`Einval\` for wrong type or oversized payload; \`Estarted\` after the consuming operation starts."
    fi

    if has_path "$root/\$id/tools/ctl" || has_glob "$root/\$id/tools/*/call"; then
        has_path "$root/\$id/tools/ctl" && section "$root/\$id/tools/ctl" "tool registry control" "0222" \
            "Rejected." \
            "Registers or removes tool directories before the session starts." \
            "\`Estarted\` after session start; \`Ebadctl\` for unknown verbs; \`Eexist\` for duplicate names." \
            "| \`new NAME\` | Create a draft tool directory named NAME. | \`Eexist\` if NAME is already present. |
| \`remove NAME\` | Remove a draft tool directory. | \`Enotfound\` if NAME is absent. |"
        section "$root/\$id/tools/\$NAME/{description,schema,status}" "tool definition files" "0666 before start, 0444 after start" \
            "Return the registered tool description, input schema, and active state." \
            "Set the description and schema before session start." \
            "\`Estarted\` after start; \`Einval\` for malformed schema; \`Enotfound\` for unknown tool names."
        section "$root/\$id/tools/\$NAME/call" "tool call pipe" "0444" \
            "Blocks until the model requests this tool, then returns a call-id plus JSON arguments." \
            "Rejected." \
            "\`Einterrupted\` when the read is canceled; \`Edestroyed\` after session destroy."
        section "$root/\$id/tools/\$NAME/return" "tool result pipe" "0222" \
            "Rejected." \
            "Writes a call-id plus result or error payload to resume the waiting model operation." \
            "\`Enotfound\` for an unknown call id; \`Einval\` for malformed results; \`Eperm\` on read."
    fi

    if has_path "$root/\$id/prompt/clone" || has_path "$root/\$id/prompt/\$n/ctl"; then
        has_path "$root/\$id/prompt/clone" && section "$root/\$id/prompt/clone" "prompt operation allocator" "0444" \
            "Allocates a new draft prompt operation and returns the relative operation id." \
            "Rejected." \
            "\`Enotstarted\` before the session is ready; \`Ebusy\` when operation allocation is saturated; \`Eperm\` on write."
        has_path "$root/\$id/prompt/\$n/ctl" && section "$root/\$id/prompt/\$n/ctl" "prompt operation control" "0222" \
            "Rejected." \
            "Runs operation verbs. \`start\` consumes \`body\` and \`constraint\`; \`append\` appends without returning output when the source API distinguishes append; \`abort\` cancels active work." \
            "\`Estarted\` for duplicate start; \`Ebadctl\` for unknown verbs; \`Enotstarted\` for abort before start." \
            "| \`start\` | Execute the operation and make output available through \`data\` or \`stream\`. | \`Einval\` for malformed body or constraint. |
| \`append\` | Append the body to session context when the source API supports append-style operations. | \`Ebadctl\` if unsupported. |
| \`abort\` | Cancel the active operation. | \`Enotstarted\` if the operation is not running. |"
        has_path "$root/\$id/prompt/\$n/body" && section "$root/\$id/prompt/\$n/body" "prompt operation input" "0666 before start, 0444 after start" \
            "Returns the staged operation input." \
            "Writes the source API's prompt/message payload. Relative binary references resolve inside the same session." \
            "\`Estarted\` after operation start; \`Einval\` for malformed payloads or missing relative references."
        has_path "$root/\$id/prompt/\$n/constraint" && section "$root/\$id/prompt/\$n/constraint" "response constraint" "0666 before start, 0444 after start" \
            "Returns the staged response constraint payload." \
            "Writes the source API's constraint shape before operation start." \
            "\`Estarted\` after operation start; \`Einval\` for unsupported or malformed constraints."
        has_path "$root/\$id/prompt/\$n/data" && section "$root/\$id/prompt/\$n/data" "complete operation output" "0444" \
            "Returns the complete response once the operation finishes." \
            "Rejected." \
            "\`Enodata\` before completion; \`Eaborted\` after abort; \`Eperm\` on write."
        has_path "$root/\$id/prompt/\$n/stream" && section "$root/\$id/prompt/\$n/stream" "incremental operation output" "0444" \
            "Blocks and returns incremental response chunks until completion." \
            "Rejected." \
            "\`Einterrupted\` when the read is canceled; \`Eaborted\` after abort; \`Eperm\` on write."
        has_path "$root/\$id/prompt/\$n/status" && section "$root/\$id/prompt/\$n/status" "operation status" "0444" \
            "Returns \`draft\`, \`running\`, \`done\`, \`aborted\`, or \`error\` plus optional diagnostics." \
            "Rejected." \
            "\`Enotfound\` after garbage collection; \`Eperm\` on write."
    fi

    emit_missing_sections
}

render_05b() {
    local survey="$WORK/probes/01-survey-api.md"
    need "$survey"
    require_tree
    local awk_paths
    awk_paths=$(printf '%s\n' "$PATHS" | tr '\n' '\034')
    awk -v root="$root" -v paths="$awk_paths" '
function trim(s) { gsub(/^[[:space:]]+|[[:space:]]+$/, "", s); return s }
function clean(s) { s = trim(s); gsub(/^`|`$/, "", s); return s }
function key(s) {
    s = tolower(clean(s))
    sub(/[[:space:]]+\([^)]*\)[[:space:]]*$/, "", s)
    sub(/\([^)]*\)[[:space:]]*$/, "", s)
    sub(/^(session|languagemodel|languagemodelsession)\./, "", s)
    sub(/^.*\./, "", s)
    gsub(/[^a-z0-9]/, "", s)
    return s
}
function has(s, pat) { return index(tolower(s), pat) > 0 }
function normpath(p, q) {
    q = p
    sub("^" root "/\\$id/tools/[^/]+", root "/$id/tools/$NAME", q)
    sub("^" root "/\\$id/in/[^/]+$", root "/$id/in/NAME", q)
    return q
}
function present(p) { return p != "" && ((p in pathset) || (normpath(p) in pathset)) }
function choose(a,b,c,d,e,f,g,h) {
    if (present(a)) return a
    if (present(b)) return b
    if (present(c)) return c
    if (present(d)) return d
    if (present(e)) return e
    if (present(f)) return f
    if (present(g)) return g
    if (present(h)) return h
    return ""
}
function add_existing(out, p) {
    if (!present(p)) return out
    if (out != "") out = out ", "
    return out p
}
function listed(a,b,c,d,e,f,g,h,i,j, out) {
    out = ""
    out = add_existing(out, a)
    out = add_existing(out, b)
    out = add_existing(out, c)
    out = add_existing(out, d)
    out = add_existing(out, e)
    out = add_existing(out, f)
    out = add_existing(out, g)
    out = add_existing(out, h)
    out = add_existing(out, i)
    out = add_existing(out, j)
    if (out == "") return "fails closed"
    return out
}
function closed(p) { if (p == "") return "fails closed"; return p }
function prompt_body() { return choose(root "/$id/prompt/$n/body", root "/$id/data", root "/$id/initial") }
function prompt_data() { return choose(root "/$id/prompt/$n/data", root "/$id/data") }
function prompt_stream() { return choose(root "/$id/prompt/$n/stream", root "/$id/stream") }
function prompt_status() { return choose(root "/$id/prompt/$n/status", root "/$id/status") }
function prompt_ctl() { return choose(root "/$id/prompt/$n/ctl", root "/$id/ctl") }
function typed_files() { return listed(root "/$id/in/ctl", root "/$id/in/NAME", prompt_body()) }
function tool_files() {
    return listed(root "/$id/tools/ctl",
        root "/$id/tools/$NAME/ctl",
        root "/$id/tools/$NAME/title",
        root "/$id/tools/$NAME/description",
        root "/$id/tools/$NAME/schema",
        root "/$id/tools/$NAME/annotations",
        root "/$id/tools/$NAME/exposed_to",
        root "/$id/tools/$NAME/status",
        root "/$id/tools/$NAME/call",
        root "/$id/tools/$NAME/return")
}
function files(surface, kind, owner, scope, pressure) {
    s = key(surface); o = tolower(owner); sc = tolower(scope); p = tolower(pressure)
    if (s == "defaulttopk" || s == "maxtopk" || s == "defaulttemperature" || s == "maxtemperature") return closed(choose(root "/model/params"))
    if (s == "contextusage" || s == "inputusage") return closed(choose(root "/$id/ctx/usage"))
    if (s == "contextwindow" || s == "inputquota") return closed(choose(root "/$id/ctx/window"))
    if (s == "monitor" || s == "downloadprogress" || s == "ontoolchange" || s == "toolchange") return closed(choose(root "/model/event", root "/$id/event"))
    if (s == "registertool") return listed(root "/$id/tools/ctl", root "/$id/tools/$NAME/ctl", root "/$id/tools/$NAME/status")
    if (s == "name" && has(o, "tool")) return listed(root "/$id/tools/ctl", root "/$id/tools/$NAME/ctl", root "/$id/tools/$NAME/status")
    if (s == "title" && has(o, "tool")) return closed(choose(root "/$id/tools/$NAME/title"))
    if (s == "description" && has(o, "tool")) return closed(choose(root "/$id/tools/$NAME/description"))
    if ((s == "schema" || s == "inputschema") && has(o, "tool")) return closed(choose(root "/$id/tools/$NAME/schema"))
    if ((s == "annotations" || s == "readonlyhint" || s == "destructivehint" || s == "idempotenthint" || s == "openworldhint" || s == "untrustedcontenthint") && has(o, "tool")) return closed(choose(root "/$id/tools/$NAME/annotations"))
    if (s == "exposedto" && has(o, "tool")) return closed(choose(root "/$id/tools/$NAME/exposed_to"))
    if ((s == "execute" || s == "requestuserinteraction") && has(o, "tool")) return listed(root "/$id/tools/$NAME/call", root "/$id/tools/$NAME/return")
    if (s == "role") return listed(root "/$id/initial", prompt_body())
    if (s == "type" && has(sc, "input item")) return typed_files()
    if (s == "type") return closed(choose(root "/$id/opts", prompt_body()))
    if (s == "languages") return closed(choose(root "/$id/opts"))
    if (s == "value") return typed_files()
    if (s == "topk" || s == "temperature") return listed(root "/$id/opts", root "/$id/status")
    if (s == "initialprompts") return closed(choose(root "/$id/initial"))
    if (s == "tools") return tool_files()
    if (s == "prompt") return listed(prompt_body(), prompt_data())
    if (s == "promptstreaming") return listed(prompt_body(), prompt_stream())
    if (s == "append") return listed(prompt_body(), prompt_ctl(), prompt_status())
    if (s == "measurecontextusage" || s == "measureinputusage") return listed(root "/$id/ctx/measure/body", root "/$id/ctx/measure/usage")
    if (s == "clone") return closed(choose(root "/$id/clone", root "/clone"))
    if (s == "create") return closed(choose(root "/clone"))
    if (s == "destroy") return closed(choose(root "/$id/ctl"))
    if (s == "responseconstraint" || s == "omitresponseconstraintinput") return closed(choose(root "/$id/prompt/$n/constraint"))
    if (s == "prefix") return closed(prompt_body())
    if (s == "signal") {
        if (has(o, "tool") || has(sc, "tool")) return closed(choose(root "/$id/tools/$NAME/ctl", root "/$id/tools/ctl"))
        if (has(o, "clone")) return closed(choose(root "/$id/ctl"))
        if (has(sc, "operation")) return closed(prompt_ctl())
        return closed(choose(root "/$id/ctl"))
    }
    if (has(p, "fail-closed")) return "fails closed"
    if (has(p, "clone/session")) { if (has(sc, "live session")) return closed(choose(root "/$id/clone")); return closed(choose(root "/clone")) }
    if (has(p, "status file")) {
        if (has(sc, "service")) { if (s ~ /param/) return closed(choose(root "/model/params")); return closed(choose(root "/model/availability", root "/$id/status")) }
        if (has(sc, "accounting")) { if (s ~ /window|quota/) return closed(choose(root "/$id/ctx/window")); if (s ~ /usage/) return closed(choose(root "/$id/ctx/usage")); return closed(choose(root "/$id/ctx/usage", root "/$id/ctx/window")) }
        return closed(choose(root "/$id/status"))
    }
    if (has(p, "event file")) { if (s ~ /download/ || has(sc, "service")) return closed(choose(root "/model/event", root "/$id/event")); return closed(choose(root "/$id/event", root "/model/event")) }
    if (has(p, "immutable option")) {
        if (s ~ /initial/) return closed(choose(root "/$id/initial"))
        if (s ~ /tool/ || has(o, "tool")) return tool_files()
        return closed(choose(root "/$id/opts"))
    }
    if (has(p, "one-shot option")) { if (s ~ /constraint/) return closed(choose(root "/$id/prompt/$n/constraint")); return closed(prompt_ctl()) }
    if (has(p, "input file")) {
        if (has(sc, "accounting")) return listed(root "/$id/ctx/measure/body", root "/$id/ctx/measure/usage")
        if (has(sc, "draft session")) return closed(choose(root "/$id/initial"))
        return closed(prompt_body())
    }
    if (has(p, "typed blob")) return typed_files()
    if (has(p, "call/return pipe")) return listed(root "/$id/tools/$NAME/call", root "/$id/tools/$NAME/return")
    if (has(p, "stream file")) return listed(root "/$id/stream", prompt_stream())
    if (has(p, "output file")) return listed(root "/$id/data", prompt_data())
    if (has(p, "ctl verb")) {
        if (has(sc, "operation")) return closed(prompt_ctl())
        if (has(sc, "service")) return closed(choose(root "/model/ctl"))
        if (has(sc, "tool") || has(o, "tool")) return closed(choose(root "/$id/tools/$NAME/ctl", root "/$id/tools/ctl"))
        return closed(choose(root "/$id/ctl"))
    }
    if (has(p, "no-op")) return "N/A"
    return closed(choose(root "/$id/status", root "/model/availability"))
}
function read_sem(surface, scope, pressure, path) {
    p = tolower(pressure); s = key(surface)
    if (path == "fails closed") return "N/A"
    if (s == "modelcontext") return "Document-scoped model context/session state."
    if (s == "registertool") return "Tool registry state and optional per-tool status."
    if (s == "defaulttopk" || s == "maxtopk" || s == "defaulttemperature" || s == "maxtemperature") return "Service parameter default or limit from model params."
    if (s == "contextusage") return "Current context usage from ctx/usage."
    if (s == "contextwindow") return "Context capacity from ctx/window."
    if (s == "monitor" || s == "downloadprogress") return "Blocking service progress event records."
    if (s == "name") return "Registered tool name and status."
    if (s == "title") return "Registered tool title."
    if (s == "description") return "Registered tool description."
    if (s == "inputschema") return "Registered tool input schema."
    if (s == "annotations" || s == "readonlyhint" || s == "destructivehint" || s == "idempotenthint" || s == "openworldhint" || s == "untrustedcontenthint") return "Registered tool annotations."
    if (s == "exposedto") return "Registered exposure policy."
    if (s == "toolname" || s == "tooldescription" || s == "toolautosubmit" || s == "toolparamdescription") return "Declarative tool metadata staged in tool files."
    if (s == "agentinvoked") return "Invocation state from status."
    if (s == "role") return "Message role inside initial context or prompt body."
    if (s == "type") return "Expected input/output type or typed content discriminator."
    if (s == "languages") return "Expected language list inside session options."
    if (s == "value") return "String or byte-backed staged content; host object identity is not preserved."
    if (s == "topk" || s == "temperature") return "Configured draft value and live readback value."
    if (s == "prompt") return "Complete response from the operation data file."
    if (s == "promptstreaming") return "Incremental response chunks from the operation stream file."
    if (s == "append") return "Operation status; append itself produces no response body."
    if (s == "measurecontextusage") return "Measured usage from ctx/measure/usage."
    if (s == "initialprompts") return "Staged initial context."
    if (s == "tools") return "Tool definition state plus blocking call records."
    if (s == "prefix") return "Message body containing the prefix flag."
    if (s == "signal") return "N/A."
    if (has(p, "clone/session")) return "Allocated session id or cloned session id."
    if (has(p, "status")) return "Tagged state, scalar limit, usage, or parameter value."
    if (has(p, "event")) return "Blocking event records."
    if (has(p, "stream")) return "Incremental output chunks."
    if (has(p, "output")) return "Complete output or aggregate session result."
    if (has(p, "call/return")) return "Blocking tool-call request records."
    if (has(p, "input file")) return "Measured usage when paired with an output file; otherwise N/A."
    if (has(p, "typed blob")) return "Written staged bytes when the slot is readable."
    return "Current configured value when readable; otherwise N/A."
}
function write_sem(surface, scope, pressure, path) {
    p = tolower(pressure); s = key(surface)
    if (path == "fails closed") return "Rejected at filesystem boundary."
    if (s == "modelcontext") return "N/A."
    if (s == "registertool") return "Registers or removes tool directories through tools/ctl."
    if (s == "defaulttopk" || s == "maxtopk" || s == "defaulttemperature" || s == "maxtemperature") return "N/A."
    if (s == "contextusage" || s == "contextwindow") return "N/A."
    if (s == "monitor" || s == "downloadprogress") return "N/A."
    if (s == "name") return "Creates or removes named tool directories through tools/ctl."
    if (s == "title") return "Stages tool title before session start."
    if (s == "description") return "Stages tool description before session start."
    if (s == "inputschema") return "Stages tool input schema before session start."
    if (s == "annotations" || s == "readonlyhint" || s == "destructivehint" || s == "idempotenthint" || s == "openworldhint" || s == "untrustedcontenthint") return "Stages tool annotations before session start."
    if (s == "exposedto") return "Stages the exposure policy before session start."
    if (s == "toolname" || s == "tooldescription" || s == "toolautosubmit" || s == "toolparamdescription") return "Stages declarative tool metadata in the tool directory."
    if (s == "agentinvoked") return "N/A."
    if (s == "role") return "Stages role-bearing messages in initial or operation body."
    if (s == "type") return "Stages expected type or typed-content discriminator."
    if (s == "languages") return "Stages expected language metadata in session options."
    if (s == "value") return "Stages strings or raw bytes; rejects live host objects that cannot cross the filesystem boundary."
    if (s == "topk" || s == "temperature") return "Stages creation-time option; freezes after start."
    if (s == "prompt") return "Writes operation body and starts or feeds the prompt operation."
    if (s == "promptstreaming") return "Writes operation body; output is consumed from stream."
    if (s == "append") return "Writes body and uses ctl append/start semantics."
    if (s == "measurecontextusage") return "Writes measurement body without appending it to context."
    if (s == "initialprompts") return "Stages initial context before session start."
    if (s == "prefix") return "Stages the message prefix field inside the operation body."
    if (s == "signal") return "Maps cancellation to the relevant ctl abort/destroy verb."
    if (s == "tools") return "Declares tool slots before session start."
    if (s == "execute") return "Writes tool result matched to a pending call id."
    if (has(p, "clone/session") || has(p, "status") || has(p, "event") || has(p, "stream") || has(p, "output")) return "N/A."
    if (has(p, "immutable option")) return "Stages creation-time config; freezes after start."
    if (has(p, "one-shot option")) return "Stages per-operation option consumed by prompt ctl."
    if (has(p, "ctl verb")) return "Runs lifecycle verb or abort/destroy action."
    if (has(p, "typed blob")) return "Stages message fields or typed bytes for relative prompt references."
    if (has(p, "call/return")) return "Writes tool result matched to a pending call id."
    if (has(p, "input file")) return "Stages prompt or measurement input."
    return "N/A."
}
BEGIN {
    npath = split(paths, pathlist, "\034")
    for (i=1; i<=npath; i++) {
        if (pathlist[i] == "") continue
        pathset[pathlist[i]] = 1
        pathset[normpath(pathlist[i])] = 1
    }
    print "Lens: deterministic source-cartographer - deriving feature mapping from probe 01 inventory."
    print ""
    print "Blind spots: path choices follow probe 01 pressure tags and the established root; nuanced per-API exceptions need manual review."
    print ""
    print "### 4. Feature-by-feature mapping table"
    print ""
    print "| API surface | file(s) | Read returns | Write does |"
    print "|---|---|---|---|"
}
/^\|/ {
    if ($0 ~ /^\|[[:space:]]*:?-+:/ || $0 ~ /\|[[:space:]]*surface[[:space:]]*\|/) next
    n = split($0, a, /\|/)
    if (n < 7) next
    surface = clean(a[2]); kind = clean(a[3]); owner = clean(a[4])
    pressure = clean(a[n-1]); scope = clean(a[n-2])
    if (surface == "" || kind == "" || scope == "" || pressure == "") next
    if (surface ~ /^:?-+:?$/) next
    path = files(surface, kind, owner, scope, pressure)
    read = read_sem(surface, scope, pressure, path)
    write = write_sem(surface, scope, pressure, path)
    label = surface
    if (owner != "") label = surface " (" owner ")"
    gsub(/\|/, "\\|", label); gsub(/\|/, "\\|", path); gsub(/\|/, "\\|", read); gsub(/\|/, "\\|", write)
    print "| `" label "` | `" path "` | " read " | " write " |"
    if (path == "fails closed") fails++; else mapped++
}
END {
    print ""
    printf "Completeness audit: %d mapped; %d fails closed.\n", mapped, fails
    if (mapped == 0) exit 1
}
' "$survey"
}

render_06() {
    require_tree
    local tool_name input_name input_type input_file
    tool_name=$(first_child "$root/\$id/tools" "tool")
    case "$tool_name" in '$'*|'') tool_name="getWeather" ;; esac
    if has_path "$root/\$id/in/img0"; then
        input_name="img0"; input_type="image"; input_file="photo.jpg"
    elif has_path "$root/\$id/in/aud0"; then
        input_name="aud0"; input_type="audio"; input_file="clip.wav"
    else
        input_name=$(first_child "$root/\$id/in" "input0"); input_type="blob"; input_file="payload.bin"
    fi

    echo "Lens: deterministic rc transcript writer - deriving examples from the accepted namespace."
    echo
    echo "Blind spots: examples use representative JSON payloads and omit API-specific schema minutiae unless those details are already encoded in the tree."
    echo
    echo "### 5. Worked end-to-end examples"
    echo
    if has_path "$root/\$id/prompt/clone"; then
        echo "#### Create a session and run a direct request"
    else
        echo "#### Create a session and inspect its direct frame"
    fi
    echo
    echo '```rc'
    echo "id=\`{cat $root/clone}"
    has_path "$root/\$id/opts" && echo "echo '{\"temperature\":0.2,\"topK\":4}' > $root/\$id/opts"
    has_path "$root/\$id/initial" && echo "echo '[{\"role\":\"system\",\"content\":\"answer tersely\"}]' > $root/\$id/initial"
    has_path "$root/\$id/ctl" && echo "echo start > $root/\$id/ctl"
    if has_path "$root/\$id/data" && has_path "$root/\$id/prompt/clone"; then
        echo "echo 'Explain the design in one sentence.' > $root/\$id/data"
        echo "cat $root/\$id/data"
    fi
    has_path "$root/\$id/status" && echo "cat $root/\$id/status"
    echo '```'
    echo

    if has_path "$root/\$id/prompt/clone"; then
        echo "#### Run a numbered streaming operation"
        echo
        echo '```rc'
        echo "id=\`{cat $root/clone}"
        has_path "$root/\$id/ctl" && echo "echo start > $root/\$id/ctl"
        echo "n=\`{cat $root/\$id/prompt/clone}"
        has_path "$root/\$id/prompt/\$n/body" && echo "echo '[{\"role\":\"user\",\"content\":\"List three constraints.\"}]' > $root/\$id/prompt/\$n/body"
        has_path "$root/\$id/prompt/\$n/constraint" && echo "echo '{\"type\":\"array\",\"maxItems\":3}' > $root/\$id/prompt/\$n/constraint"
        has_path "$root/\$id/prompt/\$n/ctl" && echo "echo start > $root/\$id/prompt/\$n/ctl"
        has_path "$root/\$id/prompt/\$n/stream" && echo "cat $root/\$id/prompt/\$n/stream"
        has_path "$root/\$id/prompt/\$n/data" && echo "cat $root/\$id/prompt/\$n/data"
        echo '```'
        echo
    fi

    if has_path "$root/\$id/ctx/measure/body"; then
        echo "#### Measure context before consuming it"
        echo
        echo '```rc'
        echo "id=\`{cat $root/clone}"
        echo "echo '[{\"role\":\"user\",\"content\":\"draft before sending\"}]' > $root/\$id/ctx/measure/body"
        has_path "$root/\$id/ctx/measure/usage" && echo "cat $root/\$id/ctx/measure/usage"
        has_path "$root/\$id/ctx/window" && echo "cat $root/\$id/ctx/window"
        has_path "$root/\$id/ctx/usage" && echo "cat $root/\$id/ctx/usage"
        echo '```'
        echo
    fi

    if has_path "$root/\$id/in/ctl"; then
        echo "#### Stage typed input inside the namespace"
        echo
        echo '```rc'
        echo "id=\`{cat $root/clone}"
        echo "echo new $input_type > $root/\$id/in/ctl"
        echo "cp $input_file $root/\$id/in/$input_name"
        has_path "$root/\$id/ctl" && echo "echo start > $root/\$id/ctl"
        if has_path "$root/\$id/prompt/clone"; then
            echo "n=\`{cat $root/\$id/prompt/clone}"
            echo "echo '[{\"role\":\"user\",\"content\":[\"describe this\",{\"ref\":\"@in/$input_name\"}]}]' > $root/\$id/prompt/\$n/body"
            has_path "$root/\$id/prompt/\$n/ctl" && echo "echo start > $root/\$id/prompt/\$n/ctl"
            has_path "$root/\$id/prompt/\$n/data" && echo "cat $root/\$id/prompt/\$n/data"
        fi
        echo '```'
        echo
    fi

    if has_path "$root/\$id/tools/ctl"; then
        echo "#### Register a tool and answer call records"
        echo
        echo '```rc'
        echo "id=\`{cat $root/clone}"
        echo "echo new $tool_name > $root/\$id/tools/ctl"
        has_path "$root/\$id/tools/$tool_name/description" && echo "echo 'Return the current value for a named location.' > $root/\$id/tools/$tool_name/description"
        has_path "$root/\$id/tools/$tool_name/schema" && echo "echo '{\"type\":\"object\",\"properties\":{\"location\":{\"type\":\"string\"}}}' > $root/\$id/tools/$tool_name/schema"
        has_path "$root/\$id/ctl" && echo "echo start > $root/\$id/ctl"
        if has_path "$root/\$id/tools/$tool_name/call"; then
            echo "cat $root/\$id/tools/$tool_name/call"
            has_path "$root/\$id/tools/$tool_name/return" && echo "echo '{\"id\":\"call-1\",\"result\":{\"text\":\"72F\"}}' > $root/\$id/tools/$tool_name/return"
        fi
        echo '```'
        echo
    fi

    if has_path "$root/\$id/clone" || has_path "$root/\$id/ctl"; then
        echo "#### Fork and tear down a session"
        echo
        echo '```rc'
        echo "id=\`{cat $root/clone}"
        has_path "$root/\$id/ctl" && echo "echo start > $root/\$id/ctl"
        has_path "$root/\$id/clone" && echo "id2=\`{cat $root/\$id/clone}"
        has_path "$root/\$id/status" && echo "cat $root/\$id/status"
        has_path "$root/\$id/ctl" && echo "echo destroy > $root/\$id/ctl"
        echo '```'
    fi
}

render_07() {
    require_tree
    echo "Lens: deterministic Plan 9 historian with Wanix capability check."
    echo
    echo "Blind spots: this pass reasons from the accepted namespace; source-specific wording remains in probes 00 and 01."
    echo
    echo "### 1. Philosophy and design approach"
    echo
    echo "The design treats the target API as a file server, not as a remote procedure wrapper. Allocation happens by opening clone files and reading the allocated id, configuration is staged in draft directories, execution begins through small control verbs, and results appear in ordinary data, stream, status, and event files."
    echo
    echo "#### 1.1 Idioms used verbatim"
    echo
    echo "- \`$root/clone\` is the allocator. Opening it creates per-client state; reading that open returns the new session name, following the familiar Plan 9 clone-file pattern."
    echo "- \`$root/\$id/{ctl,data,stream,status,event}\` is the direct session frame: control verbs, complete I/O, incremental I/O, state, and blocking events remain separate files."
    has_path "$root/\$id/prompt/clone" && echo "- \`$root/\$id/prompt/clone\` creates per-operation directories, so concurrent operations do not multiplex body, output, and status through one file."
    has_path "$root/\$id/ctx/usage" && echo "- Context accounting is read as files under \`$root/\$id/ctx\`, keeping measurement separate from mutation."
    has_path "$root/\$id/tools/ctl" && echo "- Tool callbacks use \`call\` and \`return\` pipe files, preserving blocking file I/O instead of embedding host callbacks in the namespace."
    echo
    echo "#### 1.2 Wanix capability integration"
    echo
    echo "Mounting the service grants the capability. A process that cannot walk to a session directory cannot operate that session, and a process handed only a subdirectory gets only that subset of authority. Relative references such as \`@in/img0\` keep binary inputs inside the mounted namespace and avoid leaking host paths."
    echo
    echo "#### 1.3 Where the design deviates from pure Plan 9"
    echo
    echo "The backing API has draft-time options, streaming promises, structured constraints, and callback-style tools. The filesystem keeps those lifecycle boundaries visible rather than hiding them behind one synthetic request file. JSON is used for structured payloads where the source API already exposes structured records; plain files still carry allocation, status, data, stream, and event roles."
    echo
    echo "### 6. Notes on deviations from pure Plan 9 style"
    echo
    echo "| Supported | Fails closed |"
    echo "|---|---|"
    echo "| Clone allocation for sessions and operations | Host object identity, function closures, and other values that cannot cross a file boundary |"
    echo "| Draft-then-start creation with immutable options | Mutating creation-time options after \`$root/\$id/ctl start\` |"
    echo "| Separate \`ctl\`, \`data\`, \`stream\`, \`status\`, and \`event\` files | Multiplexing unrelated lifecycle states into one opaque request file |"
    echo "| Service-wide model/provider scope when present in the accepted tree | Loose root-level status/control files not backed by the source lifecycle |"
    echo "| Per-operation directories for concurrent prompts, appends, measurements, or streams | Interleaving concurrent operation output in one shared stream |"
    echo "| Relative binary references under \`$root/\$id/in\` | Host file paths, DOM object identity, or ambient process references in prompt payloads |"
    echo "| Callback/tool \`call\` and \`return\` files | Direct invocation of host functions across the 9P boundary |"
}

render_08() {
    local root
    root=$(service_root "$tree" 2>/dev/null || true)
    [ -n "$root" ] || root="/service"
    echo "Lens: deterministic Wanix capability hardliner with release-planning pass."
    echo
    echo "Blind spots: implementation sequencing is inferred from the accepted namespace, not from a live server profile."
    echo
    echo "## Recommendations"
    echo
    echo "1. Build the smallest session service first: \`$root/clone\`, \`$root/\$id/ctl\`, \`$root/\$id/data\`, \`$root/\$id/status\`, and destroy semantics. This proves allocation, draft state, start, complete output, and cleanup."
    echo "2. Add streaming, events, and accounting next: \`stream\`, \`event\`, service/model status, and \`ctx/{window,usage,measure}\`. Keep stream reads blocking and status reads non-mutating."
    echo "3. Add advanced source-backed scopes after the session core is stable: per-operation \`prompt/\$n\`, typed input under \`in/\`, and tool \`call\`/\`return\` pipes. Treat unsupported host objects and callbacks as explicit boundary errors."
    echo "4. Harden the service: define error strings, add concurrency tests for multiple sessions and operations, test cancellation and fid cleanup, document JSON payload schemas, and keep source mirrors refreshed before publishing a design update."
    echo
    echo "## Caveats"
    echo
    echo "- The upstream API can still change; refresh the normative source before treating this as a stable wire contract."
    echo "- Browser or runtime availability can vary by platform, hardware, policy, and origin."
    echo "- Model readiness and preparation progress may not map to a portable imperative command; unsupported service-control verbs must return \`Ebadctl\`."
    echo "- Concurrent prompts, streams, and tool calls require per-operation isolation; a shared output file would be a correctness bug."
    echo "- Model output is nondeterministic, so tests should assert file protocol behavior and stable status transitions, not exact generated prose."
    echo "- Host objects, functions, DOM nodes, and ambient JavaScript capabilities lose identity at the filesystem boundary and should fail closed unless represented as bytes or JSON."
    echo "- Mount permissions are the security boundary; do not add ad hoc per-call auth files unless the source API has per-call authentication."
    echo "- Source coverage may miss issue-level currentness or implementation quirks; preserve the evidence bundle with every generated manpage."
}

render_09() {
    need "$tree"
    need "$WORK/probes/05a-flesh-out-section3.md"
    need "$WORK/probes/05b-feature-mapping.md"
    need "$WORK/probes/06-worked-examples.md"
    need "$WORK/probes/07-philosophy-and-deviations.md"
    need "$WORK/probes/08-recommendations-and-caveats.md"

    local root service api
    root=$(service_root "$tree"); [ -n "$root" ] || root="/service"
    service=$(printf '%s\n' "$root" | sed 's,^/,,; s,/.*,,'); [ -n "$service" ] || service="$SLUG"
    api=$(api_name "$WORK")

    from_heading() { awk -v start="$2" '$0 ~ start { on=1 } on { print }' "$1"; }
    philosophy_section() { awk '/^### 1\. / { on=1 } /^### 6\. / { on=0 } on { print }' "$WORK/probes/07-philosophy-and-deviations.md"; }
    deviation_section() { awk '/^### 6\. / { on=1 } on { print }' "$WORK/probes/07-philosophy-and-deviations.md"; }
    tree_section() {
        echo "### 2. Filesystem tree"
        awk '/^#+ 2\. Filesystem tree/ { on=1; next } on { print }' "$tree"
    }

    echo "# ${service}(4) - A Plan 9 / Wanix Synthetic Filesystem for ${api}"
    echo
    echo "## TL;DR"
    echo
    echo "Treats ${api} as a direct Plan 9 mapping of the whole API, centered on \`${root}/clone -> \$id/{ctl,data,stream,status,event}\`. Draft session files stage creation-time state, \`ctl\` starts and tears down work, \`data\` and \`stream\` expose complete and incremental output, and advanced capabilities live in explicit subdirectories instead of hidden language objects."
    echo
    echo "## Key Findings"
    echo
    echo "- The accepted tree preserves the direct session frame: \`${root}/clone -> \$id/{ctl,data,stream,status,event}\`."
    echo "- Creation-time options, initial context, and tools are staged before \`\$id/ctl start\`, then freeze with \`Estarted\` on late writes."
    echo "- Per-operation directories isolate body, constraint, output, stream, and status for concurrent work."
    echo "- Context accounting, typed inputs, service readiness, and callback/tool traffic are separate path families rather than overloaded payload fields."
    echo "- Host objects, function identity, and ambient process references fail closed at the filesystem boundary; bytes, JSON, and relative \`@in/...\` references cross it."
    echo
    echo "## Details"
    echo
    philosophy_section
    echo
    tree_section
    echo
    from_heading "$WORK/probes/05a-flesh-out-section3.md" '^### 3\. '
    echo
    from_heading "$WORK/probes/05b-feature-mapping.md" '^### 4\. '
    echo
    from_heading "$WORK/probes/06-worked-examples.md" '^### 5\. '
    echo
    deviation_section
    echo
    from_heading "$WORK/probes/08-recommendations-and-caveats.md" '^## Recommendations'
}

[ -d "$WORK" ] || { echo "no work dir for $SLUG" >&2; exit 1; }

case "$PROBE" in
    03-critique-adversarial)        render_03 ;;
    04-critique-constructive)       render_04 ;;
    05a-flesh-out-section3)         render_05a ;;
    05b-feature-mapping)            render_05b ;;
    06-worked-examples)             render_06 ;;
    07-philosophy-and-deviations)   render_07 ;;
    08-recommendations-and-caveats) render_08 ;;
    09-synthesize-document)         render_09 ;;
    *) echo "derive.sh: no deterministic renderer for probe '$PROBE'" >&2; exit 3 ;;
esac
