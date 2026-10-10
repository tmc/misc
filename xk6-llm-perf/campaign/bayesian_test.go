package campaign

import (
	"math"
	"testing"
)

func TestGaussianPosterior(t *testing.T) {
	g := newGP([][]float64{{0}, {1}}, []float64{0, 1})
	if g == nil {
		t.Fatal("fit failed")
	}
	mean, v := g.predict([]float64{1})
	if math.Abs(mean-1) > 1e-5 || v > 1e-5 {
		t.Fatalf("observed posterior: %g %g", mean, v)
	}
	_, middle := g.predict([]float64{.5})
	if middle < .1 {
		t.Fatalf("unobserved uncertainty lost: %g", middle)
	}
}
func TestBayesianFiniteBudget(t *testing.T) {
	c := Config{Mode: "bayesian", Dimensions: []Dimension{{"LOAD", []string{"1", "2", "3", "4", "5"}}}}
	m := &Manifest{}
	used := map[string]bool{}
	for i := 0; i < 5; i++ {
		p := next(c, grid(c.Dimensions), m)
		if p == nil || used[cellID(p)] {
			t.Fatal("repeated cell")
		}
		used[cellID(p)] = true
		v := float64(i)
		m.Candidates = append(m.Candidates, Candidate{ID: cellID(p), Parameters: p, Mean: &v, Converged: true, Feasible: true})
	}
	if next(c, grid(c.Dimensions), m) != nil {
		t.Fatal("unbounded planner")
	}
}
