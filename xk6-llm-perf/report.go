package llmperf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/tmc/misc/xk6-llm-perf/internal/jsoncheck"
	"io"
	"math"
	"sort"
)

// ReadRun reads a bounded version-1 run export and checks its record accounting.
func ReadRun(r io.Reader) (RunSummary, error) {
	const limit = 64 * 1024 * 1024
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return RunSummary{}, fmt.Errorf("read run: %w", err)
	}
	if len(data) > limit {
		return RunSummary{}, fmt.Errorf("run exceeds 64 MiB")
	}
	var run RunSummary
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&run); err != nil {
		return run, fmt.Errorf("decode run: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return run, fmt.Errorf("trailing run data")
	}
	if err := validateRun(run); err != nil {
		return run, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return run, err
	}
	for name, check := range map[string]struct{ actual, expected float64 }{
		"goodput":           {run.Goodput, float64(run.GoodRequests) / run.Duration},
		"work_goodput":      {run.WorkGoodput, float64(run.WorkGoodRequests) / run.Duration},
		"input_throughput":  {run.InputThroughput, float64(run.InputTokens) / run.Duration},
		"output_throughput": {run.OutputThroughput, float64(run.OutputTokens) / run.Duration},
	} {
		available := name != "input_throughput" || run.InputThroughputAvailable()
		available = available && (name != "output_throughput" || run.OutputThroughputAvailable())
		if !available {
			// Historical exports could contain a partial numeric rate. Check its
			// arithmetic, then discard it as whole-population rate evidence.
			if raw, present := fields[name]; present && !bytes.Equal(bytes.TrimSpace(raw), []byte("null")) && math.Abs(check.actual-check.expected) > 1e-9*math.Max(1, math.Abs(check.expected)) {
				return run, fmt.Errorf("%s disagrees with aggregate accounting", name)
			}
			if name == "input_throughput" {
				run.InputThroughput = 0
			} else {
				run.OutputThroughput = 0
			}
			continue
		}
		if raw, present := fields[name]; present {
			if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
				return run, fmt.Errorf("%s must not be null", name)
			}
		} else {
			switch name {
			case "work_goodput":
				run.WorkGoodput = check.expected
			case "goodput":
				run.Goodput = check.expected
			case "input_throughput":
				run.InputThroughput = check.expected
			case "output_throughput":
				run.OutputThroughput = check.expected
			}
			continue
		}
		if math.Abs(check.actual-check.expected) > 1e-9*math.Max(1, math.Abs(check.expected)) {
			return run, fmt.Errorf("%s disagrees with aggregate accounting", name)
		}
	}
	return run, nil
}

