# Outputs and accuracy

The Go commands consume version-1 local exports. They use no runtime network
unless an endpoint is explicitly supplied. Credentials are supplied through an
environment variable containing the HTTP Authorization value; they never enter
output documents. Output sink errors retain only status or a bounded diagnostic.

```sh
go run ./cmd/llm-output -input outputs/run.json > outputs/summary.json
go run ./cmd/llm-output -input outputs/run.json -format csv > outputs/summary.csv
go run ./cmd/llm-output -input outputs/run.json -format html > outputs/plot.html
go run ./cmd/llm-output -input outputs/run.json -format otlp > outputs/otlp.json
```

HTML is a standalone escaped table with SVG bars for mean/max. It embeds no
scripts or remote assets. Per-request statistics use successful retained records,
nearest-rank percentiles, population standard deviation, and explicit observation
counts. Missing values remain null in JSON and empty cells in CSV. Dropped records
remain visible and statistics describe the retained subset; no replay completeness
claim follows from an aggregate.

OTLP uses the official OTLP JSON schema and individual gauge observations with
Unix nanosecond timestamps. Duration and first-content delay convert milliseconds
to seconds; token observations preserve source and input/output attributes.
Token-normalized ITL uses `llmperf.client.time_per_output_token`, since it is not
an observed output-chunk interval. This avoids relabeling it as the OTel chunk
metric. Resource identity is the benchmark run. Raw HTTP paths, prompts, answers,
and credentials are omitted. It supports post-run and serial live-pipe delivery.

```sh
go run ./cmd/llm-output -input outputs/run.json -format otlp \
  -endpoint http://127.0.0.1:4318/v1/metrics
go run ./cmd/llm-output -input outputs/records.jsonl -stream -format otlp \
  -stream-run my-run -endpoint http://127.0.0.1:4318/v1/metrics
```

Streaming reads at most 64 MiB, 100,000 records and 1 MiB per JSONL line. It posts
serially with backpressure and stops on the first failure. Its stdout receipt
retains the read and accepted prefix, including on error; do not blindly replay
accepted records. No automatic retries occur. A blocked input pipe requires the
producer to close it; context cancellation bounds HTTP calls, not arbitrary
readers. HTTP redirects, embedded URL credentials and query strings are refused.
OTLP partial rejection is an error. Requests time out after 30 seconds.

MLflow accepts an existing run identity and the documented `runs/log-batch` REST
schema. Summary scalars have unit params and missing values are absent. Streaming
emits `live.*` duration/token metrics with timestamps in Unix milliseconds.

```sh
go run ./cmd/llm-output -input outputs/run.json -format mlflow \
  -mlflow-run RUN_ID -endpoint http://127.0.0.1:5000/api/2.0/mlflow/runs/log-batch
python examples/output/consumer_check.py --binary ./llm-output --directory ./receipts
```

The consumer check starts and terminates a localhost MLflow server, creates a run,
posts real summary and streaming requests, checks stored values and missing units,
and uploads/retrieves the explicit summary artifact with the official SDK.
It requires provisioned MLflow 3.4.0. It does not create remote experiments.
MLflow can partially persist an unsuccessful batch; this adapter does not retry.

W&B uses its official Python SDK rather than a private HTTP protocol. Offline is
the default; remote posting requires `--online` and separately provisioned SDK
credentials. The supplied summary becomes a results table and a versioned run
artifact. No recursive artifact-directory scan or implicit authored-text upload
occurs. Source versions and statistic policy become run config.

```sh
python examples/output/wandb_export.py outputs/summary.json \
  --project llm-benchmarks --directory outputs/wandb
```

The local acceptance receipt used W&B 0.25.0 in a scoped virtualenv. Online W&B,
authenticated remote MLflow/OTLP deployments and their dashboard policies remain
separate qualification cells. A local accepted SDK artifact does not certify
remote synchronization.

`otel_check.py` starts an explicitly provisioned localhost OTel Collector, sends
a streaming fixture, checks debug-consumer seconds/count attributes and omitted
unavailable output tokens, then terminates it. It requires a collector executable
and `llm-output` binary. The local acceptance receipt used collector 0.141.0.

## Versioned accuracy artifacts

Accuracy uses an explicitly authored artifact, separate from sanitized performance
records. Its task, task/reference versions, model/revision and evaluator/version
are required. `ReferenceHash` hashes the ordered IDs and references with canonical
JSON; declarations are checked before grading. Input is strict bounded UTF-8 JSON,
maximum 16 MiB / 100,000 cases / 64 KiB per answer. Duplicate IDs are rejected.
The report retains artifact/reference hashes and outcomes, without answer text.
Accuracy uses all declared cases; failed or unavailable cases stay in its denominator.
Coverage uses observed successful predictions. Empty observed answers score wrong.

```sh
go run ./cmd/llm-accuracy -input accuracy.json > accuracy-report.json
# Fill only missing successful predictions from independent prompts:
go run ./cmd/llm-accuracy -input accuracy.json -prompts prompts.json \
  -endpoint http://127.0.0.1:8000/v1/chat/completions > accuracy-report.json
```

`prompts.json` is a bounded JSON object of case ID to prompt. The reference answer
is never used as a prompt. Generation is serial, refuses redirects, has a 30-second
request timeout and a 30-minute total deadline, and preserves provider failures as
failed cases. The final report pins the canonical prompt-map hash. This explicit
command contacts the supplied provider; local fixtures do not qualify a real model.

Built-in evaluators include trimmed case-sensitive exact match, bare first-line
choice, exact rational values, and source-backed GSM8K / MMLU / MMLU-Pro extraction.
[GRADERS.md](../../accuracy/GRADERS.md) describes math fallback and the explicit
Linux confinement bridge for the full upstream symbolic/lighteval/code families.
No generated code is executed by the Go evaluators or on an unconfined host.

Primary API references: [MLflow REST](https://mlflow.org/docs/latest/api_reference/rest-api.html),
[W&B tables](https://docs.wandb.ai/guides/track/log/log-tables),
[W&B artifacts](https://docs.wandb.ai/models/artifacts/create-a-new-artifact-version).
The alignment source is AIPerf commit `f10eb50a1776775d80d7f13396b0f668f14e2c03`.
