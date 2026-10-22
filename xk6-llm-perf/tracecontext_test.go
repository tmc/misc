package llmperf

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/grafana/sobek"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const testParent = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"

func ExampleTraceContext() {
	record := TraceContext{TraceID: "4bf92f3577b34da6a3ce929d0e0e4736", SpanID: "00f067aa0ba902b7", TraceFlags: "01"}
	fmt.Println(record.TraceID, record.SpanID, record.TraceFlags)
	// Output: 4bf92f3577b34da6a3ce929d0e0e4736 00f067aa0ba902b7 01
}

func ExampleTraceContext_UnmarshalJSON() {
	var record TraceContext
	err := json.Unmarshal([]byte(`{"trace_id":"4bf92f3577b34da6a3ce929d0e0e4736","span_id":"00f067aa0ba902b7","trace_flags":"01","traceparent":"00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"}`), &record)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(record.SpanID)
	// Output: 00f067aa0ba902b7
}

func TestTraceContextValidation(t *testing.T) {
	for _, tt := range []struct {
		name, parent, state string
		bad                 bool
	}{
		{"disabled", "", "", false},
		{"sampled", testParent, "vendor=value", false},
		{"unsampled", strings.TrimSuffix(testParent, "01") + "00", "", false},
		{"future version", "01" + testParent[2:] + "-extra", "", false},
		{"missing parent", "", "vendor=value", true},
		{"zero trace", "00-00000000000000000000000000000000-00f067aa0ba902b7-01", "", true},
		{"zero span", "00-4bf92f3577b34da6a3ce929d0e0e4736-0000000000000000-01", "", true},
		{"uppercase", strings.ToUpper(testParent), "", true},
		{"version ff", "ff" + testParent[2:], "", true},
		{"version zero extra", testParent + "-extra", "", true},
		{"injection", testParent + "\r\nX-Test: yes", "", true},
		{"future injection", "01" + testParent[2:] + "-extra\r\nX-Test: yes", "", true},
		{"future null", "01" + testParent[2:] + "-extra\x00", "", true},
		{"future control", "01" + testParent[2:] + "-extra\x7f", "", true},
		{"state injection", testParent, "a=1\r\nX-Test: yes", true},
		{"invalid state", testParent, "bad state", true},
		{"duplicate state", testParent, "a=1,a=2", true},
		{"long parent", strings.Repeat("0", 1025), "", true},
		{"long state", testParent, strings.Repeat("a", 513), true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := configuredTraceContext(Config{TraceParent: tt.parent, TraceState: tt.state})
			if (err != nil) != tt.bad {
				t.Fatalf("error=%v, want bad=%v", err, tt.bad)
			}
		})
	}
}

func TestTraceContextPropagation(t *testing.T) {
	for _, tt := range []struct {
		name   string
		config Config
	}{
		{"disabled", Config{}}, {"root", Config{Tracing: true}},
		{"child", Config{TraceParent: testParent, TraceState: "vendor=value"}},
		{"unsampled", Config{TraceParent: strings.TrimSuffix(testParent, "01") + "00"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			seen := map[string]bool{}
			for range 100 {
				req, _ := http.NewRequest("POST", "http://localhost", nil)
				req, record := attachTraceContext(req, tt.config)
				if tt.name == "disabled" {
					if record != nil || req.Header.Get("traceparent") != "" {
						t.Fatal("tracing enabled by default")
					}
					continue
				}
				sc := trace.SpanContextFromContext(propagation.TraceContext{}.Extract(context.Background(), propagation.HeaderCarrier(req.Header)))
				if !sc.IsValid() || record.TraceID != sc.TraceID().String() || record.SpanID != sc.SpanID().String() || record.TraceParent != req.Header.Get("traceparent") {
					t.Fatal("record does not match wire", record)
				}
				if seen[record.SpanID] {
					t.Fatal("reused span ID")
				}
				seen[record.SpanID] = true
				if tt.name == "root" {
					if record.ParentSpanID != "" {
						t.Fatal("unexpected parent")
					}
				} else {
					if record.ParentSpanID != "00f067aa0ba902b7" || record.TraceID != "4bf92f3577b34da6a3ce929d0e0e4736" {
						t.Fatal("lost parent")
					}
				}
				if sc.IsSampled() != (tt.name != "unsampled") || record.TraceState != tt.config.TraceState {
					t.Fatal("lost sampling/state")
				}
			}
		})
	}
}

