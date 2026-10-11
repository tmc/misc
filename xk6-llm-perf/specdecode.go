package llmperf

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"
)

// SpecDecode retains engine-normalized speculative-decoding counters.
type SpecDecode struct {
	Engine string `json:"engine"`
	// Source identifies a request aggregate or a single choice snapshot.
	Source               string   `json:"source,omitempty"`
	ChoiceIndex          *int     `json:"choice_index,omitempty"`
	Accepted             int64    `json:"accepted"`
	Drafted              int64    `json:"drafted"`
	Steps                int64    `json:"steps"`
	Histogram            []int64  `json:"histogram"`
	AcceptanceRate       *float64 `json:"acceptance_rate"`
	MeanAcceptanceLength *float64 `json:"mean_acceptance_length"`
}

func parseSpecDecode(data json.RawMessage) (*SpecDecode, error) {
	if len(data) == 0 || string(data) == "null" {
		return nil, nil
	}
	var p struct {
		Accepted    *int64   `json:"num_accepted_draft_tokens"`
		Drafted     *int64   `json:"num_draft_tokens"`
		TRTAccepted *int64   `json:"total_accepted_draft_tokens"`
		TRTDrafted  *int64   `json:"total_draft_tokens"`
		Steps       *int64   `json:"num_spec_steps"`
		Width       *int     `json:"num_spec_tokens"`
		Histogram   []int64  `json:"acceptance_histogram"`
		VLLMRate    *float64 `json:"draft_acceptance_rate"`
		TRTRate     *float64 `json:"acceptance_rate"`
		Length      *float64 `json:"mean_acceptance_length"`
	}
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("invalid speculative decoding payload")
	}
	s := &SpecDecode{Engine: "vllm", Histogram: p.Histogram}
	a, d := p.Accepted, p.Drafted
	if p.TRTAccepted != nil || p.TRTDrafted != nil {
		if p.Accepted != nil || p.Drafted != nil {
			return nil, fmt.Errorf("mixed speculative engine signatures")
		}
		s.Engine = "tensorrt_llm"
		a, d = p.TRTAccepted, p.TRTDrafted
	}
	if s.Engine == "tensorrt_llm" && (p.Steps == nil || *p.Steps <= 0) {
		return nil, fmt.Errorf("tensorrt speculative steps must be positive")
	}
	if a == nil || d == nil || p.Steps == nil || *a < 0 || *d < 0 || *p.Steps < 0 || *a > *d || len(p.Histogram) == 0 || len(p.Histogram) > 4096 {
		return nil, fmt.Errorf("invalid speculative decoding counts")
	}
	if p.Width != nil && (*p.Width < 0 || *p.Width >= 4096 || len(p.Histogram) != *p.Width+1) {
		return nil, fmt.Errorf("invalid speculative histogram width")
	}
	var steps, accepted int64
	for i, n := range p.Histogram {
		if n < 0 || n > math.MaxInt64/int64(len(p.Histogram)) || steps > math.MaxInt64-n || accepted > math.MaxInt64-int64(i)*n {
			return nil, fmt.Errorf("invalid speculative histogram")
		}
		steps += n
		accepted += int64(i) * n
	}
	if steps != *p.Steps || accepted != *a {
		return nil, fmt.Errorf("speculative histogram disagrees with counts")
	}
	s.Accepted, s.Drafted, s.Steps = *a, *d, *p.Steps
	if s.Drafted > 0 {
		v := float64(s.Accepted) / float64(s.Drafted)
		s.AcceptanceRate = &v
	}
	if s.Steps > 0 {
		v := 1 + float64(s.Accepted)/float64(s.Steps)
		s.MeanAcceptanceLength = &v
	}
	rate := p.VLLMRate
	if s.Engine == "tensorrt_llm" {
		rate = p.TRTRate
	}
	if rate != nil && (!finite(*rate) || *rate < 0 || *rate > 1 || s.AcceptanceRate != nil && math.Abs(*rate-*s.AcceptanceRate) > 1e-9) {
		return nil, fmt.Errorf("speculative acceptance rate disagrees with counts")
	}
	if p.Length != nil && (!finite(*p.Length) || *p.Length < 1 || s.MeanAcceptanceLength != nil && math.Abs(*p.Length-*s.MeanAcceptanceLength) > 1e-9) {
		return nil, fmt.Errorf("speculative acceptance length disagrees with counts")
	}
	return s, nil
}

