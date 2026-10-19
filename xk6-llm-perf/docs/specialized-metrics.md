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

## Observed token aggregates

`AnalyzeSpecialized(run, SpecializedAggregateOptions{})` validates a complete
retained run and aggregates successful local `measurement` records. It reports
exact integer reasoning, output and cache-read/write sums with observed/missing
counts, source histograms and completeness flags. Default values require every
selected request to supply that field. Default overall thinking efficiency
additionally requires reported output and consistent reasoning subsets.

`SpecializedAggregateOptions{AIPerf: true}` selects the pinned independent-sum
policy: each available operand enters its own sum. Overall thinking efficiency
is `sum(reasoning) / sum(output)`, never the mean of request ratios and never a
paired-subset ratio. For `(reasoning=2, output=4)` followed by
`(reasoning=missing, output=6)`, it returns `2/10 = 0.2` with the missing numerator
observation recorded. If only the second output is missing but its reasoning is
`3`, it returns `5/4 = 1.25` and marks the partial operand population. It does not
clamp a source-formula result or claim complete-population coverage.

This mode accepts each validated output count source and retains its histogram;
reasoning and cache fields retain their reported meaning. A present zero
reasoning/cache sum remains zero. Zero output sum makes the ratio unavailable.
The observed output field can retain reported zero even though the pinned
upstream `OutputTokenCountMetric` omits zero from its per-record metric array.
This does not change the ratio's availability or arithmetic. Missing all
reasoning values never becomes a zero numerator. Integer overflow refuses the
analysis, and observed sums do not pass through floating-point JSON values.

The result separates computed values from `thinking_qualification`, operand
completeness and source coverage. Failed requests remain in accounting but
supply no successful specialized operands. Phase is the existing local
measurement population; there is no inferred upstream profiling credit or
endpoint capability. Runs containing warmup records or missing retained attempts
are refused by the existing run validator. This API does not emit k6 metrics or
change saved records.

The actual producers already decode response usage in `Usage.UnmarshalJSON`,
retain last present streaming fields in `mergeUsage`, and copy them in
`Client.finishMeasurement`. Offline controls exercise these exact producers and
explicit-zero metric emission without HTTP. They establish fixture mechanics,
not live-provider semantics.

| Remaining family | Pinned definition | Missing schema/evidence |
| --- | --- | --- |
| Overall thinking efficiency | Sum reasoning / sum output; not mean of request ratios | The aggregate API above supplies complete and independent-observed modes. Upstream phase-credit and endpoint-capability correspondence remain separate. |
| Image count, throughput and latency | media_counts.images; images / last-content seconds; last-content milliseconds / images (`image_metrics.py`) | Serialized request bodies and multipart parts are available, but endpoint-specific extraction and retained request-body-bound counts are a source implementation gap. Existing Measurement has no media count. Input image counts must not be invented from text tokens or treated as generated-image counts. |
| Audio duration producer | request_info.audio_duration_seconds (`audio_duration_metric.py`) | Uploaded bytes are already available to multipart encoding. A bounded decoder and asset/request-body hash binding are a source implementation gap; the existing input API remains a declaration. Unsupported codecs and live ASR qualification are separate. |
| Credit-to-start latency | (request_start_ns - credit_issued_ns) / 1e6, ms (`derived_latency.py`) | Scheduler credit timestamp and shared clock lineage. AdmissionWait/ArrivalLag are different observations. |
| Credit-drop latency | request start - credit-drop received timestamp, ns internally (`credit_drop_latency_metric.py`) | Actual worker credit-drop event. Pinned upstream treats false/zero internal credit-drop latency as unavailable; this helper introduces no substitute. |
| Prompt cache misses | Last non-null normalized explicit provider cache-miss count (`usage_cache_metrics.py`) | Explicit provider usage bytes can carry this count, but current Usage/Measurement lack a separate miss field. Adding normalized presence-aware extraction/stream merging is a source implementation gap. Never manufacture it by subtraction. |
| Theoretical prefix-cache hit | Workload block overlap accounting (`theoretical_prefix_cache.py`) | Loader/trace block metadata needs a source-hash-bound dispatch/record receipt with block size and successful phase population. This is a source implementation gap, separate from measured provider cache reads. |

These are source/software helpers. Host controls, declared media inputs and
matching arithmetic do not establish live-provider semantics, measured cache
hits, image/audio performance, scheduler equivalence or full specialized-metric
coverage.
