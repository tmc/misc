# Failure-adjusted latency distributions

`AnalyzeAdjustedLatencies` computes a separate failure-inflated view from a
validated `RunSummary`. It does not change existing statistics or output defaults.
The reference is AIPerf `f10eb50a1776775d80d7f13396b0f668f14e2c03`:
`metrics/accumulator.py:418–450`, `metrics/derived_latency.py:304–418`, and the
flagged metric classes in `metrics/types/`.

For each metric with available successful observations, append one positive
infinity for every failed record in the same window. Timeout and cancellation
records also contribute infinity; their elapsed time is not substituted for a
successful latency. Missing successful observations contribute no sample and
are counted separately. Thus `Count = SuccessfulObservations + Failures` can be
smaller than attempts. The error population includes all failures in the window,
including failures from nonstreamed requests when the successful metric is
stream-only, matching the pinned accumulator's common error count.

| Adjusted metric | Successful observation | Unit |
| --- | --- | --- |
| `adj_request_latency` | `LastContentLatency` | ms |
| `adj_time_to_first_token` | `TTFT` | ms |
| `adj_inter_token_latency` | `ITL` | ms/token |
| `adj_full_response_inter_token_latency` | `FullResponseITL` | ms/token |
| `adj_decode_duration` | `LastContentLatency - TTFT` | ms |
| `adj_full_decode_duration` | `FullResponseLatency - TTFT` | ms |

All but request latency carry upstream `STREAMING_TOKENS_ONLY`. These families
require persisted `Stream=true`; populated timing pointers alone do not infer
missing historical stream identity. Existing validated timing/token operands
supply the observations. The upstream independent endpoint
`PRODUCES_TOKENS_ONLY` flag is not encoded separately in this run schema; this
adapter covers the client's supported text/token timing subset. Match endpoint
eligibility independently before claiming complete upstream population parity.

For the sorted inflated distribution, percentile `q` selects index
`roundToEven((Count-1)*q)`. This is NumPy's `method="nearest"`, including ties to
even indices. It is distinct from ordinary linear interpolation and the existing
nearest-rank policy. The report retains p1, p5, p10, p25, p50, p75, p90, p95 and
p99. With a failure present, sum, mean and maximum are infinity, minimum is the
smallest available successful observation, and standard deviation is unavailable.
The native upstream nanosecond values are converted to milliseconds; ITL retains
its token denominator.

JSON uses `{"value":null,"positive_infinity":true}` for infinity and
`{"value":0,"positive_infinity":false}` for observed zero. An unavailable value
has a null value and false infinity flag. It never emits NaN or Infinity literals.

The pinned implementation emits no adjusted companion without errors or without
an available success-side distribution. The API retains a named unavailable
status and diagnostic populations for those cases. Dropped records make every
family unavailable rather than reconstructing unknown observations. The
`adj_effective_latency` status is always unavailable: upstream requires
`credit_issued_ns`, which admission wait and scheduled timestamps cannot replace.

Window selection uses the run's existing retained records. Boundary exclusions,
inflight work and dropped counts stay in the report. This does not reproduce an
arbitrary upstream timestamp-mask slice. Unattempted invalid local calls do not
appear in this run's retained attempt population; verify that the compared
upstream error window uses the same admission rules. No success-only filtering
can repair a mismatched error population.

```go
report, err := llmperf.AnalyzeAdjustedLatencies(run)
if err != nil {
    return err
}
metric := report.Metrics["adj_request_latency"]
if metric.Available && metric.Percentiles["p99"].PositiveInfinity {
    // Failure inflation puts this percentile above every finite latency.
}
```

Integrations should expose this report only through an explicit failure-adjusted
mode, preserve availability and infinity flags, and retain its estimator and
population policy. A fixture/oracle match establishes software arithmetic and
accounting, not on-wire timing, endpoint equivalence or performance.

Use the explicit CLI flag to add the independent `aiperf_adjusted_latencies`
JSON field:

```sh
llm-report -input run.json -aiperf-adjusted-latencies > report.json
```

This does not require `-aiperf-statistics`, select a different timestamp window,
or change ordinary success-only request distributions. Without the new flag,
the field is omitted. Adjusted infinity values remain valid explicit JSON values.
A no-error run includes the named no-companion statuses. The existing CLI still
refuses dropped-record runs because its timeline and slice reports require all
attempt records; it emits no partial JSON. Use the separate API to inspect the
adjusted unavailable statuses for such a run.
