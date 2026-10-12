// Package output renders sanitized benchmark exports and sends explicit sink requests.
package output

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"math"
	"sort"
	"strconv"

	llm "github.com/tmc/misc/xk6-llm-perf"
)

// Statistic preserves an unavailable metric as null, rather than zero.
type Statistic struct {
	Name    string         `json:"name"`
	Sources map[string]int `json:"sources,omitempty"`
	Unit    string         `json:"unit"`
	Count   int            `json:"count"`
	Mean    *float64       `json:"mean"`
	Min     *float64       `json:"min"`
	Max     *float64       `json:"max"`
	P50     *float64       `json:"p50"`
	P90     *float64       `json:"p90"`
	P99     *float64       `json:"p99"`
	Std     *float64       `json:"std"`
}

// ServingReceipt retains manifest bindings without copying arbitrary server
// setting strings into a sanitized export. A work hash is declaration identity,
// not proof of server attestation or physical qualification.
type ServingReceipt struct {
	ManifestDigest  string `json:"manifest_digest"`
	WorkHash        string `json:"work_hash,omitempty"`
	WorkUnavailable string `json:"work_unavailable,omitempty"`
}

// Document is a versioned, sanitized summary. It contains no authored text or sink credentials.
type Document struct {
	WorkQualified                      int               `json:"work_qualified,omitempty"`
	WorkAccepted                       int               `json:"work_accepted,omitempty"`
	WorkRejected                       int               `json:"work_rejected,omitempty"`
	WorkUnavailable                    int               `json:"work_unavailable,omitempty"`
	WorkGoodRequests                   int               `json:"work_good_requests,omitempty"`
	WorkPolicy                         string            `json:"work_policy,omitempty"`
	ServingManifest                    *ServingReceipt   `json:"serving_manifest,omitempty"`
	MeasurementPolicy                  string            `json:"measurement_policy,omitempty"`
	ContentIntervalPolicy              string            `json:"content_interval_policy"`
	ContentIntervalUnavailableRequests int               `json:"content_interval_unavailable_requests"`
	ContentIntervalTruncatedRequests   int               `json:"content_interval_truncated_requests"`
	TTSTPolicy                         string            `json:"ttst_policy"`
	PercentileMethod                   string            `json:"percentile_method"`
	StandardDeviationMethod            string            `json:"standard_deviation_method"`
	RetainedRecords                    int               `json:"retained_records"`
	RetainedSuccesses                  int               `json:"retained_successes"`
	ExcludedBoundaryRequests           int               `json:"excluded_boundary_requests"`
	InflightAtEnd                      int               `json:"inflight_at_end"`
	MissingInputCounts                 int               `json:"missing_input_counts"`
	MissingOutputCounts                int               `json:"missing_output_counts"`
	EstimatedInputCounts               int               `json:"estimated_input_counts"`
	EstimatedOutputCounts              int               `json:"estimated_output_counts"`
	Timeline                           *llm.Timeline     `json:"timeline"`
	TimelineUnavailable                string            `json:"timeline_unavailable,omitempty"`
	Workload                           string            `json:"workload"`
	TraceHash                          string            `json:"trace_hash"`
	Seed                               int64             `json:"seed"`
	Duration                           float64           `json:"duration_seconds"`
	WindowPolicy                       string            `json:"window_policy"`
	WarmupPolicy                       string            `json:"warmup_policy"`
	SchemaVersion                      int               `json:"schema_version"`
	RunID                              string            `json:"run_id"`
	SourceVersions                     map[string]string `json:"source_versions"`
	Attempts                           int               `json:"attempts"`
	Successes                          int               `json:"successes"`
	Failures                           int               `json:"failures"`
	DroppedRecords                     int               `json:"dropped_records"`
	Metrics                            []Statistic       `json:"metrics"`
	Policy                             string            `json:"policy"`
}

