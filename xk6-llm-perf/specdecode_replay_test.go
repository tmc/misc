package llmperf

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestSpecDecodeReplayValidation(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*Measurement)
		invalid bool
	}{
		{"valid", func(m *Measurement) {}, false},
		{"accepted count", func(m *Measurement) { m.SpecDecode.Accepted = 3 }, true},
		{"choice source", func(m *Measurement) { m.SpecDecode.Source = "choice" }, true},
		{"availability", func(m *Measurement) { m.SpecDecodeUnavailable = "invalid vendor snapshot" }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run := analysisFixture()
			run.Goodput = float64(run.GoodRequests) / run.Duration
			run.InputThroughput = float64(run.InputTokens) / run.Duration
			run.OutputThroughput = float64(run.OutputTokens) / run.Duration
			s, err := parseSpecDecode(json.RawMessage(`{"num_accepted_draft_tokens":1,"num_draft_tokens":2,"num_spec_steps":1,"acceptance_histogram":[0,1]}`))
			if err != nil {
				t.Fatal(err)
			}
			run.Records[0].SpecDecode = s
			tc.mutate(&run.Records[0])
			data, err := json.Marshal(run)
			if err != nil {
				t.Fatal(err)
			}
			_, err = ReadRun(bytes.NewReader(data))
			if (err != nil) != tc.invalid {
				t.Fatalf("ReadRun error = %v, invalid = %v", err, tc.invalid)
			}
			_, err = AnalyzeRequests(run)
			if (err != nil) != tc.invalid {
				t.Fatalf("AnalyzeRequests error = %v, invalid = %v", err, tc.invalid)
			}
		})
	}
}
