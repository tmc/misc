package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tmc/misc/xk6-llm-perf/serving"
)

func TestCompareCommand(t *testing.T) {
	p, _ := serving.PlanPairs(1, 2, []string{"latency"})
	e := serving.PairedExperiment{Plan: p}
	for i, order := range p.Orders {
		for j := range order {
			e.Runs = append(e.Runs, serving.PairRun{Pair: i, Arm: string(order[j]), ProcessID: fmt.Sprint(i, j), WorkHash: strings.Repeat("a", 64), BinaryHash: strings.Repeat("b", 64), ArtifactHash: strings.Repeat("c", 64), Successful: true, WorkAccepted: true, ClientQualified: true, Metrics: map[string]float64{"latency": 1}})
		}
	}
	b, _ := json.Marshal(e)
	path := filepath.Join(t.TempDir(), "experiment.json")
	if err := os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	if code := runCompare([]string{"-input", path, "-resamples", "1000"}, &out, &stderr); code != 0 {
		t.Fatal(code, stderr.String())
	}
	var result serving.PairedResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Qualified || !strings.Contains(result.Effects[0].StatisticalStatus, "diagnostic") {
		t.Fatal(out.String())
	}
	for _, args := range [][]string{{}, {"-input", path, "extra"}, {"-input", path, "-confidence", "1"}} {
		if runCompare(args, &out, &stderr) == 0 {
			t.Fatal("accepted invalid arguments", args)
		}
	}
}
