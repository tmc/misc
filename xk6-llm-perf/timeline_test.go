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

func TestRateAnalysisWeightedIdleAndFailure(t *testing.T) {
	run := RunSummary{SchemaVersion: 1, StartedAt: 0, FinishedAt: 10000, Duration: 10, Attempts: 3}
	run.Records = []Measurement{
		{SchemaVersion: 1, Phase: "measurement", Attempted: true, Recorded: true, DispatchedAt: 0, FinishedAt: 2000, FullResponseLatency: 2000, Outcome: "success", TTFT: floatPtr(1000), LastContentLatency: floatPtr(2000), InputTokens: count(10, "reported"), DecodeTokens: intPtr(2)},
		{SchemaVersion: 1, Phase: "measurement", Attempted: true, Recorded: true, DispatchedAt: 1000, FinishedAt: 5000, FullResponseLatency: 4000, Outcome: "success", TTFT: floatPtr(2000), LastContentLatency: floatPtr(4000), InputTokens: count(20, "reported"), DecodeTokens: intPtr(8)},
		{SchemaVersion: 1, Phase: "measurement", Attempted: true, Recorded: true, DispatchedAt: 6000, FinishedAt: 7000, FullResponseLatency: 1000, Outcome: "failure", TTFT: floatPtr(500), LastContentLatency: floatPtr(1000), InputTokens: count(900, "reported"), DecodeTokens: intPtr(900)},
	}
	got, err := AnalyzeTimeline(run)
	if err != nil {
		t.Fatal(err)
	}
	// Input area is 30 tokens over ten seconds, active three seconds.
	if got.InputRate.Effective.Mean != 3 || got.InputRate.Active.Mean != 10 || got.InputRate.Active.Duration != 3 || got.InputRate.Effective.P50 != 0 || got.InputRate.Effective.P95 != 10 || got.InputRate.Eligible != 2 || got.InputRate.Missing != 1 {
		t.Fatalf("input=%+v", got.InputRate)
	}
	// Decode tokens are 2+8; rates 2 for one second and 4 for two seconds.
	if got.OutputRate.Effective.Mean != 1 || math.Abs(got.OutputRate.Active.Mean-10.0/3) > 1e-12 || got.OutputRate.Active.P50 != 4 {
		t.Fatalf("output=%+v", got.OutputRate)
	}
}

func TestEqualTimeEdgesAndQuantileTie(t *testing.T) {
	c := concurrency([]interval{{0, 1000}, {1000, 2000}}, 0, 4000)
	if c.Effective.P50 != 0 || c.Effective.P95 != 1 || c.Active.Mean != 1 || c.Effective.Mean != .5 {
		t.Fatalf("equal-time=%+v", c)
	}
}

func TestActiveRateRetainsKnownZero(t *testing.T) {
	got, users := rateDistributions([]rateInterval{{0, 1000, 0}, {1000, 2000, 10}}, 0, 4000)
	if got.Active.Duration != 2 || got.Active.Mean != 5 || got.Active.P50 != 0 || got.Effective.Mean != 2.5 || users.Active.Mean != 5 {
		t.Fatalf("known zero=%+v users=%+v", got, users)
	}
	// Overlap sums rates, but per-user weighting divides each segment by its
	// concurrency, rather than dividing the final means after integration.
	got, users = rateDistributions([]rateInterval{{0, 1000, 2}, {500, 1500, 4}}, 0, 2000)
	if got.Effective.Mean != 3 || math.Abs(users.Active.Mean-3) > 1e-12 || users.Effective.Mean != 2.25 {
		t.Fatalf("per user=%+v total=%+v", users, got)
	}
}

func TestRateCancellationPreservesSmallAndIdle(t *testing.T) {
	// Large rates must not erase a concurrently observed small rate after the
	// large phase ends. Idle intervals must be exactly zero.
	got := rateDistribution([]rateInterval{{0, 1000, 1e20}, {0, 2000, 1}}, 0, 4000)
	if got.Effective.P50 != 0 || got.Effective.P95 != 1e20 || got.Active.P50 != 1 {
		t.Fatalf("numeric cancellation=%+v", got)
	}
}
