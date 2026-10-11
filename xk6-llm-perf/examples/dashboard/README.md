# Live LLM dashboard

Use the built-in k6 v2.3.0 web dashboard to watch extension metrics while a
streaming arrival-rate test runs. The same panels export to a self-contained
HTML report. No Grafana server or additional extension is required.

Build with `make build`, then start the local timing fixture in one terminal:

```sh
python3 examples/dashboard/fixture.py
```

In another terminal:

```sh
sh examples/dashboard/run.sh
```

Open <http://127.0.0.1:5665>. The default workload schedules two requests per
second for 60 seconds, with four initial VUs and a maximum of 16. The fixture
emits reasoning, four visible chunks, and delayed usage; its timings exercise
the dashboard and do not measure a model.

For an OpenAI-compatible streaming endpoint:

```sh
ENDPOINT_URL=http://localhost:8000/v1 MODEL=your-model RATE=5 DURATION=2m \
  sh examples/dashboard/run.sh
```

Set `OPENAI_API_KEY` if needed. `PROMPT`, `MAX_TOKENS`, `TIMEOUT`, `VUS`,
`MAX_VUS`, `MAX_TTFT_MS`, `MAX_TTFO_MS`, and `MAX_TOKEN_LATENCY_MS` configure
the workload and request SLOs. The defaults are 32 output tokens, a 15-second
timeout, and SLO limits of 1000, 2000, and 100 milliseconds respectively.
Success checks, failures, dropped arrivals, and 95% SLO compliance determine
the exit status. A single run is diagnostic; it provides no statistical
cross-implementation performance claim.

Reports go to `~/tmp/xk6-llm-perf/YYYYMMDD-dashboard/`. Set
`DASHBOARD_OUTPUT_DIR` to retain a separate directory for each run. The report
contains charts when duration exceeds three dashboard aggregation periods.
The Cumulative summary tab retains whole-run timing, request/token counters,
rates, and gauges. `summary.json` retains all raw k6 metrics and thresholds,
including token-length and discrepancy distributions. These distributions are
excluded from the dashboard table because the bundled renderer gives their
dimensionless trend values misleading rate units. The report includes chart and summary panels; live
headline stat cards are omitted by k6's report renderer. For unattended
export without a live server, use `K6_WEB_DASHBOARD_PORT=-1`. k6 otherwise
waits to exit while a dashboard browser connection remains open; close that
tab when the test finishes.

## Reading the panels

TTFT includes recognized reasoning output; TTFO starts at visible content or
tool output. Full-response latency includes trailing usage and completion;
last-content latency stops at the final output event. The second-content-event
chart measures dispatch to the second recognized content-bearing event
(`llm_dispatch_ttst`), including reasoning or visible output; it does not
identify the second physical token. Inter-token latency is
derived from token counts. The all-SSE interval includes non-content events;
the content-only mean and maximum exclude role-only, usage-only and terminal
events. Their charts show p95 across per-request means and maxima, with one
sample per request having a complete, nonempty interval vector. Requests are
not weighted by event count. The all-SSE chart shows p95 across pooled
parsed SSE inter-event gaps in each aggregation window. It is event-weighted
across requests and includes non-content events. Each content event may contain several tokens. Raw
content interval vectors and pooled event statistics remain in measurement
exports; these request-weighted charts do not replace those observations. Output tokens per second are counter rates over each
dashboard window, not averages of per-request token speeds.

The provenance panels count reported, estimated, client-counted, or unavailable
output counts. Strict measurement disables token estimates. An empty panel
means no samples were emitted, not a measured zero. Chart percentiles describe
each aggregation window; cumulative percentiles are in `summary.json`.
Base metrics aggregate all endpoints/VUs. Tagged per-model splits are not
provided by this dashboard configuration.

Provider semantics must be checked before treating the timing panels as
qualified. Provider dialects, including Ollama reasoning output, must have
known stream semantics and verified token-count provenance. Compare first
reasoning, first visible output, second content event, trailing usage, and
completion against a saved provider trace. A dashboard threshold passing
does not establish physical token timing or cross-provider parity.

## k6 compatibility

This configuration uses k6 v2.3.0's bundled dashboard assets v0.1.2 and the
`tabs → sections → panels` configuration schema. `XK6_DASHBOARD_CONFIG` is an
upstream internal customization hook, so verify UI rendering when upgrading
k6. Each chart uses one metric/aggregation query to keep series labels stable
when other metrics are absent.

Official references: [web dashboard and HTML export](https://grafana.com/docs/k6/latest/results-output/web-dashboard/),
[k6 v2.3.0 customization source](https://github.com/grafana/k6/blob/v2.3.0/internal/dashboard/customize.go),
and [bundled configuration schema](https://github.com/grafana/xk6-dashboard-assets/blob/v0.1.2/packages/view/src/Config.ts).
