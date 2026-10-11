# Native window admission

Enable explicit native admission:

```sh
llm-telemetry -input capture.jsonl -input-format nvtelemetry-v2 \
  -mapping mapping.json -start 1000 -end 3000 -max-gap 1.1 \
  -max-clock-uncertainty 0.1
```

Without a mapping, native input remains diagnostic; schema1 never supplies benchmark efficiency.
The start and end use Unix milliseconds. Gap uses seconds; uncertainty uses
milliseconds.

The mapping is a schema1 JSON object. Supply `source_hash` of the original
capture bytes, `producer_revision` and `producer_digest` matching the producer
manifest, `capture_run_id`, `benchmark_run_id`, and the exact producer `epoch_id`.
`unix_origin_nanos` maps producer process monotonic zero to Unix time;
`valid_from_nanos` and `valid_until_nanos` must cover the entire capture in that
process epoch. `uncertainty_nanos` must be an explicit nonnegative number.
`evidence` identifies the caller-provided calibration receipt. The consumer
does not independently verify its physical provenance.
`endpoint_devices` maps benchmark endpoint names to device UUID lists. This
mapping asserts measured evidence; the consumer does not perform calibration.
Unknown fields, duplicate keys, null uncertainty and trailing JSON are rejected.

Only complete loss-free schema2 streams with stable, explicitly disabled MIG
whole-device identity, or fully identified MIG instances for gauge summaries are admitted. Sensor timestamps require Unix nanoseconds
or the exact producer process monotonic domain. Per-field observation times are
processed independently of enclosing query intervals. Unknown clocks, wall
steps, missing fields, changed backends, unavailable coverage and interval-valued
observations fail closed or retain missing coverage. Integration averages require a caller `field_timing` policy keyed by native field
name, with `anchor: "end"` and nonempty `evidence`. The observation timestamp
anchors the interval end. Nonoverlapping intervals contribute duration-weighted
averages over their intersection with the window; missing coverage remains
explicit. Partial overlap requires `aggregation: "piecewise_constant_estimate"`;
it assumes a constant gauge across the interval and labels the estimate.
Qualified whole-device interval power can supply estimated power-integral
energy. Counter intervals never become interval averages. Events and traffic counters stay in raw
records; throttle bitmasks have no numerical average.

`allow_query_estimates: true` permits explicitly labeled nonenergy query-time
estimates when sensor timestamps are absent. These estimates cannot supply
energy. Power integrals remain estimates. Cumulative energy remains integer
millijoules; a bracketing counter interval retains its actual start/end offsets.
`AnalyzeEnergy` refuses a wider interval, partial coverage, reset counter or
incompatible run identity. MIG gauges require parent UUID, explicit GPU/compute instance IDs (zero is valid),
and stable identity. They never supply energy. Whole-device energy never attributes consumption to
MIG, a process or a request. Keep the raw capture alongside the mapping and
qualification receipts.

Portable synthetic tests validate admission mechanics. They do not qualify
physical timestamp accuracy, GPU collection overhead or independent energy
accuracy. Producer fields must come from `github.com/tmc/lib/nvidia/x/nvtelemetry`;
no second collector is introduced here.

For a producer reporting MIG discovery `unsupported`, an optional
`entity_scopes` mapping keyed by device UUID can declare `scope: "whole-device"`
and nonempty `evidence` identifying an external physical identity receipt.
Admission requires a stable `GPU-` UUID with no parent, enabled MIG or instance
ID presence/value. This preserves `unsupported` in raw evidence; it does not
rewrite discovery as disabled MIG. Unknown, permission denied and MIG instance
scopes cannot use the override. Physical proof provenance remains an external
qualification gate. This override supplies identity only; it cannot supply
missing sensor timing or promote query estimates to energy.

Field qualification is independent. An unknown sensor clock, stale/cache
observation, incompatible field evidence or missing interval convention leaves
that field unavailable with the entire window reported as missing and no
numeric summary. It does not discard other qualified gauges. Query-only energy
always remains unavailable, even when nonenergy query estimates are enabled.
Raw integer counters and capture bytes remain unchanged. Invalid capture
identity, process mapping, clock steps or entity scope still refuse the join.
