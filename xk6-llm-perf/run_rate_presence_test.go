package llmperf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"
)

func rateRun(input, output *int) RunSummary {
	r := formulaRun()
	m := &r.Records[0]
	m.InputTokens = TokenCount{Value: input, Source: "reported"}
	m.OutputTokens = TokenCount{Value: output, Source: "reported"}
	m.DecodeTokens = nil
	m.FirstChunkSource = "unavailable"
	m.ITL = nil
	m.FullResponseITL = nil
	r.InputTokens = 0
	r.OutputTokens = 0
	if input == nil {
		m.InputTokens.Source = "unavailable"
		r.MissingInputCounts = 1
	} else {
		r.InputTokens = int64(*input)
	}
	if output == nil {
		m.OutputTokens.Source = "unavailable"
		r.MissingOutputCounts = 1
	} else {
		r.OutputTokens = int64(*output)
	}
	r.InputThroughput = float64(r.InputTokens) / r.Duration
	r.OutputThroughput = float64(r.OutputTokens) / r.Duration
	return r
}

func TestRunRatePresence(t *testing.T) {
	for _, tt := range []struct {
		name          string
		input, output *int
	}{
		{"missing", nil, nil}, {"input only", intPtr(4), nil}, {"output only", nil, intPtr(4)},
		{"observed zero", intPtr(0), intPtr(0)}, {"complete", intPtr(4), intPtr(4)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := rateRun(tt.input, tt.output)
			data, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(data, &fields); err != nil {
				t.Fatal(err)
			}
			for _, c := range []struct {
				name  string
				count *int
			}{{"input_throughput", tt.input}, {"output_throughput", tt.output}} {
				if (string(fields[c.name]) == "null") != (c.count == nil) {
					t.Fatalf("%s=%s", c.name, fields[c.name])
				}
			}
			got, err := ReadRun(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if got.InputThroughputAvailable() != (tt.input != nil) || got.OutputThroughputAvailable() != (tt.output != nil) {
				t.Fatal("availability lost")
			}
			slices, err := TimeSlices(got, 1)
			if err != nil {
				t.Fatal(err)
			}
			data, err = json.Marshal(slices[0])
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(data, &fields); err != nil {
				t.Fatal(err)
			}
			if (string(fields["input_throughput"]) == "null") != (tt.input == nil) || (string(fields["output_throughput"]) == "null") != (tt.output == nil) {
				t.Fatalf("slice=%s", data)
			}
			// Legacy omitted rates are recomputed only for complete token populations.
			data, _ = json.Marshal(r)
			fields = nil
			json.Unmarshal(data, &fields)
			delete(fields, "input_throughput")
			delete(fields, "output_throughput")
			data, _ = json.Marshal(fields)
			if _, err := ReadRun(bytes.NewReader(data)); err != nil {
				t.Fatal(err)
			}
			if tt.input == nil {
				fields["input_throughput"] = json.RawMessage(`99`)
				data, _ = json.Marshal(fields)
				if _, err := ReadRun(bytes.NewReader(data)); err == nil {
					t.Fatal("contradictory legacy partial rate admitted")
				}
			}
		})
	}
	r := rateRun(intPtr(4), intPtr(4))
	r.Records[0].InputTokens.Source = "estimated"
	r.EstimatedInputCounts = 1
	data, _ := json.Marshal(r)
	if _, err := ReadRun(bytes.NewReader(data)); err != nil || !r.InputThroughputAvailable() {
		t.Fatalf("estimated policy changed: %v", err)
	}
}

func TestRunMixedRatePopulation(t *testing.T) {
	var state runState
	if err := state.begin(RunOptions{RunID: "mixed", MaxRecords: 2}); err != nil {
		t.Fatal(err)
	}
	for i := range 2 {
		now := time.Now()
		m := Measurement{SchemaVersion: 1, Phase: "measurement", Attempted: true, Recorded: true, Outcome: "success", RequestID: fmt.Sprint(i), requestStart: now, DispatchedAt: millis(now), FinishedAt: millis(now), InputTokens: count(4, "reported"), OutputTokens: count(0, "reported")}
		if i == 1 {
			m.InputTokens = TokenCount{Source: "unavailable"}
		}
		state.add(m)
	}
	r, err := state.finish()
	if err != nil {
		t.Fatal(err)
	}
	if r.InputTokens != 4 || r.InputThroughputAvailable() || !r.OutputThroughputAvailable() || r.OutputThroughput != 0 {
		t.Fatalf("mixed=%+v", r)
	}
}

