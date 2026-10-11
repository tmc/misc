package llmperf

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestResponsesVendorStatus(t *testing.T) {
	for _, status := range []string{"completed", "failed", "incomplete", "in_progress", "cancelled"} {
		r, err := decodeResponse([]byte(`{"status":"`+status+`","output":[{"type":"message","content":[{"text":"hello"}]}]}`), "openai-responses")
		if (err == nil) != (status == "completed") {
			t.Fatalf("%s: %v", status, err)
		}
		if r.ProviderStatus != status || r.Status != 0 {
			t.Fatalf("lost provider status: %+v", r)
		}
	}
	for _, data := range []string{`{"status":200}`, `{"status":true}`, `{"status":{}}`} {
		if _, err := decodeResponse([]byte(data), "openai-responses"); err == nil {
			t.Fatal("accepted invalid status", data)
		}
	}
}

func TestResponsesReasoningAndTerminal(t *testing.T) {
	for _, kind := range []string{"response.reasoning_text.delta", "response.reasoning_summary_text.delta"} {
		r, done, err := decodeChunk([]byte(`{"type":"`+kind+`","delta":"thought"}`), "openai-responses")
		if err != nil || done || r.Choices[0].Delta.ReasoningContent != "thought" || r.Choices[0].Delta.Content != "" {
			t.Fatalf("%s: %+v %v", kind, r, err)
		}
	}
	for _, status := range []string{"completed", "failed", "incomplete"} {
		r, done, err := decodeChunk([]byte(`{"type":"response.`+status+`","response":{"status":"`+status+`","usage":{"output_tokens":7}}}`), "openai-responses")
		if (err == nil) != (status == "completed") || done != (status == "completed") || r.ProviderStatus != status {
			t.Fatalf("%s: %+v %v %v", status, r, done, err)
		}
	}
	if _, _, err := decodeChunk([]byte(`{"type":"response.completed","response":{"status":"incomplete"}}`), "openai-responses"); err == nil {
		t.Fatal("accepted conflicting terminal status")
	}
}

func TestAnthropicStopReasons(t *testing.T) {
	for _, stop := range []string{"end_turn", "tool_use", "max_tokens"} {
		r, err := decodeResponse([]byte(`{"stop_reason":"`+stop+`","content":[{"type":"text","text":"hello"}]}`), "anthropic")
		if err != nil || r.Choices[0].FinishReason != stop {
			t.Fatalf("sync %s: %+v %v", stop, r, err)
		}
		c, done, err := decodeChunk([]byte(`{"type":"message_delta","delta":{"stop_reason":"`+stop+`"},"usage":{"output_tokens":4}}`), "anthropic")
		if err != nil || done || c.Choices[0].FinishReason != stop || c.Usage.CompletionTokens != 4 {
			t.Fatalf("stream %s: %+v %v", stop, c, err)
		}
	}
}

func TestAudioDeclaredDecoder(t *testing.T) {
	for _, text := range []string{"42", "true", "null", `"quoted"`, `{}`, `{"text":"authored transcript"}`, "", "日本語"} {
		r, err := decodeAudioResponseFormat([]byte(text), "text", "text/plain; charset=utf-8")
		if err != nil || messageText(r.Choices[0].Message.Content) != text {
			t.Fatalf("%q: %+v %v", text, r, err)
		}
		var body string
		if err := json.Unmarshal(r.Body, &body); err != nil || body != text {
			t.Fatalf("body %q: %v", r.Body, err)
		}
	}
	for _, tt := range []struct{ body, format, media string }{
		{"42", "json", "application/json"}, {`{"text":42}`, "json", "application/json"}, {`{"error":"denied"}`, "json", "application/json"},
		{`{"text":"hello"}`, "text", "application/json"}, {"hello", "json", "text/plain"}, {"hello", "srt", "text/plain"}, {"hello", "text", "application/octet-stream"},
	} {
		if _, err := decodeAudioResponseFormat([]byte(tt.body), tt.format, tt.media); err == nil {
			t.Fatalf("accepted %+v", tt)
		}
	}
	for _, format := range []string{"json", "verbose_json", ""} {
		r, err := decodeAudioResponseFormat([]byte(`{"text":"hello"}`), format, "application/json")
		if err != nil || messageText(r.Choices[0].Message.Content) != "hello" {
			t.Fatal(r, err)
		}
	}
	if _, err := decodeAudioResponseFormat([]byte(strings.Repeat("a", modalityLimit+1)), "text", "text/plain"); err == nil {
		t.Fatal("accepted oversized transcript")
	}
	if _, err := decodeAudioResponseFormat([]byte{0xff}, "text", "text/plain"); err == nil {
		t.Fatal("accepted invalid UTF-8")
	}
}