// Summarize validates a run before deriving successful-request statistics.
// Missing observations are omitted individually; Count gives the denominator.
func Summarize(run llm.RunSummary) (Document, error) {
	data, err := json.Marshal(run)
	if err != nil {
		return Document{}, err
	}
	checked, err := llm.ReadRun(bytes.NewReader(data))
	if err != nil {
		return Document{}, err
	}
	run = checked
	d := Document{SchemaVersion: 2, RunID: run.RunID, SourceVersions: run.Versions, Attempts: run.Attempts, Successes: run.Successes, Failures: run.Failures, DroppedRecords: run.DroppedRecords, Workload: run.Workload, TraceHash: run.TraceHash, Seed: run.Seed, Duration: run.Duration, WindowPolicy: run.WindowPolicy, WarmupPolicy: run.WarmupPolicy, Policy: "request statistics: successful retained requests; rates: aggregate counts divided by declared run window; nearest-rank percentiles; population standard deviation; absent values omitted"}
	if run.ServingManifest != nil {
		data, err := json.Marshal(run.ServingManifest)
		if err != nil {
			return Document{}, err
		}
		digest := sha256.Sum256(data)
		receipt := &ServingReceipt{ManifestDigest: hex.EncodeToString(digest[:])}
		receipt.WorkHash, err = llm.ServingWorkHash(*run.ServingManifest)
		if err != nil {
			receipt.WorkUnavailable = err.Error()
		}
		d.ServingManifest = receipt
	}
	d.WorkQualified, d.WorkAccepted, d.WorkRejected, d.WorkUnavailable, d.WorkGoodRequests = run.WorkQualified, run.WorkAccepted, run.WorkRejected, run.WorkUnavailable, run.WorkGoodRequests
	if run.WorkQualified > 0 {
		d.WorkPolicy = "opted-in attempted requests; accepted requires protocol success and all declared work checks; work goodput counts accepted and SLO-good requests per declared run window"
		d.Policy += "; " + d.WorkPolicy
		v := run.WorkGoodput
		d.Metrics = append(d.Metrics, Statistic{Name: "work_goodput", Unit: "request/s", Count: 1, Mean: &v})
	}
	d.MeasurementPolicy = run.MeasurementPolicy
	d.PercentileMethod = "nearest_rank"
	d.TTSTPolicy = "second recognized content event; not a physical second-token observation"
	d.ContentIntervalPolicy = "pooled retained successful recognized-content intervals; truncated and unavailable request vectors excluded; not physical token intervals"
	var intervals []float64
	for _, record := range run.Records {
		if record.Outcome == "success" {
			if record.ContentIntervalsTruncated {
				d.ContentIntervalTruncatedRequests++
			} else if record.ContentIntervals == nil {
				d.ContentIntervalUnavailableRequests++
			} else {
				intervals = append(intervals, record.ContentIntervals...)
			}
		}
	}
	d.StandardDeviationMethod = "population"
	d.RetainedRecords = len(run.Records)
	d.ExcludedBoundaryRequests = run.ExcludedBoundaryRequests
	d.InflightAtEnd = run.InflightAtEnd
	d.MissingInputCounts, d.MissingOutputCounts = run.MissingInputCounts, run.MissingOutputCounts
	d.EstimatedInputCounts, d.EstimatedOutputCounts = run.EstimatedInputCounts, run.EstimatedOutputCounts
	d.TimelineUnavailable = run.TimelineUnavailable
	if run.DroppedRecords == 0 {
		var timeline llm.Timeline
		timeline, err = llm.AnalyzeTimeline(run)
		d.Timeline = &timeline
		if err != nil {
			return Document{}, err
		}
	} else if d.TimelineUnavailable == "" {
		d.TimelineUnavailable = "dropped records"
	}
	for _, r := range run.Records {
		if r.Outcome == "success" {
			d.RetainedSuccesses++
		}
	}
	definitions := []struct {
		name, unit string
		value      func(llm.Measurement) *float64
	}{
		{"request_latency", "ms", func(m llm.Measurement) *float64 { return &m.FullResponseLatency }},
		{"ttft", "ms", func(m llm.Measurement) *float64 { return m.TTFT }},
		{"ttfo", "ms", func(m llm.Measurement) *float64 { return m.TTFO }},
		{"itl", "ms/token", func(m llm.Measurement) *float64 { return m.ITL }},
		{"input_tokens", "token", func(m llm.Measurement) *float64 {
			if m.InputTokens.Value == nil {
				return nil
			}
			v := float64(*m.InputTokens.Value)
			return &v
		}},
		{"output_tokens", "token", func(m llm.Measurement) *float64 {
			if m.OutputTokens.Value == nil {
				return nil
			}
			v := float64(*m.OutputTokens.Value)
			return &v
		}},
	}
	for _, field := range timingFields() {
		definitions = append(definitions, struct {
			name, unit string
			value      func(llm.Measurement) *float64
		}{field.name, timingUnit(field.name), field.value})
	}
	for _, m := range definitions {
		var values []float64
		for _, r := range run.Records {
			if r.Outcome == "success" {
				if v := m.value(r); v != nil {
					if math.IsNaN(*v) || math.IsInf(*v, 0) || *v < 0 {
						return Document{}, fmt.Errorf("invalid %s observation", m.name)
					}
					values = append(values, *v)
				}
			}
		}
		stat := stats(m.name, m.unit, values)
		if m.name == "input_tokens" || m.name == "output_tokens" {
			stat.Sources = make(map[string]int)
			for _, r := range run.Records {
				if r.Outcome == "success" {
					c := r.InputTokens
					if m.name == "output_tokens" {
						c = r.OutputTokens
					}
					stat.Sources[c.Source]++
				}
			}
		}
		d.Metrics = append(d.Metrics, stat)
	}
	d.Metrics = append(d.Metrics, stats("content_inter_chunk_latency", "ms", intervals))
	for _, s := range []struct {
		name, unit string
		v          float64
	}{{"goodput", "request/s", run.Goodput}, {"input_throughput", "token/s", run.InputThroughput}, {"output_throughput", "token/s", run.OutputThroughput}} {
		if s.name == "input_throughput" && !run.InputThroughputAvailable() || s.name == "output_throughput" && !run.OutputThroughputAvailable() {
			d.Metrics = append(d.Metrics, Statistic{Name: s.name, Unit: s.unit})
			continue
		}
		v := s.v
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
			return Document{}, fmt.Errorf("invalid rate")
		}
		d.Metrics = append(d.Metrics, Statistic{Name: s.name, Unit: s.unit, Count: 1, Mean: &v})
	}
	return d, nil
}
func stats(name, unit string, v []float64) Statistic {
	s := Statistic{Name: name, Unit: unit, Count: len(v)}
	if len(v) == 0 {
		return s
	}
	sort.Float64s(v)
	mean := 0.
	for i, x := range v {
		mean += (x - mean) / float64(i+1)
	}
	// Scale deviations before squaring to avoid overflow for finite inputs.
	scale := v[len(v)-1]
	variance := 0.
	if scale > 0 {
		for _, x := range v {
			delta := (x - mean) / scale
			variance += delta * delta / float64(len(v))
		}
	}
	std := scale * math.Sqrt(variance)
	q := func(p float64) *float64 { x := v[int(math.Ceil(p*float64(len(v))))-1]; return &x }
	s.Mean = &mean
	s.Min = &v[0]
	s.Max = &v[len(v)-1]
	s.P50 = q(.5)
	s.P90 = q(.9)
	s.P99 = q(.99)
	s.Std = &std
	return s
}

