# Vendor trace normalization

`traceworkload.ReadMooncake`, `ReadBailian` and `ReadTraceLab` read local JSONL
into `SessionTrace`. This session-aware artifact is separate from the flat
BurstGPT targets. It retains source SHA-256, row ordinal, session/turn identity,
optional counts/timing, cache identities and raw source fields. It does not
synthesize prompt text from token counts.

```sh
go build -o ./llm-import-trace ./cmd/llm-import-trace
./llm-import-trace -format mooncake < trace.jsonl > normalized.json
./llm-import-trace -format bailian < bailian.jsonl > normalized.json
./llm-import-trace -format tracelab < tracelab.jsonl > normalized.json
```

Each example selects its own input; do not overwrite a retained artifact from a
previous trial. Normalization performs no network, model or dataset acquisition.

A Go caller can retain the typed artifact directly:

```go
trace, err := traceworkload.ReadMooncake(input)
if err != nil {
    return err
}
return json.NewEncoder(output).Encode(trace)
```

## Format policies

| Format | Timing and grouping | Counts and cache identities |
| --- | --- | --- |
| Mooncake | Timestamps/delays are milliseconds, without rebasing/capping. First-seen sessions and each session's file order are retained. Missing IDs get stable source-hash/row IDs. | Exactly one input mode: count, literal text, messages or payload. Recorded histories and raw payloads stay independent; live messages retain live context policy. Default hash block size is 512. |
| Bailian | Seconds become milliseconds. Parent-root links define sessions; stable turn-number sorting orders each session. Missing referenced parents, duplicate chat IDs and cycles fail. | Each row's count/hash chain describes the complete request, not a delta. Blocks are 16 tokens. Request type and parent ID remain explicit metadata. |
| TraceLab | Latest input event sets submission (first output if no input). Sessions sort by submission, round index, then source order. Source UTC origin is retained; relative submission, API proxy and think-time proxy are rounded to microseconds then expressed in milliseconds. | Original input/output/reasoning/prefix/new counts are retained. Targets separately reproduce `max(input,1)` and `max(output+reasoning,1)`. Virtual 64-token IDs reuse whole prior prefix blocks and mint new IDs in a session-local namespace. |

Signed 64-bit recorded cache IDs are decimal **strings** in the artifact. They
must not pass through a JavaScript number. These IDs identify declared cache
blocks, not observed cache hits. TraceLab's virtual IDs implement the pinned
conversion, not an empirical prefix graph, prompt-content hash or real tokenizer
identity. Source fields remain in `Turn.Raw`; JSON serialization may change
whitespace or escape spelling. The complete original bytes are identified by
`SourceHash` and should be retained separately.

Optional source counts remain null when absent and zero when reported zero.
TraceLab's adjusted targets are distinct fields with an explicit count policy.
Its API/think times are derived proxies, not measured server latency or TTFT.
The `stop`/input type classification is derived from the source event fields.

## Explicit block sizes

The default readers retain Mooncake's 512-token and TraceLab's 64-token
blocks. For an explicitly configured source, use
`ReadMooncakeWithOptions(reader, BlockOptions{BlockSize: 4})` or
`ReadTraceLabWithOptions(reader, BlockOptions{BlockSize: 4})`. The zero value of
`BlockOptions` is invalid; default behavior belongs to the wrapper functions.
Options require an integer in 1..1000000. Upstream requires a positive size
without this upper limit; 1000000 is this adapter's resource ceiling.

```sh
./llm-import-trace -format mooncake -block-size 4 < trace.jsonl > normalized.json
./llm-import-trace -format tracelab -block-size 4 < tracelab.jsonl > normalized.json
```

An omitted flag selects the format default. Explicit zero, negative, boolean,
fractional and out-of-range values fail; none silently selects a default.
Bailian overrides are currently refused by this CLI, keeping its documented
16-token policy. Selected size and scope are retained on normalized sessions
and authored replay exports (`block_size`, `hash_scope`). Row extras are raw
source metadata, not loader configuration: set the option explicitly rather
than inferring a block size from ancillary row fields.

TraceLab uses the selected size in both prefix reconstruction and whole-import
allocation checks. A smaller size can exceed the existing aggregate block budget
and fail before any virtual chain is allocated. An option does not realize
cache-backed prompts or prove cache hits. This follows pinned
`config/dataset/config.py:555`, `base_trace_loader.py:150` and
`tracelab_trace.py:124` override precedence; caller configuration takes precedence
over plugin defaults.

## Authored request replay

Only authored request content can be exported now. This input needs no synthetic
prompt realization:

