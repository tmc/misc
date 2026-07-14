# xk6-llm-perf Production Readiness

This file records the current disposition of the remaining state-of-the-art
deltas tracked against AIPerf.

## Implemented

- Streaming usage capture with `stream_options.include_usage`.
- TTFT, TTFO, inter-token latency, inter-chunk latency, request latency, token
  throughput, good-request SLO, and usage discrepancy metrics.
- Native k6 HTTP metrics for extension requests via `httptrace`.
- Shared HTTP transport and shared prefill concurrency limiter.
- Warmup suppression for both `llm_*` and native `http_*` metrics.
- Round-robin `baseURLs` endpoint routing.
- Configured RTT subtraction via `networkRTT`, reported as adjusted latency
  metrics.
- Conservative local token fallback using word count, character density, and
  configurable `tokenMultiplier`.
- Trace/profile replay ergonomics through `examples/trace-replay.js`.

## Explicit Dispositions

Active RTT calibration is not built into the extension. The extension reports
adjusted latency metrics when `networkRTT` is supplied; users should obtain RTT
from their deployment environment or a separate probe. This avoids adding a
background network actor that can perturb k6's own traffic model.

Provider compatibility is validated locally with OpenAI-compatible HTTP and SSE
stubs. Live validation against OpenAI, vLLM, SGLang, TGI, or hosted gateways
requires endpoint credentials and running services, so it is not part of the
offline gate. The request/response contract remains OpenAI-compatible chat
completions.

True model-specific BPE tokenization remains optional because provider-reported
usage is preferred whenever available. The local fallback is deliberately
dependency-light and conservative; exact tokenizer parity can be added later as
an opt-in package if a deployment requires it.
