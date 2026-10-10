package llmperf

import (
	"encoding/json"
	"fmt"
	"math"
)

// SpecDecode retains engine-normalized speculative-decoding counters.
type SpecDecode struct {
	Engine               string   `json:"engine"`
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

func (c *Client) readSpecDecode(raw json.RawMessage) {
	if c.measurement == nil || len(raw) == 0 || string(raw) == "null" {
		return
	}
	s, err := parseSpecDecode(raw)
	if err != nil {
		c.measurement.SpecDecode = nil
		c.measurement.SpecDecodeUnavailable = err.Error()
		return
	}
	c.measurement.SpecDecode = s
	c.measurement.SpecDecodeUnavailable = ""
}
