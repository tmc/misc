package serving

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"

	llmperf "github.com/tmc/misc/xk6-llm-perf"
)

func policyRun() llmperf.RunSummary {
	zero := 0
	p := pairedTestPolicy()
	return llmperf.RunSummary{SchemaVersion: 1, MeasurementPolicy: p.MeasurementPolicy, WindowPolicy: p.WindowPolicy, WarmupPolicy: p.WarmupPolicy, StartedAt: 1000, FinishedAt: 2000, Duration: 1, Attempts: 1, Successes: 1, GoodRequests: 1, Goodput: 1, Records: []llmperf.Measurement{{SchemaVersion: 1, Phase: "measurement", Outcome: "success", Recorded: true, Attempted: true, Good: true, SLO: &llmperf.SLOReceipt{MaxTPOT: new(float64), MaxE2E: new(float64)}, DispatchedAt: 1000, FinishedAt: 1001, FullResponseLatency: 1, InputTokens: llmperf.TokenCount{Value: &zero, Source: "reported"}, OutputTokens: llmperf.TokenCount{Value: &zero, Source: "reported"}}}}
}
func TestRunPairPolicy(t *testing.T) {
	for _, name := range []string{"disabled with unavailable timing", "strict with unavailable timing", "missing SLO", "missing measurement"} {
		t.Run(name, func(t *testing.T) {
			r := policyRun()
			switch name {
			case "strict with unavailable timing":
				r.Records[0].SLO.MaxTTFT = 1
				r.Records[0].Good = false
				r.GoodRequests = 0
				r.Goodput = 0
			case "missing SLO":
				r.Records[0].SLO = nil
			case "missing measurement":
				r.MeasurementPolicy = ""
			}
			p, complete, err := RunPairPolicy(r)
			if err != nil || p == nil {
				t.Fatal(err)
			}
			want := name != "missing SLO" && name != "missing measurement"
			if complete != want {
				t.Fatalf("complete=%v", complete)
			}
		})
	}
}
func TestRunPairPolicyWithinRunMismatch(t *testing.T) {
	r := policyRun()
	other := r.Records[0]
	other.SLO = &llmperf.SLOReceipt{MaxTTFT: 1}
	other.Good = false
	r.Records = append(r.Records, other)
	r.Attempts = 2
	r.Successes = 2
	if _, _, err := RunPairPolicy(r); err == nil || !strings.Contains(err.Error(), "within run") {
		t.Fatal("accepted opposite explicit SLOs")
	}
}
func TestRunPairPolicyMissingAmongKnown(t *testing.T) {
	r := policyRun()
	other := r.Records[0]
	other.SLO = nil
	r.Records = append(r.Records, other)
	r.Attempts, r.Successes, r.GoodRequests, r.Goodput = 2, 2, 2, 2
	p, complete, err := RunPairPolicy(r)
	if err != nil || complete || p == nil || p.SLO == nil {
		t.Fatalf("policy=%v complete=%v error=%v", p, complete, err)
	}
}

func TestPairedPolicyAdmission(t *testing.T) {
	for _, name := range []string{"missing preregistration", "missing verifier", "missing actual", "mismatched actual", "tampered hash", "verifier refusal", "valid zero baseline"} {
		t.Run(name, func(t *testing.T) {
			e := pairedFixture(t)
			o := PairedOptions{Resamples: 1000, Confidence: .95, Verify: func(PairRun) error { return nil }, VerifyPolicy: func(PairRun, PairPolicy) error { return nil }}
			wantErr := false
			wantQualified := false
			switch name {
			case "missing preregistration":
				e.Plan.Policy = nil
			case "missing verifier":
				o.VerifyPolicy = nil
			case "missing actual":
				e.Runs[0].Policy = nil
				e.Runs[0].PolicyHash = ""
				e.Runs[0].PolicyComplete = false
			case "mismatched actual":
				p := pairedTestPolicy()
				p.SLO.MaxTTFT = 1
				e.Runs[0].Policy = p
				e.Runs[0].PolicyHash, _ = PairPolicyHash(*p)
				wantErr = true
			case "tampered hash":
				e.Runs[0].PolicyHash = e.Runs[0].WorkHash
				wantErr = true
			case "verifier refusal":
				o.VerifyPolicy = func(PairRun, PairPolicy) error { return fmt.Errorf("receipt lacks policy") }
				wantErr = true
			case "valid zero baseline":
				wantQualified = true
				for i := range e.Runs {
					e.Runs[i].Metrics["latency"] = 0
				}
			}
			r, err := AnalyzePaired(e, o)
			if (err != nil) != wantErr {
				t.Fatalf("error=%v", err)
			}
			if err == nil && r.Qualified != wantQualified {
				t.Fatalf("qualified=%v", r.Qualified)
			}
			if name == "valid zero baseline" && (r.Effects[0].Delta != 0 || r.Effects[0].Ratio != nil) {
				t.Fatal("zero changed")
			}
		})
	}
}
func ExamplePairPolicyHash() {
	_, err := PairPolicyHash(PairPolicy{})
	fmt.Println(err) // Output: unsupported or incomplete paired measurement policy
}
func ExampleRunPairPolicy() {
	_, _, err := RunPairPolicy(llmperf.RunSummary{})
	fmt.Println(err != nil) // Output: true
}
func ExampleMergePairPolicy() {
	_, err := MergePairPolicy(PairPolicy{SLO: &llmperf.SLOReceipt{MaxTPOT: new(float64), MaxE2E: new(float64)}}, PairPolicy{SLO: &llmperf.SLOReceipt{MaxTTFT: 1}})
	fmt.Println(err) // Output: slo policy differs between runs
}

