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
