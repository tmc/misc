package llmperf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestAIPerfDistribution(t *testing.T) {
	for _, tt := range []struct {
		name                  string
		values                []float64
		median, p95, p99, std float64
	}{
		{"pair", []float64{0, 10}, 5, 9.5, 9.9, 5},
		{"singleton", []float64{4}, 4, 4, 4, 0},
		{"odd", []float64{0, 10, 20}, 10, 19, 19.8, math.Sqrt(200.0 / 3)},
		{"repeated", []float64{2, 2, 2, 2}, 2, 2, 2, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			d := distributionWithOptions(tt.values, 2, "ms", ReportOptions{AIPerf: true})
			for i, got := range []*float64{d.P50, d.P95, d.P99, d.StdDev} {
				want := []float64{tt.median, tt.p95, tt.p99, tt.std}[i]
				if got == nil || math.Abs(*got-want) > 1e-12 {
					t.Fatalf("stat%d=%v want%v", i, got, want)
				}
			}
			if d.Missing != 2 || d.Samples != len(tt.values) {
				t.Fatal(d)
			}
		})
	}
	if d := distributionWithOptions(nil, 3, "ms", ReportOptions{AIPerf: true}); d.P50 != nil || d.StdDev != nil || d.Missing != 3 {
		t.Fatal(d)
	}
	if d := distribution([]float64{0, 10}, 0, "ms"); *d.P50 != 0 {
		t.Fatal("legacy estimator changed")
	}
}

func TestAIPerfRequestAnalysis(t *testing.T) {
	run := analysisFixture()
	a, err := AnalyzeRequestsWithOptions(run, ReportOptions{AIPerf: true})
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := AnalyzeRequests(run)
	if err != nil {
		t.Fatal(err)
	}
	if *a.Metrics["full_response_latency"].P50 != 200 || *legacy.Metrics["full_response_latency"].P50 != 100 {
		t.Fatal(a, legacy)
	}
	if !reflect.DeepEqual(a.Outcomes, legacy.Outcomes) || !reflect.DeepEqual(a.CountSources, legacy.CountSources) || a.Population != legacy.Population {
		t.Fatal("population changed")
	}
}

func TestAIPerfDispatchSlices(t *testing.T) {
	run := RunSummary{SchemaVersion: 1, StartedAt: 0, FinishedAt: 2000, Duration: 2, Attempts: 3, Failures: 3}
	for i, span := range [][2]float64{{0, 1000}, {1000, 1500}, {2000, 2000}} {
		run.Records = append(run.Records, Measurement{RequestID: fmt.Sprint(i), SchemaVersion: 1, Phase: "measurement", Recorded: true, Attempted: true, DispatchedAt: span[0], StartedAt: span[0], FinishedAt: span[1], FullResponseLatency: span[1] - span[0], Outcome: "failure", InputTokens: TokenCount{Source: "unavailable"}, OutputTokens: TokenCount{Source: "unavailable"}, TimestampBasis: "dispatch Unix clock anchor plus monotonic elapsed time"})
	}
	a, err := TimeSlicesWithOptions(run, 1, ReportOptions{AIPerf: true})
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := TimeSlices(run, 1)
	if err != nil {
		t.Fatal(err)
	}
	if a[0].Attempts != 1 || a[1].Attempts != 1 || legacy[0].Attempts != 0 || legacy[1].Attempts != 3 {
		t.Fatal(a, legacy)
	}
	data, err := json.Marshal(a[0])
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("completed_attempts")) || !bytes.Contains(data, []byte("selected_attempts")) || !bytes.Contains(data, []byte("dispatch_start")) {
		t.Fatal(string(data))
	}
	data, err = json.Marshal(legacy[0])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("completed_attempts")) || bytes.Contains(data, []byte("selected_attempts")) {
		t.Fatal(string(data))
	}
	if a[0].Policy == legacy[0].Policy {
		t.Fatal("missing policy distinction")
	}
	run.Records[0].TimestampBasis = ""
	if _, err = TimeSlicesWithOptions(run, 1, ReportOptions{AIPerf: true}); err == nil {
		t.Fatal("accepted unavailable dispatch evidence")
	}
}

func ExampleAnalyzeRequestsWithOptions() {
	run := RunSummary{SchemaVersion: 1, StartedAt: 1000, FinishedAt: 2000, Duration: 1}
	a, err := AnalyzeRequestsWithOptions(run, ReportOptions{AIPerf: true})
	fmt.Println(a.Metrics["ttft"].Samples, err)
	// Output: 0 <nil>
}

