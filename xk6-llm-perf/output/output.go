// Package output renders sanitized benchmark exports and sends explicit sink requests.
package output

import (
	"bytes"
	"encoding/csv"
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
	Name  string   `json:"name"`
	Unit  string   `json:"unit"`
	Count int      `json:"count"`
	Mean  *float64 `json:"mean"`
	Min   *float64 `json:"min"`
	Max   *float64 `json:"max"`
	P50   *float64 `json:"p50"`
	P90   *float64 `json:"p90"`
	P99   *float64 `json:"p99"`
	Std   *float64 `json:"std"`
}

// Document is a versioned, sanitized summary. It contains no authored text or sink credentials.
type Document struct {
	Workload       string            `json:"workload"`
	TraceHash      string            `json:"trace_hash"`
	Seed           int64             `json:"seed"`
	Duration       float64           `json:"duration_seconds"`
	WindowPolicy   string            `json:"window_policy"`
	WarmupPolicy   string            `json:"warmup_policy"`
	SchemaVersion  int               `json:"schema_version"`
	RunID          string            `json:"run_id"`
	SourceVersions map[string]string `json:"source_versions"`
	Attempts       int               `json:"attempts"`
	Successes      int               `json:"successes"`
	Failures       int               `json:"failures"`
	DroppedRecords int               `json:"dropped_records"`
	Metrics        []Statistic       `json:"metrics"`
	Policy         string            `json:"policy"`
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
	d := Document{SchemaVersion: 1, RunID: run.RunID, SourceVersions: run.Versions, Attempts: run.Attempts, Successes: run.Successes, Failures: run.Failures, DroppedRecords: run.DroppedRecords, Workload: run.Workload, TraceHash: run.TraceHash, Seed: run.Seed, Duration: run.Duration, WindowPolicy: run.WindowPolicy, WarmupPolicy: run.WarmupPolicy, Policy: "successful retained requests; nearest-rank percentiles; population standard deviation; absent values omitted"}
	for _, m := range []struct {
		name, unit string
		value      func(llm.Measurement) *float64
	}{
		{"request_latency", "ms", func(m llm.Measurement) *float64 { return &m.FullResponseLatency }},
		{"ttft", "ms", func(m llm.Measurement) *float64 { return m.TTFT }},
		{"ttfo", "ms", func(m llm.Measurement) *float64 { return m.TTFO }},
		{"itl", "ms", func(m llm.Measurement) *float64 { return m.ITL }},
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
	} {
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
		d.Metrics = append(d.Metrics, stats(m.name, m.unit, values))
	}
	for _, s := range []struct {
		name, unit string
		v          float64
	}{{"goodput", "request/s", run.Goodput}, {"input_throughput", "token/s", run.InputThroughput}, {"output_throughput", "token/s", run.OutputThroughput}} {
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
	if err := c.Write([]string{"metric", "unit", "count", "mean", "min", "max", "p50", "p90", "p99", "std"}); err != nil {
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
}}).Parse(`<!doctype html><html><meta charset="utf-8"><title>Benchmark {{.RunID}}</title><style>body{font:16px system-ui;max-width:1100px;margin:2em auto}td,th{text-align:left;padding:.5em}svg{width:150px;height:16px}rect{fill:#3569bd}</style><h1>Benchmark {{.RunID}}</h1><p>{{.Attempts}} attempts; {{.Successes}} successes; {{.Failures}} failures; {{.DroppedRecords}} records dropped.</p><p>{{.Policy}}</p><table><tr><th>Metric</th><th>Unit</th><th>n</th><th>Mean</th><th>p50</th><th>p90</th><th>p99</th><th>Mean / max</th></tr>{{range .Metrics}}<tr><td>{{.Name}}</td><td>{{.Unit}}</td><td>{{.Count}}</td><td>{{value .Mean}}</td><td>{{value .P50}}</td><td>{{value .P90}}</td><td>{{value .P99}}</td><td><svg viewBox="0 0 100 16" role="img" aria-label="mean relative to observed maximum"><rect width="{{width .}}" height="16"/></svg></td></tr>{{end}}</table></html>`))

// HTML writes a standalone, escaped table and SVG visualization without scripts or network resources.
func HTML(w io.Writer, d Document) error { return page.Execute(w, d) }