// CSV writes one metric per row, with empty cells for unavailable statistics.
func CSV(w io.Writer, d Document) error {
	c := csv.NewWriter(w)
	if err := c.Write([]string{"metric", "unit", "count", "mean", "min", "max", "p50", "p90", "p99", "std", "percentile_method", "standard_deviation_method", "population", "sources"}); err != nil {
		return err
	}
	for _, s := range d.Metrics {
		r := []string{s.Name, s.Unit, strconv.Itoa(s.Count)}
		for _, v := range []*float64{s.Mean, s.Min, s.Max, s.P50, s.P90, s.P99, s.Std} {
			if v == nil {
				r = append(r, "")
			} else {
				r = append(r, strconv.FormatFloat(*v, 'g', -1, 64))
			}
		}
		sources, err := json.Marshal(s.Sources)
		if err != nil {
			return err
		}
		policy := d.Policy
		if s.Name == "content_inter_chunk_latency" {
			policy = d.ContentIntervalPolicy
		}
		r = append(r, d.PercentileMethod, d.StandardDeviationMethod, policy, string(sources))
		if err := c.Write(r); err != nil {
			return err
		}
	}
	c.Flush()
	return c.Error()
}

var page = template.Must(template.New("plot").Funcs(template.FuncMap{"value": func(v *float64) string {
	if v == nil {
		return "N/A"
	}
	return strconv.FormatFloat(*v, 'g', 6, 64)
}, "width": func(s Statistic) string {
	if s.Mean == nil || s.Max == nil || *s.Max <= 0 {
		return "0"
	}
	return strconv.FormatFloat(100**s.Mean / *s.Max, 'f', 2, 64)
}}).Parse(`<!doctype html><html><meta charset="utf-8"><title>Benchmark {{.RunID}}</title><style>body{font:16px system-ui;max-width:1100px;margin:2em auto}td,th{text-align:left;padding:.5em}svg{width:150px;height:16px}rect{fill:#3569bd}</style><h1>Benchmark {{.RunID}}</h1><p>{{.Attempts}} attempts; {{.Successes}} successes; {{.Failures}} failures; {{.DroppedRecords}} records dropped; {{.RetainedSuccesses}} retained successes; {{.ExcludedBoundaryRequests}} boundary exclusions; {{.InflightAtEnd}} in flight at window end.</p><p>{{.Policy}}</p><p>{{.TTSTPolicy}}</p><p>{{.ContentIntervalPolicy}}; {{.ContentIntervalUnavailableRequests}} unavailable and {{.ContentIntervalTruncatedRequests}} truncated request vectors.</p><table><tr><th>Metric</th><th>Unit</th><th>n</th><th>Mean</th><th>p50</th><th>p90</th><th>p99</th><th>Mean / max</th></tr>{{range .Metrics}}<tr><td>{{.Name}}</td><td>{{.Unit}}</td><td>{{.Count}}</td><td>{{value .Mean}}</td><td>{{value .P50}}</td><td>{{value .P90}}</td><td>{{value .P99}}</td><td><svg viewBox="0 0 100 16" role="img" aria-label="mean relative to observed maximum"><rect width="{{width .}}" height="16"/></svg></td></tr>{{end}}</table></html>`))

