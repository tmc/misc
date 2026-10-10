package output

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	llm "github.com/tmc/misc/xk6-llm-perf"
	collector "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

func runFixture() llm.RunSummary {
	n := 0
	v := 20.
	return llm.RunSummary{SchemaVersion: 1, RunID: "test<script>", StartedAt: 1000, FinishedAt: 2000, Duration: 1, Attempts: 1, Successes: 1, Records: []llm.Measurement{{SchemaVersion: 1, Attempted: true, Recorded: true, Phase: "measurement", Outcome: "success", StartedAt: 1100, DispatchedAt: 1100, FinishedAt: 1200, FullResponseLatency: 100, TTFT: &v, InputTokens: llm.TokenCount{Value: &n, Source: "reported"}, OutputTokens: llm.TokenCount{Source: "unavailable"}}}}
}
func ExampleSummarize() {
	d, err := Summarize(runFixture())
	fmt.Println(d.Attempts, d.Metrics[1].Unit, d.Metrics[5].Count, err)
	// Output: 1 ms 0 <nil>
}
func ExampleCSV() {
	var b bytes.Buffer
	_ = CSV(&b, Document{Metrics: []Statistic{{Name: "ttft", Unit: "ms"}}})
	rows, _ := csv.NewReader(&b).ReadAll()
	fmt.Println(rows[1][0], rows[1][2], rows[1][3] == "")
	// Output: ttft 0 true
}
func ExampleHTML() {
	var b bytes.Buffer
	_ = HTML(&b, Document{RunID: "<script>"})
	fmt.Println(strings.Contains(b.String(), "&lt;script&gt;"))
	// Output: true
}
func ExampleOTLP() {
	b, err := OTLP(runFixture())
	var req collector.ExportMetricsServiceRequest
	_ = protojson.Unmarshal(b, &req)
	fmt.Println(len(req.ResourceMetrics[0].ScopeMetrics[0].Metrics), err)
	// Output: 3 <nil>
}
func ExampleMLflowBatch() {
	b, err := MLflowBatch(Document{Metrics: []Statistic{{Name: "ttft", Unit: "ms"}}}, "run", 123)
	fmt.Println(strings.Contains(string(b), `"metrics":[]`), err)
	// Output: true <nil>
}
func ExampleSend() {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "{}") }))
	defer s.Close()
	fmt.Println(Send(context.Background(), s.Client(), s.URL, "", []byte(`{}`)))
	// Output: <nil>
}
func TestConsumers(t *testing.T) {
	run := runFixture()
	d, err := Summarize(run)
	if err != nil {
		t.Fatal(err)
	}
	if d.Metrics[5].Count != 0 || d.Metrics[5].Mean != nil || *d.Metrics[4].Mean != 0 {
		t.Fatal("missing or reported zero lost")
	}
	var b bytes.Buffer
	if err := CSV(&b, d); err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(&b).ReadAll()
	if err != nil || rows[6][3] != "" {
		t.Fatalf("CSV missing: %v %v", rows, err)
	}
	b.Reset()
	HTML(&b, d)
	if strings.Contains(b.String(), "test<script>") {
		t.Fatal("unsafe HTML")
	}
	payload, err := OTLP(run)
	if err != nil {
		t.Fatal(err)
	}
	accepted := false
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Error("auth missing")
		}
		data, _ := io.ReadAll(r.Body)
		var req collector.ExportMetricsServiceRequest
		if err := protojson.Unmarshal(data, &req); err != nil {
			t.Error(err)
		}
		m := req.ResourceMetrics[0].ScopeMetrics[0].Metrics
		if m[0].Unit != "s" || m[0].GetGauge().DataPoints[0].GetAsDouble() != .1 || m[2].GetGauge().DataPoints[0].GetAsDouble() != 0 {
			t.Error("unit or missingness corrupted")
		}
		accepted = true
		io.WriteString(w, "{}")
	}))
	defer s.Close()
	if err := Send(context.Background(), s.Client(), s.URL+"/v1/metrics", "Bearer secret", payload); err != nil || !accepted {
		t.Fatalf("consumer: %v", err)
	}
	if bytes.Contains(payload, []byte("secret")) {
		t.Fatal("credential in artifact")
	}
}
func TestSinkFailures(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		code       int
	}{{"reject", "private token", 401}, {"partial", `{"partialSuccess":{"rejectedDataPoints":"1"}}`, 200}, {"malformed", "oops", 200}, {"redirect", "{}", 302}} {
		t.Run(tc.name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.code); io.WriteString(w, tc.body) }))
			defer s.Close()
			err := Send(context.Background(), s.Client(), s.URL+"/v1/metrics", "secret", []byte(`{}`))
			if err == nil || strings.Contains(err.Error(), "private") || strings.Contains(err.Error(), "secret") {
				t.Fatalf("error: %v", err)
			}
		})
	}
}
