package llmperf

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"time"
)

// TokenCounts supplies independently counted tokens. Tokenizer identifies the
// vocabulary and chat template used; these counts are declarations by the caller.
type TokenCounts struct {
	InputTokens      *int   `json:"inputTokens,omitempty"`
	OutputTokens     *int   `json:"outputTokens,omitempty"`
	FirstChunkTokens *int   `json:"firstChunkTokens,omitempty"`
	Tokenizer        string `json:"tokenizer"`
}

// TokenCount preserves absence and the origin of a count, including reported zero.
type TokenCount struct {
	Value  *int   `json:"value"`
	Source string `json:"source"`
}

// SLOReceipt records the limits used to classify a successful request.
// Zero disables a limit; finite non-negative values use milliseconds.
// TPOT and E2E pointers preserve unavailable limits in archived receipts.
type SLOReceipt struct {
	MaxTTFT         float64  `json:"max_ttft_ms"`
	MaxTTFO         float64  `json:"max_ttfo_ms"`
	MaxChunkLatency float64  `json:"max_chunk_latency_ms"`
	MaxTPOT         *float64 `json:"max_tpot_ms,omitempty"`
	MaxE2E          *float64 `json:"max_e2e_ms,omitempty"`
}

// Equal reports whether receipts describe the same limits and availability.
func (s SLOReceipt) Equal(t SLOReceipt) bool {
	return s.MaxTTFT == t.MaxTTFT && s.MaxTTFO == t.MaxTTFO && s.MaxChunkLatency == t.MaxChunkLatency && equalSLOLimit(s.MaxTPOT, t.MaxTPOT) && equalSLOLimit(s.MaxE2E, t.MaxE2E)
}

func equalSLOLimit(a, b *float64) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}

