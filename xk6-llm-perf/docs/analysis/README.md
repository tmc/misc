# Retained-run analysis

`go run ./cmd/llm-report -input outputs/run.json -slice 1s` derives request
statistics, client concurrency and rate timelines, and completion timeslices.
Add `-records outputs/records.jsonl` to require the JSONL record sequence to
match the manifest. Unknown fields, unsupported versions, duplicate nonempty
request IDs, negative counts, contradictory aggregates and incomplete replay
populations cause an error. Token aggregate overflow preserves records and outcome
accounting, sets `aggregate_error`, and exports all derived rates as null. Numeric
token totals in that diagnostic summary are partial, never valid complete totals.
`ReadRun`, `WriteRun` and all replay analyses refuse `aggregate_error`; ordinary
JSON marshaling remains available to retain the diagnostic receipt. `ReadRun` accepts explicitly incomplete manifests;
timeline, distribution, confidence and reconciliation analyses refuse them.

Global aggregates contain requests wholly inside the measurement window.
Timeslices allocate each retained completion to `[start,end)`, with the final
run endpoint included. A crossing request's concurrency is clipped per slice;
its successful tokens belong to its completion slice. Failed requests contribute
attempt/failure counts and request concurrency, never success token totals.
Slice input and output throughput divide successful tokens by slice duration.

Effective timeline distributions include idle time. Active distributions retain intervals with an eligible phase in flight,
including known zero-count phases. Quantiles are duration-weighted nearest rank: at an exact
cumulative-duration tie, the lower value wins. End/start edges at the same time
have no positive-duration overlap. Empty active populations have zero duration;
their zero statistics do not constitute a measured active speed.

Input phase rates assign successful input tokens uniformly from dispatch to
first content. Output phase rates assign successful decode tokens uniformly from
first to last content. These are client phase proxies in tokens/s, not observed
individual token arrivals or GPU service rates. Missing timing/counts, zero-length
phases and failed requests are excluded and counted. The output proxy excludes
multi-choice requests. A known zero count is eligible and retains its active phase duration.
Combined rates sum eligible input/decode phases. Per-user rates divide each
segment rate by its eligible phase concurrency before duration weighting.
The output rate uses `decode_tokens` (output minus first chunk), so its effective
mean differs from aggregate output throughput when the first chunk has tokens.

Request distributions use successful whole-contained requests; terminal outcomes
are counted separately. Missing values are excluded with explicit coverage. Empty
populations emit null statistics. Quantiles use `ceil(p*n)` without interpolation;
standard deviation is the population standard deviation. Full-response and decode
speeds have separate units and populations. A zero ITL does not produce an infinite
speed. Aggregate throughput is a token-total/window ratio, not a mean request speed.

Optional `-bootstrap 1000 -seed 42 -independent-observations -confidence .95`
computes a seeded percentile bootstrap of the successful-request arithmetic mean
full-response latency. This explicitly assumes independent identically distributed
requests. Correlated turns and arbitrary repeated runs do not establish that
assumption. Campaign convergence and matched AIPerf qualification are separate.

`WriteRun`/`ReadRun` and `WriteRecords`/`ReadRecords` are bounded export interfaces.
JSONL permits at most 100000 records, 1 MiB per row and 64 MiB total. A run export
is bounded to 64 MiB. Sink errors and short writes propagate. The exported records
contain sanitized measurement metadata; these writers cannot sanitize arbitrary
strings that callers place in model, endpoint or manifest fields. Use the client
producer to obtain its credential-free records and keep authored workload secrets
out of manifest values.

Host fixtures cover idle gaps, overlaps, equal-time boundaries, duration-weighted
versus request-weighted rates, zero/missing speeds, failures, count overflow,
contradictory JSONL, unknown fields, short writers and seeded estimator refusals.
Live matched AIPerf runs require both revisions and retained raw exports, matched
workloads/windows/count assets and an explicit tolerance; these host tests do not
supply that receipt.
