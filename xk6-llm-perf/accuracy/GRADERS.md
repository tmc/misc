# Versioned accuracy graders

`gsm8k-v1`, `mmlu-v1`, and `mmlu-pro-v1` implement the answer extraction in
AIPerf revision `f10eb50a1776775d80d7f13396b0f668f14e2c03`, specifically
`src/aiperf/accuracy/graders/gsm8k_grader.py`, `multiple_choice.py`,
`mmlu_pro.py`, and `_choice_extract.py`. Those upstream algorithms are
Apache-2.0 licensed; the shared choice cascade derives from TIGER-AI-Lab/MMLU-Pro.
GSM8K references must contain `#### <number>`. Predictions prefer that first
marker, then the last numeric literal. Numeric comparison uses absolute
1e-6 tolerance. MMLU bare first-line answers take priority; otherwise the last
explicit `answer is (X)`, last `Answer: X`, and standalone-letter fallback
follow the pinned source's order. `unparsed` is diagnostic and does not erase a
correct fallback answer. Invalid references fail the artifact evaluation.

`math-numeric-v1` and `aime-numeric-v1` are intentionally bounded evaluators.
They extract the last balanced boxed answer, then the last answer phrase, then
the last numeric literal. They remove literal math delimiters, left/right
markers and whitespace, and normalize simple fraction/text wrappers. Rational
values compare exactly; remaining expressions compare as strings. They do not
implement the upstream full unit/LaTeX normalization, approximate tolerance,
percentage alternatives, equation rewriting, or symbolic equivalence. Use these
names only when those narrower rules are appropriate. Exponents are never passed
to `big.Rat`; this prevents short hostile strings causing huge allocations.

## External optional-dependency worker

`examples/output/accuracy_external.py` calls the pinned upstream graders for
`math-symbolic-v1`, `lighteval-expr-v1`, `lighteval-latex-v1`,
`lighteval-gpqa-v1`, and `code-execution-v1`. It requires explicit opt-in:

```sh
python3 examples/output/accuracy_external.py \
  --allow-confined-execution \
  --aiperf-source /path/to/clean/aiperf \
  --source-revision f10eb50a1776775d80d7f13396b0f668f14e2c03 \
  --runtime-root /path/to/pinned-venv < case.json
```

The stdin request has exactly these keys:

```json
{"schema_version":1,"evaluator":"math-symbolic-v1","prediction":"\\boxed{\\sqrt{2}}","reference":"2^{1/2}","dependency_versions":{"sympy":"1.14.0","latex2sympy2-extended":"1.10.2"}}
```

Versions in the example illustrate the pin fields; prepare and qualify the
runtime before using them. Include the complete dependency manifest in your run
assets and pin every package relevant to grading. Every supplied package version
is checked before importing the grader. The symbolic mode requires both optional
packages, and refuses the upstream silent fallback. The other modes invoke their
own upstream availability checks. Code references must use the pinned
LiveCodeBench JSON test-case payload; no ad hoc host code runner is supplied.

Execution requires Linux, bubblewrap with sized tmpfs support, enabled user
namespaces, and a cgroup-v2 job whose `memory.max` is at most 2 GiB and `pids.max`
is at most 64. The worker creates network, PID, user, IPC, UTS and mount namespaces,
drops capabilities, removes inherited environment variables, mounts source and
runtime read-only, omits home and credentials, and uses 64 MiB ephemeral writable
mounts. Resource limits also bound CPU, individual process memory, file size,
file descriptors, process count and core dumps. Wall time is at most 30 seconds.
There is no host execution fallback. Prepare the runtime on the target Linux
host: its Python base must reside under the read-only `/usr`, `/lib` or `/lib64`
mounts. Runtime caches or packages requiring an external home path fail closed.

The schema-1 response retains evaluator/source/dependency identities, correctness,
unparsed status, request and worker SHA-256 hashes, and the declared confinement
mode. These receipts identify inputs and software; they do not attest that an
arbitrary caller-provided worker implements isolation. The caller must pin and
review the script before execution. Report missing sandbox/dependencies as an
external qualification gate, never as an incorrect model answer or numeric
fallback. macOS host tests cover parsing and refusal paths; they do not qualify
Linux symbolic/lighteval/LiveCodeBench execution or matched AIPerf parity.

The admission check reads the current unified cgroup from `/proc/self/cgroup`
and checks effective ancestor limits. A bounded nested job is supported even
when the hierarchy root has no controller files; an unbounded job is refused.
Run the safe admission tests with
`python3 examples/output/accuracy_external_test.py`.

On an owned Linux job with the required bounds and bubblewrap, run
`python3 examples/output/qualify_confinement.py`. This exercises the same mount
and namespace command as the bridge and refuses failed negative controls for
host-file access, source writes, inherited environment and network namespaces.
It is confinement evidence only. Real symbolic and LiveCodeBench graders,
malicious process trees and timeout cleanup require separate qualified receipts.

The code mode uses the pinned upstream `CodeExecutionGrader` with a bridge-owned
worker transport. The transport delegates the upstream batch and scoring loop,
but closes inherited protocol/death descriptors only once in each fork subtree.
The pinned upstream callbacks can otherwise close newly reused descriptor numbers
in grandchildren and turn a known-correct program into a failing score. Transport
faults fail evaluation as unavailable; they do not count as wrong model answers.
BLAS/OpenMP threads are restricted to one so optional imports and worker processes
fit the unchanged 64-process cgroup bound. Containers hosting repeated checks
should use an init process to reap namespace supervisor children.

## JSON evidence admission

Provider generation responses, evaluator artifacts and confined-worker receipts
reject duplicate members recursively before typed decoding. Member names that collide under Go case folding are also refused before
typed decoding can merge their evidence.
Parsing depth is limited to 64; each caller retains its existing byte budget.
Unique explicit empty answers and provider `error: null` remain valid observed
responses. Artifact null predictions remain unavailable, and worker null score
fields still fail their existing provenance checks. Ambiguous provider responses
become failed cases with no prediction; they cannot enter successful accuracy
populations.
