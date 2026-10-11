package servermetrics

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func ExampleRead() {
	samples, err := Read(strings.NewReader("# TYPE requests_total counter\nrequests_total{model=\"fixture\"} 0\n"))
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(samples[0].Type, *samples[0].Value, samples[0].Labels["model"])
	// Output: counter 0 fixture
}
func ExampleCounterRate() {
	a, b := 1.0, 5.0
	before := Sample{Name: "requests_total", Type: "counter", Value: &a}
	after := Sample{Name: "requests_total", Type: "counter", Value: &b}
	rate, err := CounterRate(before, after, 2*time.Second, 3*time.Second)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(rate)
	// Output: 2
}
func ExampleFetch() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "# TYPE power_watts gauge\npower_watts 42\n")
	}))
	defer server.Close()
	scrape, err := Fetch(context.Background(), server.Client(), server.URL)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(scrape.SchemaVersion, len(scrape.Samples), *scrape.Samples[0].Value)
	// Output: 1 1 42
}

func TestPrometheusParsing(t *testing.T) {
	text := `# TYPE latency histogram
latency_bucket{model="a,\"b",le="1"} 2
latency_bucket{model="a,\"b",le="+Inf"} 3
latency_count{model="a,\"b"} 3
latency_sum{model="a,\"b"} 2.5
# TYPE temperature gauge
temperature NaN
requests_total 0 1000
`
	samples, err := Read(strings.NewReader(text))
	if err != nil {
		t.Fatal(err)
	}
	if samples[0].Type != "histogram" || samples[0].Labels["model"] != "a,\"b" || samples[4].Value != nil || samples[4].Unavailable == "" || *samples[5].Value != 0 || *samples[5].Timestamp != 1000 {
		t.Fatalf("samples=%+v", samples)
	}
}
func TestPrometheusRefusals(t *testing.T) {
	for _, input := range []string{
		`x{a="1",a="2"} 1`, `x{a="unterminated} 1`, "x 1\nx 2", "# TYPE x unknown\nx 1", "x 1 bad", "x 1 # exemplar",
		"# TYPE x histogram\nx_bucket{le=\"1\"} 3\nx_bucket{le=\"+Inf\"} 2\nx_count 2",
		"# TYPE x histogram\nx_bucket{le=\"1\"} 2\nx_count 2",
		"# TYPE x histogram\nx_bucket{le=\"+Inf\"} 2\nx_count 3",
	} {
		if _, err := Read(strings.NewReader(input)); err == nil {
			t.Fatalf("accepted %q", input)
		}
	}
}
func TestCounterResetLabelsAndGaps(t *testing.T) {
	a, b := 5.0, 3.0
	before := Sample{Name: "requests_total", Type: "counter", Labels: map[string]string{"model": "a"}, Value: &a}
	after := before
	after.Value = &b
	if _, err := CounterRate(before, after, time.Second, time.Second); err == nil {
		t.Fatal("accepted reset")
	}
	b = 6
	if _, err := CounterRate(before, after, 2*time.Second, time.Second); err == nil {
		t.Fatal("bridged gap")
	}
	after.Labels = map[string]string{"model": "b"}
	if _, err := CounterRate(before, after, time.Second, time.Second); err == nil {
		t.Fatal("joined different model")
	}
}
func TestFetchFailureAndCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	if _, err := Fetch(context.Background(), server.Client(), server.URL); err == nil {
		t.Fatal("accepted HTTP failure")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Fetch(ctx, server.Client(), server.URL); err == nil {
		t.Fatal("ignored cancellation")
	}
	if _, err := Fetch(context.Background(), server.Client(), server.URL+"?secret=key"); err == nil {
		t.Fatal("accepted secret query")
	}
}

func TestCounterTimestampArithmetic(t *testing.T) {
	valueBefore, valueAfter := float64(1), float64(2)
	for _, tt := range []struct {
		name          string
		before, after int64
		bad           bool
	}{
		{"negative", -2, -1, false}, {"cross zero", -1, 0, false},
		{"overflow", -2, 1<<63 - 1, true}, {"reverse", 1, -1, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			before := Sample{Name: "counter", Type: "counter", Value: &valueBefore, Timestamp: &tt.before}
			after := Sample{Name: "counter", Type: "counter", Value: &valueAfter, Timestamp: &tt.after}
			rate, err := CounterRate(before, after, time.Millisecond, time.Second)
			if (err != nil) != tt.bad {
				t.Fatalf("rate=%v err=%v", rate, err)
			}
			if !tt.bad && rate != 1000 {
				t.Fatalf("rate=%v", rate)
			}
		})
	}
}

func TestReadTypeOrdering(t *testing.T) {
	for _, tt := range []struct {
		name, text string
		wantErr    bool
	}{
		{"late gauge", "x 1\n# TYPE x gauge\n", true},
		{"late histogram", "x_bucket{le=\"1\"} -1\n# TYPE x histogram\n", true},
		{"late summary", "x_sum 1\n# TYPE x summary\n", true},
		{"duplicate", "# TYPE x gauge\n# TYPE x gauge\nx 1\n", true},
		{"conflict", "# TYPE x gauge\n# TYPE x counter\nx 1\n", true},
		{"legal", "# TYPE x gauge\nx 1\n", false},
		{"independent suffix", "x_count 1\n# TYPE x gauge\nx 1\n", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Read(strings.NewReader(tt.text))
			if (err != nil) != tt.wantErr {
				t.Fatalf("Read error=%v, wantErr=%v", err, tt.wantErr)
			}
		})
	}
}
