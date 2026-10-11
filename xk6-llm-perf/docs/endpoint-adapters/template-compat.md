# Offline Jinja and JMESPath workflow

`scripts/template-compat.py` renders authored JSON request payloads with pinned
Jinja2, or selects values from captured JSON with pinned JMESPath. It prepares
inputs before k6 starts and evaluates saved responses after the run. It does
not execute Python on the VU loop or change request timing.

Install into a task-specific virtual environment:

```sh
work="$HOME/tmp/xk6-llm-perf/20261010-template-compat"
mkdir -p "$work"
python3 -m venv "$work/venv"
"$work/venv/bin/pip" install -r scripts/template-compat-requirements.txt
```

Create a render request. Variables are explicit JSON data, including any
authored model, modality, role, turn, or turn-history values used by the template.
The bridge does not infer AIPerf's Python `RequestInfo` object graph.

```sh
cat > "$work/render-input.json" <<'JSON'
{"schema_version":1,"operation":"render","template":"{\"model\": {{ model|tojson }}, \"input\": [{% for text in texts %}{{ text|upper|tojson }}{% if not loop.last %},{% endif %}{% endfor %}]}","variables":{"model":"local","texts":["hello","world"]},"extra":{},"turn_extra":{}}
JSON
"$work/venv/bin/python" scripts/template-compat.py \
  < "$work/render-input.json" > "$work/rendered.json"
BASE_URL=http://localhost:8000 TEMPLATE_FILE="$work/rendered.json" \
  REQUEST_PATH=/v1/embeddings RESPONSE_PROTOCOL=openai-embeddings \
  ./dist/k6 run examples/template-compat.js
```

The example loads the rendered `value` and sends it through the existing authored
request path. Select the response protocol that matches the server. The `json`
default retains arbitrary JSON and leaves token/content timing unavailable;
an offline query does not retroactively turn that response into generated text.

To select content from a saved response, provide the complete captured JSON:

```sh
printf '%s\n' '{"schema_version":1,"operation":"select","expression":"join(` `, results[?score > `0`].text)","response":{"results":[{"score":1,"text":"hello"},{"score":0,"text":"ignored"}]}}' \
  | "$work/venv/bin/python" scripts/template-compat.py > "$work/selected.json"
```

JMESPath projections, filters, functions, and indexing are evaluated by the
actual library. Explicit missing, null, false, zero, or empty selections fail,
matching the pinned upstream template endpoint's nonempty-selection condition.
Selection returns the authored JSON type; it does not run upstream
`convert_to_response_data` or token counting.

Jinja loops, conditions, filters, macros, and JSON encoding use the actual
Jinja library with `autoescape=True`, matching the pinned upstream setting.
Endpoint extras override rendered fields; turn extras override both. The
bridge uses a sandboxed environment with JSON-only variables, denies private
object reflection, disallows exponentiation, and bounds sequence multiplication.
That deliberately excludes arbitrary trusted Python object access supported by
upstream's ordinary environment. Missing-variable behavior follows Jinja's
default undefined policy; malformed rendered JSON fails. No loader is configured.

CLI input, rendered JSON, and output each have a 1 MiB limit. The CLI has a
five-second wall/CPU deadline and disables core dumps. Linux additionally caps
address space at 512 MiB; macOS does not provide this memory qualification.
Run authored templates only: these limits are not an operating-system sandbox.
Invalid JSON, duplicate keys, nonfinite values, unknown protocol fields, wrong
dependency versions, and non-object rendered payloads fail. Errors report only
their class, without echoing potentially sensitive templates or variables.

Output includes the reference revision, installed dependency versions, and
SHA-256 hashes of canonical input and result. Hashes provide reproducibility,
not authenticity or secrecy. The output contains the rendered payload or
selected response, so retain it in a private artifact directory when sensitive.

The source basis is AIPerf `f10eb50a1776775d80d7f13396b0f668f14e2c03`,
`src/aiperf/endpoints/template_endpoint.py` and its locked Jinja2 3.1.6,
JMESPath 1.0.1, and MarkupSafe 3.0.3 dependencies. This closes an offline
render/query workflow; it does not establish runtime template-endpoint parity,
upstream request-object equivalence, live SSE response queries, response
conversion, or token accounting for arbitrary selections.

Tests compare rendered loops, filters, conditions, macros, and escaping against
the upstream ordinary `Environment` setting. They exercise query expressions,
override precedence, deterministic receipts, unsafe/invalid templates, bounded
expansion, error privacy, and a real registered k6 request to a local fixture:

```sh
K6_SMOKE_BINARY="$PWD/dist/k6" "$work/venv/bin/python" \
  -m unittest discover -s scripts -p template_compat_test.py -v
```
