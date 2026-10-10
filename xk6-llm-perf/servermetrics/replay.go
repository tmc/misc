package servermetrics

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Window declares the analysis interval and maximum permitted scrape gap.
// Units map exact sample names to seconds, milliseconds, nanoseconds, bytes,
// kilobytes, megabytes, gigabytes, ratio, percent, or an authored count unit.
type Window struct {
	Start   time.Time         `json:"start"`
	End     time.Time         `json:"end"`
	MaxGap  time.Duration     `json:"max_gap"`
	Targets []string          `json:"targets"`
	Units   map[string]string `json:"units,omitempty"`
}

// Series reports only observed intervals wholly contained in the window.
// Counter Total is the increase; gauge Mean uses trapezoidal time integration.
// Missing coverage is explicit and never extrapolated. Summary quantiles and
// untyped values retain samples but have no mathematically valid aggregation.
type Series struct {
	Target         string            `json:"target"`
	Name           string            `json:"name"`
	Labels         map[string]string `json:"labels"`
	Type           string            `json:"type"`
	Unit           string            `json:"unit"`
	UnitBasis      string            `json:"unit_basis"`
	CoveredSeconds float64           `json:"covered_seconds"`
	Total          *float64          `json:"total"`
	Rate           *float64          `json:"rate"`
	Mean           *float64          `json:"mean"`
	Issues         []string          `json:"issues,omitempty"`
}

// Analysis preserves failures and coverage independently from available series.
// Histogram contains quantiles estimated from cumulative bucket increases.
// Basis records the interpolation assumption; these are not client latencies.
type Histogram struct {
	Target string            `json:"target"`
	Name   string            `json:"name"`
	Labels map[string]string `json:"labels"`
	Unit   string            `json:"unit"`
	P50    *float64          `json:"p50"`
	P90    *float64          `json:"p90"`
	P99    *float64          `json:"p99"`
	Basis  string            `json:"basis"`
	Issue  string            `json:"issue,omitempty"`
}

type Analysis struct {
	Histograms    []Histogram `json:"histograms,omitempty"`
	SchemaVersion int         `json:"schema_version"`
	Window        Window      `json:"window"`
	Series        []Series    `json:"series"`
	Failures      int         `json:"failures"`
	Issues        []string    `json:"issues,omitempty"`
}