func TestClientWireTraceContext(t *testing.T) {
	for _, tt := range []struct {
		name      string
		stream    bool
		status    int
		body      string
		badParent bool
	}{
		{name: "sync", status: 200, body: `{"id":"test","choices":[{"message":{"content":"hello"}}]}`},
		{name: "stream", stream: true, status: 200, body: "data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\ndata: [DONE]\n\n"},
		{name: "http error", status: 503, body: "unavailable"},
		{name: "broken stream", stream: true, status: 200, body: "data: not-json\n\n"},
		{name: "invalid parent", badParent: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			headers := make(chan http.Header, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				headers <- r.Header.Clone()
				if tt.stream {
					w.Header().Set("Content-Type", "text/event-stream")
				}
				w.WriteHeader(tt.status)
				fmt.Fprint(w, tt.body)
			}))
			defer server.Close()
			client := setupTestClient(t, server.URL)
			client.config.TraceParent = testParent
			client.config.TraceState = "vendor=value"
			if tt.badParent {
				client.config.TraceParent = "invalid"
			}
			rt := client.vu.Runtime()
			result := client.Complete(sobek.FunctionCall{Arguments: []sobek.Value{rt.ToValue(&CompletionRequest{Model: "test", Messages: []Message{{Role: "user", Content: "hello"}}, Stream: tt.stream})}})
			if tt.badParent {
				if _, ok := result.Export().(error); !ok {
					t.Fatal("accepted bad context")
				}
				if len(headers) != 0 {
					t.Fatal("dispatched invalid context")
				}
				return
			}
			h := <-headers
			m := client.measurement
			if m == nil || m.TraceContext == nil || m.TraceContext.TraceParent != h.Get("traceparent") || h.Get("tracestate") != "vendor=value" {
				t.Fatal("lost wire context", m)
			}
			if !m.finalized || m.DispatchedAt <= 0 || m.FinishedAt < m.DispatchedAt {
				t.Fatal("incomplete span lifetime")
			}
			if (m.Outcome == "success") != (tt.name == "sync" || tt.name == "stream") {
				t.Fatal("wrong span outcome", m.Outcome)
			}
		})
	}
}

func TestTraceContextExistingParent(t *testing.T) {
	ctx := propagation.TraceContext{}.Extract(context.Background(), propagation.MapCarrier{"traceparent": testParent})
	req, _ := http.NewRequestWithContext(ctx, "POST", "http://localhost", nil)
	_, record := attachTraceContext(req, Config{Tracing: true})
	if record.ParentSpanID != "00f067aa0ba902b7" || record.TraceID != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatal("lost Go context parent", record)
	}
}

func TestBinaryTraceContext(t *testing.T) {
	binary := os.Getenv("K6_SMOKE_BINARY")
	if binary == "" {
		t.Skip("set K6_SMOKE_BINARY to test wire trace contexts")
	}
	headers := make(chan http.Header, 24)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers <- r.Header.Clone()
		if r.Header.Get("Accept") == "text/event-stream" {
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\ndata: [DONE]\n\n")
		} else {
			fmt.Fprint(w, `{"choices":[{"message":{"content":"hello"}}]}`)
		}
	}))
	defer server.Close()
	dir := t.TempDir()
	script := filepath.Join(dir, "trace.js")
	report := filepath.Join(dir, "run.json")
	source := fmt.Sprintf(`
 import llm from 'k6/x/llm-perf';
 export const options = {scenarios:{test:{executor:'shared-iterations',vus:4,iterations:24}}};
 const client=new llm.Client({baseURL:%q,model:'test',traceparent:%q,tracestate:'vendor=value'});
 export function setup(){llm.beginMeasurement({maxRecords:24});}
 export default function(){
  const r = __ITER %% 2 ? client.request({path:'/chat/completions',protocol:'openai-chat',stream:true,body:{model:'test',messages:[{role:'user',content:'hello'}]}}) : client.chat.completions.create({messages:[{role:'user',content:'hello'}]});
  if (r instanceof Error || !r.measurement.trace_context) throw new Error('trace context missing');
 }
 export function teardown(){llm.endMeasurement();}
 export function handleSummary(){return {[%q]:JSON.stringify(llm.measurementSummary())};}
 `, server.URL, testParent, report)
	if err := os.WriteFile(script, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "run", "--no-usage-report", script)
	cmd.Env = append(os.Environ(), "K6_AUTO_EXTENSION_RESOLUTION=false")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("k6: %v\n%s", err, output)
	}
	b, err := os.ReadFile(report)
	if err != nil {
		t.Fatal(err)
	}
	var run RunSummary
	if err := json.Unmarshal(b, &run); err != nil {
		t.Fatal(err)
	}
	if run.Successes != 24 || len(run.Records) != 24 || len(headers) != 24 {
		t.Fatalf("incomplete run: %d successful, %d records, %d headers", run.Successes, len(run.Records), len(headers))
	}
	wire := map[string]bool{}
	for range 24 {
		h := <-headers
		v := h.Get("traceparent")
		if wire[v] || h.Get("tracestate") != "vendor=value" {
			t.Fatal("wire context reused or lost")
		}
		wire[v] = true
	}
	for _, m := range run.Records {
		c := m.TraceContext
		if c == nil || !wire[c.TraceParent] || c.ParentSpanID != "00f067aa0ba902b7" {
			t.Fatal("retained context does not match wire")
		}
		delete(wire, c.TraceParent)
	}
	if len(wire) != 0 {
		t.Fatal("unmatched wire contexts")
	}
}
