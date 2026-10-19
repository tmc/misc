package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestRetainedQwenTemplate(t *testing.T) {
	data, err := os.ReadFile("testdata/qwen3-ollama-0.12.7.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != "ae370d884f108d16e7cc8fd5259ebc5773a0afa6e078b11f4ed7e39a27e0dfc4" {
		t.Fatal("retained template identity changed")
	}
	s := show{Template: string(data), Capabilities: []string{"completion", "tools", "thinking"}}
	for _, tt := range []struct {
		name, effort, want string
		think              bool
	}{
		{"default", "", "<|im_start|>user\nSay hello in one short sentence. /no_think /think<|im_end|>\n<|im_start|>assistant\n", true},
		{"explicit none", "none", "<|im_start|>user\nSay hello in one short sentence. /no_think /no_think<|im_end|>\n<|im_start|>assistant\n<think>\n\n</think>\n\n", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := request{Model: "qwen3:0.6b", Messages: []message{{"user", "Say hello in one short sentence. /no_think"}}, ReasoningEffort: tt.effort}
			got, think, err := render(s, r)
			if err != nil || think != tt.think || got != tt.want {
				t.Fatalf("prompt=%q think=%v err=%v", got, think, err)
			}
		})
	}
}

func TestThinkingFraming(t *testing.T) {
	data, err := os.ReadFile("testdata/qwen3-ollama-0.12.7.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	s := show{Template: string(data), Capabilities: []string{"thinking"}}
	for _, tt := range []struct {
		name, effort string
		think        bool
		suffix       string
	}{{"default", "", true, " /no_think /think"}, {"disabled", "none", false, " /no_think /no_think"}} {
		t.Run(tt.name, func(t *testing.T) {
			r := request{Model: "qwen3:0.6b", Messages: []message{{"user", "hello /no_think"}}, ReasoningEffort: tt.effort}
			text, think, err := render(s, r)
			if err != nil || think != tt.think || !strings.Contains(text, tt.suffix) {
				t.Fatalf("text=%q think=%v err=%v", text, think, err)
			}
		})
	}
	for _, r := range []request{{Model: "qwen", Messages: []message{{"user", "hello"}, {"user", "extra"}}}, {Model: "qwen", Messages: []message{{"system", "hello"}}}, {Model: "qwen", Messages: []message{{"user", "hello"}}, ReasoningEffort: "invalid"}} {
		if _, _, err := render(s, r); err == nil {
			t.Fatal("unsupported request accepted")
		}
	}
	s.Capabilities = nil
	if _, _, err := render(s, request{Model: "qwen", Messages: []message{{"user", "hello"}}}); err == nil {
		t.Fatal("missing capability accepted")
	}
}

func TestRequestAdmission(t *testing.T) {
	for _, input := range []string{
		`{"model":"qwen","model":"other","messages":[]}`,
		`{"model":"qwen","Model":"other","messages":[]}`,
		`{"model":"qwen","messages":[{"role":"user","content":null}]}`,
		`{"model":"qwen","messages":[{"role":"user","content":"a","images":["x"]}]}`,
		`{"model":"qwen","messages":[{"role":"user","content":"a","name":"alias"}]}`,
		`{"model":"qwen","messages":[{"role":"user","content":"a","tool_calls":[]}]}`,
		`{"model":"qwen","messages":[{"role":"user","content":"a"}],"thinking":false}`,
		`{"model":"qwen","messages":[{"role":"user","content":"a"}],"temperature":NaN}`,
		"{\"model\":\"qwen\",\"messages\":[{\"role\":\"user\",\"content\":\"\xff\"}]}",
	} {
		if err := decodeJSON([]byte(input), new(request), true, false); err == nil {
			t.Fatal("unsupported request accepted")
		}
	}
	valid := `{"model":"qwen","messages":[{"role":"user","content":"a"}],"max_tokens":256,"stream":true,"temperature":0,"stream_options":{"include_usage":true}}`
	if err := decodeJSON([]byte(valid), new(request), true, false); err != nil {
		t.Fatal(err)
	}
	s := show{Template: `{{printf "%1000000000s" "x"}}`, Capabilities: []string{"thinking"}}
	if _, _, err := render(s, request{Model: "qwen", Messages: []message{{"user", "a"}}}); err == nil {
		t.Fatal("unrecognized template accepted")
	}
}

func TestModelDefaultsAdmission(t *testing.T) {
	data, err := os.ReadFile("testdata/qwen3-ollama-0.12.7.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	r := request{Model: "qwen3:0.6b", Messages: []message{{"user", "hello /no_think"}}}
	base := show{Template: string(data), Capabilities: []string{"thinking"}}
	for _, tt := range []struct {
		name, system, messages string
		valid                  bool
	}{
		{"absent", "", "", true}, {"empty", `""`, `[]`, true},
		{"system", "\"prior instructions\"", "", true}, {"messages", "", `[{"role":"assistant","content":"prior"}]`, true},
		{"null system", "null", "", true}, {"null messages", "", "null", true},
		{"array system", "[]", "", false}, {"object messages", "", "{}", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := base
			s.System = []byte(tt.system)
			s.Messages = []byte(tt.messages)
			_, _, err := render(s, r)
			if (err == nil) != tt.valid {
				t.Fatalf("valid=%v err=%v", tt.valid, err)
			}
		})
	}
}

func TestReasoningEffortAdmission(t *testing.T) {
	data, err := os.ReadFile("testdata/qwen3-ollama-0.12.7.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	s := show{Template: string(data), Capabilities: []string{"thinking"}}
	for _, effort := range []string{"low", "medium", "high"} {
		for _, nested := range []bool{false, true} {
			r := request{Model: "qwen3:0.6b", Messages: []message{{"user", "hello"}}}
			if nested {
				r.Reasoning = &struct {
					Effort string `json:"effort"`
				}{effort}
			} else {
				r.ReasoningEffort = effort
			}
			if _, _, err := render(s, r); err == nil {
				t.Fatalf("accepted effort=%s nested=%v", effort, nested)
			}
		}
	}
	// Nested source precedence applies only after both declarations are admitted.
	for _, tt := range []struct {
		top, nested string
		think       bool
	}{{"", "", true}, {"", "none", false}, {"none", "", true}, {"none", "none", false}} {
		r := request{Model: "qwen3:0.6b", Messages: []message{{"user", "hello"}}, ReasoningEffort: tt.top, Reasoning: &struct {
			Effort string `json:"effort"`
		}{tt.nested}}
		if _, think, err := render(s, r); err != nil || think != tt.think {
			t.Fatalf("top=%s nested=%s think=%v err=%v", tt.top, tt.nested, think, err)
		}
	}
}

func TestModelDefaultsAliases(t *testing.T) {
	for _, input := range []string{`{"system":"prior","System":""}`, `{"messages":[{"role":"assistant","content":"prior"}],"Messages":[]}`, `{"messages":[{"role":"assistant","content":"prior"}],"me\u017f\u017fages":[]}`} {
		if err := decodeJSON([]byte(input), new(show), false, true); err == nil {
			t.Fatal("model defaults shadowed by case alias")
		}
	}
}

func TestReconstructedSettings(t *testing.T) {
	zero, tokens := 0.0, uint64(32)
	for _, tt := range []struct {
		name    string
		request request
		want    reconstruction
	}{
		{"omitted", request{}, reconstruction{Think: true, Temperature: 1}},
		{"explicit zero", request{Temperature: &zero, MaxTokens: &tokens, Stream: true}, reconstruction{Think: true, Stream: true, Temperature: 0, MaxTokens: &tokens}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := reconstruct(tt.request, true)
			if got != tt.want {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestMaxTokensParserRange(t *testing.T) {
	data, err := os.ReadFile("testdata/qwen3-ollama-0.12.7.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	s := show{Template: string(data), Capabilities: []string{"thinking"}}
	for _, tt := range []struct {
		tokens uint64
		valid  bool
	}{
		{0, true}, {32, true}, {1<<63 - 1, true}, {1 << 63, false}, {1<<64 - 1, false},
	} {
		r := request{Model: "qwen3:0.6b", Messages: []message{{"user", "hello"}}, MaxTokens: &tt.tokens}
		_, _, err := render(s, r)
		if (err == nil) != tt.valid {
			t.Fatalf("tokens=%d err=%v", tt.tokens, err)
		}
	}
}

func TestUnsupportedServingPipeline(t *testing.T) {
	data, err := os.ReadFile("testdata/qwen3-ollama-0.12.7.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	r := request{Model: "qwen3:0.6b", Messages: []message{{"user", "hello"}}}
	for _, extra := range []string{`"renderer":"qwen3"`, `"parser":"harmony"`, `"remote_host":"https://example.invalid"`, `"remote_model":"other"`} {
		var s show
		if err := json.Unmarshal([]byte("{"+extra+"}"), &s); err != nil {
			t.Fatal(err)
		}
		s.Template, s.Capabilities = string(data), []string{"thinking"}
		if _, _, err := render(s, r); err == nil {
			t.Fatalf("accepted alternate pipeline %s", extra)
		}
	}
}

func TestTextMessageAdmission(t *testing.T) {
	data, err := os.ReadFile("testdata/qwen3-ollama-0.12.7.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	s := show{Template: string(data), Capabilities: []string{"thinking"}}
	for _, tt := range []struct {
		name     string
		messages []message
		tools    string
		valid    bool
	}{
		{"empty", nil, "", false},
		{"tool role", []message{{"tool", "result"}}, "", false},
		{"unknown role", []message{{"developer", "system"}}, "", false},
		{"text history", []message{{"system", "policy"}, {"user", "hi"}, {"assistant", "answer"}, {"user", "new"}}, "", true},
		{"empty tools", []message{{"user", "hi"}}, "[ ]", true},
		{"tools", []message{{"user", "hi"}}, `[{"type":"function"}]`, false},
		{"message cap", make([]message, 65), "", false},
		{"content cap", []message{{"user", strings.Repeat("x", 65537)}}, "", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := render(s, request{Model: "qwen3:0.6b", Messages: tt.messages, Tools: json.RawMessage(tt.tools)})
			if (err == nil) != tt.valid {
				t.Fatalf("valid=%v err=%v", tt.valid, err)
			}
		})
	}
	for _, defaults := range []string{`[{"role":"assistant","content":null}]`, `[{"role":"tool","content":"x"}]`, `[{"role":"assistant","content":"x","thinking":"hidden"}]`} {
		s.Messages = json.RawMessage(defaults)
		if _, _, err := render(s, request{Model: "qwen3:0.6b", Messages: []message{{"user", "hi"}}}); err == nil {
			t.Fatalf("accepted unsupported defaults %s", defaults)
		}
	}
}

// These complete prompts were emitted by the actual template.Parse/Execute and
// thinking.Parser at sourceRevision, not by the reconstruction under test.
// The task-local oracle adapter applies the source's model-default/filter order.
func TestPinnedSourceTextFraming(t *testing.T) {
	const reference = `[
{"name":"system-default","system":"model system","defaults":[],"messages":[{"role":"user","content":"hello"}],"think":true,"prompt":"<|im_start|>system\n\nmodel system<|im_end|>\n<|im_start|>user\nhello /think<|im_end|>\n<|im_start|>assistant\n"},
{"name":"system-none","system":"model system","defaults":[],"messages":[{"role":"user","content":"hello"}],"think":false,"prompt":"<|im_start|>system\n\nmodel system<|im_end|>\n<|im_start|>user\nhello /no_think<|im_end|>\n<|im_start|>assistant\n<think>\n\n</think>\n\n"},
{"name":"request-system-default","system":"ignored model system","defaults":[],"messages":[{"role":"system","content":"request system"},{"role":"user","content":"hello"}],"think":true,"prompt":"<|im_start|>system\n\nrequest system<|im_end|>\n<|im_start|>user\nhello /think<|im_end|>\n<|im_start|>assistant\n"},
{"name":"request-system-none","system":"ignored model system","defaults":[],"messages":[{"role":"system","content":"request system"},{"role":"user","content":"hello"}],"think":false,"prompt":"<|im_start|>system\n\nrequest system<|im_end|>\n<|im_start|>user\nhello /no_think<|im_end|>\n<|im_start|>assistant\n<think>\n\n</think>\n\n"},
{"name":"model-history-default","system":"model system","defaults":[{"role":"user","content":"old"},{"role":"assistant","content":"answer"}],"messages":[{"role":"user","content":"new"}],"think":true,"prompt":"<|im_start|>system\n\nmodel system<|im_end|>\n<|im_start|>user\nold<|im_end|>\n<|im_start|>assistant\nanswer<|im_end|>\n<|im_start|>user\nnew /think<|im_end|>\n<|im_start|>assistant\n"},
{"name":"model-history-none","system":"model system","defaults":[{"role":"user","content":"old"},{"role":"assistant","content":"answer"}],"messages":[{"role":"user","content":"new"}],"think":false,"prompt":"<|im_start|>system\n\nmodel system<|im_end|>\n<|im_start|>user\nold<|im_end|>\n<|im_start|>assistant\nanswer<|im_end|>\n<|im_start|>user\nnew /no_think<|im_end|>\n<|im_start|>assistant\n<think>\n\n</think>\n\n"},
{"name":"consecutive-users-default","system":"","defaults":[],"messages":[{"role":"user","content":"first"},{"role":"user","content":"second"}],"think":true,"prompt":"<|im_start|>user\nfirst\n\nsecond /think<|im_end|>\n<|im_start|>assistant\n"},
{"name":"consecutive-users-none","system":"","defaults":[],"messages":[{"role":"user","content":"first"},{"role":"user","content":"second"}],"think":false,"prompt":"<|im_start|>user\nfirst\n\nsecond /no_think<|im_end|>\n<|im_start|>assistant\n<think>\n\n</think>\n\n"},
{"name":"consecutive-system-default","system":"","defaults":[{"role":"system","content":"model first"},{"role":"system","content":"model second"}],"messages":[{"role":"system","content":"request"},{"role":"user","content":"new"}],"think":true,"prompt":"<|im_start|>system\n\nmodel first\n\nmodel second\n\nrequest<|im_end|>\n<|im_start|>user\nnew /think<|im_end|>\n<|im_start|>assistant\n"},
{"name":"consecutive-system-none","system":"","defaults":[{"role":"system","content":"model first"},{"role":"system","content":"model second"}],"messages":[{"role":"system","content":"request"},{"role":"user","content":"new"}],"think":false,"prompt":"<|im_start|>system\n\nmodel first\n\nmodel second\n\nrequest<|im_end|>\n<|im_start|>user\nnew /no_think<|im_end|>\n<|im_start|>assistant\n<think>\n\n</think>\n\n"},
{"name":"mixed-system-default","system":"","defaults":[],"messages":[{"role":"user","content":"first"},{"role":"system","content":"middle"},{"role":"assistant","content":"answer"},{"role":"user","content":"new"}],"think":true,"prompt":"<|im_start|>system\n\nmiddle<|im_end|>\n<|im_start|>user\nfirst<|im_end|>\n<|im_start|>assistant\nanswer<|im_end|>\n<|im_start|>user\nnew /think<|im_end|>\n<|im_start|>assistant\n"},
{"name":"mixed-system-none","system":"","defaults":[],"messages":[{"role":"user","content":"first"},{"role":"system","content":"middle"},{"role":"assistant","content":"answer"},{"role":"user","content":"new"}],"think":false,"prompt":"<|im_start|>system\n\nmiddle<|im_end|>\n<|im_start|>user\nfirst<|im_end|>\n<|im_start|>assistant\nanswer<|im_end|>\n<|im_start|>user\nnew /no_think<|im_end|>\n<|im_start|>assistant\n<think>\n\n</think>\n\n"},
{"name":"assistant-prefill-default","system":"","defaults":[],"messages":[{"role":"user","content":"hello"},{"role":"assistant","content":"prefix"}],"think":true,"prompt":"<|im_start|>user\nhello /think<|im_end|>\n<|im_start|>assistant\nprefix"},
{"name":"assistant-prefill-none","system":"","defaults":[],"messages":[{"role":"user","content":"hello"},{"role":"assistant","content":"prefix"}],"think":false,"prompt":"<|im_start|>user\nhello /no_think<|im_end|>\n<|im_start|>assistant\nprefix"},
{"name":"empty-assistant-default","system":"","defaults":[],"messages":[{"role":"user","content":"hello"},{"role":"assistant","content":""},{"role":"user","content":"new"}],"think":true,"prompt":"<|im_start|>user\nhello<|im_end|>\n<|im_start|>assistant\n<|im_end|>\n<|im_start|>user\nnew /think<|im_end|>\n<|im_start|>assistant\n"},
{"name":"empty-assistant-none","system":"","defaults":[],"messages":[{"role":"user","content":"hello"},{"role":"assistant","content":""},{"role":"user","content":"new"}],"think":false,"prompt":"<|im_start|>user\nhello<|im_end|>\n<|im_start|>assistant\n<|im_end|>\n<|im_start|>user\nnew /no_think<|im_end|>\n<|im_start|>assistant\n<think>\n\n</think>\n\n"},
{"name":"model-role-normalized-default","system":"","defaults":[{"role":"ASSISTANT","content":"previous"}],"messages":[{"role":"user","content":"new"}],"think":true,"prompt":"<|im_start|>assistant\nprevious<|im_end|>\n<|im_start|>user\nnew /think<|im_end|>\n<|im_start|>assistant\n"},
{"name":"model-role-normalized-none","system":"","defaults":[{"role":"ASSISTANT","content":"previous"}],"messages":[{"role":"user","content":"new"}],"think":false,"prompt":"<|im_start|>assistant\nprevious<|im_end|>\n<|im_start|>user\nnew /no_think<|im_end|>\n<|im_start|>assistant\n<think>\n\n</think>\n\n"},
{"name":"thinking-default","system":"","defaults":[],"messages":[{"role":"user","content":"old"},{"role":"assistant","content":"<think> private </think>  visible"},{"role":"user","content":"new"}],"think":true,"prompt":"<|im_start|>user\nold<|im_end|>\n<|im_start|>assistant\nvisible<|im_end|>\n<|im_start|>user\nnew /think<|im_end|>\n<|im_start|>assistant\n"},
{"name":"thinking-none","system":"","defaults":[],"messages":[{"role":"user","content":"old"},{"role":"assistant","content":"<think> private </think>  visible"},{"role":"user","content":"new"}],"think":false,"prompt":"<|im_start|>user\nold<|im_end|>\n<|im_start|>assistant\nvisible<|im_end|>\n<|im_start|>user\nnew /no_think<|im_end|>\n<|im_start|>assistant\n<think>\n\n</think>\n\n"},
{"name":"unclosed-default","system":"","defaults":[],"messages":[{"role":"user","content":"old"},{"role":"assistant","content":"<think> secret"},{"role":"user","content":"new"}],"think":true,"prompt":"<|im_start|>user\nold<|im_end|>\n<|im_start|>assistant\n<|im_end|>\n<|im_start|>user\nnew /think<|im_end|>\n<|im_start|>assistant\n"},
{"name":"unclosed-none","system":"","defaults":[],"messages":[{"role":"user","content":"old"},{"role":"assistant","content":"<think> secret"},{"role":"user","content":"new"}],"think":false,"prompt":"<|im_start|>user\nold<|im_end|>\n<|im_start|>assistant\n<|im_end|>\n<|im_start|>user\nnew /no_think<|im_end|>\n<|im_start|>assistant\n<think>\n\n</think>\n\n"},
{"name":"partial-default","system":"","defaults":[],"messages":[{"role":"user","content":"old"},{"role":"assistant","content":"  <thin"},{"role":"user","content":"new"}],"think":true,"prompt":"<|im_start|>user\nold<|im_end|>\n<|im_start|>assistant\n<|im_end|>\n<|im_start|>user\nnew /think<|im_end|>\n<|im_start|>assistant\n"},
{"name":"partial-none","system":"","defaults":[],"messages":[{"role":"user","content":"old"},{"role":"assistant","content":"  <thin"},{"role":"user","content":"new"}],"think":false,"prompt":"<|im_start|>user\nold<|im_end|>\n<|im_start|>assistant\n<|im_end|>\n<|im_start|>user\nnew /no_think<|im_end|>\n<|im_start|>assistant\n<think>\n\n</think>\n\n"},
{"name":"whitespace-default","system":"","defaults":[],"messages":[{"role":"user","content":"old"},{"role":"assistant","content":" \t\n"},{"role":"user","content":"new"}],"think":true,"prompt":"<|im_start|>user\nold<|im_end|>\n<|im_start|>assistant\n<|im_end|>\n<|im_start|>user\nnew /think<|im_end|>\n<|im_start|>assistant\n"},
{"name":"whitespace-none","system":"","defaults":[],"messages":[{"role":"user","content":"old"},{"role":"assistant","content":" \t\n"},{"role":"user","content":"new"}],"think":false,"prompt":"<|im_start|>user\nold<|im_end|>\n<|im_start|>assistant\n<|im_end|>\n<|im_start|>user\nnew /no_think<|im_end|>\n<|im_start|>assistant\n<think>\n\n</think>\n\n"},
{"name":"embedded-default","system":"","defaults":[],"messages":[{"role":"user","content":"old"},{"role":"assistant","content":"text <think>not leading</think>"},{"role":"user","content":"new"}],"think":true,"prompt":"<|im_start|>user\nold<|im_end|>\n<|im_start|>assistant\ntext <think>not leading</think><|im_end|>\n<|im_start|>user\nnew /think<|im_end|>\n<|im_start|>assistant\n"},
{"name":"embedded-none","system":"","defaults":[],"messages":[{"role":"user","content":"old"},{"role":"assistant","content":"text <think>not leading</think>"},{"role":"user","content":"new"}],"think":false,"prompt":"<|im_start|>user\nold<|im_end|>\n<|im_start|>assistant\ntext <think>not leading</think><|im_end|>\n<|im_start|>user\nnew /no_think<|im_end|>\n<|im_start|>assistant\n<think>\n\n</think>\n\n"},
{"name":"repeated-default","system":"","defaults":[],"messages":[{"role":"user","content":"old"},{"role":"assistant","content":"<think>a</think>  visible </think>"},{"role":"user","content":"new"}],"think":true,"prompt":"<|im_start|>user\nold<|im_end|>\n<|im_start|>assistant\nvisible </think><|im_end|>\n<|im_start|>user\nnew /think<|im_end|>\n<|im_start|>assistant\n"},
{"name":"repeated-none","system":"","defaults":[],"messages":[{"role":"user","content":"old"},{"role":"assistant","content":"<think>a</think>  visible </think>"},{"role":"user","content":"new"}],"think":false,"prompt":"<|im_start|>user\nold<|im_end|>\n<|im_start|>assistant\nvisible </think><|im_end|>\n<|im_start|>user\nnew /no_think<|im_end|>\n<|im_start|>assistant\n<think>\n\n</think>\n\n"},
{"name":"unicode-space-default","system":"","defaults":[],"messages":[{"role":"user","content":"old"},{"role":"assistant","content":" <think> a </think> visible"},{"role":"user","content":"new"}],"think":true,"prompt":"<|im_start|>user\nold<|im_end|>\n<|im_start|>assistant\nvisible<|im_end|>\n<|im_start|>user\nnew /think<|im_end|>\n<|im_start|>assistant\n"},
{"name":"unicode-space-none","system":"","defaults":[],"messages":[{"role":"user","content":"old"},{"role":"assistant","content":" <think> a </think> visible"},{"role":"user","content":"new"}],"think":false,"prompt":"<|im_start|>user\nold<|im_end|>\n<|im_start|>assistant\nvisible<|im_end|>\n<|im_start|>user\nnew /no_think<|im_end|>\n<|im_start|>assistant\n<think>\n\n</think>\n\n"},
{"name":"empty-think-default","system":"","defaults":[],"messages":[{"role":"user","content":"old"},{"role":"assistant","content":"<think></think>"},{"role":"user","content":"new"}],"think":true,"prompt":"<|im_start|>user\nold<|im_end|>\n<|im_start|>assistant\n<|im_end|>\n<|im_start|>user\nnew /think<|im_end|>\n<|im_start|>assistant\n"},
{"name":"empty-think-none","system":"","defaults":[],"messages":[{"role":"user","content":"old"},{"role":"assistant","content":"<think></think>"},{"role":"user","content":"new"}],"think":false,"prompt":"<|im_start|>user\nold<|im_end|>\n<|im_start|>assistant\n<|im_end|>\n<|im_start|>user\nnew /no_think<|im_end|>\n<|im_start|>assistant\n<think>\n\n</think>\n\n"},
{"name":"partial-close-default","system":"","defaults":[],"messages":[{"role":"user","content":"old"},{"role":"assistant","content":"<think>a</thin"},{"role":"user","content":"new"}],"think":true,"prompt":"<|im_start|>user\nold<|im_end|>\n<|im_start|>assistant\n<|im_end|>\n<|im_start|>user\nnew /think<|im_end|>\n<|im_start|>assistant\n"},
{"name":"partial-close-none","system":"","defaults":[],"messages":[{"role":"user","content":"old"},{"role":"assistant","content":"<think>a</thin"},{"role":"user","content":"new"}],"think":false,"prompt":"<|im_start|>user\nold<|im_end|>\n<|im_start|>assistant\n<|im_end|>\n<|im_start|>user\nnew /no_think<|im_end|>\n<|im_start|>assistant\n<think>\n\n</think>\n\n"}
]`
	var cases []struct {
		Name     string
		System   string
		Defaults []message
		Messages []message
		Think    bool
		Prompt   string
	}
	if err := json.Unmarshal([]byte(reference), &cases); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("testdata/qwen3-ollama-0.12.7.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range cases {
		t.Run(tt.Name, func(t *testing.T) {
			system, _ := json.Marshal(tt.System)
			defaults, _ := json.Marshal(tt.Defaults)
			s := show{Template: string(data), Capabilities: []string{"thinking"}, System: system, Messages: defaults}
			r := request{Model: "qwen3:0.6b", Messages: tt.Messages}
			if !tt.Think {
				r.ReasoningEffort = "none"
			}
			got, think, err := render(s, r)
			if err != nil || think != tt.Think || got != tt.Prompt {
				t.Fatalf("got %q think=%v err=%v; want %q", got, think, err, tt.Prompt)
			}
		})
	}
}
