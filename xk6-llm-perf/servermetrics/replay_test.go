package servermetrics

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"
)

func recording(raw []string) string {
	var b bytes.Buffer
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i, r := range raw {
		h := sha256.Sum256([]byte(r))
		s := Scrape{SchemaVersion: 1, Target: "http://host/metrics", StartedAt: start.Add(time.Duration(i) * time.Second), FinishedAt: start.Add(time.Duration(i) * time.Second), Raw: r, Hash: hex.EncodeToString(h[:])}
		json.NewEncoder(&b).Encode(Observation{SchemaVersion: 1, Target: s.Target, Scrape: &s})
	}
	return b.String()
}
func TestReplay(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	w := Window{Start: start, End: start.Add(2 * time.Second), MaxGap: time.Second, Targets: []string{"http://host/metrics"}}
	a, err := Replay(strings.NewReader(recording([]string{"# TYPE done_tokens_total counter\ndone_tokens_total 10\n# TYPE latency_ms gauge\nlatency_ms 100\n", "# TYPE done_tokens_total counter\ndone_tokens_total 14\n# TYPE latency_ms gauge\nlatency_ms 200\n", "# TYPE done_tokens_total counter\ndone_tokens_total 20\n# TYPE latency_ms gauge\nlatency_ms 300\n"})), w)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Series) != 2 || *a.Series[0].Total != 10 || *a.Series[0].Rate != 5 || math.Abs(*a.Series[1].Mean-0.2) > 1e-12 {
		t.Fatalf("%+v", a.Series)
	}
	w.Units = map[string]string{"latency_ms": "milliseconds"}
	a, err = Replay(strings.NewReader(recording([]string{"# TYPE latency_ms gauge\nlatency_ms 100\n", "# TYPE latency_ms gauge\nlatency_ms 300\n"})), w)
	if err != nil || *a.Series[0].Mean != 0.2 {
		t.Fatalf("%+v %v", a, err)
	}
	for _, tc := range []struct {
		name  string
		raw   []string
		issue string
	}{{"reset", []string{"# TYPE n counter\nn 10\n", "# TYPE n counter\nn 2\n"}, "counter reset"}, {"identity", []string{"# TYPE n counter\nn{x=\"a\"} 10\n", "# TYPE n counter\nn{x=\"b\"} 20\n"}, "label identity appeared"}, {"missing", []string{"# TYPE n counter\nn 10\n", "# TYPE n counter\nn NaN\n"}, "unavailable sample"}} {
		t.Run(tc.name, func(t *testing.T) {
			a, err := Replay(strings.NewReader(recording(tc.raw)), w)
			if err != nil {
				t.Fatal(err)
			}
			if a.Series[0].Total != nil || !strings.Contains(strings.Join(a.Series[0].Issues, " "), tc.issue) {
				t.Fatalf("%+v", a)
			}
		})
	}
	raw := recording([]string{"n 1\n"})
	raw = strings.Replace(raw, "n 1", "n 2", 1)
	if _, err := Replay(strings.NewReader(raw), w); err == nil {
		t.Fatal("tampered raw accepted")
	}
}
func TestHistogramQuantile(t *testing.T) {
	v, err := HistogramQuantile([]float64{1, 2, 3, math.Inf(1)}, []float64{1, 3, 4, 4}, 0.5)
	if err != nil || v == nil || *v != 1.5 {
		t.Fatalf("%v %v", v, err)
	}
	if _, err := HistogramQuantile([]float64{1, math.Inf(1)}, []float64{3, 2}, 0.5); err == nil {
		t.Fatal("decreasing buckets accepted")
	}
}
func ExampleDiscoverTargets() {
	targets, _ := DiscoverTargets(strings.NewReader(`["http://localhost:8000/metrics"]`))
	fmt.Println(targets[0]) // Output: http://localhost:8000/metrics
}
func ExampleHistogramQuantile() {
	v, _ := HistogramQuantile([]float64{1, 2, 3, math.Inf(1)}, []float64{1, 3, 4, 4}, 0.5)
	fmt.Println(*v) // Output: 1.5
}

