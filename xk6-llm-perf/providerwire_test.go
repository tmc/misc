package llmperf

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestProviderwireUsage(t *testing.T) {
	for _, tt := range []struct {
		name, body    string
		bad           bool
		input, output *int
	}{
		{"absent", `{"inputTokens":{},"outputTokens":{}}`, false, nil, nil},
		{"zero", `{"inputTokens":{"total":0},"outputTokens":{"total":0}}`, false, intPtrV4(0), intPtrV4(0)},
		{"buckets", `{"inputTokens":{"total":9,"noCache":2,"cacheRead":3,"cacheWrite":4},"outputTokens":{"total":5,"text":2,"reasoning":3}}`, false, intPtrV4(9), intPtrV4(5)},
		{"null", `{"inputTokens":{"total":null},"outputTokens":{}}`, true, nil, nil},
		{"fraction", `{"inputTokens":{"total":1.5},"outputTokens":{}}`, true, nil, nil},
		{"too big", `{"inputTokens":{"total":9007199254740992},"outputTokens":{}}`, true, nil, nil},
		{"inconsistent", `{"inputTokens":{"total":2,"cacheRead":3},"outputTokens":{}}`, true, nil, nil},
		{"sum mismatch", `{"inputTokens":{"total":9,"noCache":2,"cacheRead":3,"cacheWrite":3},"outputTokens":{}}`, true, nil, nil},
		{"duplicate", `{"inputTokens":{"total":1,"total":2},"outputTokens":{}}`, true, nil, nil},
		{"unknown", `{"inputTokens":{"TOTAL":1},"outputTokens":{}}`, true, nil, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			u, e := providerwireUsage([]byte(tt.body))
			if (e != nil) != tt.bad {
				t.Fatalf("error %v", e)
			}
			if tt.bad {
				return
			}
			if u.promptPresent != (tt.input != nil) || u.completionPresent != (tt.output != nil) {
				t.Fatalf("presence: %+v", u)
			}
			if tt.input != nil && u.PromptTokens != *tt.input || tt.output != nil && u.CompletionTokens != *tt.output {
				t.Fatalf("counts: %+v", u)
			}
		})
	}
}
func intPtrV4(n int) *int { return &n }
func TestProviderwireStream(t *testing.T) {
	valid := []string{`{"type":"stream-start","warnings":[]}`, `{"type":"reasoning-start","id":"r"}`, `{"type":"reasoning-delta","id":"r","delta":"think"}`, `{"type":"reasoning-end","id":"r"}`, `{"type":"text-start","id":"t"}`, `{"type":"text-delta","id":"t","delta":"answer"}`, `{"type":"text-end","id":"t"}`, `{"type":"finish","finishReason":{"unified":"stop"},"usage":{"inputTokens":{},"outputTokens":{}}}`}
	var s providerwireStream
	var text, reasoning string
	for i, b := range valid {
		c, done, e := s.decode([]byte(b))
		if e != nil {
			t.Fatalf("part %d: %v", i, e)
		}
		if done != (i == len(valid)-1) {
			t.Fatalf("terminal %d", i)
		}
		for _, ch := range c.Choices {
			text += ch.Delta.Content
			reasoning += ch.Delta.ReasoningContent
		}
		if done && (c.Usage.promptPresent || c.Usage.completionPresent) {
			t.Fatal("missing counts became zero")
		}
	}
	if text != "answer" || reasoning != "think" {
		t.Fatalf("content %q %q", text, reasoning)
	}
	if _, _, e := s.decode([]byte(valid[7])); e == nil {
		t.Fatal("duplicate finish accepted")
	}
	for _, b := range []string{`{"type":"unknown"}`, valid[2], valid[7], `{"type":"stream-start","warnings":null}`, `{"type":"stream-start","warnings":[],"extra":true}`, `{"type":"error","error":{}}`} {
		var s providerwireStream
		if _, _, e := s.decode([]byte(b)); e == nil {
			t.Fatalf("accepted %s", b)
		}
	}
}
func TestProviderwireTools(t *testing.T) {
	var s providerwireStream
	events := []string{`{"type":"stream-start","warnings":[]}`, `{"type":"tool-input-start","id":"c","toolName":"weather"}`, `{"type":"tool-input-delta","id":"c","delta":"{\"city\":\"Oslo\"}"}`, `{"type":"tool-input-end","id":"c"}`, `{"type":"tool-call","toolCallId":"c","toolName":"weather","input":"{\"city\":\"Oslo\"}"}`, `{"type":"finish","finishReason":{"unified":"tool-calls"},"usage":{"inputTokens":{"total":0},"outputTokens":{"total":0}}}`}
	var args, name string
	for _, b := range events {
		c, _, e := s.decode([]byte(b))
		if e != nil {
			t.Fatal(e)
		}
		for _, ch := range c.Choices {
			for _, call := range ch.Delta.ToolCalls {
				args += call.Function.Arguments
				name += call.Function.Name
			}
		}
	}
	if args != `{"city":"Oslo"}` || name != "weather" {
		t.Fatalf("duplicate assembly %q %q", args, name)
	}
	for _, bad := range []string{`{"type":"tool-call","toolCallId":"c","toolName":"weather","input":"{}"}`, `{"type":"tool-call","toolCallId":"x","toolName":"weather","input":"broken"}`, `{"type":"tool-call","toolCallId":"x","toolName":"weather","input":"{}","providerExecuted":true}`} {
		var s providerwireStream
		for _, b := range events[:4] {
			if _, _, e := s.decode([]byte(b)); e != nil {
				t.Fatal(e)
			}
		}
		if _, _, e := s.decode([]byte(bad)); e == nil {
			t.Fatalf("accepted %s", bad)
		}
	}
}
func TestProviderwireUnary(t *testing.T) {
	good := `{"warnings":[],"content":[{"type":"reasoning","text":"think"},{"type":"text","text":"answer"},{"type":"tool-call","toolCallId":"c","toolName":"f","input":"{}"}],"finishReason":{"unified":"stop"},"usage":{"inputTokens":{"total":4,"cacheRead":1,"cacheWrite":2},"outputTokens":{"total":3,"reasoning":1}}}`
	r, e := decodeProviderwireResponse([]byte(good))
	if e != nil {
		t.Fatal(e)
	}
	if r.Choices[0].Message.Content != "answer" || r.Choices[0].Message.ReasoningContent != "think" || len(r.Choices[0].Message.ToolCalls) != 1 || *r.Usage.CacheWriteTokens != 2 {
		t.Fatalf("response %+v", r)
	}
	for _, bad := range []string{strings.Replace(good, `"unified":"stop"`, `"unified":"error"`, 1), strings.Replace(good, `"type":"text"`, `"type":"file"`, 1), strings.Replace(good, `"warnings":[]`, `"warnings":null`, 1), `{"error":{}}`} {
		if _, e := decodeProviderwireResponse([]byte(bad)); e == nil {
			t.Fatalf("accepted %s", bad)
		}
	}
}
func TestProviderwireRequest(t *testing.T) {
	good := `{"prompt":[{"role":"system","content":"brief"},{"role":"user","content":[{"type":"text","text":"hello"}]}],"maxOutputTokens":8,"tools":[{"type":"function","name":"f","inputSchema":{"type":"object"}}],"toolChoice":{"type":"auto"}}`
	for _, tt := range []struct {
		body string
		bad  bool
	}{{good, false}, {`{"prompt":[]}`, false}, {strings.Replace(good, `"maxOutputTokens":8`, `"maxOutputTokens":1.5`, 1), true}, {strings.Replace(good, `"type":"text"`, `"type":"file"`, 1), true}, {strings.Replace(good, `"type":"text"`, `"type":"text","url":"oops"`, 1), true}, {`{"prompt":[],"model":"x"}`, true}, {`{"prompt":[],"stream":true}`, true}, {`{"prompt":[],"headers":{"Authorization":"secret"}}`, true}, {`{"prompt":[{"role":"user","content":[{"type":"text","text":null}]}]}`, true}} {
		var body map[string]any
		if e := json.Unmarshal([]byte(tt.body), &body); e != nil {
			t.Fatal(e)
		}
		if e := validateProviderwireBody(body); (e != nil) != tt.bad {
			t.Fatalf("body %s error %v", tt.body, e)
		}
	}
}

