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
	amount, why, e := costOf(m, p.Models)
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
			a, w, e := costOf(v, p.Models)
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
	a, w, e := costOf(m, map[string]TokenPrices{"m": {}})
	if e != nil || w != "" || a == nil || *a != 0 {
		t.Fatal(a, w, e)
	}
	rate := 2.0
	m.InputTokens.Value = &n
	m.CachedTokens = &z
	a, w, e = costOf(m, map[string]TokenPrices{"m": {Input: &rate}})
	if e != nil || w != "" || a == nil || *a != .00002 {
		t.Fatal(a, w, e)
	}
	m.CachedTokens = nil
	a, w, e = costOf(m, map[string]TokenPrices{"m": {Input: &rate}})
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

func TestCostLedgerUnavailableReasons(t *testing.T) {
	p, err := ReadPrices(strings.NewReader(testPrices))
	if err != nil {
		t.Fatal(err)
	}
	in, out, zero := 10, 5, 0
	priced := llm.Measurement{SchemaVersion: 1, Attempted: true, Recorded: true, Good: true, Phase: "measurement", Outcome: "success", Protocol: "anthropic", Model: "m", StartedAt: 1100, DispatchedAt: 1100, FinishedAt: 1200, FullResponseLatency: 100, InputTokens: llm.TokenCount{Value: &in, Source: "reported"}, OutputTokens: llm.TokenCount{Value: &out, Source: "reported"}, CachedTokens: &zero, CacheWriteTokens: &zero, ReasoningTokens: &zero}
	legacy := priced
	legacy.CacheWriteTokens = nil
	for _, tc := range []struct {
		name    string
		record  llm.Measurement
		dropped int
		want    string
	}{
		{"complete", priced, 0, ""},
		{"incomplete", legacy, 0, "incomplete eligible usage or prices"},
		{"dropped", priced, 1, "dropped records"},
		{"incomplete and dropped", legacy, 1, "incomplete eligible usage or prices; dropped records"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n := 1 + tc.dropped
			r := llm.RunSummary{SchemaVersion: 1, RunID: "cost", StartedAt: 1000, FinishedAt: 2000, Duration: 1, Attempts: n, Successes: n, GoodRequests: n, InputTokens: int64(in * n), OutputTokens: int64(out * n), Goodput: float64(n), InputThroughput: float64(in * n), OutputThroughput: float64(out * n), DroppedRecords: tc.dropped, Records: []llm.Measurement{tc.record}}
			l, err := DeriveCost(r, p)
			if err != nil {
				t.Fatal(err)
			}
			if l.Unavailable != tc.want || (l.Total == nil) != (tc.want != "") {
				t.Fatalf("ledger unavailable %q total %v, want %q", l.Unavailable, l.Total, tc.want)
			}
			if tc.record.CacheWriteTokens == nil && (l.Records[0].Unavailable != "cache write presence unavailable" || l.Records[0].Amount != nil) {
				t.Fatalf("record unavailable %q amount %v", l.Records[0].Unavailable, l.Records[0].Amount)
			}
		})
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
	a, w, e := costOf(m, map[string]TokenPrices{"m": rates})
	if a != nil || w != "cache_read price missing" || e != nil {
		t.Fatal(a, w, e)
	}
	rates.CacheRead = &rate
	a, w, e = costOf(m, map[string]TokenPrices{"m": rates})
	if a == nil || math.Abs(*a-.000015) > 1e-20 || w != "" || e != nil {
		t.Fatal(a, w, e)
	}
	rates.Reasoning = &rate
	m.ReasoningTokens = nil
	a, w, e = costOf(m, map[string]TokenPrices{"m": rates})
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
	a, w, e := costOf(m, models)
	if e != nil || w != "" || a == nil || math.Abs(*a-.000029) > 1e-20 {
		t.Fatal(a, w, e)
	}
	total = 30
	if _, _, e := costOf(m, models); e == nil {
		t.Fatal("contradictory provider total admitted")
	}
	max := int(^uint(0) >> 1)
	m.InputTokens.Value = &max
	m.OutputTokens.Value = &max
	m.ProviderTotalTokens = &max
	if _, _, e := costOf(m, models); e == nil {
		t.Fatal("overflowing input/output sum admitted")
	}
}
func TestLegacyAnthropicCostBeforeInclusiveBounds(t *testing.T) {
	in, out, read, write := 2, 8, 1000, 0
	rate := 1.0
	m := llm.Measurement{Protocol: "anthropic", Model: "m", InputTokens: llm.TokenCount{Value: &in, Source: "reported"}, OutputTokens: llm.TokenCount{Value: &out, Source: "reported"}, CachedTokens: &read}
	models := map[string]TokenPrices{"m": {Input: &rate, CacheRead: &rate, Output: &rate}}
	a, w, e := costOf(m, models)
	if e != nil || a != nil || w != "cache write presence unavailable" {
		t.Fatal(a, w, e)
	}
	in = 1002
	m.CacheWriteTokens = &write
	a, w, e = costOf(m, models)
	if e != nil || w != "" || a == nil || math.Abs(*a-.001010) > 1e-18 {
		t.Fatal(a, w, e)
	}
	in = 999
	if _, _, e := costOf(m, models); e == nil {
		t.Fatal("normalized contradictory cache admitted")
	}
}

