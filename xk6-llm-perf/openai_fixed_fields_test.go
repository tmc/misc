package llmperf

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ambiguousOpenAIFields pair each fixed member with a case alias.
// encoding/json would otherwise keep the last member without error.
var ambiguousOpenAIFields = []struct {
	name     string
	protocol string
	stream   bool
	body     string
}{
	{"choices", "openai-chat", true, `{"choices":[{"index":0,"delta":{"content":"a"}}],"Choices":[{"index":0,"delta":{"content":"b"}}]}`},
	{"error", "openai-chat", false, `{"error":{"message":"x"},"Error":null,"choices":[{"index":0,"message":{"content":"a"}}]}`},
	{"index", "openai-chat", true, `{"choices":[{"index":0,"Index":1,"delta":{"content":"a"}}]}`},
	{"delta", "openai-chat", true, `{"choices":[{"index":0,"delta":{"content":"a"},"Delta":{"content":"b"}}]}`},
	{"finish_reason", "openai-chat", true, `{"choices":[{"index":0,"delta":{},"finish_reason":"stop","Finish_Reason":"length"}]}`},
	{"delta.role", "openai-chat", true, `{"choices":[{"index":0,"delta":{"role":"assistant","ROLE":"user"}}]}`},
	{"delta.content", "openai-chat", true, `{"choices":[{"index":0,"delta":{"content":"a","Content":"bb"}}]}`},
	{"delta.reasoning_content", "openai-chat", true, `{"choices":[{"index":0,"delta":{"reasoning_content":"a","Reasoning_Content":"bb"}}]}`},
	{"delta.reasoning", "openai-chat", true, `{"choices":[{"index":0,"delta":{"reasoning":"a","Reasoning":"bb"}}]}`},
	{"delta.refusal", "openai-chat", true, `{"choices":[{"index":0,"delta":{"refusal":"a","Refusal":"bb"}}]}`},
	{"delta.tool_calls", "openai-chat", true, `{"choices":[{"index":0,"delta":{"tool_calls":[],"Tool_Calls":[{"index":0,"function":{"name":"f"}}]}}]}`},
	{"tool.index", "openai-chat", true, `{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"Index":1,"function":{"name":"f"}}]}}]}`},
	{"tool.id", "openai-chat", true, `{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"a","ID":"b","function":{"name":"f"}}]}}]}`},
	{"tool.type", "openai-chat", true, `{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"type":"function","Type":"x","function":{"name":"f"}}]}}]}`},
	{"tool.function", "openai-chat", true, `{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"name":"f"},"Function":{"name":"g"}}]}}]}`},
	{"function.name", "openai-chat", true, `{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"name":"f","Name":"g"}}]}}]}`},
	{"function.arguments", "openai-chat", true, `{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{}","Arguments":"{\"a\":1}"}}]}}]}`},
	{"message", "openai-chat", false, `{"choices":[{"index":0,"message":{"content":"a"},"Message":{"content":"b"}}]}`},
	{"message.content", "openai-chat", false, `{"choices":[{"index":0,"message":{"content":"a","CONTENT":"bb"}}]}`},
	{"message.finish_reason", "openai-chat", false, `{"choices":[{"index":0,"message":{"content":"a"},"finish_reason":"stop","Finish_reason":"length"}]}`},
	{"message.function.name", "openai-chat", false, `{"choices":[{"index":0,"message":{"tool_calls":[{"id":"c","type":"function","function":{"name":"f","NAME":"g","arguments":"{}"}}]}}]}`},
	{"completions.text", "openai-completions", true, `{"choices":[{"index":0,"text":"a","Text":"bb"}]}`},
	{"completions.finish_reason", "openai-completions", false, `{"choices":[{"index":0,"text":"a","finish_reason":"stop","FINISH_REASON":"length"}]}`},
	{"metrics", "openai-chat", true, `{"metrics":{"speculative_decoding":{"a":1}},"Metrics":{"speculative_decoding":{"b":2}},"choices":[]}`},
	{"metrics.speculative_decoding", "openai-chat", false, `{"metrics":{"speculative_decoding":{"a":1},"Speculative_Decoding":{"a":2}},"choices":[{"index":0,"message":{"content":"a"}}]}`},
	{"completions.metrics", "openai-completions", true, `{"metrics":{"speculative_decoding":{"a":1},"SPECULATIVE_DECODING":{"a":2}},"choices":[{"index":0,"text":"a"}]}`},
	{"responses.type", "openai-responses", true, `{"type":"response.output_text.delta","Type":"response.reasoning_text.delta","delta":"a"}`},
	{"responses.delta", "openai-responses", true, `{"type":"response.output_text.delta","delta":"a","Delta":"bbbb"}`},
	{"responses.output_index", "openai-responses", true, `{"type":"response.function_call_arguments.delta","output_index":0,"Output_Index":1,"delta":"{}"}`},
	{"responses.item", "openai-responses", true, `{"type":"response.output_item.added","item":{"type":"function_call","call_id":"c","name":"f"},"Item":{"type":"message"}}`},
	{"responses.item.type", "openai-responses", true, `{"type":"response.output_item.added","item":{"type":"function_call","Type":"message","call_id":"c","name":"f"}}`},
	{"responses.item.call_id", "openai-responses", true, `{"type":"response.output_item.added","item":{"type":"function_call","call_id":"c","Call_ID":"d","name":"f"}}`},
	{"responses.item.name", "openai-responses", true, `{"type":"response.output_item.added","item":{"type":"function_call","call_id":"c","name":"f","Name":"g"}}`},
	{"responses.response.status", "openai-responses", true, `{"type":"response.completed","response":{"status":"completed","Status":"failed"}}`},
	{"responses.response.metrics", "openai-responses", true, `{"type":"response.completed","response":{"status":"completed","metrics":{"speculative_decoding":{"a":1},"Speculative_decoding":{"a":2}}}}`},
	{"responses.status", "openai-responses", false, `{"status":"completed","Status":"failed","output":[]}`},
	{"responses.stop_reason", "openai-responses", false, `{"status":"completed","stop_reason":"stop","Stop_Reason":"length","output":[{"type":"message","content":[{"type":"output_text","text":"a"}]}]}`},
	{"responses.output", "openai-responses", false, `{"status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"a"}]}],"Output":[{"type":"message","content":[{"type":"output_text","text":"bbbb"}]}]}`},
	{"responses.output.type", "openai-responses", false, `{"status":"completed","output":[{"type":"message","Type":"reasoning","content":[{"type":"output_text","text":"a"}]}]}`},
	{"responses.output.name", "openai-responses", false, `{"status":"completed","output":[{"type":"function_call","call_id":"c","name":"f","Name":"g","arguments":"{}"}]}`},
	{"responses.output.arguments", "openai-responses", false, `{"status":"completed","output":[{"type":"function_call","call_id":"c","name":"f","arguments":"{}","Arguments":"{\"a\":1}"}]}`},
	{"responses.output.summary.text", "openai-responses", false, `{"status":"completed","output":[{"type":"reasoning","summary":[{"text":"a","TEXT":"bb"}]}]}`},
	{"responses.output.content.type", "openai-responses", false, `{"status":"completed","output":[{"type":"message","content":[{"type":"output_text","Type":"reasoning_text","text":"a"}]}]}`},
	{"responses.output.content.text", "openai-responses", false, `{"status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"a","Text":"bbbb"}]}]}`},
}

