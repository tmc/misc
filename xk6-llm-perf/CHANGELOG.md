# Changelog

## Unreleased

- Add OpenAI-compatible streaming usage capture.
- Add TTFT, TTFO, inter-token latency, inter-chunk latency, adjusted latency,
  token throughput, usage discrepancy, and good-request SLO metrics.
- Emit native k6 HTTP request metrics for extension requests.
- Share HTTP transport pools across VUs.
- Add warmup suppression for LLM and native HTTP metrics.
- Add shared prefill concurrency limiting.
- Add round-robin `baseURLs` routing.
- Add configured RTT subtraction through `networkRTT`.
- Add conservative token fallback with `tokenMultiplier`.
- Add trace replay example and production-readiness documentation.
