package llmperf

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestProviderwireHTTP(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprint(stream), func(t *testing.T) {
			body := map[string]any{"prompt": []any{map[string]any{"role": "user", "content": []any{map[string]any{"type": "text", "text": "hello"}}}}}
			want, _ := json.Marshal(body)
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				b, _ := io.ReadAll(r.Body)
				if string(b) != string(want) {
					t.Errorf("body %s want %s", b, want)
				}
				for k, v := range map[string]string{"Ai-Language-Model-Id": "fixture", "Ai-Language-Model-Specification-Version": "4", "Ai-Language-Model-Streaming": fmt.Sprint(stream), "Authorization": "Bearer test-key"} {
					if got := r.Header.Get(k); got != v {
						t.Errorf("header %s=%q want %q", k, got, v)
					}
				}
				if !stream {
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprint(w, `{"content":[{"type":"text","text":"answer"}],"warnings":[],"finishReason":{"unified":"stop"},"usage":{"inputTokens":{"total":2},"outputTokens":{"total":3}}}`)
					return
				}
				w.Header().Set("Content-Type", "text/event-stream")
				for _, s := range []string{`{"type":"stream-start","warnings":[]}`, `{"type":"reasoning-start","id":"r"}`, `{"type":"reasoning-delta","id":"r","delta":"think"}`, `{"type":"reasoning-end","id":"r"}`, `{"type":"text-start","id":"t"}`, `{"type":"text-delta","id":"t","delta":"answer"}`, `{"type":"text-end","id":"t"}`, `{"type":"finish","finishReason":{"unified":"stop"},"usage":{"inputTokens":{"total":2},"outputTokens":{"total":3}}}`} {
					fmt.Fprintf(w, "data: %s\n\n", s)
					w.(http.Flusher).Flush()
					if strings.Contains(s, "reasoning-delta") {
						time.Sleep(15 * time.Millisecond)
					}
				}
			}))
			defer server.Close()
			c := setupTestClient(t, server.URL)
			c.config.StrictMeasurement = true
			result := protocolRequest(t, c, CompletionRequest{Raw: &RawRequest{Path: "/language-model", Protocol: "providerwire-v4", Body: body, Stream: &stream}})
			if e, ok := result.(error); ok {
				t.Fatal(e)
			}
			r, ok := result.(map[string]any)
			if !ok {
				t.Fatalf("result %T %+v", result, result)
			}
			choices := r["choices"].([]any)
			message := choices[0].(map[string]any)["message"].(map[string]any)
			if message["content"] != "answer" || c.measurement.InputTokens.Value == nil || *c.measurement.InputTokens.Value != 2 || c.measurement.OutputTokens.Value == nil || *c.measurement.OutputTokens.Value != 3 || calls.Load() != 1 {
				t.Fatalf("response %+v measurement %+v", r, c.measurement)
			}
			if _, ok := body["model"]; ok {
				t.Fatal("mutated body.model")
			}
			if stream {
				m := c.measurement
				if m == nil || m.TTFT == nil || m.TTFO == nil || *m.TTFO <= *m.TTFT {
					t.Fatalf("reasoning/output timing %+v", m)
				}
			}
		})
	}
}

func TestProviderwireHTTPRefusals(t *testing.T) {
	for _, ending := range []string{"", "[DONE]", `{"type":"unknown"}`, `{"type":"finish","usage":{"inputTokens":{},"outputTokens":{}},"finishReason":{"unified":"error"}}`, `{"type":"error","error":{"message":"secret"}}`} {
		t.Run(ending, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprint(w, "data: {\"type\":\"stream-start\",\"warnings\":[]}\n\n")
				if ending != "" {
					fmt.Fprintf(w, "data: %s\n\n", ending)
				}
			}))
			defer server.Close()
			c := setupTestClient(t, server.URL)
			stream := true
			result := protocolRequest(t, c, CompletionRequest{Raw: &RawRequest{Path: "/language-model", Protocol: "providerwire-v4", Body: map[string]any{"prompt": []any{}}, Stream: &stream}})
			if _, ok := result.(error); !ok {
				t.Fatalf("admitted %T %+v", result, result)
			}
		})
	}
	if e := validateCompletionRequest(CompletionRequest{Raw: &RawRequest{Path: "/language-model", Protocol: "providerwire-v4", Body: map[string]any{"prompt": []any{}}}}); e == nil {
		t.Fatal("empty model admitted")
	}
}

func TestProviderwireCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"type\":\"stream-start\",\"warnings\":[]}\n\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer server.Close()
	c := setupTestClient(t, server.URL)
	stream := true
	result := protocolRequest(t, c, CompletionRequest{CancelAfter: "20ms", Raw: &RawRequest{Path: "/language-model", Protocol: "providerwire-v4", Body: map[string]any{"prompt": []any{}}, Stream: &stream}})
	if _, ok := result.(error); !ok || c.measurement.Outcome != "cancelled" {
		t.Fatalf("result %T measurement %+v", result, c.measurement)
	}
}

func TestProviderwireMetadataHTTP(t *testing.T) {
	for _, tt := range providerwireMetadataCases() {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				for _, b := range tt.events {
					fmt.Fprintf(w, "data: %s\n\n", b)
				}
			}))
			defer server.Close()
			c := setupTestClient(t, server.URL)
			stream := true
			v := protocolRequest(t, c, CompletionRequest{Raw: &RawRequest{Path: "/language-model", Protocol: "providerwire-v4", Body: map[string]any{"prompt": []any{}}, Stream: &stream}})
			_, failed := v.(error)
			if failed != tt.bad {
				t.Fatalf("result %T %+v", v, v)
			}
			if tt.bad {
				if c.measurement.Outcome != "failure" || c.measurement.Good {
					t.Fatalf("measurement %+v", c.measurement)
				}
				return
			}
			r := v.(map[string]any)
			if r["id"] != tt.id || r["model"] != tt.model {
				t.Fatalf("identity %+v", r)
			}
		})
	}
}