// unambiguousOpenAIFields keep case-distinct keys only where the schema is
// opaque: extension objects, content parts and function arguments.
var unambiguousOpenAIFields = []struct {
	name     string
	protocol string
	stream   bool
	body     string
}{
	{"stream", "openai-chat", true, `{"id":"x","model":"m","choices":[{"index":0,"delta":{"role":"assistant","content":"hi"},"finish_reason":null}]}`},
	{"tool stream", "openai-chat", true, `{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"c","type":"function","function":{"name":"f","arguments":"{\"A\":1,\"a\":2}"}}]}}]}`},
	{"extensions", "openai-chat", true, `{"system_fingerprint":{"A":1,"a":2},"choices":[{"index":0,"logprobs":{"A":1,"a":2},"delta":{"content":"hi","ext":{"A":1,"a":2}}}]}`},
	{"usage only", "openai-chat", true, `{"choices":[],"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`},
	{"unary", "openai-chat", false, `{"choices":[{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1}}`},
	{"unary parts", "openai-chat", false, `{"choices":[{"index":0,"message":{"role":"assistant","content":[{"type":"text","text":"hi","Text":"x"}],"tool_calls":[{"id":"c","type":"function","function":{"name":"f","arguments":"{}","Extra":{"A":1,"a":2}}}]},"finish_reason":"tool_calls"}]}`},
	{"completions", "openai-completions", true, `{"choices":[{"index":0,"text":"hi","finish_reason":"stop","logprobs":{"A":1,"a":2}}]}`},
	{"metrics", "openai-chat", true, `{"metrics":{"speculative_decoding":{"A":1,"a":2},"ext":{"A":1,"a":2}},"choices":[{"index":0,"delta":{"content":"hi"}}]}`},
	{"responses stream", "openai-responses", true, `{"type":"response.output_text.delta","output_index":0,"delta":"hi","ext":{"A":1,"a":2}}`},
	{"responses tool", "openai-responses", true, `{"type":"response.output_item.added","output_index":0,"item":{"type":"function_call","id":"i","call_id":"c","name":"f","ext":{"A":1,"a":2}}}`},
	{"responses terminal", "openai-responses", true, `{"type":"response.completed","response":{"status":"completed","usage":{"input_tokens":1,"output_tokens":1},"metadata":{"A":1,"a":2}}}`},
	{"responses unary", "openai-responses", false, `{"status":"completed","output":[{"type":"reasoning","summary":[{"text":"r"}]},{"type":"message","content":[{"type":"output_text","text":"hi","annotations":[{"A":1,"a":2}]}]},{"type":"function_call","call_id":"c","name":"f","arguments":"{\"A\":1,\"a\":2}"}],"usage":{"input_tokens":1,"output_tokens":1}}`},
}

