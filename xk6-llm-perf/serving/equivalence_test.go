package serving

import (
	"math"
	"testing"
)

func equivalenceFixture(t *testing.T) (EquivalencePlan, PairedExperiment, PairedOptions) {
	t.Helper()
	e := pairedFixture(t)
	p := EquivalencePlan{Pairs: e.Plan, Margins: []EquivalenceMargin{{Metric: "latency", Unit: "ms", Lower: -1, Upper: 1, Reason: "synthetic fixture oracle; not a model performance margin"}}, BootstrapSeed: 3, Resamples: 1000, Confidence: .95, SampleSizeReason: "four deterministic fixture pairs; not a power claim"}
	o := PairedOptions{Seed: 3, Resamples: 1000, Confidence: .95, Verify: func(PairRun) error { return nil }, VerifyPolicy: func(PairRun, PairPolicy) error { return nil }}
	return p, e, o
}

func TestEquivalenceControls(t *testing.T) {
	for _, tc := range []struct {
		name  string
		delta float64
		want  string
	}{
		{"null", 0, "PERFORMANCE_EQUIVALENT"}, {"inside", .5, "PERFORMANCE_EQUIVALENT"},
		{"positive boundary", 1, "PERFORMANCE_HOLD"}, {"negative boundary", -1, "PERFORMANCE_HOLD"},
		{"positive effect", 2, "PERFORMANCE_HOLD"}, {"negative effect", -2, "PERFORMANCE_HOLD"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, e, o := equivalenceFixture(t)
			for i := range e.Runs {
				if e.Runs[i].Arm == "B" {
					e.Runs[i].Metrics["latency"] = 10 + tc.delta
				}
			}
			hash, err := p.Hash()
			if err != nil {
				t.Fatal(err)
			}
			r, err := AnalyzeEquivalence(p, hash, e, o)
			if err != nil || r.Status != tc.want || len(r.Paired.Effects) != 1 {
				t.Fatalf("%+v %v", r, err)
			}
		})
	}
}

func TestEquivalenceRefusals(t *testing.T) {
	for _, name := range []string{"margin", "unit", "reason", "sample reason", "hash", "estimator", "policy", "failed", "process", "family", "nonfinite"} {
		t.Run(name, func(t *testing.T) {
			p, e, o := equivalenceFixture(t)
			hash, err := p.Hash()
			if err != nil {
				t.Fatal(err)
			}
			switch name {
			case "margin":
				p.Margins[0].Upper = 3
			case "unit":
				p.Margins[0].Unit = ""
			case "reason":
				p.Margins[0].Reason = ""
			case "sample reason":
				p.SampleSizeReason = ""
			case "hash":
				hash = ""
			case "estimator":
				o.Seed++
			case "policy":
				e.Runs[0].PolicyComplete = false
			case "failed":
				e.Runs[0].Successful = false
			case "process":
				e.Runs[1].ProcessID = e.Runs[0].ProcessID
			case "family":
				p.Margins[0].Metric = "throughput"
			case "nonfinite":
				p.Margins[0].Upper = math.Inf(1)
			}
			if _, err := AnalyzeEquivalence(p, hash, e, o); err == nil {
				t.Fatal("accepted invalid preregistration or population")
			}
		})
	}
}

func TestEquivalenceUnverifiedHold(t *testing.T) {
	for _, name := range []string{"receipt", "policy", "client"} {
		t.Run(name, func(t *testing.T) {
			p, e, o := equivalenceFixture(t)
			for i := range e.Runs {
				e.Runs[i].Metrics["latency"] = 10
			}
			switch name {
			case "receipt":
				o.Verify = nil
			case "policy":
				o.VerifyPolicy = nil
			case "client":
				e.Runs[0].ClientQualified = false
			}
			h, _ := p.Hash()
			r, err := AnalyzeEquivalence(p, h, e, o)
			if err != nil || r.Status != "PERFORMANCE_HOLD" || !r.WithinBounds[0] {
				t.Fatalf("%+v %v", r, err)
			}
		})
	}
}

func TestEquivalenceUncertaintyAndFamily(t *testing.T) {
	p, e, o := equivalenceFixture(t)
	for i := range e.Runs {
		if e.Runs[i].Arm == "B" {
			e.Runs[i].Metrics["latency"] = 10
			if e.Runs[i].Pair == 0 {
				e.Runs[i].Metrics["latency"] = 13
			}
		}
	}
	h, _ := p.Hash()
	r, err := AnalyzeEquivalence(p, h, e, o)
	if err != nil || r.Status != "PERFORMANCE_HOLD" || r.Paired.Effects[0].Delta >= 1 || r.WithinBounds[0] {
		t.Fatalf("mean inside margin must not replace confidence bounds: %+v %v", r, err)
	}
	p, e, o = equivalenceFixture(t)
	p.Pairs.Metrics = append(p.Pairs.Metrics, "throughput")
	e.Plan = p.Pairs
	p.Margins = append(p.Margins, EquivalenceMargin{Metric: "throughput", Unit: "requests/s", Lower: -1, Upper: 1, Reason: "synthetic second metric control"})
	for i := range e.Runs {
		e.Runs[i].Metrics["latency"] = 10
		e.Runs[i].Metrics["throughput"] = 10
		if e.Runs[i].Arm == "B" {
			e.Runs[i].Metrics["throughput"] = 12
		}
	}
	h, err = p.Hash()
	if err != nil {
		t.Fatal(err)
	}
	r, err = AnalyzeEquivalence(p, h, e, o)
	if err != nil || r.Status != "PERFORMANCE_HOLD" || len(r.WithinBounds) != 2 || !r.WithinBounds[0] || r.WithinBounds[1] {
		t.Fatalf("all declared metrics must pass: %+v %v", r, err)
	}
}

func TestEquivalenceChangedPolicy(t *testing.T) {
	p, e, o := equivalenceFixture(t)
	changed := *e.Runs[0].Policy
	changed.MeasurementPolicy = "different"
	e.Runs[0].Policy = &changed
	h, _ := p.Hash()
	if _, err := AnalyzeEquivalence(p, h, e, o); err == nil {
		t.Fatal("accepted changed applied policy")
	}
}