func TestLegacyAnthropicZeroCost(t *testing.T) {
	zero := 0
	m := llm.Measurement{Protocol: "anthropic", Model: "m", InputTokens: llm.TokenCount{Value: &zero, Source: "reported"}, OutputTokens: llm.TokenCount{Value: &zero, Source: "reported"}, CachedTokens: &zero}
	models := map[string]TokenPrices{"m": {}}
	a, w, e := costOf(m, models)
	if e != nil || a != nil || w != "cache write presence unavailable" {
		t.Fatal(a, w, e)
	}
	m.CacheWriteTokens = &zero
	a, w, e = costOf(m, models)
	if e != nil || w != "" || a == nil || *a != 0 {
		t.Fatal(a, w, e)
	}
}

func TestCostPriceModel(t *testing.T) {
	in, out, zero := 10, 5, 0
	alias, routed := 1.0, 3.0
	models := map[string]TokenPrices{"alias": {Input: &alias, Output: &alias}, "routed-v2": {Input: &routed, Output: &routed}}
	for _, tc := range []struct {
		name             string
		requested        string
		work             *llm.WorkReceipt
		wantModel, basis string
		amount           float64
		unavailable      string
	}{
		{"no work receipt", "alias", nil, "alias", "requested", 15e-6, ""},
		{"work without model", "alias", &llm.WorkReceipt{}, "alias", "requested", 15e-6, ""},
		{"observed same", "alias", &llm.WorkReceipt{ObservedModel: "alias"}, "alias", "observed", 15e-6, ""},
		{"observed routed", "alias", &llm.WorkReceipt{ObservedModel: "routed-v2"}, "routed-v2", "observed", 45e-6, ""},
		{"observed unpriced", "alias", &llm.WorkReceipt{ObservedModel: "alias-2026"}, "alias-2026", "observed", 0, "observed model price missing"},
		{"requested unpriced", "other", nil, "other", "requested", 0, "model price missing"},
		{"observed same unpriced", "other", &llm.WorkReceipt{ObservedModel: "other"}, "other", "observed", 0, "model price missing"},
		{"observed oversized", "alias", &llm.WorkReceipt{ObservedModelOversized: true}, "", "observed", 0, "observed model not retained"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := llm.Measurement{Protocol: "openai-chat", Model: tc.requested, Work: tc.work, InputTokens: llm.TokenCount{Value: &in, Source: "reported"}, OutputTokens: llm.TokenCount{Value: &out, Source: "reported"}, CachedTokens: &zero}
			if model, basis := priceModel(m); model != tc.wantModel || basis != tc.basis {
				t.Fatalf("priceModel = %q %q, want %q %q", model, basis, tc.wantModel, tc.basis)
			}
			a, w, e := costOf(m, models)
			if e != nil || w != tc.unavailable {
				t.Fatalf("recordCost = %v %q %v, want unavailable %q", a, w, e, tc.unavailable)
			}
			if tc.unavailable != "" && a != nil {
				t.Fatalf("amount = %v, want unavailable", *a)
			}
			if tc.unavailable == "" && (a == nil || math.Abs(*a-tc.amount) > 1e-18) {
				t.Fatalf("amount = %v, want %v", a, tc.amount)
			}
		})
	}
}

func TestCostLedgerRecordsPriceModel(t *testing.T) {
	p, err := ReadPrices(strings.NewReader(`{"currency":"USD","source_url":"https://example.com/prices","effective_date":"2026-10-11","models":{"alias":{"input":1,"output":1}}}`))
	if err != nil {
		t.Fatal(err)
	}
	in, out, zero := 10, 5, 0
	r := runFixture()
	r.MissingOutputCounts, r.WorkQualified, r.WorkAccepted = 0, 1, 1
	r.InputTokens, r.OutputTokens = int64(in), int64(out)
	r.InputThroughput, r.OutputThroughput = float64(in)/r.Duration, float64(out)/r.Duration
	m := &r.Records[0]
	m.Protocol, m.Model, m.CachedTokens = "openai-chat", "alias", &zero
	m.InputTokens = llm.TokenCount{Value: &in, Source: "reported"}
	m.OutputTokens = llm.TokenCount{Value: &out, Source: "reported"}
	for _, tc := range []struct {
		name, observed    string
		oversized         bool
		model, basis, why string
	}{
		{"requested basis", "", false, "alias", "requested", ""},
		{"observed alias unpriced", "alias-2026", false, "alias-2026", "observed", "observed model price missing"},
		{"observed oversized", "", true, "", "observed", "observed model not retained"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m.Work = &llm.WorkReceipt{Status: "accepted", ObservedModel: tc.observed, ObservedModelOversized: tc.oversized, ReportedOutputTokens: &out}
			l, err := DeriveCost(r, p)
			if err != nil {
				t.Fatal(err)
			}
			got := l.Records[0]
			if l.SchemaVersion != 2 || got.PriceModel != tc.model || got.PriceBasis != tc.basis || got.Unavailable != tc.why || (got.Amount == nil) != (tc.why != "") || (l.Total == nil) != (tc.why != "") {
				t.Fatalf("ledger record = %+v total %v", got, l.Total)
			}
		})
	}
}

// costOf prices m as DeriveCost does.
func costOf(m llm.Measurement, models map[string]TokenPrices) (*float64, string, error) {
	model, basis := priceModel(m)
	return recordCost(m, model, basis, models)
}
