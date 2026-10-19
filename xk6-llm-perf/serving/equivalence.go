package serving

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
)

// EquivalenceMargin bounds the mean paired delta, B minus A, in the recorded
// unit. Reason states the practical basis chosen before observing main samples.
type EquivalenceMargin struct {
	Metric string  `json:"metric"`
	Unit   string  `json:"unit"`
	Lower  float64 `json:"lower"`
	Upper  float64 `json:"upper"`
	Reason string  `json:"reason"`
}

// EquivalencePlan freezes independent process pairs, margins and estimator.
// SampleSizeReason records the power or feasibility rationale for the fixed
// pair count. Requests within a process are not independent replicates.
// Retain the plan and hash before main sampling; hashing cannot prove timing
// of preregistration or the scientific validity of the rationale.
type EquivalencePlan struct {
	Pairs            PairPlan            `json:"pairs"`
	Margins          []EquivalenceMargin `json:"margins"`
	BootstrapSeed    int64               `json:"bootstrap_seed"`
	Resamples        int                 `json:"resamples"`
	Confidence       float64             `json:"confidence"`
	SampleSizeReason string              `json:"sample_size_reason"`
}

// Hash validates and identifies a preregistration. It does not authenticate runs.
func (p EquivalencePlan) Hash() (string, error) {
	expected, err := PlanPairs(p.Pairs.Seed, len(p.Pairs.Orders), p.Pairs.Metrics)
	if err != nil {
		return "", err
	}
	if p.Pairs.Version != expected.Version || p.Pairs.Primary != expected.Primary || !reflect.DeepEqual(p.Pairs.Orders, expected.Orders) {
		return "", fmt.Errorf("equivalence: invalid process pair plan")
	}
	if p.Pairs.Policy == nil || p.Pairs.Policy.SLO == nil || p.Pairs.Policy.SLO.MaxTPOT == nil || p.Pairs.Policy.SLO.MaxE2E == nil {
		return "", fmt.Errorf("equivalence: incomplete measurement policy")
	}
	if _, err := PairPolicyHash(*p.Pairs.Policy); err != nil {
		return "", err
	}
	if p.SampleSizeReason == "" || len(p.Margins) != len(p.Pairs.Metrics) {
		return "", fmt.Errorf("equivalence: missing sample rationale or margins")
	}
	for i, m := range p.Margins {
		if m.Metric != p.Pairs.Metrics[i] || m.Unit == "" || m.Reason == "" || math.IsNaN(m.Lower) || math.IsNaN(m.Upper) || math.IsInf(m.Lower, 0) || math.IsInf(m.Upper, 0) || m.Lower >= 0 || m.Upper <= 0 {
			return "", fmt.Errorf("equivalence: invalid margin for metric %q", m.Metric)
		}
	}
	if p.Resamples < 1000 || p.Resamples > 100000 || math.IsNaN(p.Confidence) || math.IsInf(p.Confidence, 0) || p.Confidence <= 0 || p.Confidence >= 1 {
		return "", fmt.Errorf("equivalence: invalid bootstrap settings")
	}
	if int64(len(p.Pairs.Orders))*int64(p.Resamples)*int64(len(p.Margins)) > 20000000 || (1-p.Confidence)/float64(4*len(p.Margins))*float64(p.Resamples) < 1 {
		return "", fmt.Errorf("equivalence: insufficient bootstrap budget or tail resolution")
	}
	b, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), nil
}

// EquivalenceResult retains every effect and margin, including failed bounds.
// PERFORMANCE_HOLD is inconclusive and does not assert a difference.
type EquivalenceResult struct {
	Status       string          `json:"status"`
	PlanHash     string          `json:"plan_hash"`
	Plan         EquivalencePlan `json:"plan"`
	Paired       PairedResult    `json:"paired"`
	WithinBounds []bool          `json:"within_bounds"`
}

// AnalyzeEquivalence requires each simultaneous paired-delta interval strictly
// inside both practical limits. expectedHash identifies the externally retained
// preregistration. o supplies AnalyzePaired's receipt verifiers and must match
// the fixed estimator. Failed arms and incomplete policies are never discarded.
// Missing verification or client qualification retains PERFORMANCE_HOLD.
func AnalyzeEquivalence(p EquivalencePlan, expectedHash string, e PairedExperiment, o PairedOptions) (EquivalenceResult, error) {
	r := EquivalenceResult{Status: "PERFORMANCE_HOLD", Plan: p}
	hash, err := p.Hash()
	if err != nil {
		return r, err
	}
	r.PlanHash = hash
	if !digest(expectedHash) || hash != expectedHash {
		return r, fmt.Errorf("equivalence: preregistration hash differs")
	}
	if !reflect.DeepEqual(p.Pairs, e.Plan) {
		return r, fmt.Errorf("equivalence: experiment differs from preregistered pair plan")
	}
	if o.Seed != p.BootstrapSeed || o.Resamples != p.Resamples || o.Confidence != p.Confidence {
		return r, fmt.Errorf("equivalence: estimator differs from preregistration")
	}
	for i, run := range e.Runs {
		if !run.PolicyComplete || run.Policy == nil || run.Policy.SLO == nil || run.Policy.SLO.MaxTPOT == nil || run.Policy.SLO.MaxE2E == nil {
			return r, fmt.Errorf("equivalence: run %d has incomplete measurement policy", i)
		}
	}
	r.Paired, err = AnalyzePaired(e, o)
	if err != nil {
		return r, err
	}
	pass := r.Paired.Qualified
	for i, effect := range r.Paired.Effects {
		m := p.Margins[i]
		within := effect.DeltaLow > m.Lower && effect.DeltaHigh < m.Upper
		r.WithinBounds = append(r.WithinBounds, within)
		pass = pass && within
	}
	if pass {
		r.Status = "PERFORMANCE_EQUIVALENT"
	}
	return r, nil
}
