package llmperf

import (
	"fmt"
	"math"
	"testing"
)

func ExampleAnalyzeTimeline() {
	run := RunSummary{SchemaVersion: 1, StartedAt: 1000, FinishedAt: 5000, Duration: 4, Attempts: 1, Records: []Measurement{{SchemaVersion: 1, Phase: "measurement", Attempted: true, Recorded: true, DispatchedAt: 2000, FinishedAt: 4000}}}
	timeline, err := AnalyzeTimeline(run)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(timeline.Requests.Effective.Mean, timeline.Requests.Active.Mean)
	// Output: 0.5 1
}

func TestTimeWeightedConcurrency(t *testing.T) {
	c := concurrency([]interval{{0, 1000}, {500, 1500}}, 0, 3000)
	if math.Abs(c.Effective.Mean-2.0/3) > 1e-12 || math.Abs(c.Active.Mean-4.0/3) > 1e-12 || c.Active.Duration != 1.5 || c.Effective.P95 != 2 {
		t.Fatalf("concurrency=%+v", c)
	}
	// Clipped windows must not count activity outside the selected interval.
	c = concurrency([]interval{{0, 1000}, {500, 1500}}, 1000, 2000)
	if c.Effective.Mean != .5 || c.Active.Mean != 1 {
		t.Fatalf("clipped concurrency=%+v", c)
	}
}

func TestTimelineRefusesPartialRecords(t *testing.T) {
	run := RunSummary{SchemaVersion: 1, StartedAt: 1, FinishedAt: 2, Duration: .001, Attempts: 1, DroppedRecords: 1}
	if _, err := AnalyzeTimeline(run); err == nil {
		t.Fatal("partial record set accepted")
	}
}

func TestTimelinePhases(t *testing.T) {
	run := RunSummary{SchemaVersion: 1, StartedAt: 1000, FinishedAt: 5000, Duration: 4, Attempts: 1, Records: []Measurement{{SchemaVersion: 1, Phase: "measurement", Attempted: true, Recorded: true, DispatchedAt: 2000, FinishedAt: 4000, TTFT: floatPtr(500), LastContentLatency: floatPtr(1500), FullResponseLatency: 2000}}}
	timeline, err := AnalyzeTimeline(run)
	if err != nil {
		t.Fatal(err)
	}
	if timeline.Prefill.Effective.Mean != .125 || timeline.Decode.Effective.Mean != .25 || timeline.Requests.Effective.Mean != .5 {
		t.Fatalf("timeline=%+v", timeline)
	}
}