// HTML writes a standalone, escaped table and SVG visualization without scripts or network resources.
func HTML(w io.Writer, d Document) error { return page.Execute(w, d) }

// timingFields names canonical durations independently of legacy aliases.
func timingFields() []struct {
	name  string
	value func(llm.Measurement) *float64
} {
	fields := []struct {
		name  string
		value func(llm.Measurement) *float64
	}{
		{"full_response_latency", func(m llm.Measurement) *float64 { return &m.FullResponseLatency }},
		{"ttst", func(m llm.Measurement) *float64 { return m.TTST }},
		{"last_content_latency", func(m llm.Measurement) *float64 { return m.LastContentLatency }},
		{"full_response_itl", func(m llm.Measurement) *float64 { return m.FullResponseITL }},
		{"max_chunk_latency", func(m llm.Measurement) *float64 { return m.MaxChunkLatency }},
		{"admission_wait", func(m llm.Measurement) *float64 { return &m.AdmissionWait }},
		{"arrival_lag", func(m llm.Measurement) *float64 { return m.ArrivalLag }},
		{"scheduled_response_latency", func(m llm.Measurement) *float64 { return m.ScheduledResponseLatency }},
	}
	for _, phase := range []string{"dns", "connect", "tls", "sending", "waiting", "receiving"} {
		fields = append(fields, struct {
			name  string
			value func(llm.Measurement) *float64
		}{"http_" + phase, func(m llm.Measurement) *float64 {
			if m.HTTPTrace == nil {
				return nil
			}
			switch phase {
			case "dns":
				return m.HTTPTrace.DNS
			case "connect":
				return m.HTTPTrace.Connect
			case "tls":
				return m.HTTPTrace.TLS
			case "sending":
				return m.HTTPTrace.Sending
			case "waiting":
				return m.HTTPTrace.Waiting
			default:
				return m.HTTPTrace.Receiving
			}
		}})
	}
	return fields
}

func timingUnit(name string) string {
	if name == "itl" || name == "full_response_itl" {
		return "ms/token"
	}
	return "ms"
}