func ExampleTimeSlicesWithOptions() {
	run := RunSummary{SchemaVersion: 1, StartedAt: 1000, FinishedAt: 3000, Duration: 2}
	slices, err := TimeSlicesWithOptions(run, 1, ReportOptions{AIPerf: true})
	fmt.Println(len(slices), err)
	// Output: 2 <nil>
}

func TestSliceRoundTripModesAndAvailability(t *testing.T) {
	for _, selection := range []string{"", "dispatch_start"} {
		for _, missing := range [][2]int{{0, 0}, {0, 1}, {1, 0}, {1, 1}} {
			original := SliceSummary{StartedAt: 1000, FinishedAt: 2000, Attempts: 3, AttemptSelection: selection, Successes: 2, Failures: 1, GoodRequests: 1, InputTokens: 4, OutputTokens: 8, InputThroughput: 4, OutputThroughput: 8, MissingInputCounts: missing[0], MissingOutputCounts: missing[1], Policy: "fixture"}
			first, err := json.Marshal(original)
			if err != nil {
				t.Fatal(err)
			}
			var decoded SliceSummary
			if err := json.Unmarshal(first, &decoded); err != nil {
				t.Fatal(err, string(first))
			}
			if decoded.Attempts != 3 || decoded.Successes != 2 || decoded.Failures != 1 || decoded.AttemptSelection != selection || decoded.MissingInputCounts != missing[0] || decoded.MissingOutputCounts != missing[1] {
				t.Fatalf("lost accounting %+v", decoded)
			}
			second, err := json.Marshal(decoded)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(first, second) {
				t.Fatalf("changed round trip\n%s\n%s", first, second)
			}
		}
	}
}

func TestSliceDecodeNegativeControls(t *testing.T) {
	valid, err := json.Marshal(SliceSummary{Attempts: 3, Successes: 3, AttemptSelection: "dispatch_start"})
	if err != nil {
		t.Fatal(err)
	}
	mutate := func(change func(map[string]any)) string {
		var value map[string]any
		if err := json.Unmarshal(valid, &value); err != nil {
			t.Fatal(err)
		}
		change(value)
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	cases := map[string]string{
		"both counts":                     mutate(func(v map[string]any) { v["completed_attempts"] = 3 }),
		"missing selected":                mutate(func(v map[string]any) { delete(v, "selected_attempts") }),
		"missing selection":               mutate(func(v map[string]any) { delete(v, "attempt_selection") }),
		"unknown selection":               mutate(func(v map[string]any) { v["attempt_selection"] = "completion" }),
		"unknown field":                   mutate(func(v map[string]any) { v["other"] = 1 }),
		"null selected":                   mutate(func(v map[string]any) { v["selected_attempts"] = nil }),
		"negative selected":               mutate(func(v map[string]any) { v["selected_attempts"] = -1 }),
		"accounting contradiction":        mutate(func(v map[string]any) { v["selected_attempts"] = 0 }),
		"null observed rate":              mutate(func(v map[string]any) { v["input_throughput"] = nil }),
		"missing rate":                    mutate(func(v map[string]any) { delete(v, "input_throughput") }),
		"missing count numeric rate":      mutate(func(v map[string]any) { v["missing_output_counts"] = 1 }),
		"negative missing count":          mutate(func(v map[string]any) { v["missing_input_counts"] = -1 }),
		"missing count exceeds successes": mutate(func(v map[string]any) { v["missing_input_counts"] = 4; v["input_throughput"] = nil }),
		"unknown case alias":              strings.Replace(string(valid), "selected_attempts", "Selected_Attempts", 1),
		"nested alias":                    strings.Replace(string(valid), "effective", "Effective", 1),
		"duplicate":                       strings.Replace(string(valid), `"selected_attempts":3`, `"selected_attempts":3,"selected_attempts":3`, 1),
		"duplicate alias":                 strings.Replace(string(valid), `"selected_attempts":3`, `"selected_attempts":3,"Selected_Attempts":3`, 1),
		"trailing":                        string(valid) + "{}",
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			got := SliceSummary{Attempts: 7}
			if err := json.Unmarshal([]byte(data), &got); err == nil {
				t.Fatalf("accepted %s", data)
			}
			if got.Attempts != 7 {
				t.Fatal("mutated receiver on refusal")
			}
		})
	}
	legacy := mutate(func(v map[string]any) {
		delete(v, "attempt_selection")
		delete(v, "selected_attempts")
		v["completed_attempts"] = 3
	})
	var decoded SliceSummary
	if err := json.Unmarshal([]byte(legacy), &decoded); err != nil || decoded.Attempts != 3 {
		t.Fatal(decoded, err)
	}
	if _, err := json.Marshal(SliceSummary{AttemptSelection: "unknown"}); err == nil {
		t.Fatal("serialized unknown selection")
	}
}
