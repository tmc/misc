package llmperf

import (
	"bytes"
	"fmt"
	"math"
	"reflect"
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

func analysisFixture() RunSummary {
	run := RunSummary{SchemaVersion: 1, StartedAt: 0, FinishedAt: 4000, Duration: 4, Attempts: 3, Successes: 2, Failures: 1, InputTokens: 4, OutputTokens: 8}
	for i, latency := range []float64{100, 300, 900} {
		outcome := "success"
		if i == 2 {
			outcome = "timeout"
		}
		run.Records = append(run.Records, Measurement{RequestID: fmt.Sprint(i), SchemaVersion: 1, Phase: "measurement", Attempted: true, Recorded: true, DispatchedAt: 1000, FinishedAt: 1000 + latency, FullResponseLatency: latency, Outcome: outcome, InputTokens: count(2, "reported"), OutputTokens: count(4, "reported"), ITL: floatPtr(0)})
	}
	return run
}

func ExampleAnalyzeRequests() {
	a, err := AnalyzeRequests(analysisFixture())
	if err != nil {
		fmt.Println(err)
		return
	}
	d := a.Metrics["full_response_latency"]
	fmt.Println(d.Samples, *d.Mean, *d.P50, *d.P95)
	// Output: 2 200 100 300
}

func ExampleWriteRun() {
	var buf bytes.Buffer
	err := WriteRun(&buf, RunSummary{SchemaVersion: 1, StartedAt: 0, FinishedAt: 1000, Duration: 1})
	if err != nil {
		fmt.Println(err)
		return
	}
	got, err := ReadRun(&buf)
	fmt.Println(got.Duration, err)
	// Output: 1 <nil>
}

func ExampleWriteRecords() {
	var buf bytes.Buffer
	err := WriteRecords(&buf, []Measurement{{SchemaVersion: 1}})
	got, err2 := ReadRecords(&buf)
	fmt.Println(len(got), err, err2)
	// Output: 1 <nil> <nil>
}

func ExampleReadRecords() {
	rows, err := ReadRecords(strings.NewReader("{\"schema_version\":1}\n"))
	fmt.Println(len(rows), err)
	// Output: 1 <nil>
}

func ExampleReconcileRecords() {
	run := analysisFixture()
	fmt.Println(ReconcileRecords(run, run.Records))
	// Output: <nil>
}

func TestRequestPopulationAndMissing(t *testing.T) {
	got, err := AnalyzeRequests(analysisFixture())
	if err != nil {
		t.Fatal(err)
	}
	latency := got.Metrics["full_response_latency"]
	if got.Outcomes["timeout"] != 1 || latency.Samples != 2 || *latency.Mean != 200 || math.Abs(*latency.StdDev-100) > 1e-12 {
		t.Fatalf("analysis=%+v", got)
	}
	speed := got.Metrics["decode_token_speed"]
	if speed.Samples != 0 || speed.Missing != 2 || speed.Mean != nil || speed.P99 != nil {
		t.Fatalf("zero ITL=%+v", speed)
	}
	if got.Metrics["ttft"].Missing != 2 {
		t.Fatal("missing timing counted")
	}
}

func TestReplayAndRefusals(t *testing.T) {
	run := analysisFixture()
	var buf bytes.Buffer
	if err := WriteRun(&buf, run); err != nil {
		t.Fatal(err)
	}
	got, err := ReadRun(&buf)
	if err != nil {
		t.Fatal(err)
	}
	a, err := AnalyzeRequests(run)
	if err != nil {
		t.Fatal(err)
	}
	b, err := AnalyzeRequests(got)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatal("round trip changed distributions")
	}
	if err := WriteRecords(&buf, run.Records); err != nil {
		t.Fatal(err)
	}
	rows, err := ReadRecords(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if err := ReconcileRecords(got, rows); err != nil {
		t.Fatal(err)
	}
	rows[0].FullResponseLatency++
	if err := ReconcileRecords(got, rows); err == nil {
		t.Fatal("contradictory JSONL accepted")
	}
	for _, input := range []string{"{\"schema_version\":2}\n", "{\"schema_version\":1,\"secret\":1}\n", "{\"schema_version\":1} {}\n"} {
		if _, err := ReadRecords(strings.NewReader(input)); err == nil {
			t.Fatalf("accepted %q", input)
		}
	}
	if err := WriteRun(shortReportWriter{}, run); err == nil {
		t.Fatal("short run write accepted")
	}
	if err := WriteRecords(shortReportWriter{}, run.Records); err == nil {
		t.Fatal("short records write accepted")
	}
}

type shortReportWriter struct{}

func (shortReportWriter) Write(p []byte) (int, error) { return len(p) / 2, nil }

func TestAccountingNegativeControls(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*RunSummary)
	}{
		{"duplicate", func(r *RunSummary) { r.Records[1].RequestID = r.Records[0].RequestID }},
		{"source", func(r *RunSummary) { r.Records[0].OutputTokens.Source = "server_exact" }},
		{"outcome", func(r *RunSummary) { r.Records[0].Outcome = "pending" }},
		{"good failure", func(r *RunSummary) { r.Records[2].Good = true }},
		{"aggregate", func(r *RunSummary) { r.OutputTokens++ }},
		{"overflow", func(r *RunSummary) {
			r.Records[0].OutputTokens = count(math.MaxInt, "reported")
			r.Records[1].OutputTokens = count(math.MaxInt, "reported")
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			run := analysisFixture()
			test.mutate(&run)
			if err := validateRun(run); err == nil {
				t.Fatal("invalid export accepted")
			}
		})
	}
}

func TestReadRunRateAndTimelineReconciliation(t *testing.T) {
	for _, input := range []string{
		`{"schema_version":1,"started_at":0,"finished_at":1000,"duration":1,"records":[],"goodput":1}`,
		`{"schema_version":1,"started_at":0,"finished_at":1000,"duration":1,"records":[],"timeline":{"requests":{"effective":{"mean":9}}}}`,
		`{"schema_version":1,"started_at":0,"finished_at":1000,"duration":1,"records":[],"inflight_at_end":-1}`,
	} {
		if _, err := ReadRun(strings.NewReader(input)); err == nil {
			t.Fatalf("accepted %s", input)
		}
	}
}
