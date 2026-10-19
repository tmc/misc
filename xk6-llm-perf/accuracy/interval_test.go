package accuracy

import (
	"bytes"
	"encoding/json"
	"math"
	"testing"
)

func TestWilsonInterval(t *testing.T) {
	for _, tt := range []struct {
		name           string
		correct, total int
		low, high      float64
	}{
		{"none", 0, 10, 0, .2775327998628892},
		{"all", 10, 10, .7224672001371107, 1},
		{"half", 5, 10, .236593090512564, .763406909487436},
		{"mixed", 1, 4, .0455872608097006, .699358157417598},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r, err := WilsonInterval(tt.correct, tt.total, .95)
			if err != nil || !r.Available || math.Abs(r.Lower-tt.low) > 1e-12 || math.Abs(r.Upper-tt.high) > 1e-12 {
				t.Fatalf("interval = %+v, %v", r, err)
			}
		})
	}
}

func TestWilsonIntervalUnavailableAndInvalid(t *testing.T) {
	r, err := WilsonInterval(0, 0, .95)
	if err != nil || r.Available || r.Total != 0 || r.Confidence != .95 {
		t.Fatalf("empty = %+v, %v", r, err)
	}
	for _, counts := range [][2]int{{-1, 1}, {0, -1}, {2, 1}} {
		if _, err := WilsonInterval(counts[0], counts[1], .95); err == nil {
			t.Fatalf("accepted counts %v", counts)
		}
	}
	for _, confidence := range []float64{0, 1, -1, math.NaN(), math.Inf(1), math.Inf(-1)} {
		if _, err := WilsonInterval(0, 1, confidence); err == nil {
			t.Fatalf("accepted confidence %v", confidence)
		}
	}
	for _, confidence := range []float64{math.SmallestNonzeroFloat64, math.Nextafter(1, 0)} {
		r, err := WilsonInterval(1, 2, confidence)
		if err != nil || math.IsNaN(r.Lower) || math.IsNaN(r.Upper) || r.Lower < 0 || r.Upper > 1 {
			t.Fatalf("extreme = %+v, %v", r, err)
		}
	}
}

func TestWilsonIntervalDeclaredPopulation(t *testing.T) {
	b, _ := json.Marshal(fixture("exact-match-v1"))
	r, err := Evaluate(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	interval, err := WilsonInterval(r.Correct, r.Total, .95)
	if err != nil || interval.Correct != 1 || interval.Total != 4 || r.Unavailable != 1 || r.Failed != 1 {
		t.Fatalf("population: %+v %+v %v", r, interval, err)
	}
}
