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

var ambiguousAnthropicParts = []struct {
	stream bool
	body   string
}{
	{true, `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"a","Text":"b"}}`},
	{true, `{"type":"message_delta","delta":{"stop_reason":"end_turn","Stop_Reason":"max_tokens"},"usage":{"output_tokens":3}}`},
	{true, `{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"c","name":"a","Name":"b","input":{}}}`},
	{false, `{"content":[{"type":"text","text":"a","TEXT":"b"}],"usage":{"input_tokens":0,"output_tokens":0}}`},
}

func TestAnthropicNestedFixedAliases(t *testing.T) {
	for i, tt := range ambiguousAnthropicParts {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			if err := checkAnthropicEnvelope([]byte(tt.body)); err == nil || !strings.Contains(err.Error(), "duplicate") {
				t.Fatalf("nested alias: %v", err)
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.stream {
					w.Header().Set("Content-Type", "text/event-stream")
					fmt.Fprintf(w, "data: %s\n\ndata: {\"type\":\"message_stop\"}\n\n", tt.body)
				} else {
					fmt.Fprint(w, tt.body)
				}
			}))
			defer server.Close()
			c := setupTestClient(t, server.URL)
			c.config.StrictMeasurement = true
			v := protocolRequest(t, c, CompletionRequest{Raw: &RawRequest{Protocol: "anthropic", Stream: &tt.stream, Path: "/messages", Body: map[string]any{"messages": []Message{{Role: "user", Content: "hello"}}}}})
			err, failed := v.(error)
			if !failed || !strings.Contains(err.Error(), "duplicate") {
				t.Fatalf("actualclient admission: %+v", v)
			}
		})
	}
	for _, body := range []string{
		`{"type":"error","error":{"type":"a","TYPE":"b"}}`,
		`{"type":"error","error":{"message":"a","Message":"b"}}`,
	} {
		if err := checkAnthropicEnvelope([]byte(body)); err == nil {
			t.Fatal("error alias admitted")
		}
	}
	for _, body := range []string{
		`{"type":"content_block_delta","delta":{"type":"text_delta","text":"a"}}`,
		`{"type":"content_block_start","content_block":{"type":"tool_use","id":"c","name":"a","input":{"A":1,"a":2}}}`,
		`{"content":[{"type":"text","text":"a"},{"type":"tool_use","id":"c","name":"a","input":{"A":1,"a":2}}]}`,
		`{"error":{"type":"provider_error","message":"failed"}}`,
	} {
		if err := checkAnthropicEnvelope([]byte(body)); err != nil {
			t.Fatalf("ordinary fixed fields: %v", err)
		}
	}
}

func TestAnthropicNestedFixedFieldsUnchanged(t *testing.T) {
	c, _, err := decodeChunk([]byte(`{"type":"content_block_delta","delta":{"type":"text_delta","text":"a"}}`), "anthropic")
	if err != nil || len(c.Choices) != 1 || c.Choices[0].Delta.Content != "a" {
		t.Fatalf("ordinary text: %+v %v", c, err)
	}
	c, _, err = decodeChunk([]byte(`{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"c","name":"a","input":{"A":1,"a":2}}}`), "anthropic")
	if err != nil || len(c.Choices) != 1 || len(c.Choices[0].Delta.ToolCalls) != 1 || c.Choices[0].Delta.ToolCalls[0].Function.Name != "a" {
		t.Fatalf("ordinary tool: %+v %v", c, err)
	}
	r, err := decodeResponse([]byte(`{"content":[{"type":"text","text":"a"}]}`), "anthropic")
	if err != nil || len(r.Choices) != 1 || r.Choices[0].Message.Content != "a" {
		t.Fatalf("ordinary unary: %+v %v", r, err)
	}
}