// readSpecDecodePlacements treats each payload as a cumulative snapshot, not an
// increment. A malformed or ambiguous placement makes the request's vendor
// metrics unavailable permanently; text and ordinary usage remain usable.
func (c *Client) readSpecDecodePlacements(root json.RawMessage, choices []Choice) {
	if c.measurement == nil || c.measurement.SpecDecodeUnavailable != "" {
		return
	}
	var selected *SpecDecode
	fail := func(source string, err error) {
		c.measurement.SpecDecode = nil
		c.measurement.SpecDecodeUnavailable = source + ": " + err.Error()
	}
	if previous := c.measurement.SpecDecode; previous != nil && previous.ChoiceIndex != nil && (len(choices) > 1 || c.measurement.MultiChoice) {
		fail("choice", fmt.Errorf("ambiguous multi-choice speculative aggregate"))
		return
	}
	if len(root) != 0 && string(root) != "null" {
		var err error
		selected, err = parseSpecDecode(root)
		if err != nil {
			fail("root", err)
			return
		}
		selected.Source = "root"
	}
	populated := 0
	for _, choice := range choices {
		if len(choice.SpeculativeDecoding) == 0 || string(choice.SpeculativeDecoding) == "null" {
			continue
		}
		populated++
		source := fmt.Sprintf("choice[%d]", choice.Index)
		value, err := parseSpecDecode(choice.SpeculativeDecoding)
		if err != nil {
			fail(source, err)
			return
		}
		if populated > 1 || len(choices) > 1 || c.measurement.MultiChoice {
			fail(source, fmt.Errorf("ambiguous multi-choice speculative aggregate"))
			return
		}
		if selected != nil {
			if !sameSpecDecode(selected, value) {
				fail(source, fmt.Errorf("conflicting root and choice speculative snapshots"))
				return
			}
		} else {
			selected = value
			selected.Source = "choice"
			index := choice.Index
			selected.ChoiceIndex = &index
		}
	}
	if selected == nil {
		return
	}
	previous := c.measurement.SpecDecode
	if previous != nil {
		if previous.Source == "root" && selected.Source == "choice" {
			if sameSpecDecode(previous, selected) {
				return
			}
			fail("choice", fmt.Errorf("choice snapshot cannot replace request aggregate"))
			return
		}
		if previous.Engine != selected.Engine || previous.Accepted > selected.Accepted || previous.Drafted > selected.Drafted || previous.Steps > selected.Steps || len(previous.Histogram) != len(selected.Histogram) {
			fail(selected.Source, fmt.Errorf("inconsistent cumulative speculative snapshot"))
			return
		}
		if previous.ChoiceIndex != nil && selected.ChoiceIndex != nil && *previous.ChoiceIndex != *selected.ChoiceIndex {
			fail(selected.Source, fmt.Errorf("speculative snapshot changed choice identity"))
			return
		}
		for i, n := range previous.Histogram {
			if n > selected.Histogram[i] {
				fail(selected.Source, fmt.Errorf("speculative histogram decreased"))
				return
			}
		}
	}
	c.measurement.SpecDecode = selected
}

func sameSpecDecode(a, b *SpecDecode) bool {
	return a.Engine == b.Engine && a.Accepted == b.Accepted && a.Drafted == b.Drafted && a.Steps == b.Steps && slices.Equal(a.Histogram, b.Histogram)
}

func (c *Client) readSpecDecode(raw json.RawMessage) {
	c.readSpecDecodePlacements(raw, nil)
}

// validateSpecDecodeRecord validates the normalized counters stored in a run.
// Legacy records may omit source, but cannot then claim a choice identity.
func validateSpecDecodeRecord(s *SpecDecode) error {
	if s == nil {
		return nil
	}
	switch s.Source {
	case "", "root":
		if s.ChoiceIndex != nil {
			return fmt.Errorf("speculative choice identity requires choice source")
		}
	case "choice":
		if s.ChoiceIndex == nil || *s.ChoiceIndex < 0 || *s.ChoiceIndex >= 128 {
			return fmt.Errorf("invalid speculative choice identity")
		}
	default:
		return fmt.Errorf("unknown speculative source")
	}
	fields := map[string]any{"num_spec_steps": s.Steps, "acceptance_histogram": s.Histogram, "mean_acceptance_length": s.MeanAcceptanceLength}
	switch s.Engine {
	case "vllm":
		fields["num_accepted_draft_tokens"], fields["num_draft_tokens"], fields["draft_acceptance_rate"] = s.Accepted, s.Drafted, s.AcceptanceRate
	case "tensorrt_llm":
		fields["total_accepted_draft_tokens"], fields["total_draft_tokens"], fields["acceptance_rate"] = s.Accepted, s.Drafted, s.AcceptanceRate
	default:
		return fmt.Errorf("unknown speculative engine")
	}
	raw, err := json.Marshal(fields)
	if err != nil {
		return fmt.Errorf("encode speculative counters: %w", err)
	}
	checked, err := parseSpecDecode(raw)
	if err != nil {
		return err
	}
	if (checked.AcceptanceRate == nil) != (s.AcceptanceRate == nil) || (checked.MeanAcceptanceLength == nil) != (s.MeanAcceptanceLength == nil) {
		return fmt.Errorf("missing or unexpected speculative derived statistics")
	}
	return nil
}
