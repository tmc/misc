# Qualify completed work

Protocol success and SLO compliance do not establish that the server performed
an expected workload. Add `work` to a completion request to record an independent
qualification receipt:

```javascript
client.chat.completions.create({
  messages: [{role: 'user', content: 'Say hello.'}],
  work: {
    expectedModel: 'served-model',
    allowedFinishReasons: ['stop'],
    minOutputTokens: 1,
    maxOutputTokens: 32,
    requireExactUsage: true,
  },
});
```

`measurement.work.status` is `accepted`, `rejected`, or `unavailable`. A protocol
failure is rejected. A known mismatch is rejected even if another check is
unavailable. Missing evidence makes qualification unavailable. Early EOS is
accepted when its finish reason and actual output length satisfy the policy;
there is no requirement to consume the requested maximum token budget.

Bounds use provider-reported completion tokens, including reported zero. They
never substitute an estimate or a caller declaration. Exact usage requires
explicit input, output, and total usage with a consistent sum. Explicitly
contradictory counts are rejected with `exact_usage_inconsistent`; incomplete
counts remain unavailable with `exact_usage_unavailable`. A reported partial
count exceeding the reported total is also a known contradiction. Replay
reconstructs this distinction from provider count presence and the reported
total; `exact_usage=false` alone never establishes missing evidence. The model check
uses the response model, not the requested model. Empty text can qualify when
zero output is allowed. A policy with no checks accepts protocol success.

Optional `textDigest`, `responseDigest`, and `tokenIDsDigest` are lowercase
SHA-256 hexadecimal strings. Text is the UTF-8 content of the normalized primary
choice without trimming; reasoning and tool arguments are excluded.
`responseDigest` hashes Go's compact JSON encoding of an object with `model` and
`choice` keys, in that order. `choice` uses the exported `Choice` JSON schema,
including its normalized message, index, finish reason, and speculative decoding
metadata. IDs, timestamps, transport details, and usage are excluded.
No raw response text is retained. Multi-choice text/response digest and finish
reason qualification is unavailable: the streaming adapter retains only the
primary choice. Output usage remains the provider's aggregate count.

Current adapters do not retain provider token IDs. A `tokenIDsDigest` check is
therefore unavailable; it never hashes text tokens or fabricates IDs. Hashes
provide consistency checks rather than a proof of semantic correctness.

Only opted-in measured attempts emit one `llm_work_acceptance` observation.
`llm_work_accepted`, `llm_work_rejected`, and `llm_work_unavailable` count these
populations. `llm_work_good_requests` counts accepted requests that also satisfy
SLOs. Existing protocol success and `llm_good_requests` retain their meaning.
The policy is harness metadata and is never sent to the provider.

`serialization_latency_ms` measures request payload encoding before dispatch.
`parsing_latency_ms` sums response adapter decode calls after body/event reads;
it excludes waiting for HTTP/SSE reads, and is absent when decoding was never
reached. These are monotonic elapsed durations per operation, not per-event k6
samples. They do not measure all client CPU overhead or network capacity.
