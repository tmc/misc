package output

import (
	llm "github.com/tmc/misc/xk6-llm-perf"
	"math"
	"strings"
	"testing"
)

const testPrices = `{"currency":"USD","source_url":"https://example.com/prices","effective_date":"2026-10-11","models":{"m":{"input":2,"cache_read":1,"cache_write":3,"output":4,"reasoning":5}}}`

func TestReadPrices(t *testing.T) {
	for _, s := range []string{strings.Replace(testPrices, `"input":2`, `"input":-2`, 1), strings.Replace(testPrices, `"input":2`, `"input":null`, 1), strings.Replace(testPrices, `"input":2`, `"input":2,"INPUT":3`, 1), testPrices + "{}", strings.Replace(testPrices, `"currency"`, `"Currency"`, 1), strings.Replace(testPrices, `"input":2`, `"other":2`, 1), strings.Repeat(" ", 1<<20) + testPrices} {
		if _, err := ReadPrices(strings.NewReader(s)); err == nil {
			t.Fatalf("accepted invalid table %.90s", s)
		}
	}
	p, e := ReadPrices(strings.NewReader(testPrices))
	if e != nil || len(p.hash) != 64 {
		t.Fatal(p, e)
	}
}
func TestRecordCost(t *testing.T) {
	p, e := ReadPrices(strings.NewReader(testPrices))
	if e != nil {
		t.Fatal(e)
	}
	in, out, read, write, reason := 100, 50, 20, 10, 5
	m := llm.Measurement{Protocol: "anthropic", Model: "m", InputTokens: llm.TokenCount{Value: &in, Source: "reported"}, OutputTokens: llm.TokenCount{Value: &out, Source: "reported"}, CachedTokens: &read, CacheWriteTokens: &write, ReasoningTokens: &reason}
	amount, why, e := recordCost(m, p.Models)
	want := float64(70*2+20+10*3+45*4+5*5) / 1e6
	if e != nil || why != "" || amount == nil || *amount != want {
		t.Fatalf("got %v %q %v want %v", amount, why, e, want)
	}
	for _, tc := range []struct {
		name string
		edit func(*llm.Measurement)
		bad  bool
	}{
		{"missing read", func(m *llm.Measurement) { m.CachedTokens = nil }, false},
		{"legacy anthropic", func(m *llm.Measurement) { m.CacheWriteTokens = nil }, false},
		{"estimated", func(m *llm.Measurement) { m.InputTokens.Source = "estimated" }, false},
		{"provider inconsistent", func(m *llm.Measurement) { n := 151; m.ProviderTotalTokens = &n }, true},
		{"reasoning overflow", func(m *llm.Measurement) { n := 51; m.ReasoningTokens = &n }, true},
		{"cache overflow", func(m *llm.Measurement) { n := 101; m.CachedTokens = &n }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := m
			tc.edit(&v)
			a, w, e := recordCost(v, p.Models)
			if tc.bad {
				if e == nil {
					t.Fatal("accepted contradiction")
				}
			} else if e != nil || a != nil || w == "" {
				t.Fatal(a, w, e)
			}
		})
	}
}
func TestCostZeroAndOpenAI(t *testing.T) {
	z, n := 0, 10
	m := llm.Measurement{Protocol: "openai-chat", Model: "m", InputTokens: llm.TokenCount{Value: &z, Source: "reported"}, OutputTokens: llm.TokenCount{Value: &z, Source: "reported"}}
	a, w, e := recordCost(m, map[string]TokenPrices{"m": {}})
	if e != nil || w != "" || a == nil || *a != 0 {
		t.Fatal(a, w, e)
	}
	rate := 2.0
	m.InputTokens.Value = &n
	m.CachedTokens = &z
	a, w, e = recordCost(m, map[string]TokenPrices{"m": {Input: &rate}})
	if e != nil || w != "" || a == nil || *a != .00002 {
		t.Fatal(a, w, e)
	}
	m.CachedTokens = nil
	a, w, e = recordCost(m, map[string]TokenPrices{"m": {Input: &rate}})
	if e != nil || a != nil || w == "" {
		t.Fatal(a, w, e)
	}
}
func TestCostLedgerUnavailableAndProvenance(t *testing.T) {
	p, e := ReadPrices(strings.NewReader(testPrices))
	if e != nil {
		t.Fatal(e)
	}
	r := runFixture()
	l, e := DeriveCost(r, p)
	if e != nil || l.Total != nil || l.Unavailable == "" || len(l.Records) != 1 {
		t.Fatal(l, e)
	}
	p.Currency = "EUR"
	if _, e := DeriveCost(r, p); e == nil {
		t.Fatal("mutable price table retained old hash")
	}
}

