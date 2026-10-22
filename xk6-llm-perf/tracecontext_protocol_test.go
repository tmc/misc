package llmperf

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
)

func TestTraceContextVideoLifecycle(t *testing.T) {
	for _, stop := range []string{"", "poll", "download"} {
		t.Run(stop, func(t *testing.T) {
			var mu sync.Mutex
			var headers []http.Header
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				headers = append(headers, r.Header.Clone())
				mu.Unlock()
				switch r.URL.Path {
				case "/videos":
					fmt.Fprint(w, `{"id":"job","status":"queued"}`)
				case "/videos/job":
					if stop == "poll" {
						w.WriteHeader(200)
						w.(http.Flusher).Flush()
						<-r.Context().Done()
						return
					}
					fmt.Fprint(w, `{"id":"job","status":"completed"}`)
				case "/videos/job/content":
					if stop == "download" {
						w.WriteHeader(200)
						w.(http.Flusher).Flush()
						<-r.Context().Done()
						return
					}
					fmt.Fprint(w, "video bytes")
				default:
					t.Errorf("unexpected path %s", r.URL.Path)
				}
			}))
			defer server.Close()
			c := setupTestClient(t, server.URL)
			c.config.TraceParent, c.config.TraceState = testParent, "vendor=value"
			var state runState
			if err := state.begin(RunOptions{RunID: "video-trace", MaxRecords: 1}); err != nil {
				t.Fatal(err)
			}
			c.run = &state
			req := CompletionRequest{Raw: &RawRequest{Path: "/videos", Protocol: "video", Body: map[string]any{}, Poll: &VideoPolling{Interval: "10ms", MaxPolls: 2, Download: true}}}
			if stop != "" {
				req.CancelAfter = "300ms"
			}
			result := protocolRequest(t, c, req)
			if _, bad := result.(error); bad != (stop != "") {
				t.Fatalf("result %T %v", result, result)
			}
			want := "success"
			if stop != "" {
				want = "cancelled"
			}
			mu.Lock()
			captured := append([]http.Header(nil), headers...)
			mu.Unlock()
			assertTraceAttempt(t, c, captured, want)
			run, err := state.finish()
			if err != nil {
				t.Fatal(err)
			}
			if len(run.Records) != 1 || !reflect.DeepEqual(run.Records[0].TraceContext, c.measurement.TraceContext) {
				t.Fatalf("run lost trace: %+v", run.Records)
			}
			mu.Lock()
			defer mu.Unlock()
			calls := 3
			if stop == "poll" {
				calls = 2
			}
			if len(headers) != calls {
				t.Fatalf("calls=%d want %d", len(headers), calls)
			}
		})
	}
}

func TestTraceContextCancelledStream(t *testing.T) {
	headers := make(chan http.Header, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers <- r.Header.Clone()
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer server.Close()
	c := setupTestClient(t, server.URL)
	c.config.TraceParent = testParent
	result := protocolRequest(t, c, CompletionRequest{Stream: true, CancelAfter: "100ms", Messages: []Message{{Role: "user", Content: "hello"}}})
	if _, ok := result.(error); !ok {
		t.Fatal("cancelled stream succeeded")
	}
	assertTraceAttempt(t, c, []http.Header{<-headers}, "cancelled")
}

func TestTraceContextRawProtocols(t *testing.T) {
	for _, protocol := range []string{"anthropic", "openai-responses", "providerwire-v4"} {
		t.Run(protocol, func(t *testing.T) {
			headers := make(chan http.Header, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				headers <- r.Header.Clone()
				switch protocol {
				case "anthropic":
					fmt.Fprint(w, `{"content":[{"type":"text","text":"hello"}],"usage":{"input_tokens":1,"output_tokens":1}}`)
				case "openai-responses":
					fmt.Fprint(w, `{"status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"hello"}]}],"usage":{"input_tokens":1,"output_tokens":1}}`)
				case "providerwire-v4":
					fmt.Fprint(w, `{"content":[{"type":"text","text":"hello"}],"warnings":[],"finishReason":{"unified":"stop"},"usage":{"inputTokens":{"total":1},"outputTokens":{"total":1}}}`)
				}
			}))
			defer server.Close()
			c := setupTestClient(t, server.URL)
			c.config.TraceParent = testParent
			c.config.TraceState = "vendor=value"
			body := map[string]any{}
			if protocol == "providerwire-v4" {
				body["prompt"] = []any{}
			}
			result := protocolRequest(t, c, CompletionRequest{Raw: &RawRequest{Path: "/generate", Protocol: protocol, Body: body}})
			if err, ok := result.(error); ok {
				t.Fatal(err)
			}
			assertTraceAttempt(t, c, []http.Header{<-headers}, "success")
		})
	}
}

func assertTraceAttempt(t *testing.T, c *Client, headers []http.Header, outcome string) {
	t.Helper()
	m := c.measurement
	if m == nil || m.TraceContext == nil || !m.Attempted || !m.finalized || m.Outcome != outcome || m.DispatchedAt <= 0 || m.FinishedAt < m.DispatchedAt {
		t.Fatalf("invalid attempted trace %+v", m)
	}
	for _, h := range headers {
		if h.Get("traceparent") != m.TraceContext.TraceParent || h.Get("tracestate") != m.TraceContext.TraceState {
			t.Fatalf("wire=%v record=%+v", h, m.TraceContext)
		}
	}
}

func TestTraceContextRedirectRefusal(t *testing.T) {
	for _, status := range []int{301, 302, 303, 307, 308} {
		for _, sameOrigin := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/same=%t", status, sameOrigin), func(t *testing.T) {
				var mu sync.Mutex
				hits := 0
				target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { mu.Lock(); hits++; mu.Unlock() }))
				defer target.Close()
				headers := make(chan http.Header, 1)
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/target" {
						mu.Lock()
						hits++
						mu.Unlock()
						return
					}
					headers <- r.Header.Clone()
					location := target.URL + "/target"
					if sameOrigin {
						location = "/target"
					}
					http.Redirect(w, r, location, status)
				}))
				defer server.Close()
				c := setupTestClient(t, server.URL)
				c.config.TraceParent = testParent
				result := protocolRequest(t, c, CompletionRequest{Messages: []Message{{Role: "user", Content: "hello"}}})
				if _, ok := result.(error); !ok {
					t.Fatal("redirect succeeded")
				}
				assertTraceAttempt(t, c, []http.Header{<-headers}, "failure")
				mu.Lock()
				defer mu.Unlock()
				if hits != 0 {
					t.Fatal("trace propagated to redirect target")
				}
			})
		}
	}
}