func decodeOpenAIFields(protocol string, stream bool, body string) error {
	if stream {
		_, _, err := decodeChunk([]byte(body), protocol)
		return err
	}
	_, err := decodeResponse([]byte(body), protocol)
	return err
}

func TestOpenAIFixedFieldAliases(t *testing.T) {
	for _, tt := range ambiguousOpenAIFields {
		t.Run(tt.name, func(t *testing.T) {
			protocols := []string{tt.protocol}
			if tt.protocol == "openai-chat" {
				protocols = append(protocols, "")
			}
			for _, protocol := range protocols {
				if err := decodeOpenAIFields(protocol, tt.stream, tt.body); err == nil || !strings.Contains(err.Error(), "duplicate provider JSON field") {
					t.Fatalf("protocol %q admitted alias: %v", protocol, err)
				}
			}
		})
	}
	for _, tt := range unambiguousOpenAIFields {
		t.Run(tt.name, func(t *testing.T) {
			if err := decodeOpenAIFields(tt.protocol, tt.stream, tt.body); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestOpenAIClientFixedFieldAliases(t *testing.T) {
	for _, stream := range []bool{false, true} {
		for _, valid := range []bool{false, true} {
			t.Run(fmt.Sprintf("stream=%v/valid=%v", stream, valid), func(t *testing.T) {
				body := `{"choices":[{"index":0,"message":{"content":"a"},"finish_reason":"stop","Finish_Reason":"length"}]}`
				if stream {
					body = `{"choices":[{"index":0,"delta":{"content":"a"},"finish_reason":"stop","Finish_Reason":"length"}]}`
				}
				if valid {
					body = strings.Replace(body, `,"Finish_Reason":"length"`, "", 1)
				}
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if stream {
						w.Header().Set("Content-Type", "text/event-stream")
						fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", body)
					} else {
						fmt.Fprint(w, body)
					}
				}))
				defer server.Close()
				c := setupTestClient(t, server.URL)
				v := protocolRequest(t, c, CompletionRequest{Raw: &RawRequest{Protocol: "openai-chat", Stream: &stream, Path: "/chat", Body: map[string]any{"messages": []Message{{Role: "user", Content: "hello"}}}}})
				err, failed := v.(error)
				if failed == valid || failed && !strings.Contains(err.Error(), "duplicate") {
					t.Fatalf("response=%+v valid=%v", v, valid)
				}
			})
		}
	}
}