func TestHistogramReplay(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	w := Window{Start: start, End: start.Add(time.Second), MaxGap: time.Second, Targets: []string{"http://host/metrics"}}
	before := "# TYPE latency_milliseconds histogram\nlatency_milliseconds_bucket{le=\"100\"} 0\nlatency_milliseconds_bucket{le=\"200\"} 0\nlatency_milliseconds_bucket{le=\"+Inf\"} 0\nlatency_milliseconds_count 0\nlatency_milliseconds_sum 0\n"
	after := "# TYPE latency_milliseconds histogram\nlatency_milliseconds_bucket{le=\"100\"} 1\nlatency_milliseconds_bucket{le=\"200\"} 3\nlatency_milliseconds_bucket{le=\"+Inf\"} 4\nlatency_milliseconds_count 4\nlatency_milliseconds_sum 600\n"
	a, err := Replay(strings.NewReader(recording([]string{before, after})), w)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Histograms) != 1 || a.Histograms[0].P50 == nil || math.Abs(*a.Histograms[0].P50-0.15) > 1e-12 || a.Histograms[0].Unit != "seconds" {
		t.Fatalf("%+v", a.Histograms)
	}
	w.MaxGap = time.Millisecond
	a, err = Replay(strings.NewReader(recording([]string{before, after})), w)
	if err != nil || a.Histograms[0].P50 != nil {
		t.Fatalf("gap: %+v %v", a, err)
	}
}
func TestFailureBreaksInterval(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	lines := strings.Split(strings.TrimSpace(recording([]string{"# TYPE n counter\nn 1\n", "# TYPE n counter\nn 3\n"})), "\n")
	failure, _ := json.Marshal(Observation{SchemaVersion: 1, Target: "http://host/metrics", Error: "connection failed"})
	input := lines[0] + "\n" + string(failure) + "\n" + lines[1] + "\n"
	a, err := Replay(strings.NewReader(input), Window{Start: start, End: start.Add(time.Second), MaxGap: time.Second, Targets: []string{"http://host/metrics"}})
	if err != nil || a.Failures != 1 || len(a.Series) != 0 {
		t.Fatalf("%+v %v", a, err)
	}
}

func TestReplayTypeChanges(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	w := Window{Start: start, End: start.Add(3 * time.Second), MaxGap: time.Second, Targets: []string{"http://host/metrics"}}
	for _, types := range [][]string{{"gauge", "gauge", "counter", "counter"}, {"counter", "counter", "gauge", "gauge"}, {"gauge", "gauge", "gauge", "gauge"}, {"counter", "counter", "counter", "counter"}, {"gauge", "counter", "gauge", "gauge"}} {
		t.Run(strings.Join(types, "-"), func(t *testing.T) {
			var raw []string
			for i, typ := range types {
				raw = append(raw, fmt.Sprintf("# TYPE n %s\nn %d\n", typ, []int{0, 2, 100, 110}[i]))
			}
			a, err := Replay(strings.NewReader(recording(raw)), w)
			if err != nil || len(a.Series) != 1 {
				t.Fatalf("analysis=%+v err=%v", a, err)
			}
			s := a.Series[0]
			mixed := false
			for _, typ := range types {
				mixed = mixed || typ != types[0]
			}
			if mixed {
				if s.Total != nil || s.Rate != nil || s.Mean != nil || s.CoveredSeconds != 0 || !strings.Contains(strings.Join(s.Issues, ";"), "type changed") {
					t.Fatalf("mixed types aggregated: %+v", s)
				}
			} else if s.CoveredSeconds != 3 || types[0] == "gauge" && (s.Mean == nil || math.Abs(*s.Mean-314.0/6) > 1e-12) || types[0] == "counter" && (s.Total == nil || *s.Total != 110) {
				t.Fatalf("unchanged types not aggregated correctly: %+v", s)
			}
		})
	}
}