func validateRun(run RunSummary) error {
	if err := validateWorkRun(run); err != nil {
		return err
	}
	if run.ServingManifest != nil {
		if err := ValidateServingManifest(*run.ServingManifest); err != nil {
			return fmt.Errorf("serving manifest: %w", err)
		}
	}
	if run.AggregateError != "" {
		return fmt.Errorf("incomplete aggregate: %s", run.AggregateError)
	}
	if !finite(run.StartedAt) || !finite(run.FinishedAt) || !finite(run.Duration) || run.SchemaVersion != 1 || run.Duration <= 0 || run.FinishedAt <= run.StartedAt || run.Attempts < 0 || run.Successes < 0 || run.Failures < 0 || run.GoodRequests < 0 || run.Successes > run.Attempts || run.Failures != run.Attempts-run.Successes || run.GoodRequests > run.Successes || run.InputTokens < 0 || run.OutputTokens < 0 || run.DroppedRecords < 0 || len(run.Records) > 100000 || run.DroppedRecords > run.Attempts || len(run.Records) != run.Attempts-run.DroppedRecords {
		return fmt.Errorf("invalid run accounting or schema")
	}
	for _, rate := range []float64{run.Goodput, run.OutputThroughput, run.InputThroughput} {
		if !finite(rate) || rate < 0 {
			return fmt.Errorf("invalid aggregate rate")
		}
	}
	if run.MissingInputCounts < 0 || run.MissingOutputCounts < 0 || run.EstimatedInputCounts < 0 || run.EstimatedOutputCounts < 0 || run.ExcludedBoundaryRequests < 0 || run.InflightAtEnd < 0 || run.RecordLimit < 0 || run.RecordByteLimit < 0 || run.RetainedRecordBytes < 0 {
		return fmt.Errorf("invalid manifest completeness counter")
	}
	if math.Abs((run.FinishedAt-run.StartedAt)/1000-run.Duration) > .001 {
		return fmt.Errorf("run duration does not match window")
	}
	if run.DroppedRecords == 0 {
		timeline, err := reconstructTimeline(run)
		if err != nil {
			return err
		}
		if run.Timeline != nil {
			a, _ := json.Marshal(run.Timeline)
			b, _ := json.Marshal(timeline)
			if !bytes.Equal(a, b) {
				return fmt.Errorf("retained timeline disagrees with records")
			}
		}
	}
	{
		var successes, failures, good int
		var missingInput, missingOutput, estimatedInput, estimatedOutput int
		var input, output int64
		seen := map[string]bool{}
		for _, m := range run.Records {
			if err := validateSpecDecodeRecord(m.SpecDecode); err != nil {
				return fmt.Errorf("speculative record: %w", err)
			}
			if m.SpecDecode != nil && m.SpecDecodeUnavailable != "" {
				return fmt.Errorf("speculative record has contradictory availability")
			}
			if err := validateRecordTiming(m, run); err != nil {
				return err
			}
			if m.SchemaVersion != 1 || !m.Recorded || !m.Attempted || !finite(m.FullResponseLatency) || m.FullResponseLatency < 0 {
				return fmt.Errorf("invalid retained record")
			}
			if m.ContentEventCount == 0 && len(m.ContentIntervals) > 0 {
				return fmt.Errorf("content intervals lack event population")
			}
			if m.TTST != nil && (m.ContentEventCount < 2 || m.TTFT == nil || *m.TTST < *m.TTFT || m.LastContentLatency == nil || *m.TTST > *m.LastContentLatency || m.TTSTPolicy != "second recognized content event; not a physical second-token observation") {
				return fmt.Errorf("invalid second content observation")
			}
			if m.ContentEventCount < 0 || len(m.ContentIntervals) > 4096 || m.ContentEventCount > 0 && !m.ContentIntervalsTruncated && len(m.ContentIntervals) != m.ContentEventCount-1 || m.ContentIntervalsTruncated && (len(m.ContentIntervals) != 4096 || m.ContentEventCount <= 4097) {
				return fmt.Errorf("invalid content interval accounting")
			}
			var span float64
			for _, v := range m.ContentIntervals {
				if !finite(v) || v < 0 || v > m.FullResponseLatency+.001 {
					return fmt.Errorf("invalid content interval")
				}
				span += v
			}
			if m.ContentEventCount > 1 && m.TTST == nil {
				return fmt.Errorf("second content observation missing")
			}
			if m.ContentEventCount > 0 && (m.TTFT == nil || m.LastContentLatency == nil) {
				return fmt.Errorf("content event population lacks timing")
			}
			if m.ContentEventCount > 1 && m.TTST != nil && len(m.ContentIntervals) > 0 && math.Abs(*m.TTST-(*m.TTFT+m.ContentIntervals[0])) > .001 {
				return fmt.Errorf("second content timing disagrees with intervals")
			}
			if m.ContentEventCount > 0 && !m.ContentIntervalsTruncated && math.Abs(span-(*m.LastContentLatency-*m.TTFT)) > .001 {
				return fmt.Errorf("content span disagrees with intervals")
			}
			if span > m.FullResponseLatency+.001 {
				return fmt.Errorf("content intervals exceed response latency")
			}
			if m.SLO != nil && (!validSLOLimit(m.SLO.MaxTTFT) || !validSLOLimit(m.SLO.MaxTTFO) || !validSLOLimit(m.SLO.MaxChunkLatency) || m.SLO.MaxTPOT != nil && !validSLOLimit(*m.SLO.MaxTPOT) || m.SLO.MaxE2E != nil && !validSLOLimit(*m.SLO.MaxE2E)) {
				return fmt.Errorf("negative SLO limit")
			}
			if m.RequestID != "" {
				if seen[m.RequestID] {
					return fmt.Errorf("duplicate request ID")
				}
				seen[m.RequestID] = true
			}
			for _, c := range []TokenCount{m.InputTokens, m.OutputTokens} {
				if c.Value != nil && (*c.Value < 0 || (c.Source != "reported" && c.Source != "reported_derived" && c.Source != "client_declared" && c.Source != "estimated" && c.Source != "locally_tokenized")) {
					return fmt.Errorf("invalid token count provenance")
				}
				if c.Value == nil && c.Source != "unavailable" {
					return fmt.Errorf("missing token count provenance")
				}
			}
			switch m.Outcome {
			case "success":
				successes++
				if m.InputTokens.Value == nil {
					missingInput++
				}
				if m.OutputTokens.Value == nil {
					missingOutput++
				}
				if m.InputTokens.Source == "estimated" {
					estimatedInput++
				}
				if m.OutputTokens.Source == "estimated" {
					estimatedOutput++
				}
				if m.Good {
					good++
				}
				if m.InputTokens.Value != nil {
					if int64(*m.InputTokens.Value) > math.MaxInt64-input {
						return fmt.Errorf("input token aggregate overflow")
					}
					input += int64(*m.InputTokens.Value)
				}
				if m.OutputTokens.Value != nil {
					if int64(*m.OutputTokens.Value) > math.MaxInt64-output {
						return fmt.Errorf("output token aggregate overflow")
					}
					output += int64(*m.OutputTokens.Value)
				}
			case "failure", "timeout", "cancelled":
				failures++
				if m.Good {
					return fmt.Errorf("failed request classified good")
				}
			default:
				return fmt.Errorf("unknown request outcome")
			}
		}
		for _, c := range []int{run.MissingInputCounts, run.MissingOutputCounts, run.EstimatedInputCounts, run.EstimatedOutputCounts} {
			if c > run.Successes {
				return fmt.Errorf("completeness counter exceeds successful population")
			}
		}
		if missingInput > run.MissingInputCounts || missingOutput > run.MissingOutputCounts || estimatedInput > run.EstimatedInputCounts || estimatedOutput > run.EstimatedOutputCounts || run.MissingInputCounts-missingInput > run.Successes-successes || run.MissingOutputCounts-missingOutput > run.Successes-successes || run.EstimatedInputCounts-estimatedInput > run.Successes-successes || run.EstimatedOutputCounts-estimatedOutput > run.Successes-successes {
			return fmt.Errorf("retained records disagree with completeness bounds")
		}
		if run.MissingInputCounts > run.Successes-run.EstimatedInputCounts || run.MissingOutputCounts > run.Successes-run.EstimatedOutputCounts || run.Successes == 0 && (run.InputTokens != 0 || run.OutputTokens != 0) {
			return fmt.Errorf("invalid successful token population accounting")
		}
		if run.DroppedRecords == 0 && (missingInput != run.MissingInputCounts || missingOutput != run.MissingOutputCounts || estimatedInput != run.EstimatedInputCounts || estimatedOutput != run.EstimatedOutputCounts) {
			return fmt.Errorf("records disagree with completeness accounting")
		}
		if successes > run.Successes || failures > run.Failures || good > run.GoodRequests || run.Successes-successes > run.DroppedRecords || run.Failures-failures > run.DroppedRecords || run.GoodRequests-good > run.Successes-successes || input > run.InputTokens || output > run.OutputTokens {
			return fmt.Errorf("retained records exceed aggregate accounting bounds")
		}
		droppedSuccesses := run.Successes - successes
		for _, c := range []struct {
			missing, retainedMissing, estimated, retainedEstimated int
			total, retainedTotal                                   int64
		}{
			{run.MissingInputCounts, missingInput, run.EstimatedInputCounts, estimatedInput, run.InputTokens, input},
			{run.MissingOutputCounts, missingOutput, run.EstimatedOutputCounts, estimatedOutput, run.OutputTokens, output},
		} {
			missingDropped := c.missing - c.retainedMissing
			estimatedDropped := c.estimated - c.retainedEstimated
			if missingDropped > droppedSuccesses-estimatedDropped {
				return fmt.Errorf("dropped successful count populations overlap")
			}
			if missingDropped == droppedSuccesses && c.total != c.retainedTotal {
				return fmt.Errorf("tokens exceed available successful count population")
			}
		}
		if run.DroppedRecords == 0 && (successes != run.Successes || failures != run.Failures || good != run.GoodRequests || input != run.InputTokens || output != run.OutputTokens) {
			return fmt.Errorf("records disagree with aggregate accounting")
		}
	}
	if run.Observation != nil {
		if run.ObservationUnavailable != "" {
			return fmt.Errorf("observation has contradictory availability")
		}
		observed, err := observationReceipt(run)
		if err != nil {
			return err
		}
		if !run.Observation.equal(observed) {
			return fmt.Errorf("observation receipt disagrees with retained records")
		}
	}
	return nil
}

