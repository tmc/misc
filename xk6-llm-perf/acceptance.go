package llmperf

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"
)

// WorkExpectation qualifies the work performed by a protocol-successful request.
// Bounds include zero. Digests are lowercase SHA-256 hexadecimal strings.
type WorkExpectation struct {
	AllowedFinishReasons []string `json:"allowedFinishReasons,omitempty"`
	MinOutputTokens      *int     `json:"minOutputTokens,omitempty"`
	MaxOutputTokens      *int     `json:"maxOutputTokens,omitempty"`
	ExpectedModel        string   `json:"expectedModel,omitempty"`
	RequireExactUsage    bool     `json:"requireExactUsage,omitempty"`
	TextDigest           string   `json:"textDigest,omitempty"`
	ResponseDigest       string   `json:"responseDigest,omitempty"`
	TokenIDsDigest       string   `json:"tokenIDsDigest,omitempty"`
}

// WorkReceipt preserves qualification and bounded evidence without response text.
// Digests cover the normalized primary choice; multi-choice digests are unavailable.
type WorkReceipt struct {
	Expectation   WorkExpectation `json:"expectation"`
	Status        string          `json:"status"`
	Reasons       []string        `json:"reasons"`
	ObservedModel string          `json:"observed_model,omitempty"`
	// ObservedModelOversized records a reported model too long to retain,
	// so it is not mistaken for an unreported one.
	ObservedModelOversized bool     `json:"observed_model_oversized,omitempty"`
	FinishReasons          []string `json:"finish_reasons,omitempty"`
	ExactUsage             bool     `json:"exact_usage"`
	ReportedOutputTokens   *int     `json:"reported_output_tokens"`
	TextDigest             string   `json:"text_digest,omitempty"`
	ResponseDigest         string   `json:"response_digest,omitempty"`
	TokenIDsDigest         string   `json:"token_ids_digest,omitempty"`
}