func TestExtendedSLOPolicyPresence(t *testing.T) {
	for _, data := range []string{
		`{"max_ttft_ms":0,"max_ttfo_ms":0,"max_chunk_latency_ms":0}`,
		`{"max_ttft_ms":0,"max_ttfo_ms":0,"max_chunk_latency_ms":0,"max_tpot_ms":null,"max_e2e_ms":null}`,
	} {
		r := policyRun()
		r.Records[0].SLO = new(llmperf.SLOReceipt)
		if err := json.Unmarshal([]byte(data), r.Records[0].SLO); err != nil {
			t.Fatal(err)
		}
		p, complete, err := RunPairPolicy(r)
		if err != nil || complete || p == nil || p.SLO.MaxTPOT != nil || p.SLO.MaxE2E != nil {
			t.Fatalf("legacy scope: policy=%+v complete=%v err=%v", p, complete, err)
		}
		if _, err := PairPolicyHash(*p); err != nil {
			t.Fatal("legacy identity unreadable", err)
		}
	}
	r := policyRun()
	p, complete, err := RunPairPolicy(r)
	if err != nil || !complete || p.SLO.MaxTPOT == nil || *p.SLO.MaxTPOT != 0 {
		t.Fatal("explicit disabled policy incomplete", err)
	}
}

func TestExtendedPolicyValueIdentity(t *testing.T) {
	p := pairedTestPolicy()
	q := pairedTestPolicy()
	a, err := PairPolicyHash(*p)
	if err != nil {
		t.Fatal(err)
	}
	b, err := PairPolicyHash(*q)
	if err != nil || a != b {
		t.Fatal("pointer address changed hash")
	}
	*q.SLO.MaxTPOT = 1
	if _, err := MergePairPolicy(*p, *q); err == nil {
		t.Fatal("distinct TPOT policies merged")
	}
	if b, err = PairPolicyHash(*q); err != nil || a == b {
		t.Fatal("TPOT absent from hash")
	}
	q = pairedTestPolicy()
	*q.SLO.MaxE2E = 1
	if _, err := MergePairPolicy(*p, *q); err == nil {
		t.Fatal("distinct E2E policies merged")
	}
}

func TestPolicyHashCanonicalZero(t *testing.T) {
	baseline := pairedTestPolicy()
	want, err := PairPolicyHash(*baseline)
	if err != nil {
		t.Fatal(err)
	}
	if want != "6b9cfea82383db688242d8c03ea4b9b30893425a08f47193d9ecabf08520f35b" {
		t.Fatal("explicit positive-zero identity changed", want)
	}
	for _, name := range []string{"ttft", "ttfo", "chunk", "tpot", "e2e"} {
		t.Run(name, func(t *testing.T) {
			p := pairedTestPolicy()
			var limit *float64
			switch name {
			case "ttft":
				limit = &p.SLO.MaxTTFT
			case "ttfo":
				limit = &p.SLO.MaxTTFO
			case "chunk":
				limit = &p.SLO.MaxChunkLatency
			case "tpot":
				limit = p.SLO.MaxTPOT
			case "e2e":
				limit = p.SLO.MaxE2E
			}
			*limit = math.Copysign(0, -1)
			if !baseline.SLO.Equal(*p.SLO) {
				t.Fatal("signed zero changed limits")
			}
			got, err := PairPolicyHash(*p)
			if err != nil || got != want {
				t.Fatalf("hash=%s want=%s err=%v", got, want, err)
			}
			if !math.Signbit(*limit) {
				t.Fatal("hashing mutated caller's limit")
			}
		})
	}
	legacy := *baseline
	legacy.SLO = &llmperf.SLOReceipt{}
	old, err := PairPolicyHash(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if old != "61db8aaeb0ee129acd0a7e995a063bc7781568d3554d7e66ef1b578ca6e880f6" {
		t.Fatal("legacy positive-zero identity changed", old)
	}
	if old == want {
		t.Fatal("absent limits became explicit disabled zero")
	}
	legacy.SLO.MaxTTFT = math.Copysign(0, -1)
	got, err := PairPolicyHash(legacy)
	if err != nil || got != old || !math.Signbit(legacy.SLO.MaxTTFT) {
		t.Fatal("legacy zero identity or caller changed")
	}
}
