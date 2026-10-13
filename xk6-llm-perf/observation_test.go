package llmperf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"testing"
)

func observationRun() RunSummary {
	r := formulaRun()
	r.Records[0].Good = true
	r.Records[0].SLO = &SLOReceipt{}
	r.GoodRequests = 1
	r.Goodput = 1
	return r
}

func TestObservationContentWindow(t *testing.T) {
	r := observationRun()
	for _, dispatch := range []float64{1000, 1500} {
		r.Records = append(r.Records, Measurement{SchemaVersion: 1, Recorded: true, Attempted: true, Phase: "measurement", Outcome: "failure", DispatchedAt: dispatch, StartedAt: dispatch, FinishedAt: dispatch + 20, FullResponseLatency: 20, InputTokens: TokenCount{Source: "unavailable"}, OutputTokens: TokenCount{Source: "unavailable"}})
		r.Attempts++
		r.Failures++
	}
	got, err := AnalyzeObservation(r)
	if err != nil {
		t.Fatal(err)
	}
	if got.Policy != contentSpanPolicy || got.StartedAt != 1100 || got.FinishedAt != 1180 || got.Duration != .08 || got.Goodput != 12.5 || got.Attempts != 3 || got.GoodRequests != 1 || got.InputThroughput == nil || *got.InputThroughput != 50 || got.OutputThroughput == nil || *got.OutputThroughput != 50 {
		t.Fatalf("content observation=%+v", got)
	}
	// Declared-window idle and HTTP usage trailers are different windows.
	r.StartedAt = 0
	r.FinishedAt = 3000
	r.Duration = 3
	r.Goodput = 1.0 / 3
	r.InputThroughput = 4.0 / 3
	r.OutputThroughput = 4.0 / 3
	got2, err := AnalyzeObservation(r)
	if err != nil || !got2.equal(got) {
		t.Fatal("idle changed observation", got2, err)
	}
	r.Observation = &got
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := ReadRun(bytes.NewReader(data))
	if err != nil || replay.Observation == nil || !replay.Observation.equal(got) {
		t.Fatal("observation replay", err)
	}
}

func TestObservationRejectsIncompleteOrUnavailable(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*RunSummary)
	}{
		{"all failed", func(r *RunSummary) {
			m := &r.Records[0]
			m.Outcome = "failure"
			m.Good = false
			r.Successes = 0
			r.Failures = 1
			r.GoodRequests = 0
			r.Goodput = 0
			r.InputTokens = 0
			r.OutputTokens = 0
			r.InputThroughput = 0
			r.OutputThroughput = 0
		}},
		{"empty content", func(r *RunSummary) {
			r.Records[0].LastContentLatency = nil
			r.Records[0].ITL = nil
			r.Records[0].Good = false
			r.GoodRequests = 0
			r.Goodput = 0
		}},
		{"good missing content", func(r *RunSummary) { r.Records[0].LastContentLatency = nil; r.Records[0].ITL = nil }},
		{"nonpositive span", func(r *RunSummary) {
			m := &r.Records[0]
			m.LastContentLatency = floatPtr(0)
			m.TTFT = nil
			m.ITL = nil
			m.FullResponseITL = nil
			m.DecodeTokens = nil
		}},
		{"dropped record", func(r *RunSummary) { r.DroppedRecords = 1; r.Records = nil }},
		{"boundary exclusions", func(r *RunSummary) { r.ExcludedBoundaryRequests = 1 }},
		{"inflight", func(r *RunSummary) { r.InflightAtEnd = 1 }},
		{"missing timing", func(r *RunSummary) { r.Records[0].LastContentLatency = floatPtr(math.NaN()) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := observationRun()
			tc.mutate(&r)
			if _, err := AnalyzeObservation(r); err == nil {
				t.Fatal("unavailable content span admitted")
			}
		})
	}
}