```json
{"session_id":"one","messages":[{"role":"user","content":"hello"}],"output_length":8}
```

```sh
./llm-import-trace -format mooncake -output sessions \
  -execution-policy serial-recorded -path /chat/completions \
  -protocol openai-chat -model local-model < authored.jsonl > sessions.json
SESSIONS_FILE="$PWD/sessions.json" ENDPOINT_URL=http://localhost:8000/v1 \
  k6 run examples/workloads/raw-sessions.js
```

This uses the existing raw-session runner and `Client.request`. The explicit
`serial-recorded` policy means **source arrival/delay is not scheduled**: turns
run sequentially in each session, and sessions may interleave across VUs. Original
timing/count/raw metadata remains attached to each exported turn; the policy is
recorded on the exported session. This is authored-body replay, not timing or
capacity parity. The runner logs session ID, turn ID, source hash and outcome;
retain its exit/metrics and reconcile attempted turns before claiming completion.

Recorded messages use the declared model, original tools, optional output limit
and source `extra` precedence. A single-turn literal text becomes one user
message. Raw payloads retain their complete bodies; `-model` does not rewrite
them, and payload-plus-extra export is refused. Text/messages require explicit
`-model` and `openai-chat`; raw payloads support `json` or `openai-chat`. The
response adapter/path are caller declarations, not format detection.

Live/delta multi-turn assembly, count/cache-only rows and zero output limits for
formatted messages are refused for execution. Bailian/TraceLab normalization is
therefore useful data output, but their count-only rows cannot run until pinned
prompt realization and the required cache/session policy are implemented. No
filler text, cache flattening or fabricated token evidence is emitted. Export
refuses numeric values that would change through JavaScript serialization.

## Bounds and narrower behavior

Input is uncompressed UTF-8 JSONL, at most 16 MiB, 100000 nonblank rows, 1 MiB per
line, 10000 sessions, 1000 turns per session, and 1000000 tokens per count.
Nesting is bounded by the shared strict JSON parser. Duplicate JSON members,
null required fields, invalid Unicode/surrogates, fractional integer counts and
out-of-range values fail the whole import. Blank lines are allowed; source row
ordinals are physical line numbers. Output is capped at 64 MiB before writing.
Aggregate limits also apply across the complete import: at most 262144
recorded cache-ID entries or reconstructed virtual block entries, and a 64 MiB
conservative output budget. Source rows reserve 4096 bytes plus 24 times their
raw length; virtual blocks reserve another 24 bytes each. This covers metadata,
JSON escaping and authored export copies. It can refuse an input below the
16 MiB source cap. Recorded IDs are counted in a first pass before allocating
the normalized ID slice. All virtual chains are counted before the first chain
is allocated. Authored export also reserves repeated path/model bytes at the
same expansion factor before constructing replay bodies. Repeated IDs/blocks still consume budget; no deduplication hides
stored output amplification. No records are silently skipped or clipped. Optional keys must be omitted rather
than null. Ancillary fields are retained in raw rows, not executed as options.

Mooncake mixed input/context modes within a session are refused. Bailian dangling
parents/cycles are refused instead of upstream salvage; multimodal request types
are labels/counts only. Timestamps/delays are nonnegative and bounded to one day;
Bailian seconds obey the same day after conversion. TraceLab ISO timestamps
accept UTC/offset or naive UTC forms with at most microsecond precision,
1970–2100; a session spans at most one day. Prefix and newly appended counts must
sum to the input total. Missing usable event times fail instead of dropping rows.
Subagent lifecycle tools (`Agent`, `Task`, `spawn_agent`, `wait_agent`,
`close_agent`) fail: heuristic nesting reconstruction, gzipped input, Weka
reconstruction, model routing and tool-content synthesis remain unsupported.

The reference is AIPerf `f10eb50a1776775d80d7f13396b0f668f14e2c03`:
`dataset/loader/models.py`, `mooncake_trace.py`, `bailian_trace.py`,
`tracelab_trace.py`, `_tracelab_convert.py`, and plugin block-size defaults.
Checked-in `traceworkload/testdata/vendor-oracle.json` was generated by actual
pinned models/grouping/conversion functions on authored fixture rows. It tests
software transformations, not real dataset representativeness, cache hits,
model accuracy or full vendor-format parity.

Empirical prefix synthesis is separate upstream work in
`dataset/synthesis/{prefix_analyzer,radix_tree,synthesizer,empirical_sampler,rolling_hasher}.py`.
Its tree construction, multiplier/sampling/seed policy and tokenizer realization
need their own source-derived API and reference controls. These readers do not
invent a replacement synthesis algorithm.
