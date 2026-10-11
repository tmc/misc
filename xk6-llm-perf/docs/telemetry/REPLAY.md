# Offline GPU telemetry adapter

`llm-telemetry` imports a bounded continuous stream and exports a benchmark
`TelemetryWindow` plus per-field coverage and statistics. This is a local
consumer interchange named `llm-telemetry-v1`. It is not the wire schema of a
native nvidia/x/nvtelemetry collector. A producer adapter must convert its own schema
explicitly and retain its original manifest and raw bytes.

Run an offline import with explicit window and gap bounds:

```
go run ./cmd/llm-telemetry -input samples.jsonl -start 1000 -end 3000 -max-gap 1.5
```

Times are Unix milliseconds, monotonic elapsed milliseconds, and seconds for
coverage/gap durations. Readings carry canonical units, evidence, and either a
floating gauge, integer counter, or unavailable reason. Integer energy and
throttle bitmasks never pass through floating JSON numbers. Power is watts,
energy is millijoules, activity and occupancy are ratios, memory is bytes,
clocks are hertz, traffic is bytes/second, and temperature is Celsius. These
quantities are separate fields, not interchangeable measures of GPU efficiency.

Each JSONL line has a `kind`: exactly one initial `manifest`, samples, optional
ordered `phase` markers (warmup, measurement, cooldown), and one complete `end`.
The manifest specifies schema version 1, definition version
`llm-telemetry-v1`, run/collector/clock identities, clock uncertainty, device
UUID/model/driver/backend/scope, and explicit endpoint-to-device mappings.
Samples retain source observation time, query start/end, monotonic ordering,
and per-field evidence. Missing observation timestamps are rejected; query
completion does not establish sensor observation time. Terminal lost records,
unknown fields/versions, duplicate device/field identities, query overlap,
nonmonotonic device observations, truncated exports, and post-terminal records
are rejected. Bounds are 64 MiB, 100000 records, 1 MiB per line, 256 devices,
and 64 readings per sample. The exact input SHA-256 identifies the retained raw
stream. Public struct replay is revalidated through the same parser.

For gauges, replay clips adjacent sample segments to the requested window and
uses linear interpolation and trapezoidal duration weighting. Means/extrema
include coverage and missing duration. Missing readings and gaps larger than
`max-gap` break coverage. Sensor age at query completion is limited by the same
explicit bound. Counter resets inside overlapping segments are refused; no
rollover is inferred. Counter energy requires exact observation samples at both
window boundaries, subtracts uint64 values before conversion to joules, and
never interpolates cumulative boundaries. Power integration is an explicit
estimate. An available energy counter takes precedence over power; incomplete
counter coverage does not silently become an estimate.

The output `window` can be marshaled as a standalone JSON file and supplied to
`llm-report -telemetry`, or passed to `AnalyzeEnergy`. That join requires complete
request records, matching run/window/clock uncertainty, explicit mappings,
whole-device scope, full coverage and acceptable gaps. Energy includes idle
power and other workloads; it establishes neither request attribution nor MIG
energy allocation. Successful output counts retain their provenance. Zero-token
and zero-energy denominators remain unavailable.

Host tests use irregular cadence with an analytic power mean of 5.25 watts,
10.5 joules integrated power, and 8 joules counter difference above 2^53. Tests
also refuse missing samples, resets, gaps, unknown clocks, wider counter windows,
partial devices, MIG scope, invalid exports and units. These are synthetic
consumer receipts. Native collector delivery, actual unit/cadence/reference
comparison, collection-on/off overhead, each required GPU/MIG cell, cross-host
clock qualification, and matched AIPerf comparison remain separate external
gates. No hardware job or live acquisition is performed by this command.

Native schema 1 can be inspected without converting unknown sensor times or
clock uncertainty into exact benchmark observations:

```
go run ./cmd/llm-telemetry -input native.jsonl -input-format nvtelemetry-v1
```

The diagnostic report retains the original hash, manifests, counts, phases and
qualification gaps. Requested window/energy conversion is refused for this
legacy format. See [the native contract](NATIVE-CONTRACT.md) for the agreed
producer boundary and candidate version 2 diagnostic support and its remaining adoption gates.
