package accuracy

import (
	"fmt"
	"math"
)

// Interval describes a Wilson score interval for a binary proportion.
// Available is false for zero Total; Lower and Upper then have no meaning.
type Interval struct {
	Correct    int
	Total      int
	Confidence float64
	Available  bool
	Lower      float64
	Upper      float64
}

// WilsonInterval returns a two-sided Wilson score interval with confidence
// strictly between zero and one. Pass Report.Correct and Report.Total to retain
// failed and unavailable declared cases in the denominator as Report.Accuracy does.
// Zero total returns an unavailable interval.
//
// This calculation is descriptive and conditional. A confidence interpretation
// requires independent Bernoulli samples from the population being estimated.
// Fixed questions, repeated generations and selected best runs may violate that
// assumption. It does not account for dataset selection, grader error or adaptive
// search, and does not establish AIPerf population equivalence.
func WilsonInterval(correct, total int, confidence float64) (Interval, error) {
	if total < 0 || correct < 0 || correct > total {
		return Interval{}, fmt.Errorf("invalid binary counts")
	}
	if math.IsNaN(confidence) || confidence <= 0 || confidence >= 1 {
		return Interval{}, fmt.Errorf("confidence must be between zero and one")
	}
	r := Interval{Correct: correct, Total: total, Confidence: confidence}
	if total == 0 {
		return r, nil
	}
	// Avoid rounding (1+confidence)/2 to one near confidence=1.
	z := math.Sqrt2 * math.Erfinv(confidence)
	n := float64(total)
	p := float64(correct) / n
	z2 := z * z
	denom := 1 + z2/n
	center := (p + z2/(2*n)) / denom
	half := z * math.Sqrt(p*(1-p)/n+z2/(4*n*n)) / denom
	r.Available = true
	r.Lower = math.Max(0, center-half)
	r.Upper = math.Min(1, center+half)
	return r, nil
}
