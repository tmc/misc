package llmperf

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestAnthropicCacheUsage(t *testing.T) {
	for _, tt := range []struct {
		name, raw   string
		prompt      int
		present     bool
		read, write *int
	}{
		{"all", `{"input_tokens":2,"cache_read_input_tokens":3,"cache_creation_input_tokens":4}`, 9, true, intPtr(3), intPtr(4)},
		{"no cache", `{"input_tokens":2}`, 2, true, nil, nil},
		{"read", `{"input_tokens":2,"cache_read_input_tokens":3}`, 5, true, intPtr(3), nil},
		{"write", `{"input_tokens":2,"cache_creation_input_tokens":4}`, 6, true, nil, intPtr(4)},
		{"missing input", `{"cache_read_input_tokens":3,"cache_creation_input_tokens":4}`, 0, false, intPtr(3), intPtr(4)},
		{"null input", `{"input_tokens":null,"cache_creation_input_tokens":4}`, 0, false, nil, intPtr(4)},
		{"zero", `{"input_tokens":0,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}`, 0, true, intPtr(0), intPtr(0)},
		{"null cache", `{"input_tokens":2,"cache_read_input_tokens":null,"cache_creation_input_tokens":null}`, 2, true, nil, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			u, err := canonicalUsage(json.RawMessage(tt.raw), "anthropic")
			if err != nil || u.PromptTokens != tt.prompt || u.promptPresent != tt.present || !sameCount(u.CachedTokens, tt.read) || !sameCount(u.CacheWriteTokens, tt.write) {
				t.Fatalf("usage=%+v err=%v", u, err)
			}
		})
	}
	for _, field := range []string{"input_tokens", "cache_read_input_tokens", "cache_creation_input_tokens"} {
		for _, value := range []string{"-1", "1.5", "1e0", `"1"`, "9223372036854775808"} {
			raw := fmt.Sprintf(`{"%s":%s}`, field, value)
			if _, err := canonicalUsage(json.RawMessage(raw), "anthropic"); err == nil {
				t.Fatalf("accepted %s", raw)
			}
		}
	}
	max := int(^uint(0) >> 1)
	if _, err := canonicalUsage(json.RawMessage(fmt.Sprintf(`{"input_tokens":%d,"cache_creation_input_tokens":1}`, max)), "anthropic"); err == nil {
		t.Fatal("accepted sum overflow")
	}
	for _, protocol := range []string{"openai-chat", "openai-responses"} {
		raw := `{"prompt_tokens":5,"completion_tokens":2,"prompt_tokens_details":{"cached_tokens":3}}`
		if protocol == "openai-responses" {
			raw = `{"input_tokens":5,"output_tokens":2,"input_tokens_details":{"cached_tokens":3}}`
		}
		u, err := canonicalUsage(json.RawMessage(raw), protocol)
		if err != nil || u.PromptTokens != 5 || u.CacheWriteTokens != nil {
			t.Fatalf("%s changed: %+v %v", protocol, u, err)
		}
	}
}

func sameCount(a, b *int) bool { return a == nil && b == nil || a != nil && b != nil && *a == *b }

func TestAnthropicCachePartialSnapshots(t *testing.T) {
	var u Usage
	for _, event := range []string{
		`{"type":"message_start","message":{"usage":{"input_tokens":2,"cache_read_input_tokens":3}}}`,
		`{"type":"message_delta","usage":{"cache_creation_input_tokens":4}}`,
		`{"type":"message_delta","usage":{"output_tokens":5}}`,
		`{"type":"message_delta","usage":{"cache_read_input_tokens":null,"cache_creation_input_tokens":null}}`,
	} {
		c, _, err := decodeChunk([]byte(event), "anthropic")
		if err != nil {
			t.Fatal(err)
		}
		u, err = mergeUsage(u, c.Usage)
		if err != nil {
			t.Fatal(err)
		}
	}
	if u.PromptTokens != 9 || u.CompletionTokens != 5 || !sameCount(u.CacheWriteTokens, intPtr(4)) {
		t.Fatalf("merged=%+v", u)
	}
	next, _ := canonicalUsage(json.RawMessage(`{"cache_creation_input_tokens":0}`), "anthropic")
	u, err := mergeUsage(u, next)
	if err != nil || u.PromptTokens != 5 || !sameCount(u.CacheWriteTokens, intPtr(0)) {
		t.Fatalf("zero update=%+v %v", u, err)
	}
	partial, _ := canonicalUsage(json.RawMessage(`{"cache_creation_input_tokens":4}`), "anthropic")
	partial, err = mergeUsage(Usage{}, partial)
	if err != nil || partial.promptPresent {
		t.Fatal("cache-only snapshot made prompt known")
	}
	input, _ := canonicalUsage(json.RawMessage(`{"input_tokens":2}`), "anthropic")
	partial, err = mergeUsage(partial, input)
	if err != nil || partial.PromptTokens != 6 {
		t.Fatalf("late input=%+v %v", partial, err)
	}
	max := int(^uint(0) >> 1)
	base, _ := canonicalUsage(json.RawMessage(fmt.Sprintf(`{"input_tokens":%d}`, max)), "anthropic")
	next, _ = canonicalUsage(json.RawMessage(`{"cache_read_input_tokens":1}`), "anthropic")
	if _, err := mergeUsage(base, next); err == nil {
		t.Fatal("late cache overflow accepted")
	}
}

