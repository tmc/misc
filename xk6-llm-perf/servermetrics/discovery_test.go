package servermetrics

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDiscover(t *testing.T) {
	for _, raw := range []string{`["ftp://host/metrics"]`, `["http://host/metrics","http://host/metrics"]`, `["http://user:pass@host/metrics"]`, `[] {}`} {
		if _, err := DiscoverTargets(strings.NewReader(raw)); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	raw := `{"items":[{"metadata":{"namespace":"bench","labels":{"app":"vllm"},"annotations":{"prometheus.io/scrape":"true","prometheus.io/port":"8000"}},"status":{"phase":"Running","podIP":"::1"}},{"metadata":{"namespace":"other"},"status":{"phase":"Running","podIP":"127.0.0.1"}}]}`
	got, err := DiscoverPods(strings.NewReader(raw), "bench", map[string]string{"app": "vllm"})
	if err != nil || len(got) != 1 || got[0] != "http://[::1]:8000/metrics" {
		t.Fatalf("%v %v", got, err)
	}
	if _, err := DiscoverPods(strings.NewReader(raw), "", nil); err == nil {
		t.Fatal("inferred namespace")
	}
}
func TestDynamicDiscovery(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "# TYPE n counter\nn 1\n") }))
	defer s.Close()
	calls := 0
	var out bytes.Buffer
	result, err := Collect(context.Background(), s.Client(), CollectOptions{Discover: func(context.Context) ([]string, error) {
		calls++
		if calls > 1 {
			return nil, fmt.Errorf("inventory unavailable")
		}
		return []string{s.URL}, nil
	}, Interval: 10 * time.Millisecond, Duration: time.Second, MaxRecords: 10, MaxBytes: 100000}, &out)
	if err == nil || result.Complete || !strings.Contains(out.String(), `"targets":[`) {
		t.Fatalf("%+v %v %s", result, err, out.String())
	}
}
func ExampleDiscoverPods() {
	targets, _ := DiscoverPods(strings.NewReader(`{"items":[]}`), "bench", nil)
	fmt.Println(len(targets)) // Output: 0
}
func ExampleReplay() {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	a, _ := Replay(strings.NewReader(""), Window{Start: start, End: start.Add(time.Second), MaxGap: time.Second, Targets: []string{"http://host/metrics"}})
	fmt.Println(a.Issues[0]) // Output: missing target: http://host/metrics
}

func TestInvalidExpositionRetained(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "not a valid sample\n") }))
	defer s.Close()
	var out bytes.Buffer
	r, err := Collect(context.Background(), s.Client(), CollectOptions{Targets: []string{s.URL}, Interval: 10 * time.Millisecond, Duration: time.Second, MaxRecords: 1, MaxBytes: 100000}, &out)
	// A record budget ends this parser-negative fixture independently of HTTP
	// startup timing. Dedicated deadline controls keep their short budgets.
	if err != nil || r.Complete || r.Attempts != 1 || r.Failures != 1 || r.IncompleteReason != "record budget exhausted" || !strings.Contains(out.String(), `"failed_scrape"`) || !strings.Contains(out.String(), `not a valid sample`) {
		t.Fatalf("%+v %v %s", r, err, out.String())
	}
}
