package llmperf

import (
	"fmt"
	"strings"
	"testing"
)

func ExampleReadRun() {
	run, err := ReadRun(strings.NewReader(`{"schema_version":1,"started_at":1000,"finished_at":2000,"duration":1,"attempts":0,"successes":0,"failures":0,"good_requests":0,"records":[]}`))
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(run.Duration)
	// Output: 1
}

func ExampleTimeSlices() {
	run := RunSummary{SchemaVersion: 1, StartedAt: 1000, FinishedAt: 3000, Duration: 2}
	slices, err := TimeSlices(run, 1)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(len(slices))
	// Output: 2
}

func TestTimeSlicesBoundary(t *testing.T) {
	run := RunSummary{SchemaVersion: 1, StartedAt: 1000, FinishedAt: 3000, Duration: 2, Attempts: 1, Records: []Measurement{{SchemaVersion: 1, Phase: "measurement", Recorded: true, Attempted: true, DispatchedAt: 1500, FinishedAt: 2000, Outcome: "success", Good: true, OutputTokens: count(4, "reported")}}}
	s, err := TimeSlices(run, 1)
	if err != nil {
		t.Fatal(err)
	}
	if s[0].Attempts != 0 || s[1].Attempts != 1 || s[1].OutputThroughput != 4 || s[0].Requests.Effective.Mean != .5 {
		t.Fatalf("slices=%+v", s)
	}
}

func TestReadRunRejectsInconsistentAccounting(t *testing.T) {
	for _, input := range []string{`{`, `{"schema_version":2,"duration":1}`, `{"schema_version":1,"started_at":1,"finished_at":1001,"duration":1,"attempts":1,"successes":0,"failures":0,"records":[]}`} {
		if _, err := ReadRun(strings.NewReader(input)); err == nil {
			t.Fatalf("accepted %s", input)
		}
	}
}
