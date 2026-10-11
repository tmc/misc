package llmperf

import (
	"strings"
	"testing"
)

func TestResponsesTerminalEnvelope(t *testing.T) {
	for _, event := range []string{"response.completed", "response.incomplete", "response.failed"} {
		for _, tt := range []struct {
			name, response string
			valid          bool
		}{
			{"missing", "", false}, {"null", "null", false}, {"number", "7", false}, {"string", `"secret provider text"`, false}, {"boolean", "true", false}, {"array", "[]", false},
			{"nested error", `{"error":{"message":"secret provider text"}}`, false}, {"scalar error", `{"error":"secret provider text"}`, false},
			{"compatible object", `{}`, event == "response.completed"}, {"null error", `{"error":null}`, event == "response.completed"}, {"completed object", `{"status":"completed","error":null,"usage":{"input_tokens":2,"output_tokens":3}}`, event == "response.completed"},
		} {
			t.Run(event+"/"+tt.name, func(t *testing.T) {
				data := `{"type":"` + event + `"`
				if tt.response != "" {
					data += `,"response":` + tt.response
				}
				data += `}`
				_, done, err := decodeChunk([]byte(data), "openai-responses")
				if (err == nil) != tt.valid || done != tt.valid {
					t.Fatalf("done=%t err=%v", done, err)
				}
				if err != nil && (len(err.Error()) > 256 || strings.Contains(err.Error(), "secret provider text")) {
					t.Fatalf("unsafe diagnostic: %v", err)
				}
			})
		}
	}
}
