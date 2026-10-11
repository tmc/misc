package llmperf

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestInconsistentFirstChunkFallsBack(t *testing.T) {
	for _, first := range []int{0, 6, 7} {
		origin := time.Now().Add(-time.Second)
		m := &Measurement{start: origin, requestStart: origin, first: origin.Add(time.Millisecond), last: origin.Add(10 * time.Millisecond), reported: Usage{CompletionTokens: 6, completionPresent: true}, counts: &TokenCounts{FirstChunkTokens: intPtr(first)}, Outcome: "success"}
		c := &Client{config: Config{StrictMeasurement: true}}
		c.finishMeasurement(m)
		if m.ITL == nil || *m.ITL != 1.8 || m.DecodeTokens == nil || *m.DecodeTokens != 5 || m.FirstChunkSource != "assumed_one" || m.FirstChunkUnavailable == "" {
			t.Fatalf("first=%d: %+v", first, m)
		}
	}
}

func TestCompletionAdapterRetainsAllChoiceIdentities(t *testing.T) {
	payload := []byte(`{"choices":[{"index":2,"text":"other","finish_reason":"stop"},{"index":0,"text":"primary","finish_reason":"length"}]}`)
	r, err := decodeResponse(payload, "openai-completions")
	if err != nil || len(r.Choices) != 2 || r.Choices[0].Index != 2 || r.Choices[1].Index != 0 {
		t.Fatalf("sync=%+v err=%v", r, err)
	}
	chunk, _, err := decodeChunk(payload, "openai-completions")
	if err != nil || len(chunk.Choices) != 2 || chunk.Choices[0].Index != 2 || chunk.Choices[1].Delta.Content != "primary" {
		t.Fatalf("stream=%+v err=%v", chunk, err)
	}
}

func TestAdditionalProtocolTools(t *testing.T) {
	for _, tt := range []struct {
		protocol, sync string
		events         []string
	}{
		{"anthropic", `{"content":[{"type":"tool_use","id":"call","name":"weather","input":{"city":"Paris"}}],"usage":{"input_tokens":2,"output_tokens":4,"cache_read_input_tokens":1}}`, []string{
			`{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"call","name":"weather","input":{}}}`,
			`{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"city\":"}}`,
			`{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"\"Paris\"}"}}`,
			`{"type":"message_stop"}`,
		}},
		{"openai-responses", `{"output":[{"type":"reasoning","summary":[{"text":"checking"}]},{"type":"function_call","call_id":"call","name":"weather","arguments":"{\"city\":\"Paris\"}"}],"usage":{"input_tokens":2,"output_tokens":4}}`, []string{
			`{"type":"response.output_item.added","output_index":0,"item":{"type":"function_call","call_id":"call","name":"weather"}}`,
			`{"type":"response.function_call_arguments.delta","output_index":0,"delta":"{\"city\":"}`,
			`{"type":"response.function_call_arguments.delta","output_index":0,"delta":"\"Paris\"}"}`,
			`{"type":"response.completed","response":{"usage":{"output_tokens":4}}}`,
		}},
	} {
		t.Run(tt.protocol, func(t *testing.T) {
			r, err := decodeResponse([]byte(tt.sync), tt.protocol)
			if err != nil || len(r.Choices) != 1 || len(r.Choices[0].Message.ToolCalls) != 1 {
				t.Fatalf("sync=%+v err=%v", r, err)
			}
			if err := validateCompletedTools(r.Choices); err != nil {
				t.Fatal(err)
			}
			if tt.protocol == "openai-responses" && r.Choices[0].Message.ReasoningContent != "checking" {
				t.Fatal("reasoning discarded")
			}
			if tt.protocol == "anthropic" && (r.Usage.CachedTokens == nil || *r.Usage.CachedTokens != 1) {
				t.Fatal("cache count discarded")
			}
			choices := streamChoices{}
			for i, event := range tt.events {
				chunk, terminal, err := decodeChunk([]byte(event), tt.protocol)
				if err != nil {
					t.Fatal(err)
				}
				if terminal != (i == len(tt.events)-1) {
					t.Fatalf("terminal=%v event=%s", terminal, event)
				}
				if err := choices.add(chunk); err != nil {
					t.Fatal(err)
				}
			}
			got := choices.choices()
			if err := validateCompletedTools(got); err != nil {
				t.Fatal(err)
			}
			tool := got[0].Message.ToolCalls[0]
			if tool.ID != "call" || tool.Function.Name != "weather" || tool.Function.Arguments != `{"city":"Paris"}` {
				t.Fatalf("tool=%+v", tool)
			}
		})
	}
}

