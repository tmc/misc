package output

import (
	"bytes"
	"encoding/xml"
	"fmt"
	llm "github.com/tmc/misc/xk6-llm-perf"
	"strings"
	"testing"
)

func ExampleCompare() {
	a, b := runFixture(), runFixture()
	a.RunID = "a"
	b.RunID = "b"
	c, err := Compare([]llm.RunSummary{a, b})
	fmt.Println(len(c.Runs), err) // Output: 2 <nil>
}
func ExampleComparisonSVG() {
	v := 10.0
	c := Comparison{SchemaVersion: 1, Runs: []Document{{SchemaVersion: 2, RunID: "a", Metrics: []Statistic{{Name: "ttft", Unit: "ms", Count: 1, Mean: &v}}}, {SchemaVersion: 2, RunID: "b"}}}
	var b bytes.Buffer
	err := ComparisonSVG(&b, c, "ttft")
	fmt.Println(strings.Contains(b.String(), "N/A"), err) // Output: true <nil>
}
func TestComparisonPopulations(t *testing.T) {
	a, b := runFixture(), runFixture()
	a.RunID = "a<script>"
	b.RunID = "b"
	b.Records[0].TTFT = nil
	c, err := Compare([]llm.RunSummary{a, b})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := ComparisonSVG(&out, c, "ttft"); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	for _, wanted := range []string{"N/A", "n=0", "NOT statistically gated", "a&lt;script&gt;"} {
		if !strings.Contains(s, wanted) {
			t.Fatalf("missing %q", wanted)
		}
	}
	if strings.Contains(s, "a<script>") {
		t.Fatal("unsafe SVG")
	}
	decoder := xml.NewDecoder(strings.NewReader(s))
	for {
		_, err := decoder.Token()
		if err != nil {
			if err.Error() != "EOF" {
				t.Fatal(err)
			}
			break
		}
	}
	if _, err := Compare([]llm.RunSummary{a, a}); err == nil {
		t.Fatal("duplicate run accepted")
	}
	out.Reset()
	if err := ComparisonSVG(&out, c, "unknown"); err == nil {
		t.Fatal("absent metric accepted")
	}
}
func TestComparisonUnitMismatch(t *testing.T) {
	v := 1.0
	c := Comparison{SchemaVersion: 1, Runs: []Document{{SchemaVersion: 2, RunID: "a", Metrics: []Statistic{{Name: "x", Unit: "ms", Count: 1, Mean: &v}}}, {SchemaVersion: 2, RunID: "b", Metrics: []Statistic{{Name: "x", Unit: "s", Count: 1, Mean: &v}}}}}
	if err := ComparisonSVG(&bytes.Buffer{}, c, "x"); err == nil {
		t.Fatal("unit mismatch accepted")
	}
}

func TestComparisonFiniteMaximum(t *testing.T) {
	v := 1e308
	c := Comparison{SchemaVersion: 1, Runs: []Document{{SchemaVersion: 2, RunID: "a", Metrics: []Statistic{{Name: "x", Unit: "ms", Count: 1, Mean: &v}}}, {SchemaVersion: 2, RunID: "b"}}}
	var out bytes.Buffer
	if err := ComparisonSVG(&out, c, "x"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "Inf") {
		t.Fatal("finite mean overflowed SVG geometry")
	}
}

func TestComparisonXMLControls(t *testing.T) {
	v := 1.0
	c := Comparison{SchemaVersion: 1, Runs: []Document{{SchemaVersion: 2, RunID: "a\x00", Metrics: []Statistic{{Name: "x", Unit: "ms", Count: 1, Mean: &v}}}, {SchemaVersion: 2, RunID: "b"}}}
	var out bytes.Buffer
	if err := ComparisonSVG(&out, c, "x"); err != nil {
		t.Fatal(err)
	}
	decoder := xml.NewDecoder(&out)
	for {
		_, err := decoder.Token()
		if err != nil {
			if err.Error() != "EOF" {
				t.Fatal(err)
			}
			break
		}
	}
}

func TestComparisonSLOPolicies(t *testing.T) {
	a, b := runFixture(), runFixture()
	a.RunID = "a"
	b.RunID = "b"
	for _, r := range []*llm.RunSummary{&a, &b} {
		r.MeasurementPolicy = "dispatch-content-v1/success-token-counts-v1/success-slo-v1"
		r.WindowPolicy = "attempt started and completed inside window; successful token totals"
		r.WarmupPolicy = "phase=measurement and per-client warmup exhausted"
	}
	a.Records[0].SLO = &llm.SLOReceipt{MaxTTFT: 10, MaxTPOT: new(float64), MaxE2E: new(float64)}
	b.Records[0].SLO = &llm.SLOReceipt{MaxTTFT: 15, MaxTPOT: new(float64), MaxE2E: new(float64)}
	c, err := Compare([]llm.RunSummary{a, b})
	if err != nil {
		t.Fatal(err)
	}
	if c.Policies["a"].SLO.MaxTTFT != 10 || c.Policies["b"].SLO.MaxTTFT != 15 || !c.PolicyComplete["a"] || !c.PolicyComplete["b"] {
		t.Fatal("actual SLO distinction lost")
	}
	b.Records[0].SLO = nil
	c, err = Compare([]llm.RunSummary{a, b})
	if err != nil {
		t.Fatal(err)
	}
	if c.PolicyComplete["b"] || c.Policies["b"].SLO != nil {
		t.Fatal("missing SLO promoted")
	}
	a.Attempts = 2
	a.Successes = 2
	a.MissingOutputCounts = 2
	second := a.Records[0]
	second.SLO = &llm.SLOReceipt{MaxTTFT: 15, MaxTPOT: new(float64), MaxE2E: new(float64)}
	a.Records = append(a.Records, second)
	if _, err := Compare([]llm.RunSummary{a, b}); err == nil {
		t.Fatal("within-run SLO mismatch accepted")
	}
}
