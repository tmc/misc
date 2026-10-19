# AIPerf compatibility and qualification

Reference: AIPerf `f10eb50a1776775d80d7f13396b0f668f14e2c03`.
Compatibility is assessed per behavior, provider and deployment cell. The
extension does not currently claim full AIPerf parity.

## Supported comparison workflows

- [Raw session import](inputs-json.md) preserves authored payloads and session
  order through a bounded `inputs.json` converter. [BurstGPT import](burstgpt.md)
  converts declared timestamps and token targets for the existing realization
  and replay helpers. [Vendor trace normalization](vendor-traces.md) preserves
  Mooncake, Bailian and TraceLab session/count/cache metadata. Authored payloads
  can be exported under an explicit serial replay policy; count-only records
  require realization before execution. [Pinned prefix synthesis](trace-synthesis.md)
  transforms declared cache patterns and targets without claiming engine cache hits.
  [Offline text realization](trace-realization.md) verifies complete synthetic
  snapshots with the actual pinned generator, Hugging Face and an independent
  Go tokenizer before exporting authored requests.
- [DAG conversations](dag-conversations.md) run an explicitly limited forest of
  asynchronous forks, spawns and joins within one VU. The example uses stock
  `http.asyncRequest`; it does not supply extension measurement provenance or
  distributed sticky-session scheduling.
- [Accuracy recipes](accuracy-recipes.md) invoke pinned upstream task recipes
  against caller-supplied local dataset snapshots, preserving messages,
  generation limits and stops. Runtime availability, dataset authenticity,
  grading and real model accuracy each require their own qualification.
  [Emitted-grade summaries](accuracy-graded.md) separately reproduce phase-scoped
  per-response aggregation. Graded exports cannot identify requests without
  grades, so their all-attempt failure and unavailable populations remain unknown.
- [Sampling designs](sweeping/sampling-design.md) add scalar zip, scenario,
  Sobol and Latin-hypercube expansions and the pinned shared-SLO boundary
  planner. [Equivalence plans](serving/equivalence.md) require fixed process
  pairs, practical margins and receipt verification before an equivalence
  verdict; the API alone provides no performance evidence.
- [Explicit statistics conventions](metric-semantics.md) select upstream linear
  percentiles and dispatch-start time slices. Legacy nearest-rank and
  completion-based reports remain the default. This option changes those
  conventions, not the recorded population or availability of missing metrics.
- [Responses streaming](endpoint-adapters/responses.md) preserves done-only text
  and a separate pinned content timing view, with duplicate/conflicting terminal
  evidence refused. Default recognized-content timing remains distinct.
- [Failure-adjusted latencies](adjusted-metrics.md) retain failed observations as
  explicit positive infinity in separate distributions. [Specialized metrics](specialized-metrics.md)
  derive per-request thinking, prefill, cache and declared audio-duration values.
  These APIs preserve their documented population and source limits.
- [Missing-usage verification](serving/aiperf-missing-usage.md) separates
  accounting correctness from observing genuine provider omission. A response
  with reported usage cannot qualify a missing-usage cell.
- [Live measurement verification](serving/aiperf-live.md) checks retained
  streaming chat populations, phase/order/concurrency, usage and own-wire
  timing residuals. A historical DGX Ollama/Qwen3 campaign at `379fe5b7` passed
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
  The optional [typed AIPerf adapter](endpoint-adapters/aiperf-template.md) invokes
  actual pinned `RequestInfo`/`Turn` formatting and captured-response parsing;
  its payload exports feed the raw-session importer. It does not establish live
  SSE framing or extension measurement provenance for parsed responses.
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
  reported token buckets and a hashed caller-supplied price table, keyed by the
  retained response model when present. Missing buckets or rates remain
  unavailable; these estimates are not provider bills.
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
| Observer overhead | A clean owned GPU lane, equal observation windows, pinned workload/cadence and whole-process paired uncertainty; historical desktop activity cannot qualify a new run |
| Physical telemetry | The retained 13.058 W discrepancy exceeds the fixed 10 W gate; HOLD remains, with no retry or tolerance change |
| Native hardware | Independent physical reference, managed conversion/DCGM acquisition qualification and supporting MIG hardware |
| Distributed deployment | Independent-host clock mapping, RWX storage and remote registry distribution; same-host nodes do not establish these |
| Search | CPU proposal, replay and recipe controls do not establish optimal model settings or statistically qualified capacity; pinned AIPerf has metric/operator/bound outcome constraints, not an arbitrary arithmetic-expression language |
| Templates and tokenization | Typed offline formatting does not qualify live SSE conversion; context truncation, tool framing and image/audio processor token counts need their own implementation and evidence |

A historical DGX two-node Kubernetes replay at `379fe5b7` retrieved eight unique
successful worker records and exercised partial loss, startup failure, active-run
cancellation and UID-safe cleanup. Its 258 sealed artifacts describe same-host
kind nodes, hostPath storage and local image loading.

Host race/vet and registered-binary tests verify software contracts. Fixture,
real-model, physical, statistical and deployment evidence retain their separate
scopes. Failed qualification cells remain in the receipt history.
