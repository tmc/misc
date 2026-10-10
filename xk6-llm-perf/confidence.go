package llmperf

import (
	"fmt"
	"math"
	"math/rand/v2"
	"sort"
)

// ConfidenceOptions configures a percentile bootstrap of a request-latency
// mean. Independent must explicitly acknowledge the independent-observation
// assumption; correlated request samples do not establish run-level confidence.
type ConfidenceOptions struct {
	Seed        uint64
	Replicates  int
	Level       float64
	Independent bool
}

// Confidence reports a declared statistical estimator, its population, units,
// seed and bounds. It is not a matched-run performance or convergence claim.
type Confidence struct {
	Method     string  `json:"method"`
	Population string  `json:"population"`
	Unit       string  `json:"unit"`
	Samples    int     `json:"samples"`
	Replicates int     `json:"replicates"`
	Seed       uint64  `json:"seed"`
	Level      float64 `json:"level"`
	Mean       float64 `json:"mean"`
	Lower      float64 `json:"lower"`
	Upper      float64 `json:"upper"`
	Assumption string  `json:"assumption"`
}

// LatencyConfidence bootstraps the mean full-response latency of successful
// requests. It requires complete retained records and at least two observations.
func LatencyConfidence(run RunSummary, opts ConfidenceOptions) (Confidence, error) {
	var c Confidence
	if _, err := AnalyzeTimeline(run); err != nil {
		return c, err
	}
	if !opts.Independent {
		return c, fmt.Errorf("independent-observation assumption must be acknowledged")
	}
	if opts.Replicates < 100 || opts.Replicates > 10000 || !finite(opts.Level) || opts.Level <= 0 || opts.Level >= 1 {
		return c, fmt.Errorf("invalid confidence level or bootstrap budget")
	}
	var values []float64
	for _, m := range run.Records {
		if m.Outcome == "success" {
			if !finite(m.FullResponseLatency) || m.FullResponseLatency < 0 {
				return c, fmt.Errorf("invalid request latency")
			}
			values = append(values, m.FullResponseLatency)
		}
	}
	if len(values) < 2 {
		return c, fmt.Errorf("confidence requires at least two successful observations")
	}
	if len(values)*opts.Replicates > 10000000 {
		return c, fmt.Errorf("bootstrap exceeds ten million resampled observations")
	}
	c = Confidence{Method: "percentile bootstrap of arithmetic mean; nearest-rank interval", Population: "successful whole-contained requests", Unit: "ms", Samples: len(values), Replicates: opts.Replicates, Seed: opts.Seed, Level: opts.Level, Assumption: "independent identically distributed observations acknowledged by caller"}
	for _, v := range values {
		c.Mean += v / float64(len(values))
	}
	rng := rand.New(rand.NewPCG(opts.Seed, opts.Seed^0x9e3779b97f4a7c15))
	means := make([]float64, opts.Replicates)
	for i := range means {
		for range values {
			means[i] += values[rng.IntN(len(values))] / float64(len(values))
		}
	}
	sort.Float64s(means)
	tail := (1 - opts.Level) / 2
	c.Lower = means[max(0, int(math.Ceil(tail*float64(len(means))))-1)]
	c.Upper = means[min(len(means)-1, int(math.Ceil((1-tail)*float64(len(means))))-1)]
	return c, nil
}
