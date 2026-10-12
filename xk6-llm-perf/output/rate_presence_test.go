package output

import (
	"bytes"
	"os"
	"testing"

	llm "github.com/tmc/misc/xk6-llm-perf"
)

func TestSavedMissingUsageOutputRates(t *testing.T) {
	data, err := os.ReadFile("../testdata/q2-missing-usage-run.json")
	if err != nil {
		t.Fatal(err)
	}
	run, err := llm.ReadRun(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := Summarize(run)
	if err != nil {
		t.Fatal(err)
	}
	rates := 0
	for _, metric := range doc.Metrics {
		if metric.Name == "input_throughput" || metric.Name == "output_throughput" {
			rates++
			if metric.Count != 0 || metric.Mean != nil {
				t.Fatalf("admitted historical partial rate: %+v", metric)
			}
		}
	}
	if rates != 2 {
		t.Fatalf("rates=%d", rates)
	}
}