func validDigest(s string) bool {
	if len(s) != 64 || s != strings.ToLower(s) {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

func validateWorkExpectation(w *WorkExpectation) error {
	if w == nil {
		return nil
	}
	if len(w.ExpectedModel) > 1024 || len(w.AllowedFinishReasons) > 32 {
		return fmt.Errorf("work expectation exceeds size limit")
	}
	seen := map[string]bool{}
	for _, s := range w.AllowedFinishReasons {
		if s == "" || len(s) > 128 || seen[s] {
			return fmt.Errorf("invalid allowed finish reason")
		}
		seen[s] = true
	}
	for _, n := range []*int{w.MinOutputTokens, w.MaxOutputTokens} {
		if n != nil && (*n < 0 || *n > 1<<53-1) {
			return fmt.Errorf("invalid output token bound")
		}
	}
	if w.MinOutputTokens != nil && w.MaxOutputTokens != nil && *w.MinOutputTokens > *w.MaxOutputTokens {
		return fmt.Errorf("output token bounds are reversed")
	}
	for _, s := range []string{w.TextDigest, w.ResponseDigest, w.TokenIDsDigest} {
		if s != "" && !validDigest(s) {
			return fmt.Errorf("invalid work digest")
		}
	}
	return nil
}

func workHash(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func observeWork(m *Measurement, response CompletionResponse) {
	if m.Work == nil {
		return
	}
	w := m.Work
	if len(response.Model) <= 1024 {
		w.ObservedModel = response.Model
	} else {
		w.ObservedModelOversized = true
	}
	w.ExactUsage = m.reported.promptPresent && m.reported.completionPresent && m.reported.totalPresent && m.reported.CompletionTokens <= m.reported.TotalTokens && m.reported.PromptTokens == m.reported.TotalTokens-m.reported.CompletionTokens
	if m.reported.completionPresent {
		n := m.reported.CompletionTokens
		w.ReportedOutputTokens = &n
	}
	for _, choice := range response.Choices {
		reason := choice.FinishReason
		if len(reason) > 128 {
			reason = ""
		}
		w.FinishReasons = append(w.FinishReasons, reason)
	}
	if !m.MultiChoice && len(response.Choices) == 1 {
		choice := response.Choices[0]
		if text, ok := choice.Message.Content.(string); ok {
			h := sha256.Sum256([]byte(text))
			w.TextDigest = hex.EncodeToString(h[:])
		}
		// Stable encoding omits request IDs, timestamps and transport details.
		w.ResponseDigest = workHash(struct {
			Model  string `json:"model"`
			Choice Choice `json:"choice"`
		}{response.Model, choice})
	}
}

// workUsageState derives presence and consistency from retained provider evidence,
// not client declarations or estimates. A partial sum exceeding a reported total
// is a known contradiction even when another count is missing.
func workUsageState(m *Measurement) string {
	input, output := m.InputTokens, m.OutputTokens
	if m.ProviderTotalTokens == nil {
		return "incomplete"
	}
	total := *m.ProviderTotalTokens
	inputKnown := input.Source == "reported" && input.Value != nil
	outputKnown := output.Source == "reported" && output.Value != nil
	if total < 0 || inputKnown && (*input.Value < 0 || *input.Value > total) || outputKnown && (*output.Value < 0 || *output.Value > total) {
		return "inconsistent"
	}
	if !inputKnown || !outputKnown {
		return "incomplete"
	}
	if *input.Value != total-*output.Value {
		return "inconsistent"
	}
	return "consistent"
}

func classifyWork(m *Measurement) (string, []string) {
	w := m.Work
	var rejected, unavailable []string
	if m.Outcome != "success" {
		return "rejected", []string{"protocol_unsuccessful"}
	}
	e := w.Expectation
	if e.ExpectedModel != "" {
		// An oversized model cannot equal an expected model within the size limit.
		if w.ObservedModelOversized {
			rejected = append(rejected, "model_mismatch")
		} else if w.ObservedModel == "" {
			unavailable = append(unavailable, "model_unobserved")
		} else if w.ObservedModel != e.ExpectedModel {
			rejected = append(rejected, "model_mismatch")
		}
	}
	if len(e.AllowedFinishReasons) > 0 {
		if m.MultiChoice {
			unavailable = append(unavailable, "multi_choice_finish_reasons_unavailable")
		} else if len(w.FinishReasons) == 0 || w.FinishReasons[0] == "" {
			unavailable = append(unavailable, "finish_reason_unobserved")
		} else {
			allowed := false
			for _, s := range e.AllowedFinishReasons {
				allowed = allowed || s == w.FinishReasons[0]
			}
			if !allowed {
				rejected = append(rejected, "finish_reason_mismatch")
			}
		}
	}
	if e.MinOutputTokens != nil || e.MaxOutputTokens != nil {
		if w.ReportedOutputTokens == nil {
			unavailable = append(unavailable, "reported_output_tokens_unavailable")
		} else if e.MinOutputTokens != nil && *w.ReportedOutputTokens < *e.MinOutputTokens || e.MaxOutputTokens != nil && *w.ReportedOutputTokens > *e.MaxOutputTokens {
			rejected = append(rejected, "output_token_bounds")
		}
	}
	if e.RequireExactUsage {
		switch workUsageState(m) {
		case "inconsistent":
			rejected = append(rejected, "exact_usage_inconsistent")
		case "incomplete":
			unavailable = append(unavailable, "exact_usage_unavailable")
		}
	}
	for _, d := range []struct{ name, want, got string }{{"text", e.TextDigest, w.TextDigest}, {"response", e.ResponseDigest, w.ResponseDigest}, {"token_ids", e.TokenIDsDigest, w.TokenIDsDigest}} {
		if d.want != "" {
			if d.got == "" {
				unavailable = append(unavailable, d.name+"_digest_unavailable")
			} else if d.got != d.want {
				rejected = append(rejected, d.name+"_digest_mismatch")
			}
		}
	}
	if len(rejected) > 0 {
		return "rejected", append(rejected, unavailable...)
	}
	if len(unavailable) > 0 {
		return "unavailable", unavailable
	}
	return "accepted", nil
}

func validateWorkMeasurement(m *Measurement) error {
	w := m.Work
	if w == nil {
		return nil
	}
	if err := validateWorkExpectation(&w.Expectation); err != nil {
		return err
	}
	if !m.MultiChoice && len(w.FinishReasons) > 1 {
		return fmt.Errorf("work finish reasons contradict choice policy")
	}
	if len(w.ObservedModel) > 1024 || len(w.FinishReasons) > 128 {
		return fmt.Errorf("work evidence exceeds size limit")
	}
	if w.ObservedModelOversized && w.ObservedModel != "" {
		return fmt.Errorf("work model contradicts oversized model")
	}
	for _, s := range w.FinishReasons {
		if len(s) > 128 {
			return fmt.Errorf("work finish reason exceeds size limit")
		}
	}
	for _, s := range []string{w.TextDigest, w.ResponseDigest, w.TokenIDsDigest} {
		if s != "" && !validDigest(s) {
			return fmt.Errorf("invalid observed work digest")
		}
	}
	// No adapter currently exposes provider token IDs. A replay cannot invent them.
	if m.MultiChoice && (w.TextDigest != "" || w.ResponseDigest != "") {
		return fmt.Errorf("multi-choice digest evidence is unavailable")
	}
	if w.TokenIDsDigest != "" {
		return fmt.Errorf("provider token IDs are unavailable")
	}
	if w.ReportedOutputTokens != nil && (*w.ReportedOutputTokens < 0 || m.OutputTokens.Value == nil || m.OutputTokens.Source != "reported" || *w.ReportedOutputTokens != *m.OutputTokens.Value) {
		return fmt.Errorf("work output count contradicts reported tokens")
	}
	if w.ExactUsage && workUsageState(m) != "consistent" {
		return fmt.Errorf("work exact usage contradicts measurement")
	}
	if m.Outcome == "success" {
		if (w.ReportedOutputTokens != nil) != (m.OutputTokens.Source == "reported" && m.OutputTokens.Value != nil) {
			return fmt.Errorf("work output presence contradicts reported tokens")
		}
		if w.ExactUsage != (workUsageState(m) == "consistent") {
			return fmt.Errorf("work exact usage contradicts measurement")
		}
	}

	status, reasons := classifyWork(m)
	if w.Status != status || !reflect.DeepEqual(w.Reasons, reasons) {
		return fmt.Errorf("work qualification contradicts evidence")
	}
	return nil
}

// UnmarshalJSON rejects unknown and null policy fields instead of silently
// disabling a requested qualification check.
func (w *WorkExpectation) UnmarshalJSON(data []byte) error {
	keys := json.NewDecoder(strings.NewReader(string(data)))
	if tok, err := keys.Token(); err != nil || tok != json.Delim('{') {
		return fmt.Errorf("work expectation must be an object")
	}
	seen := map[string]bool{}
	for keys.More() {
		tok, err := keys.Token()
		if err != nil {
			return err
		}
		name := tok.(string)
		if seen[name] {
			return fmt.Errorf("duplicate work field %q", name)
		}
		seen[name] = true
		var raw json.RawMessage
		if err := keys.Decode(&raw); err != nil {
			return err
		}
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	if fields == nil {
		return fmt.Errorf("work expectation must be an object")
	}
	for name, value := range fields {
		if string(value) == "null" {
			return fmt.Errorf("work.%s must not be null", name)
		}
	}
	type plain WorkExpectation
	var v plain
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&v); err != nil {
		return err
	}
	*w = WorkExpectation(v)
	return validateWorkExpectation(w)
}

func (w *WorkReceipt) UnmarshalJSON(data []byte) error {
	keys := json.NewDecoder(strings.NewReader(string(data)))
	if tok, err := keys.Token(); err != nil || tok != json.Delim('{') {
		return fmt.Errorf("work receipt must be an object")
	}
	seen := map[string]bool{}
	for keys.More() {
		tok, err := keys.Token()
		if err != nil {
			return err
		}
		name := tok.(string)
		if seen[name] {
			return fmt.Errorf("duplicate work receipt field %q", name)
		}
		seen[name] = true
		var raw json.RawMessage
		if err := keys.Decode(&raw); err != nil {
			return err
		}
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	for _, name := range []string{"expectation", "status", "reasons", "exact_usage", "reported_output_tokens"} {
		raw, ok := fields[name]
		if !ok {
			return fmt.Errorf("work receipt missing %s", name)
		}
		if string(raw) == "null" && name != "reasons" && name != "reported_output_tokens" {
			return fmt.Errorf("work receipt null %s", name)
		}
	}
	type plain WorkReceipt
	var v plain
	d := json.NewDecoder(strings.NewReader(string(data)))
	d.DisallowUnknownFields()
	if err := d.Decode(&v); err != nil {
		return err
	}
	*w = WorkReceipt(v)
	return nil
}

func addWorkCounts(s *RunSummary, m Measurement) {
	if m.Work == nil {
		return
	}
	s.WorkQualified++
	switch m.Work.Status {
	case "accepted":
		s.WorkAccepted++
		if m.Good {
			s.WorkGoodRequests++
		}
	case "rejected":
		s.WorkRejected++
	case "unavailable":
		s.WorkUnavailable++
	}
}

func validateWorkRun(s RunSummary) error {
	if s.WorkQualified < 0 || s.WorkAccepted < 0 || s.WorkRejected < 0 || s.WorkUnavailable < 0 || s.WorkGoodRequests < 0 || s.WorkQualified > s.Attempts || s.WorkAccepted > s.Successes || s.WorkGoodRequests > s.GoodRequests || s.WorkGoodRequests > s.WorkAccepted || s.WorkQualified != s.WorkAccepted+s.WorkRejected+s.WorkUnavailable || !finite(s.WorkGoodput) || s.WorkGoodput < 0 {
		return fmt.Errorf("invalid work population accounting")
	}
	var retained RunSummary
	for _, m := range s.Records {
		if m.Outcome == "success" {
			retained.Successes++
			if m.Good {
				retained.GoodRequests++
			}
		}
		if err := validateWorkMeasurement(&m); err != nil {
			return err
		}
		addWorkCounts(&retained, m)
	}
	for _, c := range []struct{ total, retained int }{{s.WorkQualified, retained.WorkQualified}, {s.WorkAccepted, retained.WorkAccepted}, {s.WorkRejected, retained.WorkRejected}, {s.WorkUnavailable, retained.WorkUnavailable}, {s.WorkGoodRequests, retained.WorkGoodRequests}, {s.Attempts - s.WorkQualified, len(s.Records) - retained.WorkQualified}} {
		if c.total < c.retained || c.total-c.retained > s.DroppedRecords {
			return fmt.Errorf("work population contradicts retained evidence")
		}
	}
	if s.WorkAccepted-retained.WorkAccepted+s.WorkUnavailable-retained.WorkUnavailable > s.Successes-retained.Successes {
		return fmt.Errorf("work accepted/unavailable population exceeds dropped successes")
	}
	if s.WorkGoodRequests-retained.WorkGoodRequests > s.GoodRequests-retained.GoodRequests {
		return fmt.Errorf("work good population exceeds dropped SLO-good successes")
	}
	if s.WorkGoodRequests-retained.WorkGoodRequests > s.WorkAccepted-retained.WorkAccepted {
		return fmt.Errorf("work good population contradicts accepted population")
	}
	// A missing rate is recomputed by ReadRun; positive forged rates are rejected.
	if s.WorkGoodput != 0 && (s.Duration <= 0 || math.Abs(s.WorkGoodput-float64(s.WorkGoodRequests)/s.Duration) > 1e-9) {
		return fmt.Errorf("work goodput contradicts accounting")
	}
	return nil
}
