package llmperf

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/grafana/sobek"
	"go.k6.io/k6/v2/metrics"
)

func TestResponseProtocols(t *testing.T) {
	for _, tt := range []struct {
		protocol, body, text string
		tokens               int
	}{
		{"openai-completions", `{"choices":[{"text":"hello"}],"usage":{"completion_tokens":2}}`, "hello", 2},
		{"openai-responses", `{"output":[{"content":[{"type":"output_text","text":"hello"}]}],"usage":{"input_tokens":3,"output_tokens":2}}`, "hello", 2},
		{"anthropic", `{"content":[{"type":"text","text":"hello"}],"usage":{"input_tokens":3,"output_tokens":2}}`, "hello", 2},
		{"json", `{"data":[{"embedding":[0.1,0.2]}],"usage":{"prompt_tokens":3}}`, "", 0},
	} {
		t.Run(tt.protocol, func(t *testing.T) {
			r, err := decodeResponse([]byte(tt.body), tt.protocol)
			if err != nil {
				t.Fatal(err)
			}
			if tt.text != "" && messageText(r.Choices[0].Message.Content) != tt.text {
				t.Fatalf("response=%+v", r)
			}
			if r.Usage.CompletionTokens != tt.tokens {
				t.Fatalf("usage=%+v", r.Usage)
			}
			if tt.protocol == "json" && (len(r.Body) == 0 || r.Usage.completionPresent) {
				t.Fatal("non-text response invented tokens")
			}
		})
	}
}

func TestStreamingProtocols(t *testing.T) {
	for _, tt := range []struct {
		protocol, body, text string
		done                 bool
	}{
		{"openai-completions", `{"choices":[{"text":"hello"}]}`, "hello", false},
		{"openai-responses", `{"type":"response.output_text.delta","delta":"hello"}`, "hello", false},
		{"openai-responses", `{"type":"response.completed","response":{"id":"done","usage":{"output_tokens":2}}}`, "", true},
		{"anthropic", `{"type":"content_block_delta","delta":{"type":"text_delta","text":"hello"}}`, "hello", false},
		{"anthropic", `{"type":"message_stop"}`, "", true},
	} {
		chunk, done, err := decodeChunk([]byte(tt.body), tt.protocol)
		if err != nil {
			t.Fatal(err)
		}
		if done != tt.done || chunk.Choices[0].Delta.Content != tt.text {
			t.Fatalf("%s chunk=%+v done=%v", tt.protocol, chunk, done)
		}
	}
}

func TestRawRequestMeasured(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/embeddings" {
			t.Errorf("path=%s", r.URL.Path)
		}
		fmt.Fprint(w, `{"data":[{"embedding":[0.1,0.2]}],"usage":{"prompt_tokens":3}}`)
	}))
	defer server.Close()
	c := setupTestClient(t, server.URL)
	c.config.StrictMeasurement = true
	samples := make(chan metrics.SampleContainer, 100)
	c.metrics.samples = samples
	result := c.Complete(sobek.FunctionCall{Arguments: []sobek.Value{c.vu.Runtime().ToValue(CompletionRequest{Raw: &RawRequest{Path: "/embeddings", Protocol: "json", Body: map[string]any{"input": "hello"}}})}})
	if err, ok := result.Export().(error); ok {
		t.Fatal(err)
	}
	if c.measurement.Outcome != "success" || c.measurement.OutputTokens.Value != nil || *c.measurement.InputTokens.Value != 3 {
		t.Fatalf("record=%+v", c.measurement)
	}
}

func TestRawPathRejectsExfiltration(t *testing.T) {
	for _, path := range []string{"https://evil.test/", "//evil.test/", "/../admin", "/test?key=secret"} {
		if validateRaw(&RawRequest{Path: path, Protocol: "json", Body: map[string]any{}}) == nil {
			t.Fatalf("accepted %q", path)
		}
	}
}

func TestCompletionChunkPreservesVendorStats(t *testing.T) {
	payload := []byte(`{"choices":[{"index":0,"text":"hello","finish_reason":"stop","speculative_decoding":{"total_accepted_draft_tokens":1,"total_draft_tokens":2,"num_spec_steps":1,"acceptance_histogram":[0,1]}}],"metrics":{"speculative_decoding":{"num_accepted_draft_tokens":1,"num_draft_tokens":2,"num_spec_steps":1,"acceptance_histogram":[0,1]}}}`)
	chunk, _, err := decodeChunk(payload, "openai-completions")
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range []json.RawMessage{chunk.Metrics.SpeculativeDecoding, chunk.Choices[0].SpeculativeDecoding} {
		if _, err := parseSpecDecode(raw); err != nil {
			t.Fatal(err)
		}
	}
	if chunk.Choices[0].Delta.Content != "hello" || chunk.Choices[0].Index != 0 {
		t.Fatalf("chunk=%+v", chunk)
	}
}
