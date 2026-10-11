# Runtime JSON-only Jinja and JMESPath

`evaluateRuntimeTemplate` executes the pinned Jinja/JMESPath bridge synchronously
on the calling VU. Render a payload before `client.request`, or select a value
from `response.body` after it. The HTTP request goes directly to the server.
The evaluator's `elapsed_ms` includes interpreter and worker hash checks,
process startup, evaluation and result validation. Report that cost separately;
it contributes to iteration time and can limit throughput. No observer overhead
or capacity equivalence is qualified by this feature.

Use the [pinned bridge installation](template-compat.md), then provide absolute
paths and SHA-256 pins for the interpreter and `scripts/template-compat.py`:

```sh
work="$HOME/tmp/xk6-llm-perf/20261010-template-compat"
export TEMPLATE_PYTHON="$work/venv/bin/python"
export TEMPLATE_PYTHON_SHA256="$(shasum -a 256 "$TEMPLATE_PYTHON" | cut -d ' ' -f 1)"
export TEMPLATE_SCRIPT="$PWD/scripts/template-compat.py"
export TEMPLATE_SCRIPT_SHA256="$(shasum -a 256 "$TEMPLATE_SCRIPT" | cut -d ' ' -f 1)"
BASE_URL=http://localhost:8000 ./dist/k6 run examples/runtime-template.js
```

The example renders different inputs on each iteration, sends them through the
native embeddings adapter, selects the received embedding, and records
`template_render_ms` and `template_selection_ms`. Its `model: 'fixture'` value
must be changed to the model served by a real endpoint.

The API accepts `{python, pythonSHA256, script, scriptSHA256, request}`. `request`
is the existing bridge's strict render/select JSON protocol. Success returns
`{result, elapsed_ms, script_sha256}`, where `result` is the bridge envelope
and `result.value` is the rendered object or selected JSON value. Failure returns
an `Error`; check it before using any value. Unknown options fail.

Each invocation creates its own Unix process group and honors VU cancellation.
A seven-second parent deadline, one-second pipe cleanup budget, and group cleanup
also bound workers that retain descendants after their leader exits. The Python
worker imposes its five-second wall/CPU limits and a Linux 512 MiB address-space
limit; macOS memory is unqualified. Input and output are at most 1 MiB, JSON
nesting at most 64, and the worker source at most 64 KiB. Worker and interpreter
paths must refer to regular files. Invalid UTF-8, duplicate JSON keys, nonfinite
numbers, trailing JSON, wrong dependency versions and malformed envelopes fail.
Output and error text are not echoed on failure.

The worker executes the already hashed source bytes, so replacing its path after
the hash check cannot select different code. The operator-selected interpreter,
its libraries, parent paths and entire runtime must remain immutable through
execution. The interpreter hash does not authenticate the runtime's imported
libraries. The selected worker is trusted executable code, not an untrusted
plugin. Its canonical input/value hashes are checked as well-formed receipts
but are produced by that pinned worker; Go does not independently recompute
Python's canonical JSON hashes. Hashes provide reproducibility, not authenticity.

Jinja uses the actual pinned library with sandboxed JSON-only variables,
autoescaping, bounded expansion and no private object reflection. JMESPath uses
the actual pinned library's projections, filters and functions. This preserves
the existing bridge's bounded semantics when invoked on each iteration; it does
not expose AIPerf's ordinary Python `RequestInfo`/`Turn` objects. For example,
AIPerf's ordinary environment accepts `request_info.__class__.__name__`, while
this bridge rejects private reflection. It is not an operating-system sandbox.

Selection produces application data. It does not become a native response
adapter, count output tokens, change success/goodput decisions, or retroactively
supply TTFT/ITL evidence. Live SSE query extraction and full upstream object
semantics remain unqualified. Use a supported native protocol for measured
response conversion; use `json` only when content/token timing is unavailable.
