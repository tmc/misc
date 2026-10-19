package serving

import (
	"fmt"
	llmperf "github.com/tmc/misc/xk6-llm-perf"
	"strings"
)

func exampleEquivalencePlan() EquivalencePlan {
	pairs, _ := PlanPairs(7, 4, []string{"ttft_ms"})
	zero := 0.0
	pairs.Policy = &PairPolicy{
		MeasurementPolicy: "dispatch-content-v1/success-token-counts-v1/success-slo-v1",
		WindowPolicy:      "attempt started and completed inside window; successful token totals",
		WarmupPolicy:      "phase=measurement and per-client warmup exhausted",
		SLO:               &llmperf.SLOReceipt{MaxTPOT: &zero, MaxE2E: &zero},
	}
	return EquivalencePlan{Pairs: pairs, Margins: []EquivalenceMargin{{Metric: "ttft_ms", Unit: "ms", Lower: -1, Upper: 1, Reason: "synthetic example oracle only"}}, BootstrapSeed: 3, Resamples: 1000, Confidence: .95, SampleSizeReason: "four deterministic control pairs; no performance claim"}
}

func ExampleEquivalencePlan_Hash() {
	p := exampleEquivalencePlan()
	hash, err := p.Hash()
	fmt.Println(len(hash), err)
	// Output: 64 <nil>
}

func ExampleAnalyzeEquivalence() {
	p := exampleEquivalencePlan()
	hash, _ := p.Hash() // Retain the real plan and hash before executing main runs.
	e := PairedExperiment{Plan: p.Pairs}
	policyHash, _ := PairPolicyHash(*p.Pairs.Policy)
	for i, order := range p.Pairs.Orders {
		for j := range order {
			e.Runs = append(e.Runs, PairRun{Pair: i, Arm: string(order[j]), ProcessID: fmt.Sprintf("control-%d-%d", i, j), WorkHash: strings.Repeat("a", 64), BinaryHash: strings.Repeat("b", 64), ArtifactHash: strings.Repeat("c", 64), Policy: p.Pairs.Policy, PolicyComplete: true, PolicyHash: policyHash, WorkAccepted: true, ClientQualified: true, Successful: true, Metrics: map[string]float64{"ttft_ms": 10}})
		}
	}
	// No receipt verifier is supplied for these synthetic observations.
	r, err := AnalyzeEquivalence(p, hash, e, PairedOptions{Seed: 3, Resamples: 1000, Confidence: .95})
	fmt.Println(r.Status, r.WithinBounds, err)
	// Output: PERFORMANCE_HOLD [true] <nil>
}
