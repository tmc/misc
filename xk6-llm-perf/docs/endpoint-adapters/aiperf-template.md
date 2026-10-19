# Typed AIPerf template preprocessing

`scripts/aiperf-template.py` is an optional offline bridge to AIPerf
`f10eb50a1776775d80d7f13396b0f668f14e2c03`. It verifies all 1,025 installed
Python source files and plugin defaults before importing endpoint code. It
requires the retained AIPerf 0.13.0 environment; it installs nothing and sends
no request. Runtime package versions and source file hashes accompany output.

Only use trusted authored templates. Actual upstream Jinja uses an ordinary
`Environment(autoescape=True)`, not a security sandbox: templates can invoke
Python object behavior and read template files. Run in an isolated environment
without credentials or sensitive files. The CLI bounds CPU to 20 seconds,
address space to 2 GiB, wall time to 30 seconds, input/output to 1 MiB, JSON
nesting to 64 and captured chunks to 256. These limits do not establish safe
execution of hostile templates. Upstream diagnostics on stderr may contain
template or prompt data; retain them with the same controls as the input. Template evaluation remains outside HTTP timing.

Input has exactly `schema_version: 1`, `request_info` and `responses`.
`request_info` uses the actual upstream RequestInfo schema, including its
model endpoint and Turn objects. Unknown fields are refused recursively.
`responses` is a list of captured upstream TextResponse objects with `perf_ns`,
`text` and optional `content_type`; each may represent one already-framed
stream chunk. Empty lists are allowed. Duplicate JSON members and nonfinite
numbers are refused. Upstream model validation and defaults apply; for example,
absent max_tokens stays null while max_tokens zero is rejected upstream.

```json
{"schema_version":1,"request_info":{"credit_num":0,"credit_phase":"profiling","conversation_id":"example","turn_index":0,"x_request_id":"request","x_correlation_id":"correlation","model_endpoint":{"models":{"models":[{"name":"local"}],"model_selection_strategy":"round_robin"},"endpoint":{"type":"template","streaming":false,"template":{"body":"nv-embedqa"}}},"turns":[{"texts":[{"contents":["Hello"]}]}]},"responses":[]}
```

Save that object as `typed.json`, then use the pinned environment:

```sh
/path/to/aiperf/venv/bin/python scripts/aiperf-template.py < typed.json > typed-output.json
jq '.inputs' typed-output.json > inputs.json
go run ./cmd/llm-import-inputs -path /custom -protocol json < inputs.json > sessions.json
```

The resulting sessions feed [raw-sessions.js](../../examples/workloads/raw-sessions.js)
using the [authored session workflow](../inputs-json.md). Endpoint path and Go
response protocol are caller declarations; the offline parsed response does not
configure or replace the extension's response adapter.

The bridge directly calls TemplateEndpoint.format_payload: last-turn selection,
named media extraction, Python attribute access on turn/request_info, endpoint
extras then turn extra_body precedence, and actual template configuration all
come from upstream. It calls parse_response for captured TextResponse objects.
An explicit JMESPath result of zero, false, null, empty or missing yields null
under upstream truthiness rules. That is an upstream parsing observation, not
permission to discard contradictory Go accounting evidence. Parsed response
objects are not extension measurement, token or usage evidence.

This provides offline typed preprocessing and captured-chunk conversion only.
It does not parse raw SSE bytes, exercise network/transport framing, observe
engine-applied prompts, count image/audio processor tokens or qualify live or
performance parity. Invalid JMESPath compilation can fall back upstream; inspect
actual results rather than assuming a supplied expression was applied.

Run controls with the same pinned Python:

```sh
/path/to/aiperf/venv/bin/python scripts/aiperf_template_test.py
```
