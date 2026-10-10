package llmperf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"testing"
)

func workerFixture(id string, offset float64) WorkerRun {
	zero := 0.0
	r := RunSummary{SchemaVersion: 1, WorkerID: id, RunID: "shared", StartedAt: 1000 - offset, FinishedAt: 3000 - offset, Duration: 2, Attempts: 1, Successes: 1, GoodRequests: 1, InputTokens: 2, OutputTokens: 4, Workload: "fixture", TraceHash: "sha256:fixture", Records: []Measurement{{RequestID: id + ":1:1", SchemaVersion: 1, Phase: "measurement", Attempted: true, Recorded: true, Outcome: "success", Good: true, StartedAt: 1500 - offset, DispatchedAt: 1500 - offset, FinishedAt: 2000 - offset, InputTokens: count(2, "reported"), OutputTokens: count(4, "reported")}}}
	return WorkerRun{Run: r, ClockOffset: offset, ClockUncertainty: &zero, ClockBasis: "injected shared clock"}
}
func ExampleMergeRuns() {
	r, err := MergeRuns([]WorkerRun{workerFixture("a", 0), workerFixture("b", 100)}, MergeOptions{ExpectedWorkers: []string{"a", "b"}})
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(r.Attempts, r.OutputTokens, r.OutputThroughput, len(r.WorkerClocks))
	// Output: 2 8 4 2
}
func TestDistributedMergeReplayAndPartial(t *testing.T) {
	r, err := MergeRuns([]WorkerRun{workerFixture("a", 0), workerFixture("b", 100)}, MergeOptions{ExpectedWorkers: []string{"a", "b"}})
	if err != nil {
		t.Fatal(err)
	}
	if r.Timeline.Requests.Effective.Mean != .5 {
		t.Fatalf("timeline=%+v", r.Timeline)
	}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ReadRun(bytes.NewReader(data)); err != nil {
		t.Fatal(err)
	}
	r, err = MergeRuns([]WorkerRun{workerFixture("a", 0)}, MergeOptions{ExpectedWorkers: []string{"a", "b"}, AllowPartial: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.MissingWorkers) != 1 || r.MissingWorkers[0] != "b" || r.Attempts != 1 {
		t.Fatalf("partial=%+v", r)
	}
	if _, err := MergeRuns([]WorkerRun{workerFixture("a", 0)}, MergeOptions{ExpectedWorkers: []string{"a", "b"}}); err == nil {
		t.Fatal("silent missing worker")
	}
}
func TestDistributedMergeRefusals(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*WorkerRun)
	}{
		{"duplicate worker", func(w *WorkerRun) { w.Run.WorkerID = "a" }},
		{"unknown clock", func(w *WorkerRun) { w.ClockUncertainty = nil }},
		{"wrong window", func(w *WorkerRun) { w.ClockOffset++ }},
		{"wrong workload", func(w *WorkerRun) { w.Run.TraceHash = "other" }},
		{"missing request identity", func(w *WorkerRun) { w.Run.Records[0].RequestID = "" }},
		{"duplicate request identity", func(w *WorkerRun) { w.Run.Records[0].RequestID = "a:1:1" }},
		{"wrong accounting", func(w *WorkerRun) { w.Run.OutputTokens++ }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := workerFixture("b", 100)
			tc.change(&b)
			if _, err := MergeRuns([]WorkerRun{workerFixture("a", 0), b}, MergeOptions{ExpectedWorkers: []string{"a", "b"}}); err == nil {
				t.Fatal("accepted invalid worker")
			}
		})
	}
}

func TestDistributedExplicitWindow(t *testing.T) {
	epoch := 1791650503449.0
	for _, opts := range []RunOptions{
		{StartedAt: &epoch},
		{StartedAt: &epoch, Duration: "1s", StartDelay: "1s"},
		{StartedAt: floatPointer(0), Duration: "1s"},
		{StartedAt: floatPointer(-1), Duration: "1s"},
		{StartedAt: floatPointer(9e12), Duration: "1s"},
		{StartedAt: floatPointer(math.Inf(1)), Duration: "1s"},
		{StartedAt: floatPointer(math.NaN()), Duration: "1s"},
	} {
		var state runState
		if state.begin(opts) == nil {
			t.Fatalf("accepted invalid epoch: %+v", opts)
		}
	}
	for range 2 {
		var state runState
		if err := state.begin(RunOptions{StartedAt: &epoch, Duration: "1s"}); err != nil {
			t.Fatal(err)
		}
		if state.summary.StartedAt != epoch || millis(state.summary.end) != epoch+1000 {
			t.Fatalf("explicit window drift: %+v", state.summary)
		}
	}
}
func floatPointer(v float64) *float64 { return &v }

func TestDistributedVersionAndPolicyMismatch(t *testing.T) {
	for _, change := range []func(*WorkerRun){
		func(w *WorkerRun) { w.Run.WarmupPolicy = "changed" },
		func(w *WorkerRun) { w.Run.Versions = map[string]string{"only-here": "v1"} },
	} {
		b := workerFixture("b", 0)
		change(&b)
		if _, err := MergeRuns([]WorkerRun{workerFixture("a", 0), b}, MergeOptions{ExpectedWorkers: []string{"a", "b"}}); err == nil {
			t.Fatal("accepted mismatched manifest")
		}
	}
}

func TestDistributedStableOrder(t *testing.T) {
	a, b := workerFixture("a", 0), workerFixture("b", 100)
	opts := MergeOptions{ExpectedWorkers: []string{"a", "b"}}
	x, err := MergeRuns([]WorkerRun{a, b}, opts)
	if err != nil {
		t.Fatal(err)
	}
	y, err := MergeRuns([]WorkerRun{b, a}, opts)
	if err != nil {
		t.Fatal(err)
	}
	left, _ := json.Marshal(x)
	right, _ := json.Marshal(y)
	if !bytes.Equal(left, right) {
		t.Fatal("merge depends on worker arrival order")
	}
}

func TestDistributedTokenOverflow(t *testing.T) {
	a, b := workerFixture("a", 0), workerFixture("b", 0)
	for _, w := range []*WorkerRun{&a, &b} {
		w.Run.Records[0].InputTokens = count(math.MaxInt, "reported")
		w.Run.InputTokens = int64(math.MaxInt)
	}
	if _, err := MergeRuns([]WorkerRun{a, b}, MergeOptions{ExpectedWorkers: []string{"a", "b"}}); err == nil {
		t.Fatal("accepted token total overflow")
	}
}
