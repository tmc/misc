# Percentiles and report windows

Reports preserve their existing conventions unless an AIPerf statistics option
is explicitly selected. The option changes percentile estimation and the
membership of `llm-report` time slices. It does not establish full AIPerf metric,
failed-request population, phase-credit or throughput equivalence.

| Convention | Default | Explicit AIPerf statistics |
| --- | --- | --- |
| Percentile position | Nearest rank, `ceil(q*n)-1` | Linear interpolation at `(n-1)*q` |
| Standard deviation | Population, divisor `n` | Population, divisor `n` |
| Slice membership | Completion in `[start,end)`; final endpoint included | Dispatch start in `[start,end)`; final endpoint excluded |
| Request distributions | Successful retained requests, unavailable values omitted | Same population and availability policy |

For observations `[0,10]`, the default p50 is `0`; linear p50 is `5`, p95 is
`9.5`, and p99 is `9.9`. Population standard deviation is `5` in both modes.
Singleton observations retain their value at every percentile and have standard
deviation zero. Empty populations retain null statistics.

The explicit estimator follows AIPerf
`f10eb50a1776775d80d7f13396b0f668f14e2c03`,
`src/aiperf/metrics/metric_dicts.py:54-109` with `ddof=0`. The slice convention
follows that revision's `metrics/accumulator.py` `query_time_range` mask over
request start timestamps. Local slice membership uses the retained
`dispatched_at` clock anchor. A request starting exactly at a slice boundary
belongs to the later slice even when its completion falls later. A request
starting exactly at the final run endpoint is excluded from these slices.

Select the mode when rendering retained files:

```sh
llm-report -input run.json -slice 1s -aiperf-statistics
llm-output -input run.json -format json -aiperf-statistics
```

`llm-output` accepts the flag for non-streaming JSON, CSV and HTML reports. Its
mode applies to request statistics and pooled recognized-content interval
percentiles. These intervals remain content events, not physical token events.
The JSON `percentile_method` is `linear_n_minus_1`; CSV carries the estimator
column and HTML states the estimator in its policy text. Default reports retain
`nearest_rank`.

For Go consumers, `ReportOptions{AIPerf: true}` selects the same conventions
through `AnalyzeRequestsWithOptions`, `TimeSlicesWithOptions` and
`output.SummarizeWithOptions`. Existing `AnalyzeRequests`, `TimeSlices` and
`output.Summarize` preserve their default behavior. Each distribution analysis
and slice states its policy in the result. Dispatch slices emit
`attempt_selection: "dispatch_start"` and `selected_attempts`; legacy slices
retain `completed_attempts`. The Go `SliceSummary.Attempts` field counts the
requests selected by that slice policy.

AIPerf slices require every retained record's `timestamp_basis` to be
`dispatch Unix clock anchor plus monotonic elapsed time`. Missing historical
clock evidence produces an error before any slices are returned. An explicitly
anchored dispatch at Unix zero remains valid. This requirement validates the
recorded clock claim; it does not authenticate it or calibrate clocks between
hosts.

The option leaves rates divided by the declared run window, success-only metric
populations, missing-count handling, timeline concurrency calculations and
optional bootstrap confidence analysis unchanged. Failure outcomes remain in
outcome counts and do not become zero-latency observations. Slice token totals
use the full successful requests selected by dispatch start; they are not a
stream of individual token arrival times. Retain the source run and selected
policy with comparisons.
