package llmperf

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

var ambiguousAnthropicUsage = []string{
	`{"input_tokens":2,"output_tokens":1,"cache_creation_input_tokens":-1,"cache_creation_input_tokens":4}`,
	`{"input_tokens":2,"output_tokens":1,"cache_read_input_tokens":3,"cache_read_input_tokens":0}`,
	`{"input_tokens":2,"output_tokens":1,"cache_creation_input_tokens":-1,"CACHE_CREATION_INPUT_TOKENS":4}`,
	`{"input_tokens":2,"INPUT_TOKENS":3,"output_tokens":1}`,
	`{"input_tokens":2,"output_tokens":-1,"OUTPUT_TOKENS":1}`,
	`{"input_tokens":2,"cache_read_input_tokens":3,"cache_read_input_\u0074okens":0}`,
}

func TestAnthropicDuplicateAdmission(t *testing.T) {
	for i, usage := range ambiguousAnthropicUsage {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			if _, err := canonicalUsage(json.RawMessage(usage), "anthropic"); err == nil {
				t.Fatal("direct usage admitted duplicates")
			}
			body := fmt.Sprintf(`{"content":[{"type":"text","text":"hello"}],"usage":%s}`, usage)
			if _, err := decodeResponse([]byte(body), "anthropic"); err == nil {
				t.Fatal("unary admitted duplicates")
			}
			for _, body := range []string{fmt.Sprintf(`{"type":"message_start","message":{"usage":%s}}`, usage), fmt.Sprintf(`{"type":"message_delta","usage":%s}`, usage)} {
				if _, _, err := decodeChunk([]byte(body), "anthropic"); err == nil {
					t.Fatal("stream admitted duplicates")
				}
			}
		})
	}
	for _, body := range []string{
		`{"usage":{"input_tokens":-1},"usage":{"input_tokens":2}}`,
		`{"usage":{"input_tokens":-1},"USAGE":{"input_tokens":2}}`,
	} {
		if _, err := decodeResponse([]byte(body), "anthropic"); err == nil {
			t.Fatal("unary duplicate container admitted")
		}
	}
	for _, body := range []string{
		`{"type":"message_delta","usage":{"input_tokens":-1},"usage":{"input_tokens":2}}`,
		`{"type":"message_start","message":{"usage":{"input_tokens":-1},"USAGE":{"input_tokens":2}}}`,
		`{"type":"message_start","message":{"usage":{"input_tokens":-1}},"MESSAGE":{"usage":{"input_tokens":2}}}`,
	} {
		if _, _, err := decodeChunk([]byte(body), "anthropic"); err == nil {
			t.Fatal("stream duplicate container admitted")
		}
	}
}

func TestAnthropicClientDuplicateAdmission(t *testing.T) {
	for _, stream := range []bool{false, true} {
		for i, usage := range append(append([]string{}, ambiguousAnthropicUsage...), `{"input_tokens":0,"output_tokens":0,"cache_creation_input_tokens":0}`) {
			t.Run(fmt.Sprintf("stream%v/%d", stream, i), func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if stream {
						w.Header().Set("Content-Type", "text/event-stream")
						fmt.Fprintf(w, "data: {\"type\":\"message_start\",\"message\":{\"usage\":%s}}\n\ndata: {\"type\":\"message_stop\"}\n\n", usage)
					} else {
						fmt.Fprintf(w, `{"content":[{"type":"text","text":"hello"}],"stop_reason":"end_turn","usage":%s}`, usage)
					}
				}))
				defer server.Close()
				c := setupTestClient(t, server.URL)
				c.config.StrictMeasurement = true
				v := protocolRequest(t, c, CompletionRequest{Stream: stream, Raw: &RawRequest{Protocol: "anthropic", Stream: &stream, Path: "/messages", Body: map[string]any{"messages": []Message{{Role: "user", Content: "hello"}}}}})
				_, failed := v.(error)
				if failed != (i < len(ambiguousAnthropicUsage)) {
					t.Fatalf("response=%+v failed=%v", v, failed)
				}
				if failed && !strings.Contains(v.(error).Error(), "duplicate") {
					t.Fatalf("wrong admission refusal: %v", v)
				}
			})
		}
	}
}

func TestAnthropicOpaqueCaseKeys(t *testing.T) {
	usage := `{"input_tokens":2,"output_tokens":1,"cache_creation_input_tokens":0}`
	body := fmt.Sprintf(`{"metadata":{"A":1,"a":2},"content":[{"type":"tool_use","id":"call","name":"lookup","input":{"A":1,"a":2}}],"usage":%s}`, usage)
	response, err := decodeResponse([]byte(body), "anthropic")
	if err != nil {
		t.Fatal(err)
	}
	var args map[string]int
	if err := json.Unmarshal([]byte(response.Choices[0].Message.ToolCalls[0].Function.Arguments), &args); err != nil || len(args) != 2 {
		t.Fatalf("opaque input changed: %v %v", args, err)
	}
	for _, stream := range []bool{false, true} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !stream {
				fmt.Fprint(w, body)
				return
			}
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprintf(w, "data: {\"type\":\"message_start\",\"message\":{\"usage\":%s}}\n\n", usage)
			fmt.Fprint(w, "data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"tool_use\",\"id\":\"call\",\"name\":\"lookup\",\"input\":{\"A\":1,\"a\":2}}}\n\ndata: {\"type\":\"message_stop\"}\n\n")
		}))
		c := setupTestClient(t, server.URL)
		c.config.StrictMeasurement = true
		v := protocolRequest(t, c, CompletionRequest{Stream: stream, Raw: &RawRequest{Protocol: "anthropic", Stream: &stream, Path: "/messages", Body: map[string]any{"messages": []Message{{Role: "user", Content: "hello"}}}}})
		server.Close()
		if err, failed := v.(error); failed {
			t.Fatalf("opaque keys stream=%v: %v", stream, err)
		}
	}
}
