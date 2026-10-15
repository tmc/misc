package llmperf

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/grafana/sobek"
	"go.k6.io/k6/v2/js/modulestest"
	"go.k6.io/k6/v2/metrics"
)

func protocolRequest(t *testing.T, c *Client, req CompletionRequest) any {
	t.Helper()
	if req.Model == "" {
		req.Model = "fixture"
	}
	c.metrics.samples = make(chan metrics.SampleContainer, 2000)
	return c.Complete(sobek.FunctionCall{Arguments: []sobek.Value{c.vu.Runtime().ToValue(req)}}).Export()
}

func TestInferenceRedirectRefusal(t *testing.T) {
	for _, status := range []int{301, 302, 303, 307, 308} {
		for _, protocol := range []string{"openai-chat", "anthropic"} {
			for _, sameOrigin := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%d/same=%t", protocol, status, sameOrigin), func(t *testing.T) {
					var redirected atomic.Int32
					target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { redirected.Add(1) }))
					defer target.Close()
					origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if r.URL.Path == "/redirected" {
							redirected.Add(1)
							return
						}
						location := target.URL + "/redirected"
						if sameOrigin {
							location = "/redirected"
						}
						http.Redirect(w, r, location, status)
					}))
					defer origin.Close()
					c := setupTestClient(t, origin.URL)
					if status%2 == 1 {
						enableVUState(t, c, make(chan metrics.SampleContainer, 2000))
						c.vu.State().Transport = http.DefaultTransport
						c.config.UseK6Transport = true
					}
					req := CompletionRequest{Messages: []Message{{Role: "user", Content: "private prompt"}}}
					if protocol == "anthropic" {
						req.Raw = &RawRequest{Protocol: protocol, Path: "/messages", Body: map[string]any{"messages": []Message{{Role: "user", Content: "private prompt"}}}}
					}
					v := protocolRequest(t, c, req)
					if _, ok := v.(error); !ok {
						t.Fatalf("redirect accepted: %v", v)
					}
					if redirected.Load() != 0 {
						t.Fatal("redirect forwarded credentials or body")
					}
				})
			}
		}
	}
}

func TestReasoningAliasNormalization(t *testing.T) {
	for _, fields := range []string{`"reasoning":"r"`, `"reasoning_content":"r"`, `"reasoning_content":"","reasoning":"r"`, `"reasoning_content":"r","reasoning":"ignored"`} {
		var m Message
		if err := json.Unmarshal([]byte("{"+fields+"}"), &m); err != nil {
			t.Fatal(err)
		}
		if m.ReasoningContent != "r" {
			t.Fatalf("message: %+v", m)
		}
		chunk, _, err := decodeChunk([]byte(`{"choices":[{"index":0,"delta":{`+fields+`}}]}`), "openai-chat")
		if err != nil || chunk.Choices[0].Delta.ReasoningContent != "r" {
			t.Fatalf("chunk %+v: %v", chunk, err)
		}
	}
	for _, visible := range []bool{false, true} {
		t.Run(fmt.Sprint(visible), func(t *testing.T) {
			wire := "data: {\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\"}}]}\n\n" +
				"data: {\"choices\":[{\"index\":0,\"delta\":{\"reasoning\":\"think\"}}]}\n\n"
			if visible {
				wire += "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"answer\"}}]}\n\n"
			}
			wire += "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":2,\"completion_tokens\":3,\"total_tokens\":5}}\n\ndata: [DONE]\n\n"
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprint(w, wire)
			}))
			defer s.Close()
			c := setupTestClient(t, s.URL)
			v := protocolRequest(t, c, CompletionRequest{Stream: true, Messages: []Message{{Role: "user", Content: "hello"}}})
			if e, ok := v.(error); ok {
				t.Fatal(e)
			}
			if c.measurement.TTFT == nil || (c.measurement.TTFO != nil) != visible {
				t.Fatalf("timings: %+v", c.measurement)
			}
			choices := v.(map[string]any)["choices"].([]any)
			if choices[0].(map[string]any)["message"].(map[string]any)["reasoning_content"] != "think" {
				t.Fatal(v)
			}
		})
	}
}

func TestChoiceEOFCompletion(t *testing.T) {
	for _, terminal := range []string{"", "data: [DONE]\n\n"} {
		wire := "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"a\"},\"finish_reason\":\"stop\"},{\"index\":1,\"delta\":{\"content\":\"b\"}}]}\n\n" + terminal
		c := setupTestClient(t, "http://unused")
		resp := &http.Response{StatusCode: 200, Body: ioNopCloser{strings.NewReader(wire)}}
		_, err := c.handleStreamingResponse(resp, time.Now(), 0, func() {}, false)
		if (err == nil) != (terminal != "") {
			t.Fatalf("terminal %q: %v", terminal, err)
		}
	}
}

