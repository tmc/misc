package output

import (
	"fmt"
	llm "github.com/tmc/misc/xk6-llm-perf"
	"strings"
	"testing"
)

func TestAIPerfStatisticsAndPooledIntervals(t *testing.T) {
	run := runFixture()
	zero, ten := 0., 10.
	r := &run.Records[0]
	r.TTFT = &zero
	r.LastContentLatency = &ten
	r.TTST = &zero
	r.TTSTPolicy = "second recognized content event; not a physical second-token observation"
	r.ContentEventCount = 3
	r.ContentIntervals = []float64{0, 10}
	a, err := SummarizeWithOptions(run, llm.ReportOptions{AIPerf: true})
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := Summarize(run)
	if err != nil {
		t.Fatal(err)
	}
	if a.PercentileMethod != "linear_n_minus_1" || legacy.PercentileMethod != "nearest_rank" || a.StandardDeviationMethod != "population" {
		t.Fatal(a)
	}
	if !strings.Contains(a.Policy, "successful retained requests") || strings.Contains(a.Policy, "nearest-rank") {
		t.Fatal(a.Policy)
	}
	for _, d := range []Document{a, legacy} {
		found := false
		for _, s := range d.Metrics {
			if s.Name == "content_inter_chunk_latency" {
				found = true
				want := 0.
				if d.PercentileMethod == "linear_n_minus_1" {
					want = 5
				}
				if s.Count != 2 || s.P50 == nil || *s.P50 != want || *s.Std != 5 {
					t.Fatal(s)
				}
			}
		}
		if !found {
			t.Fatal("missing pooled intervals")
		}
	}
	if a.Attempts != legacy.Attempts || a.Failures != legacy.Failures || a.Successes != legacy.Successes {
		t.Fatal("population changed")
	}
}

func TestAIPerfOutputPair(t *testing.T) {
	s := statsWithOptions("test", "ms", []float64{0, 10}, llm.ReportOptions{AIPerf: true})
	if *s.P50 != 5 || *s.P90 != 9 || *s.P99 != 9.9 || *s.Std != 5 {
		t.Fatal(s)
	}
}

func ExampleSummarizeWithOptions() {
	d, err := SummarizeWithOptions(runFixture(), llm.ReportOptions{AIPerf: true})
	fmt.Println(d.PercentileMethod, err)
	// Output: linear_n_minus_1 <nil>
}