func TestProviderAdapterInvalidTypes(t *testing.T) {
	for _, tt := range []struct{ protocol, data string }{
		{"anthropic", `{"stop_reason":7,"content":[{"text":"hi"}]}`},
		{"openai-responses", `{"output":[{"content":[{"text":7}]}]}`},
		{"openai-responses", `{"error":{"message":"no"},"output":[]}`},
		{"openai-chat", `{"error":{"message":"no"},"choices":[{"message":{"content":"partial"}}]}`},
	} {
		if _, err := decodeResponse([]byte(tt.data), tt.protocol); err == nil {
			t.Fatalf("accepted %+v", tt)
		}
	}
	for _, tt := range []struct{ protocol, data string }{
		{"anthropic", `{"type":"message_delta","delta":{"stop_reason":7}}`},
		{"openai-responses", `{"type":"response.reasoning_text.delta","delta":7}`},
		{"openai-responses", `{"type":"response.completed"}`},
		{"openai-responses", `{"type":"response.completed","response":{"status":7}}`},
	} {
		if _, _, err := decodeChunk([]byte(tt.data), tt.protocol); err == nil {
			t.Fatalf("accepted %+v", tt)
		}
	}
	r, err := decodeResponse([]byte(`{"status":"completed","output":[{"type":"reasoning","content":[{"type":"reasoning_text","text":"thought"}]},{"type":"function_call","call_id":"call1","name":"lookup","arguments":"{}"}]}`), "openai-responses")
	if err != nil || len(r.Choices) != 1 || r.Choices[0].Message.ReasoningContent != "thought" || len(r.Choices[0].Message.ToolCalls) != 1 {
		t.Fatal(r, err)
	}
}

func TestResponsesTerminalSpecDecode(t *testing.T) {
	valid := `{"num_accepted_draft_tokens":1,"num_draft_tokens":2,"num_spec_steps":1,"acceptance_histogram":[0,1]}`
	other := `{"num_accepted_draft_tokens":0,"num_draft_tokens":2,"num_spec_steps":1,"acceptance_histogram":[1,0]}`
	for _, tt := range []struct {
		name, root, terminal string
		unavailable          bool
	}{
		{"terminal only", "null", valid, false}, {"identical", valid, valid, false}, {"conflicting", valid, other, true}, {"malformed root", `{}`, valid, true}, {"malformed terminal", valid, `{}`, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			data := `{"type":"response.completed","metrics":{"speculative_decoding":` + tt.root + `},"response":{"status":"completed","metrics":{"speculative_decoding":` + tt.terminal + `}}}`
			chunk, done, err := decodeChunk([]byte(data), "openai-responses")
			if err != nil || !done {
				t.Fatalf("text response failed: %v", err)
			}
			if (chunk.specDecodeUnavailable != "") != tt.unavailable {
				t.Fatalf("diagnostic: %q", chunk.specDecodeUnavailable)
			}
			if !tt.unavailable {
				value, err := parseSpecDecode(chunk.Metrics.SpeculativeDecoding)
				if err != nil || value == nil || value.Accepted != 1 {
					t.Fatal(value, err)
				}
			}
		})
	}
}

func TestAnthropicUsageOnlyMessageDelta(t *testing.T) {
	for _, wire := range []string{
		`{"type":"message_delta","usage":{"output_tokens":3}}`,
		`{"type":"message_delta","delta":{},"usage":{"output_tokens":3}}`,
		`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":3}}`,
	} {
		chunk, done, err := decodeChunk([]byte(wire), "anthropic")
		if err != nil || done || !chunk.Usage.completionPresent || chunk.Usage.CompletionTokens != 3 {
			t.Fatalf("%s: %+v done=%t err=%v", wire, chunk, done, err)
		}
	}
	for _, wire := range []string{
		`{"type":"message_delta","delta":7,"usage":{"output_tokens":3}}`,
		`{"type":"message_delta","delta":{"stop_reason":7},"usage":{"output_tokens":3}}`,
	} {
		if _, _, err := decodeChunk([]byte(wire), "anthropic"); err == nil {
			t.Fatalf("accepted invalid delta %s", wire)
		}
	}
}