func TestStreamModelChanged(t *testing.T) {
	for _, tt := range []struct {
		name, second string
		ok           bool
	}{
		{"same", "cheap", true},
		{"omitted", "", true},
		{"changed", "pricey", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			wire := "data: {\"model\":\"cheap\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"a\"}}]}\n\n" +
				"data: {\"model\":\"" + tt.second + "\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":1,\"total_tokens\":2}}\n\ndata: [DONE]\n\n"
			c := setupTestClient(t, "http://unused")
			resp := &http.Response{StatusCode: 200, Body: ioNopCloser{strings.NewReader(wire)}}
			got, err := c.handleStreamingResponse(resp, time.Now(), 0, func() {}, false)
			if (err == nil) != tt.ok || tt.ok && got.Model != "cheap" {
				t.Fatalf("model %q, err %v", got.Model, err)
			}
		})
	}
}

type ioNopCloser struct{ *strings.Reader }

func (ioNopCloser) Close() error { return nil }

// cancelBeforeServerTransport models a request deadline expiring while the
// transport waits to send headers. No server handler has entered at that point.
type cancelBeforeServerTransport struct{}

func (cancelBeforeServerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.Body != nil {
		defer r.Body.Close()
	}
	<-r.Context().Done()
	return nil, r.Context().Err()
}

func TestCancellationBeforeServerEntryReleasesAdmission(t *testing.T) {
	var entered atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { entered.Add(1) }))
	defer server.Close()
	c := setupTestClient(t, server.URL)
	c.transport = cancelBeforeServerTransport{}
	c.config.ControlGroup = "early"
	c.controls = &controlGroups{}
	if err := c.controls.set("early", ControlLimits{Request: 1, Prefill: 1}); err != nil {
		t.Fatal(err)
	}
	v := protocolRequest(t, c, CompletionRequest{CancelAfter: "20ms", Messages: []Message{{Role: "user", Content: "a"}}})
	if _, ok := v.(error); !ok || c.measurement.Outcome != "cancelled" || !c.measurement.Attempted {
		t.Fatalf("result=%v measurement=%+v", v, c.measurement)
	}
	if entered.Load() != 0 {
		t.Fatal("server entered before transport dispatch")
	}
	state, err := c.controls.snapshot("early")
	if err != nil || state.Requests != 0 || state.Prefills != 0 {
		t.Fatalf("retained slots: %+v %v", state, err)
	}
}

func TestTransportCancellationReleasesAdmission(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprint(stream), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			entered := make(chan struct{})
			stopped := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/ready" {
					fmt.Fprint(w, `{"choices":[{"message":{"content":"ok"}}]}`)
					return
				}
				_, _ = io.Copy(io.Discard, r.Body)
				if stream {
					w.Header().Set("Content-Type", "text/event-stream")
					fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"role\":\"assistant\"}}]}\n\n")
					w.(http.Flusher).Flush()
				}
				close(entered)
				<-r.Context().Done()
				close(stopped)
			}))
			defer server.Close()
			c := setupTestClient(t, server.URL)
			c.vu.(*modulestest.VU).CtxField = ctx
			c.config.ControlGroup = "cancel"
			c.controls = &controlGroups{}
			if err := c.controls.set("cancel", ControlLimits{Request: 1, Prefill: 1}); err != nil {
				t.Fatal(err)
			}
			// Only this goroutine uses the runtime until it reports completion. The
			// test thread controls context and reads mutex-protected admission state.
			result := make(chan any, 1)
			go func() {
				result <- protocolRequest(t, c, CompletionRequest{Stream: stream, Messages: []Message{{Role: "user", Content: "a"}}})
			}()
			select {
			case <-entered:
			case <-time.After(time.Second):
				cancel()
				t.Fatal("server did not enter")
			}
			state, err := c.controls.snapshot("cancel")
			if err != nil || state.Requests != 1 || state.Prefills != 1 {
				cancel()
				t.Fatalf("missing active slots: %+v %v", state, err)
			}
			cancel()
			select {
			case v := <-result:
				if _, ok := v.(error); !ok || c.measurement.Outcome != "cancelled" {
					t.Fatalf("result=%v measurement=%+v", v, c.measurement)
				}
			case <-time.After(time.Second):
				t.Fatal("cancelled request did not join")
			}
			select {
			case <-stopped:
			case <-time.After(time.Second):
				t.Fatal("server context retained")
			}
			state, err = c.controls.snapshot("cancel")
			if err != nil || state.Requests != 0 || state.Prefills != 0 {
				t.Fatalf("retained slots: %+v %v", state, err)
			}
			// A new active VU context on the same client must acquire released slots.
			c.vu.(*modulestest.VU).CtxField = context.Background()
			v := protocolRequest(t, c, CompletionRequest{CancelAfter: "1s", Raw: &RawRequest{Protocol: "openai-chat", Path: "/ready", Body: map[string]any{"messages": []Message{{Role: "user", Content: "a"}}}}})
			if e, ok := v.(error); ok {
				t.Fatalf("subsequent request: %v", e)
			}
		})
	}
}