func TestProviderwirePinnedFixtures(t *testing.T) {
	b, e := os.ReadFile("testdata/providerwire-v4/stream.json")
	if e != nil {
		t.Fatal(e)
	}
	var events []json.RawMessage
	if e = json.Unmarshal(b, &events); e != nil {
		t.Fatal(e)
	}
	var s providerwireStream
	for i, b := range events {
		_, done, e := s.decode(b)
		if e != nil || done != (i == len(events)-1) {
			t.Fatalf("part %d terminal %t error %v", i, done, e)
		}
	}
	b, e = os.ReadFile("testdata/providerwire-v4/unary.json")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = decodeProviderwireResponse(b); e != nil {
		t.Fatal(e)
	}
	b, e = os.ReadFile("testdata/providerwire-v4/request.json")
	if e != nil {
		t.Fatal(e)
	}
	var body map[string]any
	if e = json.Unmarshal(b, &body); e != nil {
		t.Fatal(e)
	}
	if e = validateProviderwireBody(body); e != nil {
		t.Fatal(e)
	}
}

func TestProviderwireBounds(t *testing.T) {
	for _, name := range []string{"blocks", "tools"} {
		t.Run(name, func(t *testing.T) {
			var s providerwireStream
			_, _, e := s.decode([]byte(`{"type":"stream-start","warnings":[]}`))
			if e != nil {
				t.Fatal(e)
			}
			for i := 0; i <= 128; i++ {
				var b string
				if name == "blocks" {
					b = fmt.Sprintf(`{"type":"text-start","id":"%d"}`, i)
				} else {
					b = fmt.Sprintf(`{"type":"tool-input-start","id":"%d","toolName":"f"}`, i)
				}
				_, _, e = s.decode([]byte(b))
				if (e != nil) != (i == 128) {
					t.Fatalf("index %d error %v", i, e)
				}
			}
		})
	}
	if _, e := providerwireObject([]byte{'{', '"', 'x', '"', ':', '"', 0xff, '"', '}'}, "x", ""); e == nil {
		t.Fatal("invalid UTF-8 accepted")
	}
}