// Replay validates bounded JSONL against the retained raw exposition before
// aggregating. Query completion time defines intervals; exporter timestamps
// remain raw evidence and cannot silently move a reading into this window.
func Replay(r io.Reader, w Window) (Analysis, error) {
	a := Analysis{SchemaVersion: 1, Window: w}
	if !w.End.After(w.Start) || w.MaxGap <= 0 || len(w.Targets) == 0 || len(w.Targets) > 16 || len(w.Units) > 1000 || w.End.Sub(w.Start) > 24*time.Hour || w.MaxGap > 24*time.Hour {
		return a, fmt.Errorf("invalid analysis window")
	}
	expected := map[string]bool{}
	for _, t := range w.Targets {
		if !validTarget(t) || expected[t] {
			return a, fmt.Errorf("duplicate expected target")
		}
		expected[t] = true
	}
	scan := bufio.NewScanner(io.LimitReader(r, 256*1024*1024+1))
	scan.Buffer(make([]byte, 4096), 24*1024*1024)
	last := map[string]*Scrape{}
	series := map[string]*Series{}
	integrals := map[string]float64{}
	totals := map[string]float64{}
	seen := map[string]bool{}
	bytes, n := 0, 0
	for scan.Scan() {
		bytes += len(scan.Bytes()) + 1
		n++
		if bytes > 256*1024*1024 || n > 100000 {
			return a, fmt.Errorf("replay budget exceeded")
		}
		var o Observation
		d := json.NewDecoder(strings.NewReader(scan.Text()))
		d.DisallowUnknownFields()
		if err := d.Decode(&o); err != nil {
			return a, fmt.Errorf("decode observation: %w", err)
		}
		if d.Decode(new(any)) != io.EOF {
			return a, fmt.Errorf("trailing observation data")
		}
		if o.SchemaVersion != 1 || !validTarget(o.Target) {
			return a, fmt.Errorf("unexpected observation schema or target")
		}
		seen[o.Target] = true
		if !expected[o.Target] {
			a.Issues = appendUnique(a.Issues, "discovery target added: "+o.Target)
		}
		if len(o.Targets) > 0 {
			for _, target := range w.Targets {
				found := false
				for _, v := range o.Targets {
					if v == target {
						found = true
					}
				}
				if !found {
					a.Issues = appendUnique(a.Issues, "discovery target absent: "+target)
				}
			}
			for _, target := range o.Targets {
				if !expected[target] {
					a.Issues = appendUnique(a.Issues, "discovery target added: "+target)
				}
			}
		}
		if o.FailedScrape != nil {
			fs := o.FailedScrape
			h := sha256.Sum256([]byte(fs.Raw))
			if o.Error == "" || o.Scrape != nil || fs.Target != o.Target || fs.SchemaVersion != 1 || fs.Hash != hex.EncodeToString(h[:]) {
				return a, fmt.Errorf("invalid failed scrape receipt")
			}
		}
		if o.Scrape == nil {
			if o.Error == "" {
				return a, fmt.Errorf("missing scrape and failure")
			}
			a.Failures++
			delete(last, o.Target)
			continue
		}
		if o.Error != "" {
			return a, fmt.Errorf("scrape with failure")
		}
		s := o.Scrape
		h := sha256.Sum256([]byte(s.Raw))
		if s.SchemaVersion != 1 || s.Target != o.Target || s.Hash != hex.EncodeToString(h[:]) || s.FinishedAt.Before(s.StartedAt) {
			return a, fmt.Errorf("invalid scrape receipt")
		}
		parsed, err := Read(strings.NewReader(s.Raw))
		if err != nil {
			return a, err
		}
		s.Samples = parsed
		prev := last[o.Target]
		if prev != nil && !s.FinishedAt.After(prev.FinishedAt) {
			return a, fmt.Errorf("scrapes not ordered")
		}
		last[o.Target] = s
		if prev == nil {
			continue
		}
		if prev.FinishedAt.Before(w.Start) || s.FinishedAt.After(w.End) {
			continue
		}
		dt := s.FinishedAt.Sub(prev.FinishedAt)
		old := map[string]Sample{}
		for _, p := range prev.Samples {
			old[identity(p.Name, p.Labels)] = p
		}
		current := map[string]bool{}
		for _, p := range parsed {
			id := identity(p.Name, p.Labels)
			current[id] = true
			k := o.Target + "\n" + id
			q := series[k]
			if q == nil {
				unit, factor, basis, err := normalizeUnit(p.Name, w.Units)
				_ = factor
				if err != nil {
					return a, err
				}
				q = &Series{Target: o.Target, Name: p.Name, Labels: p.Labels, Type: p.Type, Unit: unit, UnitBasis: basis}
				series[k] = q
			}
			b, ok := old[id]
			if !ok {
				q.Issues = appendUnique(q.Issues, "label identity appeared")
				continue
			}
			if dt > w.MaxGap {
				q.Issues = appendUnique(q.Issues, "cadence gap")
				continue
			}
			if b.Type != p.Type {
				q.Issues = appendUnique(q.Issues, "type changed")
				continue
			}
			if b.Value == nil || p.Value == nil {
				q.Issues = appendUnique(q.Issues, "unavailable sample")
				continue
			}
			_, factor, _, _ := normalizeUnit(p.Name, w.Units)
			if p.Type == "histogram" && (strings.HasSuffix(p.Name, "_bucket") || strings.HasSuffix(p.Name, "_count")) {
				factor = 1
				q.Unit = "count"
			}
			counter := p.Type == "counter" || p.Type == "histogram" && (strings.HasSuffix(p.Name, "_bucket") || strings.HasSuffix(p.Name, "_count") || strings.HasSuffix(p.Name, "_sum"))
			if counter {
				delta := *p.Value - *b.Value
				if *b.Value < 0 || delta < 0 {
					q.Issues = appendUnique(q.Issues, "counter reset")
					continue
				}
				totals[k] += delta * factor
				q.CoveredSeconds += dt.Seconds()
			} else if p.Type == "gauge" {
				integrals[k] += (*p.Value/2 + *b.Value/2) * factor * dt.Seconds()
				q.CoveredSeconds += dt.Seconds()
			} else {
				q.Issues = appendUnique(q.Issues, "no aggregation for summary or untyped sample")
			}
		}
		for id, p := range old {
			if !current[id] {
				a.Issues = appendUnique(a.Issues, "label identity disappeared: "+o.Target+" "+p.Name)
			}
		}
	}
	if err := scan.Err(); err != nil {
		return a, fmt.Errorf("read replay: %w", err)
	}
	for t := range expected {
		if !seen[t] {
			a.Issues = appendUnique(a.Issues, "missing target: "+t)
		}
	}
	keys := make([]string, 0, len(series))
	for k := range series {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		q := series[k]
		if q.CoveredSeconds > 0 {
			if q.Type == "gauge" {
				v := integrals[k] / q.CoveredSeconds
				q.Mean = &v
			} else {
				v := totals[k]
				rate := v / q.CoveredSeconds
				q.Total = &v
				q.Rate = &rate
			}
			if q.CoveredSeconds < w.End.Sub(w.Start).Seconds() {
				q.Issues = appendUnique(q.Issues, "incomplete window coverage")
			}
		}
		for _, v := range []*float64{q.Total, q.Rate, q.Mean} {
			if v != nil && (math.IsNaN(*v) || math.IsInf(*v, 0)) {
				return a, fmt.Errorf("aggregate overflow")
			}
		}
		a.Series = append(a.Series, *q)
	}
	a.Histograms = summarizeHistograms(a.Series, w.Units)
	return a, nil
}

