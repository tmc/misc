# Offline accuracy recipes

`scripts/accuracy-recipes.py` invokes the actual benchmark and conversation
functions from AIPerf `f10eb50a1776775d80d7f13396b0f668f14e2c03` against explicitly
supplied local dataset rows. It verifies the complete installed Python source
hash, plugin-default hash and package version before rendering. It never calls
Hugging Face dataset loaders. No model or grader runs during rendering.

Use an existing qualified AIPerf Python environment. The adapter adds no
requirements and does not install missing optional packages. Missing datasets,
configs, splits and recipe dependencies fail; they do not select another recipe.

An input JSON document selects one benchmark and pins its local snapshots:

```json
{"schema_version":1,"benchmark":"gsm8k","model":"example","model_revision":"model-digest","snapshots":[{"dataset":"openai/gsm8k","config":"main","revision":"caller-pinned-dataset-revision","splits":{"test":[{"question":"What is 1+1?","answer":"Add one. #### 2"}]}}]}
```

This row is a fixture, not the real GSM8K evaluation population. Dataset revision
strings are caller declarations. Snapshot hashes identify the supplied bytes;
they do not attest that rows came from the named upstream dataset.

```sh
HF_HUB_OFFLINE=1 HF_DATASETS_OFFLINE=1 timeout 60 \
  /path/to/pinned-venv/bin/python scripts/accuracy-recipes.py task.json \
  --output outputs/recipe
```

The output directory must not exist. Input is limited to 16 MiB, 100 snapshot
entries and 10,000 total rows. Output artifacts match the existing 16 MiB
artifact and 4 MiB request-file consumer budgets; oversized populations are
refused rather than truncated. No automatic fetching or retry occurs.

Supported benchmark names are `mmlu`, `mmlu_pro`, `aime`, `hellaswag`, `bigbench`,
`aime24`, `aime25`, `math_500`, `gsm8k`, `gpqa_diamond` and `lcb_codegeneration`.
Each snapshot supplies the exact dataset/config identity and all needed splits.
For example, MMLU uses one `lighteval/mmlu` snapshot per selected subject with
`test` and `dev` (or `validation`) rows. MMLU-Pro uses `validation` and `test`.
LiveCodeBench uses the actual configured release subset, normally `v4_v5`, and
local `test` rows. Its loading-script option is recorded but no loading script
executes: the adapter constructs a dataset directly from the supplied rows.

Optional `tasks`, `n_shots`, `enable_cot` and `system_prompt` fields are passed
through. Omitted shot/CoT/system values use the pinned plugin defaults. An empty
system string explicitly disables the default. The actual upstream loader owns
few-shot selection, chat roles, CoT, system injection, deterministic GPQA choice
permutation, token limit and stops. The pinned GSM8K implementation omits stops
although its docstring describes stops in the external recipe; the adapter
preserves the implementation. AIME draws shots from the evaluation split and
can include a question in its own context, as the pinned implementation does;
this is not a held-out design.

Outputs:

- `artifact.json`: unfilled prediction artifact with ordered case/reference hash.
- `requests.json`: independent case-to-request mapping with authored messages,
  `max_tokens` and optional `stop`.
- `prompts.json`: flat upstream prompts for inspection. Use `requests.json` to
  preserve chat roles, system messages and generation settings.
- `recipe.json`: task labels, per-case metadata, actual dataset-load identities,
  defaults/overrides, source/runtime/dependency identities and hashes.
- `manifest.json`: SHA-256 of every output above.

To generate and evaluate later, after separately authorizing the endpoint and
its workload budget:

```sh
llm-accuracy -input outputs/recipe/artifact.json \
  -requests outputs/recipe/requests.json -endpoint http://127.0.0.1:8000/v1/chat/completions
```

`-requests` and legacy `-prompts` are mutually exclusive. Request decoding refuses
unknown fields, duplicate/case-colliding keys, nulls and trailing JSON. Requests
remain serial; each HTTP request retains the existing 30-second CLI timeout and
failures remain failed cases. Credentials use only the existing authorization
environment option and Authorization header. The Go API is
`accuracy.GenerateRequests`; legacy `accuracy.Generate` still sends one user
message and does not add token or stop defaults.

External symbolic/lighteval/code evaluators still require their separately
qualified confined worker. HellaSwag/BigBench prompt generation needs the pinned
optional DeepEval templates. Their artifacts explicitly select `aiperf-exact-match-v1`, matching the pinned
Python grader's stripped, nonempty, case-sensitive equality including Python's
U+001C–U+001F whitespace. Legacy `exact-match-v1` keeps Go trimming semantics.
Prompt transformation alone does not establish full benchmark accuracy. Actual datasets, model
answers, held-out accuracy, sandbox execution and statistical claims each need
separate evidence. The adapter never executes generated code.
