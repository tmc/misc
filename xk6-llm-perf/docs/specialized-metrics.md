# Specialized per-request metrics

Reference: AIPerf `f10eb50a1776775d80d7f13396b0f668f14e2c03`.
The Go methods below produce descriptive post-run values with explicit units,
count sources and unavailable reasons. They do not emit k6 metrics or change
saved Measurement records. A nil value exports as JSON null; reported zero is
retained. Callers can marshal the returned structs with encoding/json.

Use records admitted by `ReadRun`, or validate the equivalent complete record
before calling these methods. Operand and population checks do not validate
every measurement invariant or authenticate its producer.

`Measurement.SpecializedMetrics()` selects recorded, attempted, successful
`measurement` records. Warmups, failed requests and unrecorded attempts have no
specialized values. This is a deliberately narrower population than some
upstream metric processors; it is not universal M08 parity.

| Output | Formula and unit | Required evidence |
| --- | --- | --- |
| thinking_efficiency | reported reasoning / reported total output, ratio | Both nonnegative; output positive; reasoning cannot exceed total output. Reasoning is already part of output, never added again. |
| prefill_throughput_per_user | input count / (TTFT milliseconds / 1000), tokens/s/user | Streaming record, finite positive TTFT, nonnegative input count with reported, client_declared or estimated source. Returned source preserves that distinction. |
| usage_prompt_cache_read_tokens | reported CachedTokens, tokens | Present nonnegative reported count, including zero. |
| usage_prompt_cache_write_tokens | reported CacheWriteTokens, tokens | Present nonnegative reported count, including zero. |

The pinned definitions are in
[`thinking_efficiency_metrics.py`](https://github.com/ai-dynamo/aiperf/blob/f10eb50a1776775d80d7f13396b0f668f14e2c03/src/aiperf/metrics/types/thinking_efficiency_metrics.py),
[`prefill_throughput_per_user.py`](https://github.com/ai-dynamo/aiperf/blob/f10eb50a1776775d80d7f13396b0f668f14e2c03/src/aiperf/metrics/types/prefill_throughput_per_user.py),
and [`usage_cache_metrics.py`](https://github.com/ai-dynamo/aiperf/blob/f10eb50a1776775d80d7f13396b0f668f14e2c03/src/aiperf/metrics/types/usage_cache_metrics.py).
The helper additionally rejects inconsistent reasoning subsets and requires
reported reasoning/output sources for comparison. Prefill is a client timing
proxy, not measured server prefill execution. Cache reads and writes do not
prove a physical cache hit or imply a cache-hit ratio.

`Measurement.AudioMetrics(AudioMetricInput)` accepts duration as a separate
post-run input. It requires an `audio-transcription` record from the same
successful measurement population, matching nonempty RequestID, a positive
finite duration, literal unit `s`, nonempty duration-producer description and
a lowercase SHA256 identifying the source asset. These checks validate the
input schema and lineage declarations; they do not decode the asset or prove
that the declared digest/duration matches its bytes. The result retains that
input. Nonfinite declared durations are exported as null with an unavailable
reason, so invalid numeric input cannot poison JSON output.

Audio duration is in seconds. RTFx is duration seconds divided by positive
finite LastContentLatency seconds. It excludes a usage-only trailer and does
not substitute full HTTP response duration. A ten-second clip with a
one-second last-content latency produces ratio10. The pinned definitions are
[`audio_duration_metric.py`](https://github.com/ai-dynamo/aiperf/blob/f10eb50a1776775d80d7f13396b0f668f14e2c03/src/aiperf/metrics/types/audio_duration_metric.py),
[`rtfx_metric.py`](https://github.com/ai-dynamo/aiperf/blob/f10eb50a1776775d80d7f13396b0f668f14e2c03/src/aiperf/metrics/types/rtfx_metric.py),
and [`request_latency_metric.py`](https://github.com/ai-dynamo/aiperf/blob/f10eb50a1776775d80d7f13396b0f668f14e2c03/src/aiperf/metrics/types/request_latency_metric.py).
No field is added to Measurement, and no duration is inferred from tokens or
payload byte count. Real ASR/media and hardware qualification remain open.

Runnable examples accompany both Measurement methods and their result/input
types in specialized_metrics_example_test.go.

| Remaining family | Pinned definition | Missing schema/evidence |
| --- | --- | --- |
| Overall thinking efficiency | Sum reasoning / sum output; not mean of request ratios | Explicit complete run population and independent numerator/denominator availability/aggregation policy. This helper returns per-request values only. |
| Image count, throughput and latency | media_counts.images; images / last-content seconds; last-content milliseconds / images (`image_metrics.py`) | Observed request media-count lineage through endpoint extraction; image-capability attribution. Existing Measurement has no media count. Input image counts must not be invented from text tokens or treated as generated-image counts. |
| Audio duration producer | request_info.audio_duration_seconds (`audio_duration_metric.py`) | Asset decoding or source duration receipt bound to the actual request. The explicit input API retains a declaration rather than verifying the producer. |
| Credit-to-start latency | (request_start_ns - credit_issued_ns) / 1e6, ms (`derived_latency.py`) | Scheduler credit timestamp and shared clock lineage. AdmissionWait/ArrivalLag are different observations. |
| Credit-drop latency | request start - credit-drop received timestamp, ns internally (`credit_drop_latency_metric.py`) | Actual worker credit-drop event. Pinned upstream treats false/zero internal credit-drop latency as unavailable; this helper introduces no substitute. |
| Prompt cache misses | Last non-null normalized explicit provider cache-miss count (`usage_cache_metrics.py`) | Measurement has no separate miss count; preserve actual protocol usage before deriving. Never manufacture it by subtracting cache reads from input tokens. |
| Theoretical prefix-cache hit | Workload block overlap accounting (`theoretical_prefix_cache.py`) | Ordered token blocks, block size and explicit theoretical population, separate from reported provider cache reads. |

These are source/software helpers. Host controls, declared media inputs and
matching arithmetic do not establish live-provider semantics, measured cache
hits, image/audio performance, scheduler equivalence or full specialized-metric
coverage.
