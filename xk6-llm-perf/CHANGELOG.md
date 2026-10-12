# Changelog

## Unreleased

- Include Anthropic cache reads and writes in reported prompt tokens; retain
  optional cache-write counts without changing previously saved receipts.

- Update to k6 v2.3.0 and pin xk6 v1.4.14.
- Add named imports, strict option decoding, JavaScript errors, and API typings.
- Preserve VU tags and fractional milliseconds; honor cancellation for metrics.
- Add event-loop contract tests and an offline built-binary smoke gate.
- Fix concurrent trace replay indexing and SSE field parsing.
- Return streaming usage and metadata; bound HTTP error body reads.
- Add OpenAI-compatible streaming usage capture.
- Add TTFT, TTFO, content-chunk intervals, inter-chunk latency, adjusted latency,
  token throughput, usage discrepancy, and good-request SLO metrics.
- Emit native k6 HTTP request metrics for extension requests.
- Share HTTP transport pools across VUs.
- Add warmup suppression for LLM and native HTTP metrics.
- Add shared prefill concurrency limiting.
- Add round-robin `baseURLs` routing.
- Add configured RTT subtraction through `networkRTT`.
- Add heuristic token fallback with `tokenMultiplier`.
- Add trace replay example.
