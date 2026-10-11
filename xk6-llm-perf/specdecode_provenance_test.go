package llmperf

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func TestSpecDecodePlacementProvenance(t *testing.T) {
	valid := json.RawMessage(`{"num_accepted_draft_tokens":1,"num_draft_tokens":2,"num_spec_steps":1,"acceptance_histogram":[0,1]}`)
	next := json.RawMessage(`{"num_accepted_draft_tokens":2,"num_draft_tokens":4,"num_spec_steps":2,"acceptance_histogram":[0,2]}`)
	invalid := json.RawMessage(`{"num_spec_steps":1}`)
	for _, tt := range []struct {
		name       string
		run        func(*Client)
		diagnostic string
		accepted   int64
		source     string
	}{
		{"invalid then valid", func(c *Client) { c.readSpecDecode(invalid); c.readSpecDecode(valid) }, "invalid speculative", 0, ""},
		{"valid then invalid", func(c *Client) { c.readSpecDecode(valid); c.readSpecDecode(invalid) }, "invalid speculative", 0, ""},
		{"identical placements", func(c *Client) { c.readSpecDecodePlacements(valid, []Choice{{Index: 0, SpeculativeDecoding: valid}}) }, "", 1, "root"},
		{"conflicting placements", func(c *Client) { c.readSpecDecodePlacements(valid, []Choice{{Index: 0, SpeculativeDecoding: next}}) }, "conflicting root", 0, ""},
		{"malformed choice with valid root", func(c *Client) { c.readSpecDecodePlacements(valid, []Choice{{Index: 0, SpeculativeDecoding: invalid}}) }, "choice[0]", 0, ""},
		{"multiple choices", func(c *Client) {
			c.readSpecDecodePlacements(nil, []Choice{{Index: 0, SpeculativeDecoding: valid}, {Index: 1}})
		}, "ambiguous multi-choice", 0, ""},
		{"root aggregate multiple choices", func(c *Client) { c.readSpecDecodePlacements(valid, []Choice{{Index: 0}, {Index: 1}}) }, "", 1, "root"},
		{"repeated cumulative snapshots", func(c *Client) { c.readSpecDecode(valid); c.readSpecDecode(valid); c.readSpecDecode(next) }, "", 2, "root"},
		{"cumulative regression", func(c *Client) { c.readSpecDecode(next); c.readSpecDecode(valid) }, "inconsistent cumulative", 0, ""},
		{"choice identity changed", func(c *Client) {
			c.readSpecDecodePlacements(nil, []Choice{{Index: 0, SpeculativeDecoding: valid}})
			c.readSpecDecodePlacements(nil, []Choice{{Index: 1, SpeculativeDecoding: next}})
		}, "choice identity", 0, ""},
		{"later secondary choice", func(c *Client) {
			c.readSpecDecodePlacements(nil, []Choice{{Index: 0, SpeculativeDecoding: valid}})
			c.measurement.MultiChoice = true
			c.readSpecDecodePlacements(nil, []Choice{{Index: 1}})
		}, "ambiguous multi-choice", 0, ""},
		{"root retained for equal choice", func(c *Client) {
			c.readSpecDecode(valid)
			c.readSpecDecodePlacements(nil, []Choice{{Index: 0, SpeculativeDecoding: valid}})
		}, "", 1, "root"},
		{"choice cannot supersede root", func(c *Client) {
			c.readSpecDecode(valid)
			c.readSpecDecodePlacements(nil, []Choice{{Index: 0, SpeculativeDecoding: next}})
		}, "cannot replace request aggregate", 0, ""},
		{"single choice", func(c *Client) { c.readSpecDecodePlacements(nil, []Choice{{Index: 3, SpeculativeDecoding: valid}}) }, "", 1, "choice"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{measurement: &Measurement{}}
			tt.run(c)
			if tt.diagnostic != "" {
				if c.measurement.SpecDecode != nil || !strings.Contains(c.measurement.SpecDecodeUnavailable, tt.diagnostic) {
					t.Fatalf("got %+v diagnostic %q", c.measurement.SpecDecode, c.measurement.SpecDecodeUnavailable)
				}
				return
			}
			s := c.measurement.SpecDecode
			if s == nil || s.Accepted != tt.accepted || s.Source != tt.source || c.measurement.SpecDecodeUnavailable != "" {
				t.Fatalf("got %+v diagnostic %q", s, c.measurement.SpecDecodeUnavailable)
			}
			if tt.source == "choice" && (s.ChoiceIndex == nil || *s.ChoiceIndex != 3) {
				t.Fatalf("lost choice identity: %+v", s)
			}
		})
	}
}

func TestSpecDecodeHistogramOverflow(t *testing.T) {
	body := `{"num_accepted_draft_tokens":0,"num_draft_tokens":0,"num_spec_steps":0,"acceptance_histogram":[9223372036854775807,1]}`
	if _, err := parseSpecDecode(json.RawMessage(body)); err == nil {
		t.Fatalf("accepted overflow above %d", int64(math.MaxInt64))
	}
}

func TestSpecDecodeRecordProvenance(t *testing.T) {
	one := 1
	negative := -1
	large := 128
	for _, tt := range []struct {
		source string
		choice *int
		valid  bool
	}{
		{"", nil, true}, {"root", nil, true}, {"choice", &one, true}, {"", &one, false}, {"root", &one, false}, {"choice", nil, false}, {"choice", &negative, false}, {"choice", &large, false}, {"arbitrary", nil, false},
	} {
		s, err := parseSpecDecode(json.RawMessage(`{"num_accepted_draft_tokens":1,"num_draft_tokens":1,"num_spec_steps":1,"acceptance_histogram":[0,1]}`))
		if err != nil {
			t.Fatal(err)
		}
		s.Source, s.ChoiceIndex = tt.source, tt.choice
		if err := validateSpecDecodeRecord(s); (err == nil) != tt.valid {
			t.Fatalf("%+v: %v", tt, err)
		}
	}
	s, err := parseSpecDecode(json.RawMessage(`{"num_accepted_draft_tokens":1,"num_draft_tokens":1,"num_spec_steps":1,"acceptance_histogram":[0,1]}`))
	if err != nil {
		t.Fatal(err)
	}
	s.Accepted = 2
	if err := validateSpecDecodeRecord(s); err == nil {
		t.Fatal("accepted inconsistent counts")
	}
}