func TestAnthropicCacheReceiptReplay(t *testing.T) {
	// Previously saved prompt counts are observations, never recomputed.
	for _, field := range []string{"", `,"cache_write_tokens":0`, `,"cache_write_tokens":4`} {
		raw := `{"cached_tokens":3,"input_tokens":{"value":2,"source":"reported"}` + field + `}`
		var m Measurement
		if err := json.Unmarshal([]byte(raw), &m); err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		if m.InputTokens.Value == nil || *m.InputTokens.Value != 2 {
			t.Fatal("old prompt recomputed")
		}
		if !strings.Contains(string(data), `"cache_write_tokens":`) {
			t.Fatal("missing additive field")
		}
	}
}

func TestAnthropicCacheContradictoryTotal(t *testing.T) {
	u, err := canonicalUsage(json.RawMessage(`{"input_tokens":2,"cache_read_input_tokens":3,"cache_creation_input_tokens":4,"output_tokens":5,"total_tokens":7}`), "anthropic")
	if err != nil {
		t.Fatal(err)
	}
	m := Measurement{InputTokens: TokenCount{Value: intPtr(u.PromptTokens), Source: "reported"}, OutputTokens: TokenCount{Value: intPtr(u.CompletionTokens), Source: "reported"}, ProviderTotalTokens: intPtr(u.TotalTokens)}
	if workUsageState(&m) != "inconsistent" {
		t.Fatal("contradictory total accepted")
	}
}

func TestAnthropicNativeUsageSources(t *testing.T) {
	for _, raw := range []string{
		`{"prompt_tokens":3,"cache_read_input_tokens":1,"completion_tokens":5}`,
		`{"input_tokens":null,"prompt_tokens":3,"output_tokens":null,"completion_tokens":5}`,
	} {
		u, err := canonicalUsage(json.RawMessage(raw), "anthropic")
		if err != nil || u.promptPresent || u.completionPresent || u.anthropicInput != nil {
			t.Fatalf("native absence lost: %+v %v", u, err)
		}
	}
	for _, raw := range []string{
		`{"input_tokens":2,"cache_read_input_tokens":1,"prompt_tokens":2}`,
		`{"output_tokens":2,"completion_tokens":3}`,
	} {
		if _, err := canonicalUsage(json.RawMessage(raw), "anthropic"); err == nil {
			t.Fatalf("accepted contradictory aliases: %s", raw)
		}
	}
	u, err := canonicalUsage(json.RawMessage(`{"input_tokens":2,"cache_read_input_tokens":1,"prompt_tokens":3,"output_tokens":5,"completion_tokens":5}`), "anthropic")
	if err != nil || u.PromptTokens != 3 || u.CompletionTokens != 5 {
		t.Fatalf("consistent aliases: %+v %v", u, err)
	}
	alias, err := canonicalUsage(json.RawMessage(`{"prompt_tokens":99,"completion_tokens":99,"cache_creation_input_tokens":4}`), "anthropic")
	if err != nil {
		t.Fatal(err)
	}
	merged, err := mergeUsage(u, alias)
	if err != nil || merged.PromptTokens != 7 || merged.CompletionTokens != 5 {
		t.Fatalf("late aliases replaced native counts: %+v %v", merged, err)
	}
}
