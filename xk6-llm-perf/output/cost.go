package output

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/url"
	"time"
	"unicode/utf8"

	llm "github.com/tmc/misc/xk6-llm-perf"
	"github.com/tmc/misc/xk6-llm-perf/internal/jsoncheck"
)

// Prices contains caller-declared currency rates per million tokens.
// Missing rates are unavailable, not free. No prices are supplied by this package.
type Prices struct {
	Currency      string                 `json:"currency"`
	SourceURL     string                 `json:"source_url"`
	EffectiveDate string                 `json:"effective_date"`
	Models        map[string]TokenPrices `json:"models"`
	hash          string
	canonical     []byte
}

// TokenPrices separates inclusive input and completion counts into billing buckets.
type TokenPrices struct {
	Input      *float64 `json:"input,omitempty"`
	CacheRead  *float64 `json:"cache_read,omitempty"`
	CacheWrite *float64 `json:"cache_write,omitempty"`
	Output     *float64 `json:"output,omitempty"`
	Reasoning  *float64 `json:"reasoning,omitempty"`
}

// ReadPrices reads at most 1 MiB of strict JSON and hashes the exact input bytes.
func ReadPrices(r io.Reader) (Prices, error) {
	b, err := io.ReadAll(io.LimitReader(r, (1<<20)+1))
	if err != nil {
		return Prices{}, err
	}
	if len(b) > 1<<20 || !utf8.Valid(b) {
		return Prices{}, fmt.Errorf("invalid price table size or UTF-8")
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	if err := jsoncheck.Check(d, jsoncheck.Options{RejectNull: true}); err != nil {
		return Prices{}, fmt.Errorf("price table: %w", err)
	}
	if err := priceFields(b); err != nil {
		return Prices{}, err
	}
	d = json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	var p Prices
	if err := d.Decode(&p); err != nil {
		return Prices{}, fmt.Errorf("price table: %w", err)
	}
	if err := validatePrices(p); err != nil {
		return Prices{}, err
	}
	h := sha256.Sum256(b)
	p.hash = hex.EncodeToString(h[:])
	p.canonical, _ = json.Marshal(p)
	return p, nil
}
func validatePrices(p Prices) error {
	if len(p.Currency) != 3 {
		return fmt.Errorf("currency must be three uppercase letters")
	}
	for _, c := range p.Currency {
		if c < 'A' || c > 'Z' {
			return fmt.Errorf("currency must be three uppercase letters")
		}
	}
	u, e := url.Parse(p.SourceURL)
	if e != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil {
		return fmt.Errorf("invalid price source URL")
	}
	if _, e := time.Parse("2006-01-02", p.EffectiveDate); e != nil {
		return fmt.Errorf("invalid price effective date")
	}
	if len(p.Models) == 0 || len(p.Models) > 4096 {
		return fmt.Errorf("invalid price model count")
	}
	for model, r := range p.Models {
		if model == "" || len(model) > 1024 {
			return fmt.Errorf("invalid price model")
		}
		for _, v := range []*float64{r.Input, r.CacheRead, r.CacheWrite, r.Output, r.Reasoning} {
			if v != nil && (math.IsNaN(*v) || math.IsInf(*v, 0) || *v < 0) {
				return fmt.Errorf("invalid rate for %s", model)
			}
		}
	}
	return nil
}

// CostRecord preserves unavailable and excluded records without assigning a zero charge.
type CostRecord struct {
	Index       int      `json:"index"`
	Model       string   `json:"model"`
	Amount      *float64 `json:"amount"`
	Unavailable string   `json:"unavailable,omitempty"`
	Excluded    string   `json:"excluded,omitempty"`
}

// CostLedger is a derived estimate for retained measured successes, not a provider bill.
// Total is unavailable if any eligible record is unavailable or records were dropped.
type CostLedger struct {
	SchemaVersion int          `json:"schema_version"`
	RunID         string       `json:"run_id"`
	Currency      string       `json:"currency"`
	SourceURL     string       `json:"source_url"`
	EffectiveDate string       `json:"effective_date"`
	PriceHash     string       `json:"price_sha256"`
	Policy        string       `json:"policy"`
	Total         *float64     `json:"total"`
	KnownSubtotal float64      `json:"known_subtotal"`
	Unavailable   string       `json:"unavailable,omitempty"`
	Records       []CostRecord `json:"records"`
}

// DeriveCost validates the run using Summarize before applying explicit prices.
// Prices must originate from ReadPrices so provenance identifies the exact table.
func DeriveCost(run llm.RunSummary, p Prices) (CostLedger, error) {
	if _, err := Summarize(run); err != nil {
		return CostLedger{}, err
	}
	if err := validatePrices(p); err != nil {
		return CostLedger{}, err
	}
	canonical, err := json.Marshal(p)
	if err != nil || !bytes.Equal(canonical, p.canonical) {
		return CostLedger{}, fmt.Errorf("price table changed after admission")
	}
	if len(p.hash) != 64 {
		return CostLedger{}, fmt.Errorf("price provenance missing")
	}
	if _, err := hex.DecodeString(p.hash); err != nil {
		return CostLedger{}, fmt.Errorf("invalid price provenance")
	}
	l := CostLedger{SchemaVersion: 1, RunID: run.RunID, Currency: p.Currency, SourceURL: p.SourceURL, EffectiveDate: p.EffectiveDate, PriceHash: p.hash, Policy: "caller-declared prices per million reported tokens; retained measurement successes only; cache buckets subtract from inclusive input; reasoning is a subset of output; no provider charge claim", Records: []CostRecord{}}
	for i, m := range run.Records {
		r := CostRecord{Index: i, Model: m.Model}
		if m.Phase != "measurement" {
			r.Excluded = "non-measurement phase"
		} else if m.Outcome != "success" {
			r.Excluded = "unsuccessful request"
		} else {
			amount, why, err := recordCost(m, p.Models)
			if err != nil {
				return CostLedger{}, fmt.Errorf("cost record %d: %w", i, err)
			}
			r.Amount, r.Unavailable = amount, why
			if amount != nil {
				l.KnownSubtotal += *amount
				if math.IsInf(l.KnownSubtotal, 0) {
					return CostLedger{}, fmt.Errorf("cost total overflow")
				}
			} else {
				l.Unavailable = "incomplete eligible usage or prices"
			}
		}
		l.Records = append(l.Records, r)
	}
	if run.DroppedRecords > 0 {
		l.Unavailable = "dropped records"
	}
	if l.Unavailable == "" {
		v := l.KnownSubtotal
		l.Total = &v
	}
	return l, nil
}
func recordCost(m llm.Measurement, models map[string]TokenPrices) (*float64, string, error) {
	if m.InputTokens.Value == nil || m.OutputTokens.Value == nil {
		return nil, "missing token counts", nil
	}
	if m.InputTokens.Source != "reported" || m.OutputTokens.Source != "reported" {
		return nil, "token counts not directly reported", nil
	}
	in, out := *m.InputTokens.Value, *m.OutputTokens.Value
	if in < 0 || out < 0 {
		return nil, "", fmt.Errorf("negative token count")
	}
	read, write := 0, 0
	if in > 0 && m.CachedTokens == nil {
		return nil, "cache bucket presence unavailable", nil
	}
	if m.CachedTokens != nil {
		read = *m.CachedTokens
	}
	if m.CacheWriteTokens != nil {
		write = *m.CacheWriteTokens
	}
	switch m.Protocol {
	case "", "openai-chat", "openai-completions", "openai-responses":
		if write != 0 {
			return nil, "", fmt.Errorf("unsupported cache write bucket")
		}
	case "anthropic":
		if m.CacheWriteTokens == nil {
			return nil, "cache write presence unavailable", nil
		}
	case "providerwire-v4":
		if (in > 0 || read > 0) && m.CacheWriteTokens == nil {
			return nil, "cache write presence unavailable", nil
		}
	default:
		return nil, "unsupported protocol billing convention", nil
	}
	if read < 0 || write < 0 || read > in || write > in-read {
		return nil, "", fmt.Errorf("cache buckets exceed inclusive input")
	}
	reason := 0
	if m.ReasoningTokens != nil {
		reason = *m.ReasoningTokens
		if reason < 0 || reason > out {
			return nil, "", fmt.Errorf("reasoning exceeds output")
		}
	}
	if m.ProviderTotalTokens != nil {
		total := *m.ProviderTotalTokens
		// Subtraction avoids overflowing an input+output sum.
		if total < in || total-in != out {
			return nil, "", fmt.Errorf("provider usage inconsistent")
		}
	}
	rates, ok := models[m.Model]
	if !ok {
		return nil, "model price missing", nil
	}
	if (m.Protocol == "" || m.Protocol == "openai-chat" || m.Protocol == "openai-completions" || m.Protocol == "openai-responses") && rates.CacheWrite != nil {
		return nil, "", fmt.Errorf("cache write price unsupported for protocol")
	}
	type bucket struct {
		n    int
		rate *float64
		name string
	}
	buckets := []bucket{{in - read - write, rates.Input, "input"}, {read, rates.CacheRead, "cache_read"}, {write, rates.CacheWrite, "cache_write"}, {out, rates.Output, "output"}}
	if rates.Reasoning != nil {
		if out > 0 && m.ReasoningTokens == nil {
			return nil, "reasoning count unavailable", nil
		}
		buckets[3].n = out - reason
		buckets = append(buckets, bucket{reason, rates.Reasoning, "reasoning"})
	}
	total := 0.0
	for _, b := range buckets {
		if b.n == 0 {
			continue
		}
		if b.rate == nil {
			return nil, b.name + " price missing", nil
		}
		total += float64(b.n) / 1e6 * *b.rate
	}
	if math.IsInf(total, 0) || math.IsNaN(total) {
		return nil, "", fmt.Errorf("cost overflow")
	}
	return &total, "", nil
}

// Typed decoding accepts case-folded field aliases; the price format does not.
func priceFields(b []byte) error {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(b, &top); err != nil {
		return err
	}
	for key := range top {
		switch key {
		case "currency", "source_url", "effective_date", "models":
		default:
			return fmt.Errorf("unknown price field %q", key)
		}
	}
	var models map[string]map[string]json.RawMessage
	if err := json.Unmarshal(top["models"], &models); err != nil {
		return err
	}
	for _, rates := range models {
		for key := range rates {
			switch key {
			case "input", "cache_read", "cache_write", "output", "reasoning":
			default:
				return fmt.Errorf("unknown rate field %q", key)
			}
		}
	}
	return nil
}
