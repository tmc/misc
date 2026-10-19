package main

import (
	"crypto/sha256"
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
		{"system", "\"prior instructions\"", "", false}, {"messages", "", `[{"role":"assistant","content":"prior"}]`, false},
		{"null system", "null", "", false}, {"null messages", "", "null", false},
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