// SliceSummary counts requests selected by Policy. Attempts counts completions
// by default, or dispatch starts when AttemptSelection is dispatch_start.
// Concurrency is independently clipped at slice boundaries.
type SliceSummary struct {
	StartedAt           float64     `json:"started_at"`
	FinishedAt          float64     `json:"finished_at"`
	Attempts            int         `json:"completed_attempts"`
	AttemptSelection    string      `json:"attempt_selection,omitempty"`
	Successes           int         `json:"successes"`
	Failures            int         `json:"failures"`
	GoodRequests        int         `json:"good_requests"`
	InputTokens         int64       `json:"input_tokens"`
	InputThroughput     float64     `json:"input_throughput"`
	OutputTokens        int64       `json:"output_tokens"`
	Goodput             float64     `json:"goodput"`
	OutputThroughput    float64     `json:"output_throughput"`
	MissingInputCounts  int         `json:"missing_input_counts"`
	MissingOutputCounts int         `json:"missing_output_counts"`
	Requests            Concurrency `json:"requests"`
	Policy              string      `json:"policy"`
}

// MarshalJSON emits a null token rate for a slice with missing successful
// counts. Empty slices and observed zero counts retain a measured zero rate.
func (s SliceSummary) MarshalJSON() ([]byte, error) {
	if s.AttemptSelection != "" && s.AttemptSelection != "dispatch_start" {
		return nil, fmt.Errorf("unknown slice attempt selection")
	}
	type summary SliceSummary
	var input, output *float64
	if s.MissingInputCounts == 0 {
		input = &s.InputThroughput
	}
	if s.MissingOutputCounts == 0 {
		output = &s.OutputThroughput
	}
	var completed, selected *int
	if s.AttemptSelection == "dispatch_start" {
		selected = &s.Attempts
	} else {
		completed = &s.Attempts
	}
	return json.Marshal(struct {
		summary
		CompletedAttempts *int     `json:"completed_attempts,omitempty"`
		SelectedAttempts  *int     `json:"selected_attempts,omitempty"`
		InputThroughput   *float64 `json:"input_throughput"`
		OutputThroughput  *float64 `json:"output_throughput"`
	}{summary: summary(s), CompletedAttempts: completed, SelectedAttempts: selected, InputThroughput: input, OutputThroughput: output})
}

