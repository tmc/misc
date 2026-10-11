package serving

import (
	"fmt"
	llmperf "github.com/tmc/misc/xk6-llm-perf"
	"math"
	"strings"
	"testing"
)

func pairedFixture(t *testing.T) PairedExperiment {
	t.Helper()
	p, err := PlanPairs(7, 4, []string{"latency"})
	if err != nil {
		t.Fatal(err)
	}
	p.Policy = pairedTestPolicy()
	policyHash, _ := PairPolicyHash(*p.Policy)
	e := PairedExperiment{Plan: p}
	for i, order := range p.Orders {
		for j := range order {
			arm := string(order[j])
			v := 10.0
			if arm == "B" {
				v = 12
			}
			e.Runs = append(e.Runs, PairRun{Pair: i, Arm: arm, ProcessID: fmt.Sprintf("p%d%d", i, j), WorkHash: strings.Repeat("a", 64), BinaryHash: strings.Repeat("b", 64), ArtifactHash: strings.Repeat("c", 64), Policy: p.Policy, PolicyComplete: true, PolicyHash: policyHash, Successful: true, WorkAccepted: true, ClientQualified: true, Metrics: map[string]float64{"latency": v}})
		}
	}
	return e
}
func TestPaired(t *testing.T) {
	o := PairedOptions{Seed: 3, Resamples: 1000, Confidence: .95}
	e := pairedFixture(t)
	r, err := AnalyzePaired(e, o)
	if err != nil || r.Effects[0].Delta != 2 || *r.Effects[0].Ratio != 1.2 {
		t.Fatalf("%+v %v", r, err)
	}
	for i := range e.Runs {
		if e.Runs[i].Arm == "A" {
			e.Runs[i].Metrics["latency"] = 0
		}
	}
	r, err = AnalyzePaired(e, o)
	if err != nil || r.Effects[0].Ratio != nil || r.Effects[0].Delta != 12 {
		t.Fatalf("%+v %v", r, err)
	}
}
func TestPairedRefusals(t *testing.T) {
	for _, name := range []string{"missing", "order", "process", "work", "failed", "binary", "metric"} {
		t.Run(name, func(t *testing.T) {
			e := pairedFixture(t)
			switch name {
			case "missing":
				e.Runs = e.Runs[:7]
			case "order":
				e.Runs[0].Arm = "X"
			case "process":
				e.Runs[1].ProcessID = e.Runs[0].ProcessID
			case "work":
				e.Runs[0].WorkHash = strings.Repeat("d", 64)
			case "failed":
				e.Runs[0].Successful = false
			case "capacity":
				e.Runs[0].ClientQualified = false
			case "binary":
				e.Runs[0].BinaryHash = strings.Repeat("d", 64)
			case "metric":
				e.Runs[0].Metrics = map[string]float64{}
			}
			if _, err := AnalyzePaired(e, PairedOptions{Seed: 3, Resamples: 1000, Confidence: .95}); err == nil {
				t.Fatal("accepted confounded experiment")
			}
		})
	}
}
func ExamplePlanPairs() {
	p, _ := PlanPairs(1, 2, []string{"latency_ms"})
	fmt.Println(p.Version, len(p.Orders)) // Output: 1 2
}
func TestReadPairedStrict(t *testing.T) {
	for _, input := range []string{`{"plan":null}`, `{"plan":{"version":1,"version":1}}`, `{"unknown":1}`, `{} {}`} {
		if _, err := ReadPairedExperiment(strings.NewReader(input)); err == nil {
			t.Fatalf("accepted %s", input)
		}
	}
}
func TestQualificationRequiresVerifier(t *testing.T) {
	e := pairedFixture(t)
	o := PairedOptions{Seed: 3, Resamples: 1000, Confidence: .95}
	r, err := AnalyzePaired(e, o)
	if err != nil || r.Qualified || !strings.Contains(r.Effects[0].StatisticalStatus, "diagnostic") {
		t.Fatalf("%+v %v", r, err)
	}
	o.Verify = func(PairRun) error { return fmt.Errorf("unverified receipt") }
	if _, err = AnalyzePaired(e, o); err == nil {
		t.Fatal("accepted invalid evidence")
	}
	o.Verify = func(PairRun) error { return nil }
	r, err = AnalyzePaired(e, o)
	if err != nil || r.Qualified {
		t.Fatal("policy attestation not required", err)
	}
	o.VerifyPolicy = func(PairRun, PairPolicy) error { return nil }
	r, err = AnalyzePaired(e, o)
	if err != nil || !r.Qualified {
		t.Fatalf("%+v %v", r, err)
	}
}
func TestOverflowRefused(t *testing.T) {
	e := pairedFixture(t)
	for i := range e.Runs {
		if e.Runs[i].Arm == "A" {
			e.Runs[i].Metrics["latency"] = -1e308
		} else {
			e.Runs[i].Metrics["latency"] = 1e308
		}
	}
	if _, err := AnalyzePaired(e, PairedOptions{Seed: 3, Resamples: 1000, Confidence: .95}); err == nil {
		t.Fatal("accepted overflowing delta")
	}
}
func TestBootstrapInvalidConfidence(t *testing.T) {
	e := pairedFixture(t)
	for _, c := range []float64{math.NaN(), math.Inf(1), .9999999} {
		if _, err := AnalyzePaired(e, PairedOptions{Resamples: 1000, Confidence: c}); err == nil {
			t.Fatal("accepted invalid confidence", c)
		}
	}
}
func TestReadPairedDepth(t *testing.T) {
	if _, err := ReadPairedExperiment(strings.NewReader(strings.Repeat("[", 70) + "0" + strings.Repeat("]", 70))); err == nil {
		t.Fatal("accepted excessive nesting")
	}
}

func TestUnqualifiedClientRemainsDiagnostic(t *testing.T) {
	e := pairedFixture(t)
	e.Runs[0].ClientQualified = false
	r, err := AnalyzePaired(e, PairedOptions{Resamples: 1000, Confidence: .95, Verify: func(PairRun) error { return nil }, VerifyPolicy: func(PairRun, PairPolicy) error { return nil }})
	if err != nil || r.Qualified || !strings.Contains(r.Effects[0].StatisticalStatus, "diagnostic") {
		t.Fatalf("%+v %v", r, err)
	}
}
func ExampleReadPairedExperiment() {
	_, err := ReadPairedExperiment(strings.NewReader(`{"plan":null}`))
	fmt.Println(err) // Output: null experiment field
}
func ExampleAnalyzePaired() {
	plan, _ := PlanPairs(1, 2, []string{"latency"})
	_, err := AnalyzePaired(PairedExperiment{Plan: plan}, PairedOptions{Resamples: 1000, Confidence: .95})
	fmt.Println(err) // Output: missing or additional arm runs
}

func pairedTestPolicy() *PairPolicy {
	return &PairPolicy{MeasurementPolicy: "dispatch-content-v1/success-token-counts-v1/success-slo-v1", WindowPolicy: "attempt started and completed inside window; successful token totals", WarmupPolicy: "phase=measurement and per-client warmup exhausted", SLO: &llmperf.SLOReceipt{MaxTPOT: new(float64), MaxE2E: new(float64)}}
}