func TestObservationReceiptReplayRejectsMutation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*ObservationReceipt)
	}{
		{"policy", func(o *ObservationReceipt) { o.Policy = "declared-window" }},
		{"start", func(o *ObservationReceipt) { o.StartedAt = 1000 }},
		{"end", func(o *ObservationReceipt) { o.FinishedAt = 1200 }},
		{"duration", func(o *ObservationReceipt) { o.Duration = 1 }},
		{"goodput", func(o *ObservationReceipt) { o.Goodput = 1 }},
		{"population", func(o *ObservationReceipt) { o.Attempts++ }},
		{"good count", func(o *ObservationReceipt) { o.GoodRequests++ }},
		{"input rate", func(o *ObservationReceipt) { *o.InputThroughput = 4 }},
		{"output rate", func(o *ObservationReceipt) { *o.OutputThroughput = 4 }},
		{"input rate null", func(o *ObservationReceipt) { o.InputThroughput = nil }},
		{"output rate null", func(o *ObservationReceipt) { o.OutputThroughput = nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := observationRun()
			o, err := AnalyzeObservation(r)
			if err != nil {
				t.Fatal(err)
			}
			tc.mutate(&o)
			r.Observation = &o
			data, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ReadRun(bytes.NewReader(data)); err == nil {
				t.Fatal("forged observation accepted")
			}
		})
	}
}

func TestObservationTokenRates(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*RunSummary)
		in, out *float64
	}{
		{"complete", func(r *RunSummary) {}, floatPtr(50), floatPtr(50)},
		{"reported zero", func(r *RunSummary) {
			r.Records[0].InputTokens = count(0, "reported")
			r.InputTokens = 0
			r.InputThroughput = 0
		}, floatPtr(0), floatPtr(50)},
		{"input missing", func(r *RunSummary) {
			r.Records[0].InputTokens = TokenCount{Source: "unavailable"}
			r.InputTokens = 0
			r.MissingInputCounts = 1
			r.InputThroughput = 0
		}, nil, floatPtr(50)},
		{"output missing", func(r *RunSummary) {
			m := &r.Records[0]
			m.OutputTokens = TokenCount{Source: "unavailable"}
			m.DecodeTokens, m.ITL, m.FullResponseITL, m.FirstChunkSource = nil, nil, nil, ""
			r.OutputTokens = 0
			r.MissingOutputCounts = 1
			r.OutputThroughput = 0
		}, floatPtr(50), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := observationRun()
			tc.mutate(&r)
			got, err := AnalyzeObservation(r)
			if err != nil {
				t.Fatal(err)
			}
			if !got.equal(ObservationReceipt{Policy: got.Policy, StartedAt: 1100, FinishedAt: 1180, Duration: got.Duration, Attempts: 1, GoodRequests: 1, Goodput: got.Goodput, InputThroughput: tc.in, OutputThroughput: tc.out}) {
				t.Fatalf("rates in=%v out=%v", got.InputThroughput, got.OutputThroughput)
			}
			r.Observation = &got
			var buf bytes.Buffer
			if err := WriteRun(&buf, r); err != nil {
				t.Fatal(err)
			}
			replay, err := ReadRun(&buf)
			if err != nil || !replay.Observation.equal(got) {
				t.Fatal("replay", err)
			}
		})
	}
}

