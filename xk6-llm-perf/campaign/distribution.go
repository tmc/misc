package campaign

import (
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"sort"
)

// DistributionConverged requires every pair of retained run distributions to
// meet an empirical two-sample Kolmogorov-Smirnov distance bound. The tolerance
// is an effect-size bound, not a hypothesis-test significance or confidence.
// Each run must contain between 2 and 10,000 finite observations.
func DistributionConverged(samples [][]float64, tolerance float64, minimum int) bool {
	if minimum < 2 || len(samples) < minimum || !finite(tolerance) || tolerance < 0 || tolerance > 1 {
		return false
	}
	sorted := make([][]float64, len(samples))
	for i, s := range samples {
		if len(s) < 2 || len(s) > 10000 {
			return false
		}
		sorted[i] = append([]float64(nil), s...)
		for _, v := range s {
			if !finite(v) {
				return false
			}
		}
		sort.Float64s(sorted[i])
	}
	for i := range sorted {
		for j := 0; j < i; j++ {
			if ksDistance(sorted[i], sorted[j]) > tolerance {
				return false
			}
		}
	}
	return true
}
func ksDistance(a, b []float64) float64 {
	i, j := 0, 0
	distance := 0.0
	for i < len(a) || j < len(b) {
		value := math.Inf(1)
		if i < len(a) {
			value = a[i]
		}
		if j < len(b) {
			value = math.Min(value, b[j])
		}
		for i < len(a) && a[i] <= value {
			i++
		}
		for j < len(b) && b[j] <= value {
			j++
		}
		distance = math.Max(distance, math.Abs(float64(i)/float64(len(a))-float64(j)/float64(len(b))))
	}
	return distance
}
func distribution(path string) ([]float64, string, error) {
	b, e := readLimited(path, 1<<20)
	if e != nil {
		return nil, "", e
	}
	var values []float64
	if e = json.Unmarshal(b, &values); e != nil {
		return nil, "", e
	}
	if len(values) < 2 || len(values) > 10000 {
		return nil, "", fmt.Errorf("distribution requires 2..10000 samples")
	}
	for _, v := range values {
		if !finite(v) {
			return nil, "", fmt.Errorf("nonfinite distribution sample")
		}
	}
	return values, digest(b), nil
}
func updateDistribution(cell *Candidate, c Config, root string) error {
	if c.Convergence != "distribution" {
		return nil
	}
	var samples [][]float64
	failures := 0
	for _, a := range cell.Attempts {
		if a.Error != "" || a.ExitCode != 0 || a.Value == nil {
			failures++
			continue
		}
		values, hash, e := distribution(filepath.Join(root, a.Directory, c.DistributionFile))
		if e != nil || hash != a.DistributionHash || len(values) != a.DistributionCount {
			return fmt.Errorf("distribution receipt missing or changed")
		}
		samples = append(samples, values)
	}
	cell.Converged = DistributionConverged(samples, c.Tolerance, c.MinRuns)
	cell.Failed = failures > c.Retries || len(samples) >= c.MaxRuns && !cell.Converged || len(cell.Attempts) >= c.MaxRuns+c.Retries && !cell.Converged
	return nil
}
