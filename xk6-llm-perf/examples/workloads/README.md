# Reproducible workloads

`examples/lib/workload.js` supplies bounded JSON/JSONL datasets, deterministic
sampling, weighted normal sequence-length targets, exact-payload token receipts,
shared-prefix synthesis, cache-bust markers, absolute arrival replay, synchronous
multi-turn conversations and dependency-aware DAGs. `examples/lib/control.js`
supplies bounded control POSTs and round-robin or seeded endpoint selection.

Run both fixture integrations with a freshly built extension:

```
K6_SMOKE_BINARY=dist/k6 go test . -run 'TestBinary(Workload|Absolute)' -count=1
```

`TestBinaryWorkloadAcceptance` invokes the independent `tools/llm-tokenize`
module against its pinned vocabulary, then imports its actual JSON receipt into
k6. `verifiedCounter` requires an exact text SHA-256, vocabulary SHA-256 and
raw-text scope. Modifying the payload invalidates the count. Preserve the full
receipt (model, revision and tokenizer module) with your corpus. Plain-text
counts do not count chat framing; use the tool's pinned chat-frame workflow for
independent chat counts. The corpus hash from `assetReceipt` covers the raw
source JSON bytes, including row order and whitespace.

A sequence distribution is an array such as
`[{input:64,output:32,weight:70,inputSD:10,outputSD:8}, ...]`. `sequences` returns
seeded integer targets, clamped to 1..1,000,000. `realize` selects from a finite
candidate pool and refuses any unavailable exact input budget. Returned rows
retain target pair identity, output budget, actual input count and provenance.
`distributionReceipt` reports realized pair frequencies and input counts separately
from the declared distribution. The output budget sets a generation limit; it is never a realized output count.
Supply independently tokenized candidate receipts for every target you want.

`cacheSynthesis` counts concatenated payloads, because token counts are not
additive across prefix boundaries. Shared prefixes retain group identities;
cache-bust markers precede prefixes and are unique within a corpus. Provider
cache hits remain observations, not claims inferred from text identity.

`trace` validates unique identities and monotonic offsets bounded to one day.
`replayArrival` waits relative to one epoch, returns intended/actual arrival and
lag, and drops late work without calling the inference callback. Keep all
receipts, including dropped rows: they are workload events outside dispatched
LLM attempts. Partition row indices once with `exec.scenario.iterationInTest`
for a single shared-iterations scenario; never independently sample the same
trace in each VU. Under overload the explicit maxLag policy controls dropping.

Conversations retain context in one VU, stop at the first returned Error, and
wait each turn gap after the preceding response completes. `append` is default;
`independent` resets to the initial context each turn. Validate all turns before
sending. Each turn may include `request` options such as `max_tokens`,
`temperature` or `work`; the helper owns `messages` and `stream` and rejects
those keys, in any case folding, in `request` or its `extensions` before
dispatch. Engine fields the
typed request lacks go in `extensions`, for example
`request: {max_tokens: 256, extensions: {ignore_eos: true}}` to hold vLLM,
SGLang or TensorRT-LLM output length fixed; every turn still appends the live
reply and records its measurement. The client refuses any other extension key
that names a typed field, such as `model`, or has a null or undefined value,
when that turn is sent. See
[multi-turn.js](multi-turn.js).
DAGs validate cycles before side effects; failed parents skip their
descendants, and callbacks receive the completed parent-result map.

Control POSTs run once in setup/teardown around a logical benchmark phase.
They use k6/http with `traffic=control` and never emit extension request metrics;
filter control traffic when interpreting built-in HTTP metrics. Hook timeouts
are greater than zero and at most 60 seconds, redirects disabled, response bodies discarded and failures
fatal. There is no automatic retry: repeating a hook or inference request after
a lost response could duplicate work. Authentication is supplied separately in
headers, which never enter receipts. Endpoint rotation returns endpointIndex
and the original response/Error; failure does not trigger alternate replay.

Host fixtures qualify deterministic behavior and accounting. Provider tokenizer
compatibility, actual KV-cache hits and live server profiler behavior require
separate target-specific evidence.