// UnmarshalJSON restores the selected attempt count and preserves unavailable
// token rates. Count names must agree with the explicit selection convention.
func (s *SliceSummary) UnmarshalJSON(data []byte) error {
	if err := jsoncheck.Check(json.NewDecoder(bytes.NewReader(data)), jsoncheck.Options{FoldNames: true}); err != nil {
		return fmt.Errorf("decode slice: %w", err)
	}
	var members map[string]json.RawMessage
	if err := json.Unmarshal(data, &members); err != nil {
		return fmt.Errorf("decode slice: %w", err)
	}
	// All schema member names are lowercase ASCII. Reject encoding/json aliases
	// at every level rather than allowing a differently spelled evidence key.
	var names any
	if err := json.Unmarshal(data, &names); err != nil {
		return err
	}
	var canonical func(any) bool
	canonical = func(value any) bool {
		switch value := value.(type) {
		case map[string]any:
			for name, child := range value {
				for _, c := range name {
					if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_') {
						return false
					}
				}
				if !canonical(child) {
					return false
				}
			}
		case []any:
			for _, child := range value {
				if !canonical(child) {
					return false
				}
			}
		}
		return true
	}
	if !canonical(names) {
		return fmt.Errorf("noncanonical slice member name")
	}
	for name, raw := range members {
		if name != "input_throughput" && name != "output_throughput" && bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return fmt.Errorf("null slice field %s", name)
		}
	}
	type summary SliceSummary
	var wire struct {
		summary
		CompletedAttempts *int     `json:"completed_attempts"`
		SelectedAttempts  *int     `json:"selected_attempts"`
		InputThroughput   *float64 `json:"input_throughput"`
		OutputThroughput  *float64 `json:"output_throughput"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		return fmt.Errorf("decode slice: %w", err)
	}
	restored := SliceSummary(wire.summary)
	switch restored.AttemptSelection {
	case "":
		if wire.CompletedAttempts == nil || wire.SelectedAttempts != nil {
			return fmt.Errorf("slice requires only completed attempts")
		}
		restored.Attempts = *wire.CompletedAttempts
	case "dispatch_start":
		if wire.SelectedAttempts == nil || wire.CompletedAttempts != nil {
			return fmt.Errorf("dispatch slice requires only selected attempts")
		}
		restored.Attempts = *wire.SelectedAttempts
	default:
		return fmt.Errorf("unknown slice attempt selection")
	}
	if restored.Attempts < 0 || restored.Successes < 0 || restored.Failures < 0 || restored.Successes > restored.Attempts || restored.Failures != restored.Attempts-restored.Successes || restored.GoodRequests < 0 || restored.GoodRequests > restored.Successes || restored.InputTokens < 0 || restored.OutputTokens < 0 {
		return fmt.Errorf("invalid slice accounting")
	}
	for _, rate := range []struct {
		name        string
		missing     int
		value       *float64
		destination *float64
	}{
		{"input_throughput", restored.MissingInputCounts, wire.InputThroughput, &restored.InputThroughput},
		{"output_throughput", restored.MissingOutputCounts, wire.OutputThroughput, &restored.OutputThroughput},
	} {
		if _, present := members[rate.name]; !present {
			return fmt.Errorf("missing slice %s", rate.name)
		}
		if rate.missing < 0 || rate.missing > restored.Successes || (rate.missing > 0) != (rate.value == nil) {
			return fmt.Errorf("contradictory slice %s availability", rate.name)
		}
		if rate.value != nil {
			if !finite(*rate.value) || *rate.value < 0 {
				return fmt.Errorf("invalid slice %s", rate.name)
			}
			*rate.destination = *rate.value
		}
	}
	*s = restored
	return nil
}

// ReportOptions selects explicit reporting conventions. Its zero value preserves
// nearest-rank percentiles and completion-based time slices.
type ReportOptions struct {
	// AIPerf uses linear (n-1)*q percentiles and dispatch-start [start,end)
	// time slices. It does not change success/failure populations or rate policy.
	AIPerf bool
}

// TimeSlices derives non-overlapping slices from a complete retained run.
// width uses seconds. The last slice ends at the declared run end.
func TimeSlices(run RunSummary, width float64) ([]SliceSummary, error) {
	return TimeSlicesWithOptions(run, width, ReportOptions{})
}

// TimeSlicesWithOptions derives slices using the selected window convention.
// AIPerf dispatch slices require recorded dispatch clock-basis evidence; a
// missing timestamp must not silently become a dispatch at Unix zero.
func TimeSlicesWithOptions(run RunSummary, width float64, opts ReportOptions) ([]SliceSummary, error) {
	if width <= 0 || math.IsNaN(width) || math.IsInf(width, 0) {
		return nil, fmt.Errorf("slice width must be positive and finite")
	}
	if _, err := AnalyzeTimeline(run); err != nil {
		return nil, err
	}
	if opts.AIPerf {
		for _, m := range run.Records {
			if m.TimestampBasis != "dispatch Unix clock anchor plus monotonic elapsed time" {
				return nil, fmt.Errorf("dispatch time unavailable for AIPerf slices")
			}
		}
	}
	if math.Ceil(run.Duration/width) > 10000 {
		return nil, fmt.Errorf("more than 10000 time slices")
	}
	var slices []SliceSummary
	for start := run.StartedAt; start < run.FinishedAt; {
		end := math.Min(run.FinishedAt, start+width*1000)
		if end <= start {
			return nil, fmt.Errorf("slice width below timestamp resolution")
		}
		s := SliceSummary{StartedAt: start, FinishedAt: end, Policy: "completion in [start,end); final run endpoint inclusive; tokens from successful completions"}
		if opts.AIPerf {
			s.Policy = "dispatch start in [start,end); final endpoint excluded; tokens from successful requests selected by dispatch start"
			s.AttemptSelection = "dispatch_start"
		}
		var intervals []interval
		for _, m := range run.Records {
			intervals = append(intervals, interval{m.DispatchedAt, m.FinishedAt})
			selected := m.FinishedAt >= start && (m.FinishedAt < end || end == run.FinishedAt && m.FinishedAt == end)
			if opts.AIPerf {
				selected = m.DispatchedAt >= start && m.DispatchedAt < end
			}
			if !selected {
				continue
			}
			s.Attempts++
			if m.Outcome == "success" {
				s.Successes++
				if m.Good {
					s.GoodRequests++
				}
				if m.InputTokens.Value != nil {
					s.InputTokens += int64(*m.InputTokens.Value)
				} else {
					s.MissingInputCounts++
				}
				if m.OutputTokens.Value != nil {
					s.OutputTokens += int64(*m.OutputTokens.Value)
				} else {
					s.MissingOutputCounts++
				}
			} else {
				s.Failures++
			}
		}
		duration := (end - start) / 1000
		if s.MissingInputCounts == 0 {
			s.InputThroughput = float64(s.InputTokens) / duration
		}
		s.Goodput = float64(s.GoodRequests) / duration
		if s.MissingOutputCounts == 0 {
			s.OutputThroughput = float64(s.OutputTokens) / duration
		}
		s.Requests = concurrency(intervals, start, end)
		slices = append(slices, s)
		start = end
	}
	return slices, nil
}

// Distribution summarizes available successful-request observations. Empty
// populations have null statistics. Quantiles use the estimator identified by RequestAnalysis.QuantilePolicy.
type Distribution struct {
	Unit    string   `json:"unit"`
	Samples int      `json:"samples"`
	Missing int      `json:"missing"`
	Mean    *float64 `json:"mean"`
	StdDev  *float64 `json:"population_stddev"`
	Min     *float64 `json:"min"`
	Max     *float64 `json:"max"`
	P50     *float64 `json:"p50"`
	P95     *float64 `json:"p95"`
	P99     *float64 `json:"p99"`
}

// RequestAnalysis separates successful distributions from terminal failures.
type RequestAnalysis struct {
	Population     string                    `json:"population"`
	CountSources   map[string]map[string]int `json:"count_sources"`
	Outcomes       map[string]int            `json:"outcomes"`
	Metrics        map[string]Distribution   `json:"metrics"`
	QuantilePolicy string                    `json:"quantile_policy"`
}

// AnalyzeRequests derives latency, count, and speed distributions. Failures stay
// in outcome counts and are excluded from success-only metrics, not encoded as zero.
func AnalyzeRequests(run RunSummary) (RequestAnalysis, error) {
	return AnalyzeRequestsWithOptions(run, ReportOptions{})
}

// AnalyzeRequestsWithOptions selects the percentile estimator without changing
// retained successful-request populations, missing values or outcome accounting.
func AnalyzeRequestsWithOptions(run RunSummary, opts ReportOptions) (RequestAnalysis, error) {
	if _, err := AnalyzeTimeline(run); err != nil {
		return RequestAnalysis{}, err
	}
	result := RequestAnalysis{Population: "successful whole-contained requests; failures only in outcomes", CountSources: map[string]map[string]int{"input": {}, "output": {}}, Outcomes: map[string]int{}, Metrics: map[string]Distribution{}, QuantilePolicy: "nearest rank ceil(p*n), population standard deviation; unavailable values excluded"}
	if opts.AIPerf {
		result.QuantilePolicy = "linear interpolation at (n-1)*q; population standard deviation; unavailable values excluded; successful retained population unchanged"
	}
	type metric struct {
		name, unit string
		value      func(Measurement) *float64
	}
	countValue := func(c TokenCount) *float64 {
		if c.Value == nil {
			return nil
		}
		v := float64(*c.Value)
		return &v
	}
	metrics := []metric{
		{"full_response_latency", "ms", func(m Measurement) *float64 { return &m.FullResponseLatency }},
		{"ttft", "ms", func(m Measurement) *float64 { return m.TTFT }},
		{"ttfo", "ms", func(m Measurement) *float64 { return m.TTFO }},
		{"last_content_latency", "ms", func(m Measurement) *float64 { return m.LastContentLatency }},
		{"admission_wait", "ms", func(m Measurement) *float64 { return &m.AdmissionWait }},
		{"scheduled_response_latency", "ms", func(m Measurement) *float64 { return m.ScheduledResponseLatency }},
		{"itl", "ms/token", func(m Measurement) *float64 { return m.ITL }},
		{"full_response_itl", "ms/token", func(m Measurement) *float64 { return m.FullResponseITL }},
		{"input_tokens", "tokens", func(m Measurement) *float64 { return countValue(m.InputTokens) }},
		{"output_tokens", "tokens", func(m Measurement) *float64 { return countValue(m.OutputTokens) }},
		{"request_token_speed", "tokens/s", func(m Measurement) *float64 {
			if m.OutputTokens.Value == nil || m.FullResponseLatency <= 0 {
				return nil
			}
			v := float64(*m.OutputTokens.Value) * 1000 / m.FullResponseLatency
			return &v
		}},
		{"decode_token_speed", "tokens/s", func(m Measurement) *float64 {
			if m.ITL == nil || *m.ITL <= 0 {
				return nil
			}
			v := 1000 / *m.ITL
			return &v
		}},
	}
	for _, m := range run.Records {
		switch m.Outcome {
		case "success", "failure", "timeout", "cancelled":
			result.Outcomes[m.Outcome]++
			if m.Outcome == "success" {
				result.CountSources["input"][m.InputTokens.Source]++
				result.CountSources["output"][m.OutputTokens.Source]++
			}
		default:
			return RequestAnalysis{}, fmt.Errorf("unknown request outcome")
		}
	}
	for _, metric := range metrics {
		var values []float64
		missing := 0
		for _, m := range run.Records {
			if m.Outcome != "success" {
				continue
			}
			v := metric.value(m)
			if v == nil {
				missing++
				continue
			}
			if !finite(*v) || *v < 0 {
				return RequestAnalysis{}, fmt.Errorf("invalid %s observation", metric.name)
			}
			values = append(values, *v)
		}
		result.Metrics[metric.name] = distributionWithOptions(values, missing, metric.unit, opts)
	}
	return result, nil
}

func distribution(values []float64, missing int, unit string) Distribution {
	return distributionWithOptions(values, missing, unit, ReportOptions{})
}

func distributionWithOptions(values []float64, missing int, unit string, opts ReportOptions) Distribution {
	d := Distribution{Unit: unit, Samples: len(values), Missing: missing}
	if len(values) == 0 {
		return d
	}
	sort.Float64s(values)
	mean := 0.0
	for _, v := range values {
		mean += v / float64(len(values))
	}
	std := 0.0
	for _, v := range values {
		delta := v - mean
		std = math.Hypot(std, delta/math.Sqrt(float64(len(values))))
	}
	q := func(p float64) *float64 {
		v := values[max(0, int(math.Ceil(p*float64(len(values))))-1)]
		if opts.AIPerf {
			index := p * float64(len(values)-1)
			lo := int(index)
			hi := min(lo+1, len(values)-1)
			v = values[lo] + (index-float64(lo))*(values[hi]-values[lo])
		}
		return &v
	}
	lo, hi := values[0], values[len(values)-1]
	d.Mean, d.StdDev, d.Min, d.Max = &mean, &std, &lo, &hi
	d.P50, d.P95, d.P99 = q(.5), q(.95), q(.99)
	return d
}

// WriteRun validates and writes a canonical versioned run export. Incomplete
// retained records remain explicitly marked and cannot support replay analyses.
func WriteRun(w io.Writer, run RunSummary) error {
	if err := validateRun(run); err != nil {
		return err
	}
	run.WorkGoodput = float64(run.WorkGoodRequests) / run.Duration
	run.Goodput = float64(run.GoodRequests) / run.Duration
	run.InputThroughput = 0
	if run.InputThroughputAvailable() {
		run.InputThroughput = float64(run.InputTokens) / run.Duration
	}
	run.OutputThroughput = 0
	if run.OutputThroughputAvailable() {
		run.OutputThroughput = float64(run.OutputTokens) / run.Duration
	}
	if run.Observation != nil {
		// validateRun matched the receipt; this completes a legacy one.
		observed, err := observationReceipt(run)
		if err != nil {
			return err
		}
		run.Observation = &observed
	}
	data, err := json.Marshal(run)
	if err != nil {
		return fmt.Errorf("encode run: %w", err)
	}
	if len(data)+1 > 64*1024*1024 {
		return fmt.Errorf("run exceeds 64 MiB")
	}
	data = append(data, '\n')
	n, err := w.Write(data)
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	if err != nil {
		return fmt.Errorf("write run: %w", err)
	}
	return nil
}

// WriteRecords writes sanitized request records as bounded version-1 JSONL.
func WriteRecords(w io.Writer, records []Measurement) error {
	if len(records) > 100000 {
		return fmt.Errorf("more than 100000 records")
	}
	var data bytes.Buffer
	for _, m := range records {
		if m.SchemaVersion != 1 {
			return fmt.Errorf("unsupported record schema")
		}
		row, err := json.Marshal(m)
		if err != nil {
			return fmt.Errorf("encode record: %w", err)
		}
		if len(row) > 1024*1024 {
			return fmt.Errorf("record exceeds 1 MiB")
		}
		if data.Len()+len(row)+1 > 64*1024*1024 {
			return fmt.Errorf("records exceed 64 MiB")
		}
		data.Write(row)
		data.WriteByte('\n')
	}
	n, err := w.Write(data.Bytes())
	if err == nil && n != data.Len() {
		err = io.ErrShortWrite
	}
	if err != nil {
		return fmt.Errorf("write records: %w", err)
	}
	return nil
}

// ReadRecords reads strict, bounded version-1 JSONL. ReconcileRecords checks
// these records against the manifest and the retained canonical record set.
func ReadRecords(r io.Reader) ([]Measurement, error) {
	data, err := io.ReadAll(io.LimitReader(r, 64*1024*1024+1))
	if err != nil {
		return nil, fmt.Errorf("read records: %w", err)
	}
	if len(data) > 64*1024*1024 {
		return nil, fmt.Errorf("records exceed 64 MiB")
	}
	var records []Measurement
	for _, row := range bytes.Split(data, []byte{'\n'}) {
		if len(bytes.TrimSpace(row)) == 0 {
			continue
		}
		if len(row) > 1024*1024 {
			return nil, fmt.Errorf("record exceeds 1 MiB")
		}
		var m Measurement
		decoder := json.NewDecoder(bytes.NewReader(row))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&m); err != nil {
			return nil, fmt.Errorf("decode record: %w", err)
		}
		var extra any
		if decoder.Decode(&extra) != io.EOF {
			return nil, fmt.Errorf("trailing record data")
		}
		if m.SchemaVersion != 1 {
			return nil, fmt.Errorf("unsupported record schema")
		}
		records = append(records, m)
		if len(records) > 100000 {
			return nil, fmt.Errorf("more than 100000 records")
		}
	}
	return records, nil
}

// ReconcileRecords requires a full-fidelity manifest and identical ordered
// records. Record IDs, units, timings and provenance must survive round trip.
func ReconcileRecords(run RunSummary, records []Measurement) error {
	if err := validateRun(run); err != nil {
		return err
	}
	if run.DroppedRecords != 0 || len(records) != run.Attempts {
		return fmt.Errorf("reconciliation requires all attempted records")
	}
	a, err := json.Marshal(run.Records)
	if err != nil {
		return err
	}
	// Empty nil and empty array represent the same empty population.
	if len(records) == 0 {
		return nil
	}
	b, err := json.Marshal(records)
	if err != nil {
		return err
	}
	if !bytes.Equal(a, b) {
		return fmt.Errorf("JSONL records disagree with retained manifest records")
	}
	return nil
}

func validateRecordTiming(m Measurement, run RunSummary) error {
	if m.RequestBodyBytes < 0 || m.ResponseBodyBytes < 0 || m.MaxConnsPerHost < 0 || m.MaxConnsPerHost > 65536 || m.TransportProfile != "" && m.TransportProfile != "shared-http2-enabled-v1" && m.TransportProfile != "k6-transport" || m.TransportProfile != "shared-http2-enabled-v1" && m.MaxConnsPerHost != 0 {
		return fmt.Errorf("invalid client transport or wire body evidence")
	}
	for _, v := range []*float64{m.SerializationLatency, m.ParsingLatency} {
		if v != nil && (!finite(*v) || *v < 0) {
			return fmt.Errorf("invalid client overhead timing")
		}
	}
	if m.ParsingLatency != nil && *m.ParsingLatency > m.FullResponseLatency+.002 {
		return fmt.Errorf("parsing timing exceeds response latency")
	}
	if err := validateWorkMeasurement(&m); err != nil {
		return err
	}
	if m.TimestampBasis != "" && m.TimestampBasis != "dispatch Unix clock anchor plus monotonic elapsed time" {
		return fmt.Errorf("unknown request timestamp basis")
	}
	if !finite(m.AdmissionWait) || m.AdmissionWait < 0 || !finite(m.RTT) || m.RTT < 0 {
		return fmt.Errorf("invalid admission or RTT observation")
	}
	for _, v := range []*float64{m.ArrivalLag, m.ScheduledResponseLatency, m.PromptDifference, m.OutputDifference} {
		if v != nil && (!finite(*v) || *v < 0) {
			return fmt.Errorf("invalid optional observation")
		}
	}
	if (m.ArrivalLag != nil || m.ScheduledResponseLatency != nil) && m.ScheduledAt == nil {
		return fmt.Errorf("scheduled observation lacks schedule anchor")
	}
	if m.ScheduledAt != nil {
		if !finite(*m.ScheduledAt) {
			return fmt.Errorf("invalid schedule timestamp")
		}
		if m.ArrivalLag != nil && math.Abs(*m.ArrivalLag-(m.DispatchedAt-*m.ScheduledAt)) > .002 {
			return fmt.Errorf("arrival lag disagrees with timestamps")
		}
		if m.ScheduledResponseLatency != nil && math.Abs(*m.ScheduledResponseLatency-(m.FinishedAt-*m.ScheduledAt)) > .001 {
			return fmt.Errorf("scheduled latency disagrees with timestamps")
		}
	}
	if m.SLO != nil && m.Outcome == "success" {
		good := true
		for _, b := range []struct {
			limit float64
			value *float64
		}{{m.SLO.MaxTTFT, m.TTFT}, {m.SLO.MaxTTFO, m.TTFO}, {m.SLO.MaxChunkLatency, m.MaxChunkLatency}, {sloLimit(m.SLO.MaxTPOT), m.ITL}, {sloLimit(m.SLO.MaxE2E), m.LastContentLatency}} {
			if b.limit > 0 && (b.value == nil || *b.value > b.limit) {
				good = false
			}
		}
		if good != m.Good {
			return fmt.Errorf("good outcome disagrees with SLO receipt")
		}
	}

	if m.TimestampBasis != "" && (!finite(m.StartedAt) || m.StartedAt > m.DispatchedAt || m.AdmissionWait > m.DispatchedAt-m.StartedAt+.002) {
		return fmt.Errorf("admission wait exceeds projected request preparation interval")
	}
	if !finite(m.DispatchedAt) || !finite(m.FinishedAt) || m.DispatchedAt < run.StartedAt || m.FinishedAt > run.FinishedAt || m.FinishedAt < m.DispatchedAt || m.Phase != "measurement" {
		return fmt.Errorf("record outside measurement window")
	}
	if math.Abs(m.FullResponseLatency-(m.FinishedAt-m.DispatchedAt)) > .002 {
		return fmt.Errorf("full response latency %.9f disagrees with timestamp span %.9f", m.FullResponseLatency, m.FinishedAt-m.DispatchedAt)
	}
	for _, v := range []*float64{m.TTFT, m.TTFO, m.LastContentLatency, m.MaxChunkLatency, m.ITL, m.FullResponseITL, m.TTST} {
		if v != nil && (!finite(*v) || *v < 0 || *v > m.FullResponseLatency+.001) {
			return fmt.Errorf("invalid optional response timing")
		}
	}
	if m.TTFT != nil && m.LastContentLatency != nil && *m.LastContentLatency < *m.TTFT {
		return fmt.Errorf("last content precedes first content")
	}
	if m.TTFO != nil && m.TTFT != nil && *m.TTFO < *m.TTFT {
		return fmt.Errorf("output precedes first content")
	}
	if m.DecodeTokens != nil && (*m.DecodeTokens < 0 || m.OutputTokens.Value != nil && *m.DecodeTokens > *m.OutputTokens.Value) {
		return fmt.Errorf("invalid decode count")
	}
	if m.ITL != nil || m.FullResponseITL != nil {
		if m.MultiChoice || m.DecodeTokens == nil || *m.DecodeTokens <= 0 || m.OutputTokens.Value == nil || *m.OutputTokens.Value < 2 || m.TTFT == nil {
			return fmt.Errorf("token-normalized timing lacks eligible operands")
		}
		if m.ITL != nil && m.LastContentLatency == nil {
			return fmt.Errorf("inter-token latency lacks last-content anchor")
		}
		if m.ITL != nil && math.Abs(*m.ITL-(*m.LastContentLatency-*m.TTFT)/float64(*m.DecodeTokens)) > .002 {
			return fmt.Errorf("inter-token latency disagrees with content span and decode count")
		}
		if m.FullResponseITL != nil && math.Abs(*m.FullResponseITL-(m.FullResponseLatency-*m.TTFT)/float64(*m.DecodeTokens)) > .002 {
			return fmt.Errorf("full-response inter-token latency disagrees with response span and decode count")
		}
	}
	if m.DecodeTokens != nil && *m.DecodeTokens > 0 {
		if m.MultiChoice || m.OutputTokens.Value == nil || *m.OutputTokens.Value < 2 {
			return fmt.Errorf("decode count lacks eligible output population")
		}
		first := 1
		switch m.FirstChunkSource {
		case "assumed_one":
			if m.FirstChunkTokens != nil {
				if *m.FirstChunkTokens > 0 && *m.FirstChunkTokens < *m.OutputTokens.Value || m.FirstChunkUnavailable != "first chunk count outside output count" || m.FirstChunkOrigin != "reported" && m.FirstChunkOrigin != "client_declared" {
					return fmt.Errorf("invalid first-chunk fallback provenance")
				}
			} else if m.FirstChunkUnavailable != "" || m.FirstChunkOrigin != "" {
				return fmt.Errorf("first-chunk fallback lacks original count")
			}
		case "reported", "client_declared":
			if m.FirstChunkTokens == nil || *m.FirstChunkTokens <= 0 || *m.FirstChunkTokens >= *m.OutputTokens.Value || m.FirstChunkUnavailable != "" || m.FirstChunkOrigin != "" && m.FirstChunkOrigin != m.FirstChunkSource {
				return fmt.Errorf("invalid exact first-chunk provenance")
			}
			first = *m.FirstChunkTokens
		default:
			return fmt.Errorf("decode count lacks first-chunk provenance")
		}
		if *m.DecodeTokens != *m.OutputTokens.Value-first {
			return fmt.Errorf("decode count disagrees with output and first-chunk counts")
		}
	}
	if m.CacheWriteTokens != nil && *m.CacheWriteTokens < 0 || m.ReasoningTokens != nil && *m.ReasoningTokens < 0 || m.CachedTokens != nil && *m.CachedTokens < 0 || m.ProviderTotalTokens != nil && *m.ProviderTotalTokens < 0 {
		return fmt.Errorf("negative auxiliary token count")
	}
	return nil
}
