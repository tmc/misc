package serving

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"sort"

	"github.com/tmc/misc/xk6-llm-perf/internal/jsoncheck"
)

// PairPlan fixes the process-run population and order before execution.
type PairPlan struct {
	Version int         `json:"version"`
	Seed    int64       `json:"seed"`
	Orders  []string    `json:"orders"`
	Metrics []string    `json:"metrics"`
	Primary string      `json:"primary"`
	Policy  *PairPolicy `json:"policy,omitempty"`
}

// PairRun is one independent process aggregate, never a request observation.
type PairRun struct {
	Pair            int                `json:"pair"`
	Arm             string             `json:"arm"`
	ProcessID       string             `json:"process_id"`
	WorkHash        string             `json:"work_hash"`
	BinaryHash      string             `json:"binary_hash"`
	Policy          *PairPolicy        `json:"policy,omitempty"`
	PolicyComplete  bool               `json:"policy_complete"`
	PolicyHash      string             `json:"policy_hash,omitempty"`
	ArtifactHash    string             `json:"artifact_hash"`
	WorkAccepted    bool               `json:"work_accepted"`
	ClientQualified bool               `json:"client_qualified"`
	Successful      bool               `json:"successful"`
	Metrics         map[string]float64 `json:"metrics"`
}

// PairedExperiment retains the preregistration and every attempted arm.
type PairedExperiment struct {
	Plan PairPlan  `json:"plan"`
	Runs []PairRun `json:"runs"`
}

// PairedOptions controls a seeded bootstrap of independent pairs.
type PairedOptions struct {
	Seed       int64
	Resamples  int
	Confidence float64
	Verify     func(PairRun) error `json:"-"`
	// VerifyPolicy must authenticate policy_hash against actual run receipts.
	// Returning nil asserts verified policy provenance; a caller hash alone does not.
	VerifyPolicy func(PairRun, PairPolicy) error `json:"-"`
}

// Effect reports mean paired deltas and ratios with family-wise bounds.
type Effect struct {
	Metric            string   `json:"metric"`
	Delta             float64  `json:"delta"`
	DeltaLow          float64  `json:"delta_low"`
	DeltaHigh         float64  `json:"delta_high"`
	Ratio             *float64 `json:"ratio"`
	RatioLow          *float64 `json:"ratio_low"`
	RatioHigh         *float64 `json:"ratio_high"`
	StatisticalStatus string   `json:"statistical_status"`
}

// PairedResult records assumptions as part of the reported result.
type PairedResult struct {
	Qualified     bool     `json:"qualified"`
	Pairs         int      `json:"pairs"`
	BootstrapSeed int64    `json:"bootstrap_seed"`
	Resamples     int      `json:"resamples"`
	Confidence    float64  `json:"confidence"`
	Method        string   `json:"method"`
	Assumptions   string   `json:"assumptions"`
	Effects       []Effect `json:"effects"`
}

// PlanPairs preregisters balanced AB/BA orders, shuffled with seed.
func PlanPairs(seed int64, pairs int, metrics []string) (PairPlan, error) {
	p := PairPlan{Version: 1, Seed: seed, Metrics: append([]string(nil), metrics...)}
	if len(metrics) > 0 {
		p.Primary = metrics[0]
	}
	if pairs < 2 || pairs > 10000 || pairs%2 != 0 {
		return p, fmt.Errorf("pairs must be even and between 2 and 10000")
	}
	if len(metrics) == 0 || len(metrics) > 64 {
		return p, fmt.Errorf("metric family must contain 1 to 64 metrics")
	}
	seen := map[string]bool{}
	for _, m := range metrics {
		if m == "" || seen[m] {
			return p, fmt.Errorf("empty or duplicate metric")
		}
		seen[m] = true
	}
	for i := 0; i < pairs; i++ {
		if i%2 == 0 {
			p.Orders = append(p.Orders, "AB")
		} else {
			p.Orders = append(p.Orders, "BA")
		}
	}
	rand.New(rand.NewSource(seed)).Shuffle(pairs, func(i, j int) { p.Orders[i], p.Orders[j] = p.Orders[j], p.Orders[i] })
	return p, nil
}
func digest(s string) bool { b, e := hex.DecodeString(s); return e == nil && len(b) == sha256.Size }