func TestSavedMissingUsageRates(t *testing.T) {
	path := "testdata/q2-missing-usage-run.json"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	original, err := ReadRun(bytes.NewReader(data))
	if err != nil || original.InputThroughputAvailable() || original.OutputThroughputAvailable() {
		t.Fatalf("historical rate admission: %v", err)
	}
	var canonical bytes.Buffer
	if err := WriteRun(&canonical, original); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(canonical.Bytes(), []byte(`"input_throughput":null`)) || !bytes.Contains(canonical.Bytes(), []byte(`"output_throughput":null`)) {
		t.Fatal("canonical export kept false zero rate")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	delete(fields, "input_throughput")
	delete(fields, "output_throughput")
	data, _ = json.Marshal(fields)
	r, err := ReadRun(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if r.Successes != 4 || r.MissingInputCounts != 4 || r.MissingOutputCounts != 4 || r.InputThroughputAvailable() || r.OutputThroughputAvailable() {
		t.Fatal("native capture accounting changed")
	}
	data, err = json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	if string(fields["input_throughput"]) != "null" || string(fields["output_throughput"]) != "null" {
		t.Fatal("missing rates became zero")
	}
	if _, err := ReadRun(bytes.NewReader(data)); err != nil {
		t.Fatal(err)
	}
}

func ExampleRunSummary_InputThroughputAvailable() {
	r := RunSummary{MissingInputCounts: 1}
	fmt.Println(r.InputThroughputAvailable())
	// Output: false
}
func ExampleRunSummary_OutputThroughputAvailable() {
	r := RunSummary{OutputTokens: 0}
	fmt.Println(r.OutputThroughputAvailable())
	// Output: true
}

func TestMergedMissingUsageRates(t *testing.T) {
	a, b := workerFixture("a", 0), workerFixture("b", 100)
	b.Run.Records[0].InputTokens = TokenCount{Source: "unavailable"}
	b.Run.InputTokens = 0
	b.Run.MissingInputCounts = 1
	merged, err := MergeRuns([]WorkerRun{a, b}, MergeOptions{ExpectedWorkers: []string{"a", "b"}})
	if err != nil {
		t.Fatal(err)
	}
	if merged.InputTokens != 2 || merged.InputThroughputAvailable() || !merged.OutputThroughputAvailable() || merged.OutputThroughput != 4 {
		t.Fatalf("merged rates=%+v", merged)
	}
	data, err := json.Marshal(merged)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ReadRun(bytes.NewReader(data)); err != nil {
		t.Fatal(err)
	}
}

func TestRunCompleteNullRateRefusal(t *testing.T) {
	r := rateRun(intPtr(0), intPtr(0))
	data, _ := json.Marshal(r)
	var fields map[string]json.RawMessage
	json.Unmarshal(data, &fields)
	fields["input_throughput"] = json.RawMessage(`null`)
	data, _ = json.Marshal(fields)
	if _, err := ReadRun(bytes.NewReader(data)); err == nil {
		t.Fatal("complete null rate admitted")
	}
	empty := RunSummary{SchemaVersion: 1, StartedAt: 1000, FinishedAt: 2000, Duration: 1, Records: []Measurement{}}
	data, _ = json.Marshal(empty)
	if _, err := ReadRun(bytes.NewReader(data)); err != nil || !empty.InputThroughputAvailable() || !empty.OutputThroughputAvailable() {
		t.Fatalf("empty zero policy: %v", err)
	}
}

func ExampleSliceSummary_MarshalJSON() {
	data, err := json.Marshal(SliceSummary{MissingInputCounts: 1})
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(bytes.Contains(data, []byte(`"input_throughput":null`)), bytes.Contains(data, []byte(`"output_throughput":0`)))
	// Output: true true
}