func TestProviderwireSensitiveErrors(t *testing.T) {
	secret := strings.Repeat("private-provider-data", 100)
	for _, b := range []string{fmt.Sprintf(`{"type":%q}`, secret), fmt.Sprintf(`{"type":"stream-start","warnings":[],%q:1}`, secret)} {
		var s providerwireStream
		_, _, e := s.decode([]byte(b))
		if e == nil || strings.Contains(e.Error(), "private-provider-data") || len(e.Error()) > 256 {
			t.Fatalf("unsafe error %v", e)
		}
	}
}

func providerwireMetadataCases() []struct {
	name      string
	events    []string
	bad       bool
	id, model string
} {
	start := `{"type":"stream-start","warnings":[]}`
	meta := `{"type":"response-metadata","id":"first","modelId":"first-model"}`
	changed := `{"type":"response-metadata","id":"changed","modelId":"changed-model"}`
	text := []string{`{"type":"text-start","id":"t"}`, `{"type":"text-delta","id":"t","delta":"answer"}`, `{"type":"text-end","id":"t"}`}
	finish := `{"type":"finish","finishReason":{"unified":"stop"},"usage":{"inputTokens":{"total":2},"outputTokens":{"total":3}}}`
	cases := []struct {
		name      string
		events    []string
		bad       bool
		id, model string
	}{
		{name: "no-metadata", events: append(append([]string{start}, text...), finish)},
		{name: "single-metadata", events: append(append([]string{start, meta}, text...), finish), id: "first", model: "first-model"},
		{name: "duplicate-metadata", events: []string{start, meta, changed}, bad: true},
		{name: "late-text-start", events: []string{start, text[0], changed}, bad: true},
		{name: "late-visible", events: []string{start, text[0], text[1], changed}, bad: true},
		{name: "late-reasoning", events: []string{start, `{"type":"reasoning-start","id":"r"}`, changed}, bad: true},
		{name: "late-tool", events: []string{start, `{"type":"tool-input-start","id":"c","toolName":"f"}`, changed}, bad: true},
		{name: "late-tool-call", events: []string{start, `{"type":"tool-call","toolCallId":"c","toolName":"f","input":"{}"}`, changed}, bad: true},
	}
	return cases
}

func TestProviderwireMetadataState(t *testing.T) {
	for _, tt := range providerwireMetadataCases() {
		t.Run(tt.name, func(t *testing.T) {
			var s providerwireStream
			var id, model string
			var lastErr error
			for _, b := range tt.events {
				c, _, e := s.decode([]byte(b))
				if e != nil {
					lastErr = e
					if c.ID != "" || c.Model != "" {
						t.Fatalf("refused metadata exposed identity %+v", c)
					}
					break
				}
				if c.ID != "" {
					id = c.ID
				}
				if c.Model != "" {
					model = c.Model
				}
			}
			if (lastErr != nil) != tt.bad {
				t.Fatalf("error %v", lastErr)
			}
			if !tt.bad && (id != tt.id || model != tt.model) {
				t.Fatalf("identity %q %q", id, model)
			}
		})
	}
}