// ReadPairedExperiment rejects unknown fields, trailing data, and oversized input.
func ReadPairedExperiment(r io.Reader) (PairedExperiment, error) {
	var e PairedExperiment
	b, err := io.ReadAll(io.LimitReader(r, 16<<20+1))
	if err != nil {
		return e, err
	}
	if len(b) > 16<<20 {
		return e, fmt.Errorf("experiment exceeds 16 MiB")
	}
	if err := validatePairJSON(json.NewDecoder(bytes.NewReader(b))); err != nil {
		return e, err
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err = d.Decode(&e); err != nil {
		return e, err
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return e, fmt.Errorf("trailing experiment data")
	}
	return e, nil
}

// AnalyzePaired refuses missing, failed, or confounded process runs. No run is
// dropped or retried. Bounds use paired percentile bootstrap and Bonferroni
// correction over the preregistered metric family.
func AnalyzePaired(e PairedExperiment, o PairedOptions) (PairedResult, error) {
	result := PairedResult{BootstrapSeed: o.Seed, Resamples: o.Resamples, Confidence: o.Confidence, Pairs: len(e.Plan.Orders), Method: "seeded paired percentile bootstrap; Bonferroni simultaneous delta and ratio bounds", Assumptions: "independent process pairs; matched effective work; stable execution conditions; primary estimand is mean paired delta for the primary metric; aggregate estimands fixed before execution; bootstrap coverage is asymptotic"}
	p, err := PlanPairs(e.Plan.Seed, len(e.Plan.Orders), e.Plan.Metrics)
	if err != nil {
		return result, err
	}
	if e.Plan.Primary != p.Primary {
		return result, fmt.Errorf("primary metric differs from preregistration")
	}
	if e.Plan.Version != 1 {
		return result, fmt.Errorf("unsupported plan version")
	}
	for i := range p.Orders {
		if p.Orders[i] != e.Plan.Orders[i] {
			return result, fmt.Errorf("plan order differs from preregistration")
		}
	}
	if len(e.Runs) != 2*len(p.Orders) {
		return result, fmt.Errorf("missing or additional arm runs")
	}
	if o.Resamples < 1000 || o.Resamples > 100000 || math.IsNaN(o.Confidence) || math.IsInf(o.Confidence, 0) || o.Confidence <= 0 || o.Confidence >= 1 {
		return result, fmt.Errorf("invalid bootstrap options")
	}
	expectedPolicy := ""
	if e.Plan.Policy != nil {
		expectedPolicy, err = PairPolicyHash(*e.Plan.Policy)
		if err != nil {
			return result, err
		}
	}
	observedPolicy := ""
	observedEvidence := PairPolicy{}
	policyVerified := expectedPolicy != "" && o.VerifyPolicy != nil && e.Plan.Policy.SLO.MaxTPOT != nil && e.Plan.Policy.SLO.MaxE2E != nil
	processes := map[string]bool{}
	work := ""
	hashes := map[string]string{}
	for i, r := range e.Runs {
		if o.Verify != nil {
			if err := o.Verify(r); err != nil {
				return result, fmt.Errorf("verify run %d: %w", i, err)
			}
		}
		if r.Policy != nil {
			observedEvidence, err = MergePairPolicy(observedEvidence, *r.Policy)
			if err != nil {
				return result, err
			}
			if e.Plan.Policy != nil {
				if _, err := MergePairPolicy(*e.Plan.Policy, *r.Policy); err != nil {
					return result, err
				}
			}
			if r.PolicyComplete && r.Policy.MeasurementPolicy != "" && r.Policy.WindowPolicy != "" && r.Policy.WarmupPolicy != "" && r.Policy.SLO != nil && r.Policy.SLO.MaxTPOT != nil && r.Policy.SLO.MaxE2E != nil {
				actual, err := PairPolicyHash(*r.Policy)
				if err != nil {
					return result, err
				}
				if actual != r.PolicyHash {
					return result, fmt.Errorf("run policy hash contradicts policy evidence")
				}
			} else {
				policyVerified = false
				if r.PolicyHash != "" {
					return result, fmt.Errorf("incomplete policy has qualified identity")
				}
			}
		} else {
			policyVerified = false
		}
		if r.PolicyHash != "" {
			if !digest(r.PolicyHash) {
				return result, fmt.Errorf("invalid run policy hash")
			}
			if observedPolicy != "" && observedPolicy != r.PolicyHash {
				return result, fmt.Errorf("measurement or SLO policy differs between runs")
			}
			observedPolicy = r.PolicyHash
			if expectedPolicy != "" && r.PolicyHash != expectedPolicy {
				return result, fmt.Errorf("run policy differs from preregistration")
			}
		} else {
			policyVerified = false
		}
		if expectedPolicy != "" && r.PolicyHash != "" && o.VerifyPolicy != nil {
			if err := o.VerifyPolicy(r, *e.Plan.Policy); err != nil {
				return result, fmt.Errorf("verify run policy %d: %w", i, err)
			}
		}
		arm := string(p.Orders[i/2][i%2])
		if r.Pair != i/2 || r.Arm != arm {
			return result, fmt.Errorf("run %d violates pair order", i)
		}
		if r.ProcessID == "" || processes[r.ProcessID] {
			return result, fmt.Errorf("duplicate or missing process identity")
		}
		processes[r.ProcessID] = true
		if !digest(r.WorkHash) || !digest(r.BinaryHash) || !digest(r.ArtifactHash) {
			return result, fmt.Errorf("run %d lacks digest provenance", i)
		}
		if work == "" {
			work = r.WorkHash
		}
		if work != r.WorkHash {
			return result, fmt.Errorf("mismatched effective work")
		}
		if prior := hashes[r.Arm]; prior != "" && prior != r.BinaryHash {
			return result, fmt.Errorf("arm binary changed")
		}
		hashes[r.Arm] = r.BinaryHash
		if !r.Successful || !r.WorkAccepted {
			return result, fmt.Errorf("run %d failed qualification", i)
		}
		if len(r.Metrics) != len(p.Metrics) {
			return result, fmt.Errorf("run metric family mismatch")
		}
		for _, m := range p.Metrics {
			v, ok := r.Metrics[m]
			if !ok || math.IsNaN(v) || math.IsInf(v, 0) {
				return result, fmt.Errorf("invalid metric %q", m)
			}
		}
	}
	n := len(p.Orders)
	if int64(n)*int64(o.Resamples)*int64(len(p.Metrics)) > 20000000 {
		return result, fmt.Errorf("bootstrap exceeds 20 million pair evaluations")
	}
	alpha := (1 - o.Confidence) / float64(2*len(p.Metrics))
	if alpha*float64(o.Resamples)/2 < 1 {
		return result, fmt.Errorf("insufficient bootstrap tail resolution for metric family")
	}
	quantile := func(v []float64, q float64) float64 { sort.Float64s(v); return v[int(math.Floor(q*float64(len(v)-1)))] }
	for _, m := range p.Metrics {
		d := make([]float64, n)
		rat := make([]float64, n)
		available := true
		for i := 0; i < n; i++ {
			a, b := e.Runs[2*i], e.Runs[2*i+1]
			if a.Arm == "B" {
				a, b = b, a
			}
			d[i] = b.Metrics[m] - a.Metrics[m]
			if math.IsInf(d[i], 0) {
				return result, fmt.Errorf("delta overflow")
			}
			if a.Metrics[m] == 0 {
				available = false
			} else {
				rat[i] = b.Metrics[m] / a.Metrics[m]
				if math.IsInf(rat[i], 0) {
					return result, fmt.Errorf("ratio overflow")
				}
			}
		}
		mean := func(v []float64) float64 {
			s := 0.0
			for _, x := range v {
				s += x / float64(len(v))
			}
			return s
		}
		ds := make([]float64, o.Resamples)
		rs := make([]float64, o.Resamples)
		rng := rand.New(rand.NewSource(o.Seed))
		for k := range ds {
			for sample := 0; sample < n; sample++ {
				j := rng.Intn(n)
				ds[k] += d[j] / float64(n)
				rs[k] += rat[j] / float64(n)
			}
		}
		if math.IsNaN(mean(d)) || math.IsInf(mean(d), 0) || math.IsNaN(mean(rat)) || math.IsInf(mean(rat), 0) {
			return result, fmt.Errorf("paired aggregate overflow")
		}
		for _, v := range append(ds, rs...) {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return result, fmt.Errorf("bootstrap aggregate overflow")
			}
		}
		f := Effect{Metric: m, Delta: mean(d), DeltaLow: quantile(ds, alpha/2), DeltaHigh: quantile(ds, 1-alpha/2), StatisticalStatus: "qualified paired bootstrap; asymptotic coverage"}
		if available {
			v, l, h := mean(rat), quantile(rs, alpha/2), quantile(rs, 1-alpha/2)
			f.Ratio = &v
			f.RatioLow = &l
			f.RatioHigh = &h
		} else {
			f.StatisticalStatus = "ratio unavailable: zero baseline; paired delta qualified"
		}
		result.Effects = append(result.Effects, f)
	}
	result.Qualified = o.Verify != nil && policyVerified
	for _, r := range e.Runs {
		if !r.ClientQualified {
			result.Qualified = false
		}
	}
	if !result.Qualified {
		for i := range result.Effects {
			result.Effects[i].StatisticalStatus = "diagnostic only: external provenance, measurement/SLO policy or client-capacity gates unverified; " + result.Effects[i].StatisticalStatus
		}
	}
	return result, nil
}

// validatePairJSON catches duplicate object members and explicit null before
// decoding Go values, where those distinctions would otherwise disappear.
func validatePairJSON(d *json.Decoder) error {
	err := jsoncheck.Value(d, jsoncheck.Options{RejectNull: true})
	switch err {
	case jsoncheck.ErrDepth:
		return fmt.Errorf("experiment JSON nesting exceeds 64 levels")
	case jsoncheck.ErrNull:
		return fmt.Errorf("null experiment field")
	case jsoncheck.ErrDuplicate:
		return fmt.Errorf("duplicate experiment field")
	}
	return err
}