func TestObservationLegacyReceipt(t *testing.T) {
	r := observationRun()
	o, err := AnalyzeObservation(r)
	if err != nil {
		t.Fatal(err)
	}
	r.Observation = &o
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	observed := fields["observation"].(map[string]any)
	delete(observed, "input_throughput")
	delete(observed, "output_throughput")
	legacy, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := ReadRun(bytes.NewReader(legacy))
	if err != nil {
		t.Fatal(err)
	}
	if replay.Observation.InputThroughput != nil || replay.Observation.OutputThroughput != nil {
		t.Fatal("legacy receipt admitted token rates")
	}
	var buf bytes.Buffer
	if err := WriteRun(&buf, replay); err != nil {
		t.Fatal(err)
	}
	canonical, err := ReadRun(&buf)
	if err != nil || !canonical.Observation.equal(o) || canonical.Observation.legacy {
		t.Fatal("canonical rewrite", err)
	}
	delete(observed, "goodput")
	observed["goodput"] = 1.0
	if _, err := ReadRun(bytes.NewReader(mustJSON(t, fields))); err == nil {
		t.Fatal("legacy receipt skipped goodput check")
	}
	observed["goodput"] = o.Goodput
	observed["input_throughput"] = 50.0
	if _, err := ReadRun(bytes.NewReader(mustJSON(t, fields))); err == nil {
		t.Fatal("one token rate accepted")
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func ExampleAnalyzeObservation() {
	zero := 0
	last := 80.0
	run := RunSummary{SchemaVersion: 1, StartedAt: 1000, FinishedAt: 2000, Duration: 1, Attempts: 1, Successes: 1, GoodRequests: 1, Goodput: 1}
	run.Records = []Measurement{{SchemaVersion: 1, Recorded: true, Attempted: true, Phase: "measurement", Outcome: "success", Good: true, DispatchedAt: 1100, FinishedAt: 1200, FullResponseLatency: 100, LastContentLatency: &last, InputTokens: TokenCount{Value: &zero, Source: "reported"}, OutputTokens: TokenCount{Value: &zero, Source: "reported"}}}
	observed, err := AnalyzeObservation(run)
	fmt.Println(observed.Policy, observed.Goodput, err)
	// Output: single-process-successful-content-span-v1 12.5 <nil>
}

func TestObservationZeroLatencyRecords(t *testing.T) {
	r := observationRun()
	r.Records[0].LastContentLatency = floatPtr(0)
	r.Records[0].TTFT = nil
	r.Records[0].ITL = nil
	r.Records[0].FullResponseITL = nil
	r.Records[0].DecodeTokens = nil
	if _, err := AnalyzeObservation(r); err == nil {
		t.Fatal("sole zero-duration content span admitted")
	}
	other := r.Records[0]
	other.StartedAt = 1290
	other.DispatchedAt = 1300
	other.FinishedAt = 1400
	other.ScheduledAt = nil
	other.ArrivalLag = nil
	other.ScheduledResponseLatency = nil
	r.Records = append(r.Records, other)
	r.Attempts = 2
	r.Successes = 2
	r.GoodRequests = 2
	r.Goodput = 2
	r.InputTokens = 8
	r.OutputTokens = 8
	r.InputThroughput = 8
	r.OutputThroughput = 8
	observed, err := AnalyzeObservation(r)
	if err != nil || observed.Duration != .2 || observed.Goodput != 10 {
		t.Fatal("zero-latency records rejected positive globalspan", observed, err)
	}
}

func TestObservationRejectsMergedClockDomains(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*RunSummary)
	}{
		{"unmapped workers", func(r *RunSummary) { r.Workers = []string{"a", "b"} }},
		{"mapped workers", func(r *RunSummary) {
			r.WorkerClocks = map[string]WorkerClock{"a": {Basis: "shared clock"}, "b": {Basis: "shared clock"}}
		}},
		{"missing workers", func(r *RunSummary) { r.MissingWorkers = []string{"b"} }},
		{"aggregate identity", func(r *RunSummary) { r.WorkerID = "aggregate" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := observationRun()
			tc.mutate(&r)
			if _, err := AnalyzeObservation(r); err == nil {
				t.Fatal("cross-process domain qualified")
			}
		})
	}
}

func ExampleObservationReceipt() {
	receipt := ObservationReceipt{Policy: "single-process-successful-content-span-v1", Duration: .5, Attempts: 2, GoodRequests: 1, Goodput: 2}
	fmt.Printf("%s: %.1f requests/s\n", receipt.Policy, receipt.Goodput)
	// Output: single-process-successful-content-span-v1: 2.0 requests/s
}
