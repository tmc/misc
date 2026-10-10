package llmperf

import (
	"encoding/json"
	"testing"
)

func TestSpecDecodeAdapters(t *testing.T) {
	for _, tt := range []struct {
		name, body, engine string
		bad                bool
	}{
		{"vllm", `{"num_accepted_draft_tokens":3,"num_draft_tokens":6,"num_spec_steps":2,"num_spec_tokens":2,"acceptance_histogram":[0,1,1]}`, "vllm", false},
		{"trt", `{"total_accepted_draft_tokens":3,"total_draft_tokens":6,"num_spec_steps":2,"acceptance_histogram":[0,1,1]}`, "tensorrt_llm", false},
		{"bad histogram", `{"num_accepted_draft_tokens":3,"num_draft_tokens":6,"num_spec_steps":2,"acceptance_histogram":[0,0,1]}`, "", true},
		{"negative", `{"num_accepted_draft_tokens":-1,"num_draft_tokens":6,"num_spec_steps":2,"acceptance_histogram":[0,1,1]}`, "", true},
		{"zero", `{"num_accepted_draft_tokens":0,"num_draft_tokens":0,"num_spec_steps":0,"acceptance_histogram":[0]}`, "vllm", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s, err := parseSpecDecode(json.RawMessage(tt.body))
			if (err != nil) != tt.bad {
				t.Fatalf("parse=%+v,%v", s, err)
			}
			if tt.bad {
				return
			}
			if s.Engine != tt.engine {
				t.Fatalf("engine=%s", s.Engine)
			}
			if tt.name == "zero" {
				if s.AcceptanceRate != nil || s.MeanAcceptanceLength != nil {
					t.Fatal("zero divisor metric invented")
				}
				return
			}
			if *s.AcceptanceRate != .5 || *s.MeanAcceptanceLength != 2.5 {
				t.Fatalf("stats=%+v", s)
			}
		})
	}
}

func TestSpecDecodeVendorContradictions(t *testing.T) {
	for _, body := range []string{
		`{"total_accepted_draft_tokens":0,"total_draft_tokens":0,"num_spec_steps":0,"acceptance_histogram":[0]}`,
		`{"num_accepted_draft_tokens":3,"num_draft_tokens":6,"num_spec_steps":2,"acceptance_histogram":[0,1,1],"draft_acceptance_rate":0.9}`,
		`{"num_accepted_draft_tokens":3,"num_draft_tokens":6,"num_spec_steps":2,"acceptance_histogram":[0,1,1],"mean_acceptance_length":3}`,
		`{"num_accepted_draft_tokens":3,"num_draft_tokens":6,"total_accepted_draft_tokens":3,"total_draft_tokens":6,"num_spec_steps":2,"acceptance_histogram":[0,1,1]}`,
	} {
		if _, err := parseSpecDecode(json.RawMessage(body)); err == nil {
			t.Fatalf("accepted contradictory vendor payload %s", body)
		}
	}
}
