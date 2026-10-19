# Preregistered process equivalence

`serving.AnalyzeEquivalence` compares the mean paired delta, B minus A, with
practical lower and upper limits fixed before main sampling. It wraps
`AnalyzePaired`; the existing bootstrap and descriptive comparison remain
unchanged. The complete metric family receives the existing simultaneous
Bonferroni delta and ratio intervals. Only the delta intervals determine
this equivalence decision.

Create an `EquivalencePlan` with the balanced `PairPlan`, complete measurement
and SLO policy, one signed margin per metric in the same order, fixed bootstrap
seed/resamples/confidence, and a sample-size rationale. Each margin needs a unit
and practical reason independent of the observed main differences. Save the
plan and its `Hash` before execution. The hash identifies bytes and does not
prove preregistration time, adequate power, or correct measurement units.

Collect each arm in a separate sequential process, in the planned order.
Requests within a process are not independent replicates. Retain failed arms;
do not replace them with favorable retries. Use an excluded pilot only to
estimate noise and feasibility, then freeze a new main plan with an adequate
fixed pair count. The paired percentile bootstrap has asymptotic coverage;
a small degenerate control does not establish performance equivalence.

Call `AnalyzeEquivalence` with the retained hash, all process receipts and
`PairedOptions` matching the plan. `Verify` and `VerifyPolicy` must authenticate
actual artifacts and applied measurement policy. Every delta confidence
interval must lie strictly within both declared limits, and all receipt,
work and client-capacity gates must qualify, for `PERFORMANCE_EQUIVALENT`.
Otherwise the result remains `PERFORMANCE_HOLD`. A nonsignificant difference
is not evidence of equivalence. Missing/incomplete policies, changed plans,
failed arms and duplicate process identities are errors.

The runnable Go examples use synthetic observations and deliberately omit receipt
verification, so they report `PERFORMANCE_HOLD` even when intervals fit.
No model-performance margins or sample-size defaults are supplied. The live
wire verifier's 15 ms / 0.25 ms residual tolerances remain semantic checks.
