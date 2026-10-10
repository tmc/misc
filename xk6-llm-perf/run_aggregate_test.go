package llmperf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"
)

func TestRunTokenOverflowRetainsRecordsAndRefusesReplay(t *testing.T) {
	if uint64(math.MaxInt) < uint64(math.MaxInt64) {
		t.Skip("requires 64-bit int token counts")
	}
	var state runState
	if err := state.begin(RunOptions{RunID: "overflow", MaxRecords: 2}); err != nil {
		t.Fatal(err)
	}
	for i := range 2 {
		now := time.Now()
		state.add(Measurement{SchemaVersion: 1, Phase: "measurement", Attempted: true, Recorded: true, Outcome: "success", RequestID: fmt.Sprint(i), requestStart: now, DispatchedAt: millis(now), FinishedAt: millis(now), InputTokens: count(math.MaxInt, "reported"), OutputTokens: count(math.MaxInt, "reported")})
	}
	run, err := state.finish()
	if err != nil {
		t.Fatal(err)
	}
	if run.AggregateError == "" || run.Attempts != 2 || run.Successes != 2 || len(run.Records) != 2 || run.DroppedRecords != 0 || run.InputTokens != math.MaxInt64 || run.OutputTokens != math.MaxInt64 {
		t.Fatalf("overflow accounting=%+v", run)
	}
	if run.Timeline != nil || !strings.Contains(run.TimelineUnavailable, "incomplete aggregate") {
		t.Fatal("overflow timeline claimed available")
	}
	data, err := json.Marshal(run)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"goodput", "input_throughput", "output_throughput"} {
		if string(fields[name]) != "null" {
			t.Fatalf("%s=%s, want null", name, fields[name])
		}
	}
	if _, err := ReadRun(bytes.NewReader(data)); err == nil || !strings.Contains(err.Error(), "incomplete aggregate") {
		t.Fatalf("read overflow export: %v", err)
	}
	if err := WriteRun(&bytes.Buffer{}, run); err == nil {
		t.Fatal("canonical export accepted incomplete aggregate")
	}
	if _, err := AnalyzeTimeline(run); err == nil {
		t.Fatal("timeline accepted incomplete aggregate")
	}
	if _, err := AnalyzeRequests(run); err == nil {
		t.Fatal("statistics accepted incomplete aggregate")
	}
	if _, err := TimeSlices(run, 1); err == nil {
		t.Fatal("slices accepted incomplete aggregate")
	}
	if _, err := LatencyConfidence(run, ConfidenceOptions{Replicates: 100, Level: .95, Independent: true}); err == nil {
		t.Fatal("confidence accepted incomplete aggregate")
	}
}

func ExampleRunSummary_MarshalJSON() {
	data, err := json.Marshal(RunSummary{AggregateError: "token aggregate exceeds int64 range"})
	if err != nil {
		fmt.Println(err)
		return
	}
	var fields map[string]json.RawMessage
	json.Unmarshal(data, &fields)
	fmt.Println(string(fields["input_throughput"]))
	// Output: null
}
