package llmperf

import (
	"bytes"
	"encoding/json"
	"fmt"
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
		"input_throughput":  {run.InputThroughput, float64(run.InputTokens) / run.Duration},
		"output_throughput": {run.OutputThroughput, float64(run.OutputTokens) / run.Duration},
	} {
		if _, present := fields[name]; present && math.Abs(check.actual-check.expected) > 1e-9*math.Max(1, math.Abs(check.expected)) {
			return run, fmt.Errorf("%s disagrees with aggregate accounting", name)
		}
	}
	return run, nil
}

func validateRun(run RunSummary) error {
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
		timeline, err := AnalyzeTimeline(run)
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
		var input, output int64
		seen := map[string]bool{}
		for _, m := range run.Records {
			if m.SchemaVersion != 1 || !m.Recorded || !m.Attempted || !finite(m.FullResponseLatency) || m.FullResponseLatency < 0 {
				return fmt.Errorf("invalid retained record")
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
		if run.DroppedRecords == 0 && (successes != run.Successes || failures != run.Failures || good != run.GoodRequests || input != run.InputTokens || output != run.OutputTokens) {
			return fmt.Errorf("records disagree with aggregate accounting")
		}
	}
	return nil
}

// SliceSummary counts completions in a half-open time interval. Requests may
// start before the slice; concurrency is clipped at its boundaries.
type SliceSummary struct {
	StartedAt        float64     `json:"started_at"`
	FinishedAt       float64     `json:"finished_at"`
	Attempts         int         `json:"completed_attempts"`
	Successes        int         `json:"successes"`
	Failures         int         `json:"failures"`
	GoodRequests     int         `json:"good_requests"`
	InputTokens      int64       `json:"input_tokens"`
	InputThroughput  float64     `json:"input_throughput"`
	OutputTokens     int64       `json:"output_tokens"`
	Goodput          float64     `json:"goodput"`
	OutputThroughput float64     `json:"output_throughput"`
	Requests         Concurrency `json:"requests"`
	Policy           string      `json:"policy"`
}

// TimeSlices derives non-overlapping slices from a complete retained run.
// width uses seconds. The last slice ends at the declared run end.
func TimeSlices(run RunSummary, width float64) ([]SliceSummary, error) {
	if width <= 0 || math.IsNaN(width) || math.IsInf(width, 0) {
		return nil, fmt.Errorf("slice width must be positive and finite")
	}
	if _, err := AnalyzeTimeline(run); err != nil {
		return nil, err
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
		var intervals []interval
		for _, m := range run.Records {
			intervals = append(intervals, interval{m.DispatchedAt, m.FinishedAt})
			if m.FinishedAt < start || m.FinishedAt >= end && !(end == run.FinishedAt && m.FinishedAt == end) {
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
				}
				if m.OutputTokens.Value != nil {
					s.OutputTokens += int64(*m.OutputTokens.Value)
				}
			} else {
				s.Failures++
			}
		}
		duration := (end - start) / 1000
		s.InputThroughput = float64(s.InputTokens) / duration
		s.Goodput = float64(s.GoodRequests) / duration
		s.OutputThroughput = float64(s.OutputTokens) / duration
		s.Requests = concurrency(intervals, start, end)
		slices = append(slices, s)
		start = end
	}
	return slices, nil
}

// Distribution summarizes available successful-request observations. Empty
// populations have null statistics. Quantiles use nearest rank without interpolation.
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
	if _, err := AnalyzeTimeline(run); err != nil {
		return RequestAnalysis{}, err
	}
	result := RequestAnalysis{Population: "successful whole-contained requests; failures only in outcomes", CountSources: map[string]map[string]int{"input": {}, "output": {}}, Outcomes: map[string]int{}, Metrics: map[string]Distribution{}, QuantilePolicy: "nearest rank ceil(p*n), population standard deviation; unavailable values excluded"}
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
		result.Metrics[metric.name] = distribution(values, missing, metric.unit)
	}
	return result, nil
}

func distribution(values []float64, missing int, unit string) Distribution {
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
	q := func(p float64) *float64 { v := values[max(0, int(math.Ceil(p*float64(len(values))))-1)]; return &v }
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
	run.Goodput = float64(run.GoodRequests) / run.Duration
	run.InputThroughput = float64(run.InputTokens) / run.Duration
	run.OutputThroughput = float64(run.OutputTokens) / run.Duration
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