func appendUnique(s []string, v string) []string {
	for _, x := range s {
		if x == v {
			return s
		}
	}
	return append(s, v)
}
func normalizeUnit(name string, units map[string]string) (string, float64, string, error) {
	u, ok := units[name]
	basis := "configured"
	if !ok {
		basis = "name suffix"
		base := name
		for _, s := range []string{"_bucket", "_sum", "_count", "_total"} {
			base = strings.TrimSuffix(base, s)
		}
		for _, s := range []string{"nanoseconds", "milliseconds", "seconds", "gigabytes", "megabytes", "kilobytes", "bytes", "percent", "ratio", "tokens", "requests", "joules", "watts", "celsius", "ms", "ns"} {
			if strings.HasSuffix(base, "_"+s) {
				u = s
				break
			}
		}
		if u == "" {
			return "unknown", 1, "unknown", nil
		}
	}
	switch u {
	case "seconds":
		return "seconds", 1, basis, nil
	case "milliseconds", "ms":
		return "seconds", 0.001, basis, nil
	case "nanoseconds", "ns":
		return "seconds", 1e-9, basis, nil
	case "bytes":
		return "bytes", 1, basis, nil
	case "kilobytes":
		return "bytes", 1000, basis, nil
	case "megabytes":
		return "bytes", 1e6, basis, nil
	case "gigabytes":
		return "bytes", 1e9, basis, nil
	case "percent":
		return "ratio", 0.01, basis, nil
	case "ratio", "tokens", "requests", "count", "errors", "joules", "watts", "celsius":
		return u, 1, basis, nil
	}
	return "", 0, "", fmt.Errorf("unsupported configured unit %q", u)
}

// HistogramQuantile estimates a quantile from cumulative window bucket deltas.
// It interpolates uniformly within the containing bucket, not exact observations.
// An infinite terminal bucket returns its finite lower bound. Empty counts are
// unavailable. Summary quantiles must not be passed as histogram buckets.
func HistogramQuantile(bounds, counts []float64, q float64) (*float64, error) {
	if len(bounds) < 2 || len(bounds) != len(counts) || math.IsNaN(q) || q < 0 || q > 1 {
		return nil, fmt.Errorf("invalid histogram quantile")
	}
	for i := range bounds {
		if math.IsNaN(bounds[i]) || math.IsInf(bounds[i], -1) || math.IsNaN(counts[i]) || math.IsInf(counts[i], 0) || counts[i] < 0 || i > 0 && (bounds[i] <= bounds[i-1] || counts[i] < counts[i-1]) {
			return nil, fmt.Errorf("invalid cumulative histogram")
		}
	}
	if !math.IsInf(bounds[len(bounds)-1], 1) {
		return nil, fmt.Errorf("missing infinite histogram bucket")
	}
	total := counts[len(counts)-1]
	if total == 0 {
		return nil, nil
	}
	rank := q * total
	lower, count := 0.0, 0.0
	for i, b := range bounds {
		if rank <= counts[i] {
			if math.IsInf(b, 1) {
				return &lower, nil
			}
			if i == 0 && b <= 0 {
				return &b, nil
			}
			if counts[i] == count {
				return &b, nil
			}
			v := lower + (b-lower)*(rank-count)/(counts[i]-count)
			return &v, nil
		}
		lower = b
		count = counts[i]
	}
	return nil, nil
}

func summarizeHistograms(series []Series, units map[string]string) []Histogram {
	groups := map[string][]Series{}
	for _, s := range series {
		if s.Type == "histogram" && strings.HasSuffix(s.Name, "_bucket") {
			labels := map[string]string{}
			for k, v := range s.Labels {
				if k != "le" {
					labels[k] = v
				}
			}
			key := s.Target + "\n" + identity(strings.TrimSuffix(s.Name, "_bucket"), labels)
			groups[key] = append(groups[key], s)
		}
	}
	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []Histogram
	for _, k := range keys {
		ss := groups[k]
		sort.Slice(ss, func(i, j int) bool {
			a, _ := strconv.ParseFloat(ss[i].Labels["le"], 64)
			b, _ := strconv.ParseFloat(ss[j].Labels["le"], 64)
			return a < b
		})
		name := strings.TrimSuffix(ss[0].Name, "_bucket")
		u, f, _, _ := normalizeUnit(name, units)
		h := Histogram{Target: ss[0].Target, Name: name, Labels: map[string]string{}, Unit: u, Basis: "linear interpolation within cumulative window bucket deltas; infinite bucket uses finite lower bound"}
		for k, v := range ss[0].Labels {
			if k != "le" {
				h.Labels[k] = v
			}
		}
		var bounds, counts []float64
		for _, s := range ss {
			b, e := strconv.ParseFloat(s.Labels["le"], 64)
			if e != nil || s.Total == nil || s.CoveredSeconds != ss[0].CoveredSeconds || len(s.Issues) > 0 {
				h.Issue = "incomplete or incompatible bucket intervals"
				break
			}
			bounds = append(bounds, b*f)
			counts = append(counts, *s.Total)
		}
		if h.Issue == "" {
			var e error
			h.P50, e = HistogramQuantile(bounds, counts, 0.5)
			if e != nil {
				h.Issue = e.Error()
			} else {
				h.P90, _ = HistogramQuantile(bounds, counts, 0.9)
				h.P99, _ = HistogramQuantile(bounds, counts, 0.99)
			}
		}
		out = append(out, h)
	}
	return out
}
