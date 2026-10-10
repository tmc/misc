package campaign

import (
	"fmt"
	"math"
	"testing"
)

func TestDistributionConvergence(t *testing.T) {
	for _, tt := range []struct {
		name string
		s    [][]float64
		tol  float64
		want bool
	}{{"same", [][]float64{{1, 2, 3}, {3, 2, 1}}, 0, true}, {"shift", [][]float64{{1, 2}, {3, 4}}, .5, false}, {"equal ties", [][]float64{{1, 1}, {1, 1, 1}}, 0, true}, {"all pairs", [][]float64{{0, 0}, {0, 1}, {1, 1}}, .6, false}, {"few", [][]float64{{1}, {1}}, 1, false}, {"nonfinite", [][]float64{{1, 2}, {math.NaN(), 2}}, 1, false}} {
		t.Run(tt.name, func(t *testing.T) {
			if got := DistributionConverged(tt.s, tt.tol, 2); got != tt.want {
				t.Fatalf("got %v want %v", got, tt.want)
			}
		})
	}
}
func ExampleDistributionConverged() {
	fmt.Println(DistributionConverged([][]float64{{1, 2, 3}, {1, 2, 3}}, .05, 2))
	// Output: true
}