func sloLimit(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

// Measurement is a sanitized request record. Durations are milliseconds;
// timestamps are Unix milliseconds. No prompt, response text, or credential is stored.
type Measurement struct {
	RequestBodyBytes          int64        `json:"request_body_bytes,omitempty"`
	ResponseBodyBytes         int64        `json:"response_body_bytes,omitempty"`
	MaxConnsPerHost           int          `json:"max_connections_per_host,omitempty"`
	TransportProfile          string       `json:"transport_profile,omitempty"`
	Stream                    bool         `json:"stream,omitempty"`
	SerializationLatency      *float64     `json:"serialization_latency_ms,omitempty"`
	ParsingLatency            *float64     `json:"parsing_latency_ms,omitempty"`
	Work                      *WorkReceipt `json:"work,omitempty"`
	ContentIntervals          []float64    `json:"content_intervals_ms"`
	ContentIntervalsTruncated bool         `json:"content_intervals_truncated"`
	ContentEventCount         int          `json:"content_event_count"`
	TTST                      *float64     `json:"ttst"`
	TTSTPolicy                string       `json:"ttst_policy,omitempty"`
	secondContent             time.Time
	SLO                       *SLOReceipt `json:"slo,omitempty"`
	ChoicePolicy              string      `json:"choice_policy"`
	MultiChoice               bool        `json:"multi_choice"`
	ProviderTotalTokens       *int        `json:"provider_total_tokens"`
	ClientTokenizer           string      `json:"client_tokenizer,omitempty"`
	RequestID                 string      `json:"request_id,omitempty"`
	ScheduledAt               *float64    `json:"scheduled_at"`
	ArrivalLag                *float64    `json:"arrival_lag"`                // Unavailable if admission fails before dispatch.
	ScheduledResponseLatency  *float64    `json:"scheduled_response_latency"` // Includes completion of failed admission.
	HTTPTrace                 *HTTPTrace  `json:"http_trace"`
	trace                     *transportTrace
	Protocol                  string      `json:"protocol"`
	SchemaVersion             int         `json:"schema_version"`
	Phase                     string      `json:"phase"`
	Endpoint                  string      `json:"endpoint"`
	Model                     string      `json:"model"`
	Outcome                   string      `json:"outcome"`
	ErrorStage                string      `json:"error_stage,omitempty"`
	Status                    int         `json:"status"`
	Attempted                 bool        `json:"attempted"`
	Recorded                  bool        `json:"recorded"`
	Good                      bool        `json:"good"`
	StartedAt                 float64     `json:"started_at"`
	DispatchedAt              float64     `json:"dispatched_at"`
	FinishedAt                float64     `json:"finished_at"`
	TimestampBasis            string      `json:"timestamp_basis,omitempty"`
	AdmissionWait             float64     `json:"admission_wait"`
	FullResponseLatency       float64     `json:"full_response_latency"`
	LastContentLatency        *float64    `json:"last_content_latency"`
	TTFT                      *float64    `json:"ttft"`
	TTFO                      *float64    `json:"ttfo"`
	MaxChunkLatency           *float64    `json:"max_chunk_latency"`
	ITL                       *float64    `json:"itl"`
	FullResponseITL           *float64    `json:"full_response_itl"`
	DecodeTokens              *int        `json:"decode_tokens"`
	FirstChunkTokens          *int        `json:"first_chunk_tokens,omitempty"`
	FirstChunkOrigin          string      `json:"first_chunk_origin,omitempty"`
	FirstChunkUnavailable     string      `json:"first_chunk_unavailable,omitempty"`
	FirstChunkSource          string      `json:"first_chunk_source"`
	InputTokens               TokenCount  `json:"input_tokens"`
	OutputTokens              TokenCount  `json:"output_tokens"`
	RTT                       float64     `json:"rtt"`
	RTTSource                 string      `json:"rtt_source"`
	UsageDiscrepancy          bool        `json:"usage_discrepancy"`
	PromptDifference          *float64    `json:"prompt_difference"`
	OutputDifference          *float64    `json:"output_difference"`
	ReasoningTokens           *int        `json:"reasoning_tokens"`
	CachedTokens              *int        `json:"cached_tokens"`
	CacheWriteTokens          *int        `json:"cache_write_tokens"`
	SpecDecode                *SpecDecode `json:"spec_decode"`
	SpecDecodeUnavailable     string      `json:"spec_decode_unavailable,omitempty"`
	start                     time.Time
	requestStart              time.Time
	first                     time.Time
	firstOutput               time.Time
	last                      time.Time
	maxChunk                  time.Duration
	counts                    *TokenCounts
	firstChunk                *int
	reported                  Usage
	estimatedInput            int
	estimatedOutput           int
	finalized                 bool
	noInputEstimate           bool
	noOutputEstimate          bool
	responseEnd               time.Time
	runGeneration             uint64
}

func (u *Usage) UnmarshalJSON(data []byte) error {
	var v struct {
		Prompt        *int `json:"prompt_tokens"`
		Completion    *int `json:"completion_tokens"`
		Total         *int `json:"total_tokens"`
		CacheWrite    *int `json:"cache_write_tokens"`
		PromptDetails struct {
			Cached *int `json:"cached_tokens"`
		} `json:"prompt_tokens_details"`
		CompletionDetails struct {
			Reasoning *int `json:"reasoning_tokens"`
		} `json:"completion_tokens_details"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*u = Usage{}
	for _, p := range []*int{v.Prompt, v.Completion, v.Total, v.PromptDetails.Cached, v.CompletionDetails.Reasoning, v.CacheWrite} {
		if p != nil && *p < 0 {
			return fmt.Errorf("negative usage count")
		}
	}
	if v.Prompt != nil {
		u.PromptTokens, u.promptPresent = *v.Prompt, true
	}
	if v.Completion != nil {
		u.CompletionTokens, u.completionPresent = *v.Completion, true
	}
	if v.Total != nil {
		u.TotalTokens, u.totalPresent = *v.Total, true
	}
	u.CachedTokens, u.ReasoningTokens = v.PromptDetails.Cached, v.CompletionDetails.Reasoning
	u.CacheWriteTokens = v.CacheWrite
	return nil
}

func sanitizedEndpoint(endpoint string) string {
	u, err := url.Parse(endpoint)
	if err != nil {
		return ""
	}
	u.User, u.RawQuery, u.Fragment = nil, "", ""
	return u.String()
}

func ms(d time.Duration) float64            { return float64(d) / float64(time.Millisecond) }
func millis(t time.Time) float64            { return float64(t.UnixMicro()) / 1000 }
func count(n int, source string) TokenCount { return TokenCount{Value: &n, Source: source} }

func resolveCount(n int, present bool, declared *int, estimated int, strict bool) TokenCount {
	if present {
		return count(n, "reported")
	}
	if declared != nil {
		return count(*declared, "client_declared")
	}
	if strict {
		return TokenCount{Source: "unavailable"}
	}
	return count(estimated, "estimated")
}

func (m *Measurement) content(now time.Time, output bool, u Usage) {
	m.ContentEventCount++
	if m.ContentEventCount == 2 {
		m.secondContent = now
	}
	if m.first.IsZero() {
		m.ContentIntervals = []float64{}
		m.first = now
		if u.completionPresent {
			n := u.CompletionTokens
			m.firstChunk = &n
		}
	}
	if output && m.firstOutput.IsZero() {
		m.firstOutput = now
	}
	if !m.last.IsZero() {
		gap := now.Sub(m.last)
		m.maxChunk = maxDuration(m.maxChunk, gap)
		if len(m.ContentIntervals) < 4096 {
			m.ContentIntervals = append(m.ContentIntervals, ms(gap))
		} else {
			m.ContentIntervalsTruncated = true
		}
	}
	m.last = now
}

func (c *Client) finishMeasurement(m *Measurement) {
	if m.finalized {
		return
	}
	m.finalized = true
	if m.ChoicePolicy == "" {
		m.ChoicePolicy = "choice 0 timing; provider request-level usage"
	}
	now := m.responseEnd
	if now.IsZero() {
		now = time.Now()
	}
	anchor := m.requestStart
	if anchor.IsZero() {
		anchor = m.start
	}
	m.FullResponseLatency = ms(now.Sub(anchor))
	m.FinishedAt = millis(now)
	if m.Attempted && !m.requestStart.IsZero() {
		if m.DispatchedAt == 0 {
			m.DispatchedAt = millis(m.requestStart)
		}
		// Unix clock samples and monotonic durations need not advance identically.
		// Keep all request timing on one monotonic span with a wall-clock anchor.
		m.FinishedAt = m.DispatchedAt + m.FullResponseLatency
		if !m.start.IsZero() {
			m.StartedAt = m.DispatchedAt - ms(m.requestStart.Sub(m.start))
		}
		m.TimestampBasis = "dispatch Unix clock anchor plus monotonic elapsed time"
	} else if !m.start.IsZero() {
		if m.StartedAt == 0 {
			m.StartedAt = millis(m.start)
		}
		m.FinishedAt = m.StartedAt + m.FullResponseLatency
		m.TimestampBasis = "request Unix clock anchor plus monotonic elapsed time"
	}
	if m.ScheduledAt != nil {
		if m.Attempted {
			v := m.DispatchedAt - *m.ScheduledAt
			m.ArrivalLag = &v
		}
		full := m.FinishedAt - *m.ScheduledAt
		m.ScheduledResponseLatency = &full
	}
	if m.trace != nil {
		v := m.trace.finish(now)
		m.HTTPTrace = &v
	}
	if !m.secondContent.IsZero() {
		v := ms(m.secondContent.Sub(anchor))
		m.TTST = &v
		m.TTSTPolicy = "second recognized content event; not a physical second-token observation"
	}
	if !m.first.IsZero() {
		v := ms(m.first.Sub(anchor))
		m.TTFT = &v
	}
	if !m.firstOutput.IsZero() {
		v := ms(m.firstOutput.Sub(anchor))
		m.TTFO = &v
	}
	if !m.last.IsZero() {
		v := ms(m.last.Sub(anchor))
		m.LastContentLatency = &v
	}
	if m.maxChunk > 0 {
		v := ms(m.maxChunk)
		m.MaxChunkLatency = &v
	}
	var input, output, first *int
	if m.counts != nil {
		m.ClientTokenizer = m.counts.Tokenizer
		input, output, first = m.counts.InputTokens, m.counts.OutputTokens, m.counts.FirstChunkTokens
	}
	m.InputTokens, m.OutputTokens = c.resolveCounts(m)
	if m.reported.totalPresent {
		v := m.reported.TotalTokens
		m.ProviderTotalTokens = &v
	}
	m.ReasoningTokens, m.CachedTokens = m.reported.ReasoningTokens, m.reported.CachedTokens
	m.CacheWriteTokens = m.reported.CacheWriteTokens
	tpot, e2e := c.config.MaxTPOT, c.config.MaxE2E
	m.SLO = &SLOReceipt{MaxTTFT: c.config.MaxTTFT, MaxTTFO: c.config.MaxTTFO, MaxChunkLatency: c.config.MaxTokenLatency, MaxTPOT: &tpot, MaxE2E: &e2e}
	m.FirstChunkSource = "unavailable"
	if first != nil {
		m.firstChunk, m.FirstChunkSource = first, "client_declared"
	} else if m.firstChunk != nil {
		m.FirstChunkSource = "reported"
	}
	m.FirstChunkTokens = m.firstChunk
	if m.firstChunk != nil {
		m.FirstChunkOrigin = m.FirstChunkSource
	}
	if !m.MultiChoice && m.OutputTokens.Value != nil && *m.OutputTokens.Value >= 2 && m.TTFT != nil && m.LastContentLatency != nil {
		n := *m.OutputTokens.Value
		if m.firstChunk != nil && *m.firstChunk > 0 && *m.firstChunk < n {
			n -= *m.firstChunk
		} else {
			n--
			m.FirstChunkSource = "assumed_one"
			if m.firstChunk != nil {
				m.FirstChunkUnavailable = "first chunk count outside output count"
			}
		}
		m.DecodeTokens = &n
		v := (*m.LastContentLatency - *m.TTFT) / float64(n)
		m.ITL = &v
		full := (m.FullResponseLatency - *m.TTFT) / float64(n)
		m.FullResponseITL = &full
	}
	m.Good = m.Outcome == "success" && c.measurementGood(m)
	tolerance := 10.0
	if c.config.DiscrepancyTolerance != nil {
		tolerance = *c.config.DiscrepancyTolerance
	}
	// The denominator is the independent client count, never a substituted provider count.
	if input != nil && *input > 0 && m.reported.promptPresent {
		v := math.Abs(float64(m.reported.PromptTokens-*input)) / float64(*input) * 100
		m.PromptDifference = &v
	}
	if output != nil && *output > 0 && m.reported.completionPresent {
		v := math.Abs(float64(m.reported.CompletionTokens-*output)) / float64(*output) * 100
		m.OutputDifference = &v
	}
	for _, v := range []*float64{m.PromptDifference, m.OutputDifference} {
		if v != nil && *v > tolerance {
			m.UsageDiscrepancy = true
		}
	}
	if m.Work != nil {
		m.Work.Status, m.Work.Reasons = classifyWork(m)
	}
	if m.Recorded && m.Attempted {
		if m.Work != nil {
			c.metrics.RecordMetric("llm_work_"+m.Work.Status, 1)
			accepted := 0.0
			if m.Work.Status == "accepted" {
				accepted = 1
				if m.Good {
					c.metrics.RecordMetric("llm_work_good_requests", 1)
				}
			}
			c.metrics.RecordMetric("llm_work_acceptance", accepted)
		}
		if m.ReasoningTokens != nil {
			c.metrics.RecordMetric("llm_reasoning_tokens", float64(*m.ReasoningTokens))
		}
		if m.CacheWriteTokens != nil {
			c.metrics.RecordMetric("llm_cache_write_tokens", float64(*m.CacheWriteTokens))
		}
		if m.CachedTokens != nil {
			c.metrics.RecordMetric("llm_cached_tokens", float64(*m.CachedTokens))
		}
		if s := m.SpecDecode; s != nil {
			c.metrics.RecordMetric("llm_spec_accepted_tokens", float64(s.Accepted))
			c.metrics.RecordMetric("llm_spec_draft_tokens", float64(s.Drafted))
			if s.AcceptanceRate != nil {
				c.metrics.RecordMetric("llm_spec_acceptance_rate", *s.AcceptanceRate)
			}
			if s.MeanAcceptanceLength != nil {
				c.metrics.RecordMetric("llm_spec_acceptance_length", *s.MeanAcceptanceLength)
			}
		}
		c.metrics.RecordMetric("llm_attempts", 1)
		good := 0.0
		if m.Good {
			good = 1
		}
		c.metrics.RecordMetric("llm_good_requests", good)
		c.metrics.RecordMetric("llm_slo_compliance", good)
		c.metrics.RecordMetric("llm_good_request", good)
		c.metrics.RecordMetric("llm_slo_ttft_ms", m.SLO.MaxTTFT)
		c.metrics.RecordMetric("llm_slo_tpot_ms", *m.SLO.MaxTPOT)
		c.metrics.RecordMetric("llm_slo_e2e_ms", *m.SLO.MaxE2E)
		success := 0.0
		if m.Outcome == "success" {
			success = 1
		}
		c.metrics.RecordMetric("llm_successes", success)
		c.metrics.RecordMetric("llm_failures", 1-success)
		c.metrics.RecordMetric("llm_admission_wait", m.AdmissionWait)
		c.metrics.RecordMetric("llm_full_response_latency", m.FullResponseLatency)
		if !m.ContentIntervalsTruncated && len(m.ContentIntervals) > 0 {
			var total, largest float64
			for _, v := range m.ContentIntervals {
				total += v
				largest = max(largest, v)
			}
			c.metrics.RecordMetric("llm_mean_content_interval", total/float64(len(m.ContentIntervals)))
			c.metrics.RecordMetric("llm_max_content_interval", largest)
		}
		for name, v := range map[string]*float64{"llm_dispatch_ttft": m.TTFT, "llm_dispatch_ttfo": m.TTFO, "llm_dispatch_ttst": m.TTST, "llm_last_content_latency": m.LastContentLatency, "llm_inter_token_latency": m.ITL, "llm_full_response_itl": m.FullResponseITL, "llm_usage_prompt_difference": m.PromptDifference, "llm_usage_output_difference": m.OutputDifference} {
			if v != nil {
				c.metrics.RecordMetric(name, *v)
			}
		}
		if m.OutputTokens.Value != nil && m.FullResponseLatency > 0 {
			c.metrics.RecordMetric("llm_request_token_speed", float64(*m.OutputTokens.Value)/(m.FullResponseLatency/1000))
		}
		if m.ITL != nil && *m.ITL > 0 {
			c.metrics.RecordMetric("llm_decode_token_speed", 1000 / *m.ITL)
		}
		if m.UsageDiscrepancy {
			c.metrics.RecordMetric("llm_usage_discrepant_requests", 1)
		}
	}
	if c.run != nil {
		c.run.add(*m)
	}
}

func (c *Client) measurementGood(m *Measurement) bool {
	for _, s := range []struct {
		limit float64
		value *float64
	}{{c.config.MaxTTFT, m.TTFT}, {c.config.MaxTTFO, m.TTFO}, {c.config.MaxTokenLatency, m.MaxChunkLatency}, {c.config.MaxTPOT, m.ITL}, {c.config.MaxE2E, m.LastContentLatency}} {
		if s.limit > 0 && (s.value == nil || !finite(*s.value) || *s.value < 0 || *s.value > s.limit) {
			return false
		}
	}
	return true
}

func (c *Client) resolveCounts(m *Measurement) (TokenCount, TokenCount) {
	var input, output *int
	if m.counts != nil {
		input, output = m.counts.InputTokens, m.counts.OutputTokens
	}
	in := resolveCount(m.reported.PromptTokens, m.reported.promptPresent, input, m.estimatedInput, c.config.StrictMeasurement || m.noInputEstimate)
	out := resolveCount(m.reported.CompletionTokens, m.reported.completionPresent, output, m.estimatedOutput, c.config.StrictMeasurement || m.noOutputEstimate)
	if generatedTokenProtocol(m.Protocol) && !m.reported.completionPresent && output == nil && m.reported.totalPresent && m.reported.promptPresent && m.reported.TotalTokens >= m.reported.PromptTokens {
		out = count(m.reported.TotalTokens-m.reported.PromptTokens, "reported_derived")
	}
	return in, out
}

func validSLOLimit(v float64) bool { return finite(v) && v >= 0 }