func TestHistogramConfiguredUnits(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	w := Window{Start: start, End: start.Add(time.Second), MaxGap: time.Second, Targets: []string{"http://host/metrics"}}
	before := "# TYPE latency histogram\nlatency_bucket{le=\"100\"} 0\nlatency_bucket{le=\"200\"} 0\nlatency_bucket{le=\"+Inf\"} 0\nlatency_count 0\nlatency_sum 0\n"
	after := "# TYPE latency histogram\nlatency_bucket{le=\"100\"} 1\nlatency_bucket{le=\"200\"} 3\nlatency_bucket{le=\"+Inf\"} 4\nlatency_count 4\nlatency_sum 600\n"
	for _, tt := range []struct {
		name    string
		units   map[string]string
		wantErr bool
	}{
		{"exact", map[string]string{"latency_bucket": "milliseconds", "latency_sum": "milliseconds"}, false},
		{"bucket only", map[string]string{"latency_bucket": "milliseconds"}, false},
		{"sum only", map[string]string{"latency_sum": "milliseconds"}, false},
		{"family", map[string]string{"latency": "milliseconds"}, false},
		{"aliases", map[string]string{"latency": "ms", "latency_bucket": "milliseconds", "latency_sum": "ms", "latency_count": "count"}, false},
		{"bucket sum conflict", map[string]string{"latency_bucket": "milliseconds", "latency_sum": "seconds"}, true},
		{"family conflict", map[string]string{"latency": "seconds", "latency_bucket": "milliseconds"}, true},
		{"bad family", map[string]string{"latency": "fortnights"}, true},
		{"bad bucket", map[string]string{"latency_bucket": "fortnights"}, true},
		{"bad sum", map[string]string{"latency_sum": "fortnights"}, true},
		{"bad count", map[string]string{"latency_count": "seconds"}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			w.Units = tt.units
			a, err := Replay(strings.NewReader(recording([]string{before, after})), w)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("invalid units accepted: %+v", a)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(a.Histograms) != 1 || a.Histograms[0].P50 == nil || math.Abs(*a.Histograms[0].P50-0.15) > 1e-12 || a.Histograms[0].Unit != "seconds" {
				t.Fatalf("histograms=%+v", a.Histograms)
			}
			var sum, count *Series
			for i := range a.Series {
				s := &a.Series[i]
				if s.Name == "latency_sum" {
					sum = s
				}
				if s.Name == "latency_count" {
					count = s
				}
			}
			if sum == nil || sum.Total == nil || math.Abs(*sum.Total-0.6) > 1e-12 || sum.Unit != "seconds" || count == nil || count.Total == nil || *count.Total != 4 || count.Unit != "count" {
				t.Fatalf("series=%+v", a.Series)
			}
		})
	}
}

func TestReplayTypeChangeAcrossMissingIdentity(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	w := Window{Start: start, End: start.Add(3 * time.Second), MaxGap: time.Second, Targets: []string{"http://host/metrics"}}
	for _, types := range [][]string{{"gauge", "counter"}, {"counter", "gauge"}} {
		t.Run(strings.Join(types, "-"), func(t *testing.T) {
			raw := []string{fmt.Sprintf("# TYPE n %s\nn 0\n", types[0]), fmt.Sprintf("# TYPE n %s\nn 2\n", types[0]), "other 1\n", fmt.Sprintf("# TYPE n %s\nn 100\n", types[1])}
			a, err := Replay(strings.NewReader(recording(raw)), w)
			if err != nil {
				t.Fatal(err)
			}
			for _, s := range a.Series {
				if s.Name == "n" && (s.Total != nil || s.Rate != nil || s.Mean != nil || s.CoveredSeconds != 0 || !strings.Contains(strings.Join(s.Issues, ";"), "type changed")) {
					t.Fatalf("mixed types aggregated after missing identity: %+v", s)
				}
			}
		})
	}
}

func TestReplayTypeChangeAcrossFailedScrape(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	lines := strings.Split(strings.TrimSpace(recording([]string{"# TYPE n gauge\nn 0\n", "# TYPE n gauge\nn 2\n", "other 1\n", "# TYPE n counter\nn 100\n"})), "\n")
	failure, err := json.Marshal(Observation{SchemaVersion: 1, Target: "http://host/metrics", Error: "failed scrape"})
	if err != nil {
		t.Fatal(err)
	}
	lines[2] = string(failure)
	a, err := Replay(strings.NewReader(strings.Join(lines, "\n")+"\n"), Window{Start: start, End: start.Add(3 * time.Second), MaxGap: time.Second, Targets: []string{"http://host/metrics"}})
	if err != nil || len(a.Series) != 1 {
		t.Fatalf("analysis=%+v err=%v", a, err)
	}
	s := a.Series[0]
	if s.Mean != nil || s.Total != nil || s.Rate != nil || s.CoveredSeconds != 0 || !strings.Contains(strings.Join(s.Issues, ";"), "type changed") {
		t.Fatalf("mixed types aggregated after failed scrape: %+v", s)
	}
}
