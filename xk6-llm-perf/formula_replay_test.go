package llmperf

import (
	"bytes"
	"encoding/json"
	"testing"
)

func formulaRun() RunSummary {
	return RunSummary{SchemaVersion: 1, StartedAt: 1000, FinishedAt: 2000, Duration: 1, Attempts: 1, Successes: 1, InputTokens: 4, OutputTokens: 4, InputThroughput: 4, OutputThroughput: 4, Records: []Measurement{{SchemaVersion: 1, Recorded: true, Attempted: true, Phase: "measurement", Outcome: "success", StartedAt: 1090, DispatchedAt: 1100, FinishedAt: 1200, FullResponseLatency: 100, TTFT: floatPtr(20), LastContentLatency: floatPtr(80), DecodeTokens: intPtr(3), FirstChunkSource: "assumed_one", ITL: floatPtr(20), FullResponseITL: floatPtr(80.0 / 3), ScheduledAt: floatPtr(1090), ArrivalLag: floatPtr(10), ScheduledResponseLatency: floatPtr(110), InputTokens: count(4, "reported"), OutputTokens: count(4, "reported")}}}
}
func readFormulaRun(r RunSummary) error {
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	_, err = ReadRun(bytes.NewReader(data))
	return err
}
func TestReplayFormulaRefusals(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*Measurement)
	}{
		{"ITL only", func(m *Measurement) { m.ITL = floatPtr(1) }},
		{"full ITL only", func(m *Measurement) { m.FullResponseITL = floatPtr(1) }},
		{"missing divisor", func(m *Measurement) { m.DecodeTokens = nil }},
		{"zero divisor", func(m *Measurement) { m.DecodeTokens = intPtr(0) }},
		{"missing first anchor", func(m *Measurement) { m.TTFT = nil }},
		{"missing last anchor", func(m *Measurement) { m.LastContentLatency = nil }},
		{"multiple choices", func(m *Measurement) { m.MultiChoice = true }},
		{"unknown first provenance", func(m *Measurement) { m.FirstChunkSource = "unavailable" }},
		{"exact first count mismatch", func(m *Measurement) { m.FirstChunkSource = "reported"; m.FirstChunkTokens = intPtr(2) }},
		{"exact first count absent", func(m *Measurement) { m.FirstChunkSource = "reported" }},
		{"false fallback", func(m *Measurement) {
			m.FirstChunkTokens = intPtr(2)
			m.FirstChunkOrigin = "reported"
			m.FirstChunkUnavailable = "first chunk count outside output count"
		}},
		{"arrival lag only", func(m *Measurement) { m.ArrivalLag = floatPtr(90) }},
		{"arrival anchor absent", func(m *Measurement) { m.ScheduledAt = nil; m.ScheduledResponseLatency = nil }},
		{"scheduled response anchor absent", func(m *Measurement) { m.ScheduledAt = nil; m.ArrivalLag = nil }},
		{"admission exceeds preparation", func(m *Measurement) {
			m.TimestampBasis = "dispatch Unix clock anchor plus monotonic elapsed time"
			m.AdmissionWait = 11
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := formulaRun()
			tc.mutate(&r.Records[0])
			if err := readFormulaRun(r); err == nil {
				t.Fatal("contradictory replay accepted")
			}
			if _, err := AnalyzeRequests(r); err == nil {
				t.Fatal("direct analysis accepted contradiction")
			}
		})
	}
}
func TestReplayFormulaValidControls(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*RunSummary)
	}{
		{"base", func(r *RunSummary) {}},
		{"full-response metric only", func(r *RunSummary) { r.Records[0].ITL = nil; r.Records[0].LastContentLatency = nil }},
		{"zero content span", func(r *RunSummary) {
			m := &r.Records[0]
			m.TTFT = floatPtr(80)
			m.LastContentLatency = floatPtr(80)
			m.ITL = floatPtr(0)
			m.FullResponseITL = floatPtr(20.0 / 3)
		}},
		{"zero full span", func(r *RunSummary) {
			m := &r.Records[0]
			m.TTFT = floatPtr(100)
			m.LastContentLatency = floatPtr(100)
			m.ITL = floatPtr(0)
			m.FullResponseITL = floatPtr(0)
		}},
		{"exact first chunk", func(r *RunSummary) {
			m := &r.Records[0]
			m.FirstChunkSource = "client_declared"
			m.FirstChunkTokens = intPtr(2)
			m.FirstChunkOrigin = "client_declared"
			m.DecodeTokens = intPtr(2)
			m.ITL = floatPtr(30)
			m.FullResponseITL = floatPtr(40)
		}},
		{"invalid first chunk fallback", func(r *RunSummary) {
			m := &r.Records[0]
			m.FirstChunkTokens = intPtr(4)
			m.FirstChunkOrigin = "reported"
			m.FirstChunkUnavailable = "first chunk count outside output count"
		}},
		{"unavailable normalized timing", func(r *RunSummary) {
			m := &r.Records[0]
			m.ITL = nil
			m.FullResponseITL = nil
			m.DecodeTokens = nil
			m.FirstChunkSource = "unavailable"
		}},
		{"strict missing output", func(r *RunSummary) {
			m := &r.Records[0]
			m.ITL = nil
			m.FullResponseITL = nil
			m.DecodeTokens = nil
			m.FirstChunkSource = "unavailable"
			m.OutputTokens = TokenCount{Source: "unavailable"}
			r.OutputTokens = 0
			r.OutputThroughput = 0
			r.MissingOutputCounts = 1
		}},
		{"multiple choice unavailable", func(r *RunSummary) {
			m := &r.Records[0]
			m.MultiChoice = true
			m.ITL = nil
			m.FullResponseITL = nil
			m.DecodeTokens = nil
			m.FirstChunkSource = "unavailable"
		}},
		{"queue is preparation subset", func(r *RunSummary) {
			m := &r.Records[0]
			m.TimestampBasis = "dispatch Unix clock anchor plus monotonic elapsed time"
			m.AdmissionWait = 5
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := formulaRun()
			tc.mutate(&r)
			if err := readFormulaRun(r); err != nil {
				t.Fatal(err)
			}
			if _, err := AnalyzeRequests(r); err != nil {
				t.Fatal(err)
			}
		})
	}
}
func partialFormulaRun() RunSummary {
	r := formulaRun()
	r.Attempts = 2
	r.Successes = 2
	r.DroppedRecords = 1
	r.MissingInputCounts = 2
	r.MissingOutputCounts = 2
	r.InputTokens = 0
	r.OutputTokens = 0
	r.InputThroughput = 0
	r.OutputThroughput = 0
	m := &r.Records[0]
	m.InputTokens = TokenCount{Source: "unavailable"}
	m.OutputTokens = TokenCount{Source: "unavailable"}
	m.ITL = nil
	m.FullResponseITL = nil
	m.DecodeTokens = nil
	m.FirstChunkSource = "unavailable"
	return r
}
func TestPartialTokenPopulationRefusals(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*RunSummary)
	}{
		{"invent input", func(r *RunSummary) { r.InputTokens = 99; r.InputThroughput = 99 }},
		{"invent output", func(r *RunSummary) { r.OutputTokens = 99; r.OutputThroughput = 99 }},
		{"known retained input plus missing drop", func(r *RunSummary) {
			r.Records[0].InputTokens = count(4, "reported")
			r.MissingInputCounts = 1
			r.InputTokens = 99
			r.InputThroughput = 99
		}},
		{"overlapping missing and estimated drops", func(r *RunSummary) {
			r.Records[0].InputTokens = count(4, "reported")
			r.MissingInputCounts = 1
			r.EstimatedInputCounts = 1
			r.InputTokens = 4
			r.InputThroughput = 4
		}},
		{"dropped failure classified good", func(r *RunSummary) {
			r.Successes = 1
			r.Failures = 1
			r.MissingInputCounts = 1
			r.MissingOutputCounts = 1
			r.GoodRequests = 1
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := partialFormulaRun()
			tc.mutate(&r)
			if err := readFormulaRun(r); err == nil {
				t.Fatal("impossible partial population accepted")
			}
		})
	}
}
func TestPartialTokenPopulationValidControls(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*RunSummary)
	}{
		{"all missing", func(r *RunSummary) {}},
		{"known zero retained count", func(r *RunSummary) { r.Records[0].InputTokens = count(0, "reported"); r.MissingInputCounts = 1 }},
		{"known retained count only", func(r *RunSummary) {
			r.Records[0].InputTokens = count(4, "reported")
			r.MissingInputCounts = 1
			r.InputTokens = 4
			r.InputThroughput = 4
		}},
		{"dropped available count", func(r *RunSummary) { r.MissingInputCounts = 1; r.InputTokens = 99; r.InputThroughput = 99 }},
		{"dropped available zero", func(r *RunSummary) { r.MissingInputCounts = 1 }},
		{"dropped estimated count", func(r *RunSummary) {
			r.MissingInputCounts = 1
			r.EstimatedInputCounts = 1
			r.InputTokens = 99
			r.InputThroughput = 99
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := partialFormulaRun()
			tc.mutate(&r)
			if err := readFormulaRun(r); err != nil {
				t.Fatal(err)
			}
			if _, err := AnalyzeRequests(r); err == nil {
				t.Fatal("incomplete request analysis admitted")
			}
		})
	}
}

func TestMissingOutputCannotSupportNormalizedTiming(t *testing.T) {
	r := formulaRun()
	r.OutputTokens = 0
	r.OutputThroughput = 0
	r.MissingOutputCounts = 1
	r.Records[0].OutputTokens = TokenCount{Source: "unavailable"}
	if err := readFormulaRun(r); err == nil {
		t.Fatal("missing output count supported normalized timing")
	}
}