func TestCompletedToolArgumentSafety(t *testing.T) {
	for _, args := range []string{`{`, `[]`, `null`, `{"city":`} {
		choice := Choice{Message: Message{ToolCalls: []ToolCall{normalizedTool(0, "call", "weather", args)}}}
		if validateCompletedTools([]Choice{choice}) == nil {
			t.Fatalf("accepted %s", args)
		}
	}
	// Fragments are valid transport data until a terminal marker arrives.
	chunk, _, err := decodeChunk([]byte(`{"type":"response.function_call_arguments.delta","output_index":0,"delta":"{"}`), "openai-responses")
	if err != nil || chunk.Choices[0].Delta.ToolCalls[0].Function.Arguments != "{" {
		t.Fatal("partial fragment rejected")
	}
}

func TestBinaryProtocolTools(t *testing.T) {
	binary := os.Getenv("K6_SMOKE_BINARY")
	if binary == "" {
		t.Skip("set K6_SMOKE_BINARY to qualify protocol tool fixtures")
	}
	binary, err := filepath.Abs(binary)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Stream bool `json:"stream"`
		}
		if json.NewDecoder(r.Body).Decode(&body) != nil {
			t.Error("invalid body")
		}
		if !body.Stream {
			if r.URL.Path == "/messages" {
				fmt.Fprint(w, `{"content":[{"type":"tool_use","id":"call","name":"weather","input":{"city":"Paris"}}]}`)
			} else {
				fmt.Fprint(w, `{"output":[{"type":"function_call","call_id":"call","name":"weather","arguments":"{\"city\":\"Paris\"}"}]}`)
			}
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		if r.URL.Path == "/messages" {
			for _, event := range []string{`{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"call","name":"weather","input":{}}}`, `{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"city\":\"Paris\"}"}}`, `{"type":"message_stop"}`} {
				fmt.Fprintf(w, "data: %s\n\n", event)
			}
		} else {
			args := `{\"city\":\"Paris\"}`
			if r.URL.Path == "/bad" {
				args = "{"
			}
			for _, event := range []string{`{"type":"response.output_item.added","output_index":0,"item":{"type":"function_call","call_id":"call","name":"weather"}}`, `{"type":"response.function_call_arguments.delta","output_index":0,"delta":"` + args + `"}`, `{"type":"response.completed","response":{}}`} {
				fmt.Fprintf(w, "data: %s\n\n", event)
			}
		}
	}))
	defer server.Close()
	script := `import llm from 'k6/x/llm-perf';import {check} from 'k6';
 export const options={vus:1,iterations:1,thresholds:{checks:['rate==1'],llm_successes:['count==4'],llm_failures:['count==1']}};
 const c=new llm.Client({baseURL:__ENV.ENDPOINT_URL,strictMeasurement:true});
 export default function(){for(const protocol of ['anthropic','openai-responses'])for(const stream of [false,true]){
 const r=c.request({path:protocol==='anthropic'?'/messages':'/responses',protocol,body:{stream}});
 check(r,{tools:r=>!(r instanceof Error)&&r.choices[0].message.tool_calls[0].function.arguments==='{"city":"Paris"}'&&r.measurement.output_tokens.value===null, timing:r=>!stream||r.measurement.ttft!==null&&r.measurement.ttfo!==null});
 }const bad=c.request({path:'/bad',protocol:'openai-responses',body:{stream:true}});check(bad,{invalid:r=>r instanceof Error&&r.measurement.outcome==='failure'});}`
	path := filepath.Join(t.TempDir(), "tools.js")
	if err := os.WriteFile(path, []byte(script), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "run", "--no-color", path)
	cmd.Env = append(os.Environ(), "ENDPOINT_URL="+server.URL, "K6_NO_USAGE_REPORT=true", "K6_AUTO_EXTENSION_RESOLUTION=false")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("protocol tools: %v\n%s", err, out)
	}
}

func TestScheduledAdmissionFailureHasNoDispatchLag(t *testing.T) {
	origin := time.Unix(100, 0)
	scheduled := millis(origin.Add(-time.Millisecond))
	m := &Measurement{start: origin, ScheduledAt: &scheduled, responseEnd: origin.Add(10 * time.Millisecond), Outcome: "failure", ErrorStage: "admission"}
	c := &Client{config: Config{StrictMeasurement: true}}
	c.finishMeasurement(m)
	if m.ArrivalLag != nil || m.ScheduledResponseLatency == nil || *m.ScheduledResponseLatency != 11 || m.Attempted {
		t.Fatalf("admission measurement=%+v", m)
	}
}