func TestVUAbortReleasesControls(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stopped := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		cancel()
		<-r.Context().Done()
		close(stopped)
	}))
	defer server.Close()
	c := setupTestClient(t, server.URL)
	c.vu.(*modulestest.VU).CtxField = ctx
	c.config.ControlGroup = "abort"
	c.controls = &controlGroups{}
	if err := c.controls.set("abort", ControlLimits{Request: 1, Prefill: 1}); err != nil {
		t.Fatal(err)
	}
	v := protocolRequest(t, c, CompletionRequest{Messages: []Message{{Role: "user", Content: "private"}}})
	if _, ok := v.(error); !ok || c.measurement.Outcome != "cancelled" {
		t.Fatalf("abort %v %+v", v, c.measurement)
	}
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("server context retained")
	}
	state, err := c.controls.snapshot("abort")
	if err != nil || state.Requests != 0 || state.Prefills != 0 {
		t.Fatalf("retained controls: %+v %v", state, err)
	}
}

func TestResponseChoiceIdentity(t *testing.T) {
	for _, tt := range []struct {
		name, body string
		valid      bool
	}{
		{"legacy singleton", `{"choices":[{"message":{"content":"a"}}]}`, true},
		{"out of order", `{"choices":[{"index":1,"message":{"content":"b"}},{"index":0,"message":{"content":"a"}}]}`, true},
		{"duplicate", `{"choices":[{"index":0,"message":{"content":"a"}},{"index":0,"message":{"content":"b"}}]}`, false},
		{"negative", `{"choices":[{"index":-1,"message":{"content":"a"}}]}`, false},
		{"oversized", `{"choices":[{"index":128,"message":{"content":"a"}}]}`, false},
		{"error with partial", `{"error":{"message":"provider secret"},"choices":[{"message":{"content":"partial"}}]}`, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, tt.body) }))
			defer s.Close()
			c := setupTestClient(t, s.URL)
			v := protocolRequest(t, c, CompletionRequest{Messages: []Message{{Role: "user", Content: "private"}}})
			_, failed := v.(error)
			if failed == tt.valid {
				t.Fatalf("result %v", v)
			}
			if failed && strings.Contains(fmt.Sprint(v), "provider secret") {
				t.Fatal("error leaked provider text")
			}
		})
	}
}

func TestSpecDecodeHTTPProvenance(t *testing.T) {
	invalid := `{"num_accepted_draft_tokens":2,"num_draft_tokens":1,"num_spec_steps":1,"acceptance_histogram":[0,1]}`
	valid := `{"num_accepted_draft_tokens":1,"num_draft_tokens":1,"num_spec_steps":1,"acceptance_histogram":[0,1]}`
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprint(stream), func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if stream {
					w.Header().Set("Content-Type", "text/event-stream")
					fmt.Fprintf(w, "data: {\"metrics\":{\"speculative_decoding\":%s},\"choices\":[{\"delta\":{\"content\":\"text\"}}]}\n\ndata: {\"metrics\":{\"speculative_decoding\":%s},\"choices\":[{\"finish_reason\":\"stop\",\"delta\":{}}]}\n\ndata: [DONE]\n\n", invalid, valid)
				} else {
					fmt.Fprintf(w, `{"metrics":{"speculative_decoding":%s},"choices":[{"speculative_decoding":%s,"message":{"content":"text"}}]}`, invalid, valid)
				}
			}))
			defer s.Close()
			c := setupTestClient(t, s.URL)
			v := protocolRequest(t, c, CompletionRequest{Stream: stream, Messages: []Message{{Role: "user", Content: "private"}}})
			if e, ok := v.(error); ok {
				t.Fatal(e)
			}
			if c.measurement.SpecDecode != nil || c.measurement.SpecDecodeUnavailable == "" {
				t.Fatalf("lost provenance: %+v", c.measurement)
			}
			if v.(map[string]any)["choices"].([]any)[0].(map[string]any)["message"].(map[string]any)["content"] != "text" {
				t.Fatal(v)
			}
		})
	}
}
