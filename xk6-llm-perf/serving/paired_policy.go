package serving

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"

	llmperf "github.com/tmc/misc/xk6-llm-perf"
)

// PairPolicy preregisters the measurement population and explicit SLO limits.
// Zero limits disable that SLO; an absent SLO is unavailable evidence.
type PairPolicy struct {
	MeasurementPolicy string              `json:"measurement_policy"`
	WindowPolicy      string              `json:"window_policy"`
	WarmupPolicy      string              `json:"warmup_policy"`
	SLO               *llmperf.SLOReceipt `json:"slo,omitempty"`
}

// PairPolicyHash identifies a supported measurement and declared SLO policy.
// Legacy receipts retain their identity; hashing does not make missing limits available.
// A hash establishes identity, not evidence that a run applied the policy.
func PairPolicyHash(p PairPolicy) (string, error) {
	if p.MeasurementPolicy != "dispatch-content-v1/success-token-counts-v1/success-slo-v1" || p.WindowPolicy != "attempt started and completed inside window; successful token totals" || p.WarmupPolicy != "phase=measurement and per-client warmup exhausted" {
		return "", fmt.Errorf("unsupported or incomplete paired measurement policy")
	}
	if p.SLO == nil {
		return "", fmt.Errorf("paired slo policy is unavailable")
	}
	for _, limit := range []float64{p.SLO.MaxTTFT, p.SLO.MaxTTFO, p.SLO.MaxChunkLatency} {
		if math.IsNaN(limit) || math.IsInf(limit, 0) || limit < 0 {
			return "", fmt.Errorf("invalid paired slo limit")
		}
	}
	for _, limit := range []*float64{p.SLO.MaxTPOT, p.SLO.MaxE2E} {
		if limit != nil && (math.IsNaN(*limit) || math.IsInf(*limit, 0) || *limit < 0) {
			return "", fmt.Errorf("invalid paired slo limit")
		}
	}

	slo := *p.SLO
	for _, limit := range []*float64{&slo.MaxTTFT, &slo.MaxTTFO, &slo.MaxChunkLatency} {
		if *limit == 0 {
			*limit = 0
		}
	}
	slo.MaxTPOT = canonicalSLOLimit(slo.MaxTPOT)
	slo.MaxE2E = canonicalSLOLimit(slo.MaxE2E)
	p.SLO = &slo
	b, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), nil
}

// RunPairPolicy derives policy from a validated run and its actual SLO receipts.
// The boolean reports complete extended policy evidence, including explicit TPOT
// and E2E limits. Missing fields remain unavailable.
// Known policy disagreement is an error, including
// disagreement in a run that also contains requests with unavailable receipts.
func RunPairPolicy(run llmperf.RunSummary) (*PairPolicy, bool, error) {
	if _, err := llmperf.AnalyzeRequests(run); err != nil {
		return nil, false, err
	}
	var slo *llmperf.SLOReceipt
	missing := len(run.Records) == 0
	for _, m := range run.Records {
		if m.SLO == nil {
			missing = true
			continue
		}
		if slo == nil {
			v := *m.SLO
			slo = &v
		} else if !slo.Equal(*m.SLO) {
			return nil, false, fmt.Errorf("slo policy differs within run")
		}
	}

	p := &PairPolicy{MeasurementPolicy: run.MeasurementPolicy, WindowPolicy: run.WindowPolicy, WarmupPolicy: run.WarmupPolicy, SLO: slo}
	if p.MeasurementPolicy != "" && p.WindowPolicy != "" && p.WarmupPolicy != "" && p.SLO != nil {
		if _, err := PairPolicyHash(*p); err != nil {
			return nil, false, err
		}
	}
	return p, !missing && p.SLO != nil && p.SLO.MaxTPOT != nil && p.SLO.MaxE2E != nil && p.MeasurementPolicy != "" && p.WindowPolicy != "" && p.WarmupPolicy != "", nil
}

// MergePairPolicy reconciles known policy evidence. Missing fields remain
// unavailable, never inferred. Any known disagreement is an error.
func MergePairPolicy(p, q PairPolicy) (PairPolicy, error) {
	for _, x := range []struct {
		a *string
		b string
	}{{&p.MeasurementPolicy, q.MeasurementPolicy}, {&p.WindowPolicy, q.WindowPolicy}, {&p.WarmupPolicy, q.WarmupPolicy}} {
		if *x.a != "" && x.b != "" && *x.a != x.b {
			return p, fmt.Errorf("measurement policy differs between runs")
		}
		if *x.a == "" {
			*x.a = x.b
		}
	}
	if p.SLO != nil && q.SLO != nil && !p.SLO.Equal(*q.SLO) {
		return p, fmt.Errorf("slo policy differs between runs")
	}
	if p.SLO == nil && q.SLO != nil {
		v := *q.SLO
		p.SLO = &v
	}
	return p, nil
}

// canonicalSLOLimit copies a declared limit and gives zero a single encoding.
func canonicalSLOLimit(limit *float64) *float64 {
	if limit == nil {
		return nil
	}
	value := *limit
	if value == 0 {
		value = 0
	}
	return &value
}
