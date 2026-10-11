package output

import (
	"encoding/json"
	"os"
	"testing"

	llm "github.com/tmc/misc/xk6-llm-perf"
)

func TestWorkSummary(t *testing.T) {
	path := os.Getenv("WORK_SMOKE_RUN")
	if path == "" {
		t.Skip("registered binary receipt is supplied explicitly")
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	run, err := llm.ReadRun(f)
	if err != nil {
		t.Fatal(err)
	}
	d, err := Summarize(run)
	if err != nil {
		t.Fatal(err)
	}
	if d.WorkQualified != 3 || d.WorkAccepted != 1 || d.WorkRejected != 1 || d.WorkUnavailable != 1 || d.WorkGoodRequests != 1 || d.WorkPolicy == "" {
		t.Fatal(d)
	}
	found := false
	for _, metric := range d.Metrics {
		if metric.Name == "work_goodput" {
			found = true
			if metric.Mean == nil || *metric.Mean != run.WorkGoodput {
				t.Fatal(metric)
			}
		}
	}
	if !found {
		t.Fatal("work_goodput missing")
	}
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(b, &fields); err != nil {
		t.Fatal(err)
	}
	if _, ok := fields["work_qualified"]; !ok {
		t.Fatal("work population missing")
	}
}
