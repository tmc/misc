# Replay authored inputs.json sessions

`llm-import-inputs` converts a local AIPerf inputs.json file to the sessions
consumed by [raw-sessions.js](../examples/workloads/raw-sessions.js). It performs
no network requests. Build it from this repository:

```sh
go build -o ./llm-import-inputs ./cmd/llm-import-inputs
./llm-import-inputs -path /chat/completions -protocol openai-chat < inputs.json > sessions.json
SESSIONS_FILE="$PWD/sessions.json" ENDPOINT_URL=http://localhost:8000/v1 \
  k6 run examples/workloads/raw-sessions.js
```

The k6 binary must register `k6/x/llm-perf`. Set `API_KEY` if needed. Conversion
output contains the full authored payloads, which can include sensitive data;
retain it with the same access controls as the source input.

For example, this input describes two independent authored turns in one session:

```json
{"data":[{"session_id":"example","payloads":[{"model":"local-model","messages":[{"role":"user","content":"First question"}],"max_tokens":16,"stream":false},{"model":"local-model","messages":[{"role":"user","content":"Independent question"}],"max_tokens":16,"stream":false}]}]}
```

The converter retains the session ID and turn order. It assigns each turn a
zero-based decimal ID and records SHA-256 of the exact input bytes on each
session. Output is an array of `{session_id, source_sha256, turns}`; each turn
contains `{id, request:{path, protocol, body}}`, directly accepted by
`Client.request`. Choose `json` for generic JSON responses or `openai-chat` for
chat completion response accounting. Other response adapters are deliberately
refused by this converter. The path and adapter are caller declarations.

The runner uses `shared-iterations`, with one iteration per session and
`iterationInTest` selecting each session once. Turns execute sequentially on
that session's VU. Bodies remain complete requests: the runner inserts no
messages, previous responses, model, streaming flag or token limit. Separate
sessions may interleave when `VUS` is greater than one (maximum 64). Each HTTP
request has a 30-second timeout; the scenario has a five-minute limit. It attempts
remaining authored turns after a failure, logs a session/turn/outcome receipt,
and fails its checks and failure-count thresholds. Logs omit request bodies.
Transport or provider failure does not disappear from the attempted population.
A scenario timeout can leave turns unattempted; retain the k6 run outcome and
reconcile log identities against the converted input before claiming completion.

| Scenario | Executor | Acceptance question |
| --- | --- | --- |
| `raw-sessions.js` | shared iterations, one per session | Are each session's independent authored payloads dispatched in order and are failed attempts retained? |

## Bounds and compatibility

The reference is AIPerf `f10eb50a1776775d80d7f13396b0f668f14e2c03`,
`src/aiperf/dataset/loader/inputs_json.py:21` and
`src/aiperf/dataset/loader/models.py:478`. Its loader preserves raw payloads and
rejects duplicate session IDs. This converter implements a stricter offline
subset; it does not reproduce all Python coercions or transport behavior.

Input is at most 16 MiB, with 1–10000 sessions, 1–1000 payloads per session,
100000 total turns, and at most 1 MiB per payload. The only root key is `data`;
each entry has exactly `session_id` and `payloads`. Session IDs must be unique,
nonempty strings of at most 256 UTF-8 bytes, without NUL or line breaks.
Payloads must be JSON objects. Empty objects and nested null values are retained;
null sessions, null payloads and empty turn lists are refused. Duplicate JSON
members, trailing documents, nesting beyond the shared parser limit, invalid
UTF-8 and lone UTF-16 surrogate escapes are refused.

Payload preservation is semantic, not byte-for-byte: output and the client
serialize JSON again. To avoid silent JavaScript number loss, number spellings
are limited to 128 bytes, integral values to the safe integer range, and other
numbers must retain their decimal value through float64's shortest JSON spelling.
Negative zero, overflowing/underflowing and excessive-precision numbers are
refused. Ordinary values such as `0.1` are accepted. The original input hash
identifies the source bytes, not the final wire serialization.

Conversion and runner-spy controls establish software behavior only. Actual k6
registration, on-wire requests and provider/model semantics require their own
pinned binary and endpoint receipts. This workflow makes no capacity or
performance equivalence claim.
