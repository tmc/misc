package llmperf

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

var ambiguousOpenAIUsage = []string{
	`{"prompt_tokens":-1,"prompt_tokens":4,"completion_tokens":1}`,
	`{"prompt_tokens":1,"Prompt_Tokens":9,"completion_tokens":1}`,
	`{"prompt_tokens":2,"completion_tokens":-1,"COMPLETION_TOKENS":1}`,
	`{"prompt_tokens_details":{"cached_tokens":3,"Cached_Tokens":0}}`,
	`{"completion_tokens_details":{"reasoning_tokens":-1,"reasoning_tokens":1}}`,
	`{"prompt_tokens_details":{"cached_tokens":-1},"Prompt_Tokens_Details":{"cached_tokens":0}}`,
}

func openAIAdmissionBody(protocol, usage string, stream bool) string {
	if protocol == "openai-responses" {
		response := fmt.Sprintf(`{"status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"hello"}]}],"usage":%s}`, usage)
		if stream {
			return fmt.Sprintf(`{"type":"response.completed","response":%s}`, response)
		}
		return response
	}
	if protocol == "openai-completions" {
		return fmt.Sprintf(`{"choices":[{"index":0,"text":"hello","finish_reason":"stop"}],"usage":%s}`, usage)
	}
	if stream {
		return fmt.Sprintf(`{"choices":[{"index":0,"delta":{"content":"hello"},"finish_reason":"stop"}],"usage":%s}`, usage)
	}
	return fmt.Sprintf(`{"choices":[{"index":0,"message":{"content":"hello"},"finish_reason":"stop"}],"usage":%s}`, usage)
}

func TestOpenAIUsageAdmission(t *testing.T) {
	for _, usage := range ambiguousOpenAIUsage {
		var u Usage
		if err := json.Unmarshal([]byte(usage), &u); err == nil {
			t.Fatalf("ambiguous usage admitted: %s", usage)
		}
	}
	for _, protocol := range []string{"openai-chat", "openai-completions", "openai-responses"} {
		for _, stream := range []bool{false, true} {
			for i, usage := range append(append([]string{}, ambiguousOpenAIUsage...), `{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0,"metadata":{"A":1,"a":2},"unknown":null}`) {
				t.Run(fmt.Sprintf("%s/%v/%d", protocol, stream, i), func(t *testing.T) {
					body := openAIAdmissionBody(protocol, usage, stream)
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if stream {
							w.Header().Set("Content-Type", "text/event-stream")
							fmt.Fprintf(w, "data: %s\n\n", body)
							if protocol != "openai-responses" {
								fmt.Fprint(w, "data: [DONE]\n\n")
							}
						} else {
							fmt.Fprint(w, body)
						}
					}))
					defer server.Close()
					c := setupTestClient(t, server.URL)
					c.config.StrictMeasurement = true
					v := protocolRequest(t, c, CompletionRequest{Raw: &RawRequest{Protocol: protocol, Stream: &stream, Path: "/work", Body: map[string]any{"messages": []Message{{Role: "user", Content: "hello"}}}}})
					err, failed := v.(error)
					want := i < len(ambiguousOpenAIUsage)
					if failed != want || failed && !strings.Contains(err.Error(), "duplicate") {
						t.Fatalf("response=%+v wantfail=%v", v, want)
					}
				})
			}
		}
	}
	for _, protocol := range []string{"openai-chat", "openai-completions", "openai-responses"} {
		body := `{"usage":{"prompt_tokens":-1},"USAGE":{"prompt_tokens":2}}`
		if _, err := decodeResponse([]byte(body), protocol); err == nil {
			t.Fatal("outer usage alias admitted")
		}
		if _, _, err := decodeChunk([]byte(body), protocol); err == nil {
			t.Fatal("stream usage alias admitted")
		}
	}
	for _, body := range []string{
		`{"type":"response.completed","response":{"usage":{"input_tokens":-1},"Usage":{"input_tokens":2}}}`,
		`{"type":"response.completed","response":{"usage":{"input_tokens":-1}},"Response":{"usage":{"input_tokens":2}}}`,
	} {
		if _, _, err := decodeChunk([]byte(body), "openai-responses"); err == nil {
			t.Fatal("nested Responses alias admitted")
		}
	}
	for _, usage := range []string{`{"input_tokens":2,"prompt_tokens":3}`, `{"output_tokens":2,"completion_tokens":3}`, `{"input_tokens_details":{"cached_tokens":2},"prompt_tokens_details":{"cached_tokens":3}}`, `{"output_tokens_details":{"reasoning_tokens":2},"completion_tokens_details":{"reasoning_tokens":3}}`, `{"input_tokens":2,"prompt_tokens":-1}`, `{"input_tokens":1,"Prompt_Tokens":9}`} {
		if _, err := canonicalUsage(json.RawMessage(usage), "openai-responses"); err == nil {
			t.Fatal("contradictory native alias admitted")
		}
	}
	for _, usage := range []string{`{}`, `null`, `{"prompt_tokens":null,"completion_tokens":null}`, `{"prompt_tokens":0,"completion_tokens":0}`, `{"input_tokens":2,"prompt_tokens":2,"output_tokens":0,"completion_tokens":0}`} {
		if _, err := canonicalUsage(json.RawMessage(usage), "openai-responses"); err != nil {
			t.Fatalf("presence semantics changed: %s %v", usage, err)
		}
	}
}

func TestResponsesCaseAndDetailAliases(t *testing.T) {
	u, err := canonicalUsage(json.RawMessage(`{"Input_Tokens":2,"Output_Tokens":1,"Input_Tokens_Details":{"Cached_Tokens":1},"Output_Tokens_Details":{"Reasoning_Tokens":0}}`), "openai-responses")
	if err != nil || !u.promptPresent || u.PromptTokens != 2 || u.CompletionTokens != 1 || !sameCount(u.CachedTokens, intPtr(1)) || !sameCount(u.ReasoningTokens, intPtr(0)) {
		t.Fatalf("singlecase identity: %+v %v", u, err)
	}
	u, err = canonicalUsage(json.RawMessage(`{"input_tokens":2,"prompt_tokens":2,"input_tokens_details":{"cached_tokens":1,"opaque":{"A":1,"a":2}},"prompt_tokens_details":{"cached_tokens":1},"output_tokens_details":{"reasoning_tokens":0},"completion_tokens_details":{"reasoning_tokens":0}}`), "openai-responses")
	if err != nil || !sameCount(u.CachedTokens, intPtr(1)) || !sameCount(u.ReasoningTokens, intPtr(0)) {
		t.Fatalf("consistentdetail identity: %+v %v", u, err)
	}
}
