# Output metric contract

`llm-output` produces sanitized summary schema version 2. Run input remains
schema version 1. Summary consumers must accept version 2 before upgrading;
`examples/output/wandb_export.py` accepts both summary versions.

All request statistics use successful retained records. Their `count` is the
number of available observations, so omitted values differ from observed zero.
`retained_records`, `retained_successes`, `dropped_records`, boundary exclusions,
in-flight requests and missing/estimated token counters describe coverage.
Token statistics include counts by provenance, including unavailable counts.
Rates use aggregate counts divided by the declared run window, including idle
time, rather than averaging per-request rates. An incomplete aggregate fails
validation; the exporter does not report unknown totals as zero throughput.

| Metric | Endpoint |
| --- | --- |
| `full_response_latency` | Dispatch through HTTP response completion, including trailing usage |
| `last_content_latency` | Dispatch through last nonempty content |
| `request_latency` | Legacy alias of `full_response_latency` |
| `ttft` | Dispatch through first token-bearing content, including reasoning |
| `ttfo` | Dispatch through first visible output |
| `itl` | Existing canonical content-based decode-token estimate |
| `full_response_itl` | Canonical estimate including response completion |
| `max_chunk_latency` | Canonical maximum observed content-chunk gap |

`request_latency` remains available in version 2 with its original endpoint.
Use the explicit names in new consumers. It is deprecated as an ambiguous name;
removal requires a future summary major version. AIPerf's request latency uses
last content, so comparisons must map it to `last_content_latency` explicitly.
Renaming exports does not qualify parser accuracy or numerical parity.

Durations are milliseconds in JSON, CSV and HTML; ITL estimates are
milliseconds per token (corrected from version 1's ambiguous `ms` label). OTLP
ITL gauges use seconds per token. OTLP and live MLflow durations
are seconds. `gen_ai.client.operation.duration` continues to measure the entire
HTTP operation, including failures. Canonical extra duration gauges use the
`llmperf.client.` namespace; live MLflow prefixes those names with `live.`.
OTLP includes retained successes and failures individually, whereas summary
percentiles use retained successes only. Missing observations emit no gauge.

Summary percentiles use nearest rank: sorted observation at `ceil(p*n)`, with
one-based indexing. Standard deviation divides squared deviations by `n`
(population estimator). JSON names both estimators explicitly; CSV carries these
names and the population policy on each row. These differ from interpolated
percentiles; do not silently match an interpolating consumer's numbers.

Additional summary fields preserve full-response ITL, maximum chunk latency,
admission wait, arrival lag, scheduled response latency and completed HTTP
transport phases. Reused-connection setup phases remain unavailable. The JSON
`timeline` preserves duration-weighted request/prefill/decode concurrency and
client phase-rate proxy semantics from canonical analysis. Dropped records make
that timeline unavailable. HTTP waiting and client phase rates are not server
latency or GPU activity measurements.

Metric exports contain no prompt, generated text or sink credentials. Sanitized
run records cannot reconstruct generated text or per-token event vectors.
Accuracy workflows use explicitly supplied prediction artifacts and independent
prompt inputs; see [accuracy usage](../examples/output/README.md). Benchmark raw
text capture is an intentionally separate opt-in capability, not implemented by
this summary adapter. `ttst` uses the second recognized content event, with its explicit policy in
JSON; it is not an observation of a physical second token. `content_inter_chunk_latency`
pools actual recognized-content intervals from retained successes. Missing vectors
and truncated vectors are excluded, with request counts and population policy
retained. Bundled events cannot reveal individual token times. No synthetic
inter-chunk vectors or physical second-token metrics are inferred.

W&B consumes one bounded byte snapshot and logs its SHA-256 in run config and
artifact metadata. SDK initialization cannot switch the artifact to a later
version of the producer's file. The staged snapshot remains available through
run completion and is then removed. Its displayed table includes each metric's
population, estimators and token-source counts; schema-2 coverage and content
interval policies also remain in config.
