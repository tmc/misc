# AIPerf compatibility and qualification

Reference: AIPerf `f10eb50a1776775d80d7f13396b0f668f14e2c03`.
Compatibility is assessed per behavior, provider and deployment cell. The
extension does not currently claim full AIPerf parity.

## Supported comparison workflows

- [Live measurement verification](serving/aiperf-live.md) checks retained
  streaming chat populations, phase/order/concurrency, usage and own-wire
  timing residuals. A fresh DGX Ollama/Qwen3 campaign at `379fe5b7` passed
  107 consistency checks over 48 measured requests. A later frozen `31a4a12e`
  C4/C8 cell passed the same accounting checks over 64 measured requests;
  hash-bound verification policies replay both matrices. Both tools retained
  two real Ollama 404 failures without successful-request or token claims.
  This is a bounded semantic
  result; the verifier prints its lack of a statistical performance gate.
- [Pinned Hugging Face preprocessing](../accuracy/HF-TOKENIZER.md) runs the
  actual local model chat template and emits counting provenance. Official
  Qwen3 assets produced the same 19 IDs through Hugging Face and the Go
  tokenizer with equivalent literal framing. Ollama reported 21 prompt tokens
  in the live cell. [Pinned Ollama reconstruction](../cmd/llm-ollama-frame/README.md)
  explains the difference: the retained template appends `/think`, yielding
  21 identical Go/Hugging Face IDs. Actual engine prompts, applied flags and
  GGUF tokenizer IDs remain unobserved.
- [Jinja/JMESPath preprocessing](endpoint-adapters/template-compat.md) uses
  the pinned upstream language implementations for offline authored payloads
  and captured response selection. The [runtime evaluator](endpoint-adapters/runtime-template.md)
  provides explicit per-VU JSON-only render/select calls, with evaluator cost
  reported separately from HTTP timing. It preserves the documented sandbox
  restrictions and does not alter client SSE or token accounting.
- [Optuna campaigns](optuna-campaign.md) use the actual pinned TPE sampler to
  select categorical cells evaluated by independent repeated k6 processes.
  [Numeric and Pareto search](optuna-search.md) adds continuous/logarithmic/stepped
  domains and joint objectives from those same hashed run summaries. Interrupted
  and failed attempts remain visible; checkpoint replay verifies proposal and
  artifact identities.
- [Outcome search](outcome-search.md) adds actual averaged metric constraints,
  CPU BoTorch qLogNEI/qLogNEHVI acquisitions and named search configurations.
  [Grid recipes](grid-recipes.md) and [SLA boundary planners](sweeping/sla-planners.md)
  provide retained-curve analysis and the actual pinned monotonic/smooth-isotonic
  planner mechanics. These optional adapters require their pinned runtimes;
  CPU fixture results establish no model capacity or optimizer optimality.
- [Simultaneous goodput SLOs](goodput-slo.md) apply fractional-millisecond TTFT,
  token-normalized ITL and last-content latency limits together. Missing enabled
  observations cannot be good; recorded failures remain in the fraction's
  denominator. Single-process observation-span goodput uses the earliest successful content
  request through the final successful content response; the existing declared
  window rate remains distinct. Missing or incomplete span evidence stays
  unavailable. Old receipts remain readable with extended-policy availability
  shown explicitly.
- [Saved-run comparison](serving/comparison.md) preserves per-run metric
  populations in JSON, SVG, HTML, optional pinned PNG rendering and a loopback
  refreshing dashboard. These are descriptive views with explicit statistical
  status.
- [Explicit-price cost estimates](cost.md) derive a separate ledger from
  reported token buckets and a hashed caller-supplied price table. Missing
  buckets or rates remain unavailable; these estimates are not provider bills.
- [ProviderWire v4](endpoint-adapters/providerwire-v4.md) supports an authored
  text/reasoning/function-tool subset against pinned Grafana AI SDK schemas.
  Schema and protocol fixtures do not establish hosted-provider compatibility.
- [Native producer interchange](telemetry/NATIVE-CONTRACT.md) retains latest
  managed/DCGM lineage for diagnostics. Such lineage is refused for qualified
  window/energy admission until its conversion and timing evidence is verified.

## Remaining parity gates

| Area | Remaining evidence or implementation |
| --- | --- |
| Live endpoints | Other provider/model/dialect cells and real missing-usage populations; bounded Ollama chat success/error cells do not cover them |
| Applied work identity | Serving template/tokenizer and effective engine settings must be observed, not inferred from model names or declaration hashes |
| Performance | Independent process comparisons with preregistered equivalence margins, adequate sample size, matched populations and qualified client capacity |
| Observer overhead | A clean owned GPU lane, equal observation windows, pinned workload/cadence and whole-process paired uncertainty; current DGX desktop activity is a confound |
| Physical telemetry | The retained 13.058 W discrepancy exceeds the fixed 10 W gate; HOLD remains, with no retry or tolerance change |
| Native hardware | Independent physical reference, managed conversion/DCGM acquisition qualification and supporting MIG hardware |
| Distributed deployment | Independent-host clock mapping, RWX storage and remote registry distribution; same-host nodes do not establish these |
| Search | CPU proposal, replay and recipe controls do not establish optimal model settings or statistically qualified capacity; pinned AIPerf has metric/operator/bound outcome constraints, not an arbitrary arithmetic-expression language |
| Templates and tokenization | JSON-only evaluation does not reproduce full upstream Python RequestInfo/Turn object behavior, live SSE conversion or image/audio processor token counts |

A fresh DGX two-node Kubernetes replay at `379fe5b7` retrieved eight unique
successful worker records and exercised partial loss, startup failure, active-run
cancellation and UID-safe cleanup. Its 258 sealed artifacts describe same-host
kind nodes, hostPath storage and local image loading.

Host race/vet and registered-binary tests verify software contracts. Fixture,
real-model, physical, statistical and deployment evidence retain their separate
scopes. Failed qualification cells remain in the receipt history.
