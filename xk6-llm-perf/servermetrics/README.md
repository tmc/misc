# Server metric recording and replay

Build `go build ./cmd/llm-scrape`. Record configured URLs:

```
llm-scrape -targets targets.json -duration 30s -interval 1s > scrapes.jsonl
llm-scrape -input scrapes.jsonl -window window.json > analysis.json
```

`targets.json` is an array of complete HTTP(S) URLs. The file is reread each
scrape tick. Each observation retains the target inventory, raw exposition,
SHA-256, query start/end, parsed samples, or an explicit error. Serial HTTP
queries use a timeout and refuse redirects. Recording failures and budgets
produce a nonzero command exit and an incomplete receipt on stderr.

For configured Kubernetes discovery, supply `-pods pod-list.json -namespace
bench`. An authorized caller obtains the PodList; the tool reads it repeatedly.
Only Running pods in that explicit namespace with `prometheus.io/scrape=true`
and an annotated port are selected. API authorization, RBAC, live discovery
updates and target reachability require separate cluster qualification.

The window JSON contains RFC3339 `start`/`end`, `max_gap` in nanoseconds,
`targets`, and optional exact-name `units`. Replay reparses retained raw data and
checks its hash. It uses only adjacent query-completion intervals wholly inside
the declared window. Counter resets, disappearing labels, failed scrapes,
unavailable readings and cadence gaps retain issues; none become zero samples.
Gauge means use trapezoidal integration. Counter rates divide the observed
increase by covered time, not the full window. Coverage is explicit. Unknown
units remain unknown; explicit units override suffix inference. SI size units
use powers of 1000. Histogram buckets and counts remain counts; their boundaries
and sums use the family unit.

Histogram quantiles use cumulative bucket increases over compatible complete
intervals. They assume uniform observations within each bucket; an infinite
terminal bucket returns its finite lower bound. Summary exporter quantiles are
not merged or represented as exact latency observations. Client-side timings
and server metrics remain distinct measurements.

The host fixtures qualify parsing, configured discovery, replay, unit conversion,
resets, missing labels/targets, histogram interpolation, cancellation and CLI
failure handling. They do not qualify live exporter overhead, Kubernetes
credentials/watch behavior, or semantic equivalence between vendor metrics.