func TestPriceModelIdentity(t *testing.T) {
	p, err := ReadPrices(strings.NewReader(`{"currency":"USD","source_url":"https://example.com/prices","effective_date":"2026-10-11","models":{"Model":{"input":1},"model":{"input":2}}}`))
	if err != nil || len(p.Models) != 2 || *p.Models["Model"].Input != 1 || *p.Models["model"].Input != 2 {
		t.Fatal(p, err)
	}
}

func TestCostBucketPrices(t *testing.T) {
	in, out, read, write, reason := 10, 5, 2, 0, 3
	rate := 1.0
	m := llm.Measurement{Protocol: "anthropic", Model: "m", InputTokens: llm.TokenCount{Value: &in, Source: "reported"}, OutputTokens: llm.TokenCount{Value: &out, Source: "reported"}, CachedTokens: &read, CacheWriteTokens: &write, ReasoningTokens: &reason}
	rates := TokenPrices{Input: &rate, Output: &rate}
	a, w, e := recordCost(m, map[string]TokenPrices{"m": rates})
	if a != nil || w != "cache_read price missing" || e != nil {
		t.Fatal(a, w, e)
	}
	rates.CacheRead = &rate
	a, w, e = recordCost(m, map[string]TokenPrices{"m": rates})
	if a == nil || math.Abs(*a-.000015) > 1e-20 || w != "" || e != nil {
		t.Fatal(a, w, e)
	}
	rates.Reasoning = &rate
	m.ReasoningTokens = nil
	a, w, e = recordCost(m, map[string]TokenPrices{"m": rates})
	if a != nil || w != "reasoning count unavailable" || e != nil {
		t.Fatal(a, w, e)
	}
}

func TestReportedCostIgnoresClientEstimate(t *testing.T) {
	in, out, zero, total := 21, 8, 0, 29
	rate := 1.0
	difference := 10.526
	m := llm.Measurement{Protocol: "openai-chat", Model: "m", InputTokens: llm.TokenCount{Value: &in, Source: "reported"}, OutputTokens: llm.TokenCount{Value: &out, Source: "reported"}, CachedTokens: &zero, ProviderTotalTokens: &total, PromptDifference: &difference, UsageDiscrepancy: true}
	models := map[string]TokenPrices{"m": {Input: &rate, Output: &rate}}
	a, w, e := recordCost(m, models)
	if e != nil || w != "" || a == nil || math.Abs(*a-.000029) > 1e-20 {
		t.Fatal(a, w, e)
	}
	total = 30
	if _, _, e := recordCost(m, models); e == nil {
		t.Fatal("contradictory provider total admitted")
	}
	max := int(^uint(0) >> 1)
	m.InputTokens.Value = &max
	m.OutputTokens.Value = &max
	m.ProviderTotalTokens = &max
	if _, _, e := recordCost(m, models); e == nil {
		t.Fatal("overflowing input/output sum admitted")
	}
}
func TestLegacyAnthropicCostBeforeInclusiveBounds(t *testing.T) {
	in, out, read, write := 2, 8, 1000, 0
	rate := 1.0
	m := llm.Measurement{Protocol: "anthropic", Model: "m", InputTokens: llm.TokenCount{Value: &in, Source: "reported"}, OutputTokens: llm.TokenCount{Value: &out, Source: "reported"}, CachedTokens: &read}
	models := map[string]TokenPrices{"m": {Input: &rate, CacheRead: &rate, Output: &rate}}
	a, w, e := recordCost(m, models)
	if e != nil || a != nil || w != "cache write presence unavailable" {
		t.Fatal(a, w, e)
	}
	in = 1002
	m.CacheWriteTokens = &write
	a, w, e = recordCost(m, models)
	if e != nil || w != "" || a == nil || math.Abs(*a-.001010) > 1e-18 {
		t.Fatal(a, w, e)
	}
	in = 999
	if _, _, e := recordCost(m, models); e == nil {
		t.Fatal("normalized contradictory cache admitted")
	}
}

func TestLegacyAnthropicZeroCost(t *testing.T) {
	zero := 0
	m := llm.Measurement{Protocol: "anthropic", Model: "m", InputTokens: llm.TokenCount{Value: &zero, Source: "reported"}, OutputTokens: llm.TokenCount{Value: &zero, Source: "reported"}, CachedTokens: &zero}
	models := map[string]TokenPrices{"m": {}}
	a, w, e := recordCost(m, models)
	if e != nil || a != nil || w != "cache write presence unavailable" {
		t.Fatal(a, w, e)
	}
	m.CacheWriteTokens = &zero
	a, w, e = recordCost(m, models)
	if e != nil || w != "" || a == nil || *a != 0 {
		t.Fatal(a, w, e)
	}
}
