package llmperf

import (
	"fmt"
	"reflect"
	"testing"
)

func confidenceFixture() RunSummary {
	run := RunSummary{SchemaVersion: 1, StartedAt: 1000, FinishedAt: 5000, Duration: 4, Attempts: 2}
	for range 2 {
		run.Records = append(run.Records, Measurement{SchemaVersion: 1, Phase: "measurement", Recorded: true, Attempted: true, DispatchedAt: 2000, FinishedAt: 2100, FullResponseLatency: 100, Outcome: "success"})
	}
	return run
}
func ExampleLatencyConfidence() {
	c, err := LatencyConfidence(confidenceFixture(), ConfidenceOptions{Seed: 42, Replicates: 100, Level: .95, Independent: true})
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(c.Mean, c.Lower, c.Upper, c.Unit)
	// Output: 100 100 100 ms
}
func TestConfidenceSeedAndRefusals(t *testing.T) {
	run := confidenceFixture()
	run.Records[1].FullResponseLatency = 300
	opts := ConfidenceOptions{Seed: 42, Replicates: 100, Level: .95, Independent: true}
	a, err := LatencyConfidence(run, opts)
	if err != nil {
		t.Fatal(err)
	}
	b, err := LatencyConfidence(run, opts)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) || a.Mean != 200 || a.Lower != 100 || a.Upper != 300 {
		t.Fatalf("confidence=%+v %+v", a, b)
	}
	for _, bad := range []ConfidenceOptions{{Seed: 42, Replicates: 100, Level: .95}, {Replicates: 0, Level: .95, Independent: true}, {Replicates: 100, Level: 1, Independent: true}} {
		if _, err := LatencyConfidence(run, bad); err == nil {
			t.Fatal("accepted invalid estimator")
		}
	}
	run.DroppedRecords = 1
	if _, err := LatencyConfidence(run, opts); err == nil {
		t.Fatal("accepted partial records")
	}
}
