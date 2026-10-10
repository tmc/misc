package campaign

import "math"

// bayesian proposes expected improvement under an RBF Gaussian-process model.
// Ordinal grid coordinates are scaled to [0,1]. A bounded nearest-neighbor
// feasibility estimate discounts candidates near failed SLA/execution cells.
// Only the latest 64 observations train the model, bounding cubic fitting cost.
func bayesian(c Config, cells []map[string]string, m *Manifest, used map[string]bool) map[string]string {
	var x [][]float64
	var y []float64
	observations := m.Candidates
	if len(observations) > 64 {
		observations = observations[len(observations)-64:]
	}
	for _, cell := range observations {
		if cell.Mean == nil {
			continue
		}
		v := *cell.Mean
		if c.Minimize {
			v = -v
		}
		x = append(x, coordinates(c, cell.Parameters))
		y = append(y, v)
	}
	if len(y) < 2 {
		return nil
	}
	mean := 0.0
	best := math.Inf(-1)
	for _, v := range y {
		mean += v / float64(len(y))
		best = math.Max(best, v)
	}
	variance := 0.0
	for _, v := range y {
		variance += (v - mean) * (v - mean) / float64(len(y))
	}
	scale := math.Sqrt(variance)
	if scale == 0 {
		scale = math.Max(1, math.Abs(mean))
	}
	for i := range y {
		y[i] = (y[i] - mean) / scale
	}
	best = (best - mean) / scale
	model := newGP(x, y)
	if model == nil {
		return nil
	}
	score := -1.0
	var chosen map[string]string
	for _, p := range cells {
		if used[cellID(p)] {
			continue
		}
		point := coordinates(c, p)
		prediction, variance := model.predict(point)
		sd := math.Sqrt(math.Max(0, variance))
		improvement := prediction - best
		ei := math.Max(0, improvement)
		if sd > 1e-12 {
			z := improvement / sd
			ei = improvement*.5*(1+math.Erf(z/math.Sqrt2)) + sd*math.Exp(-z*z/2)/math.Sqrt(2*math.Pi)
		}
		pass, total := 1.0, 2.0
		for _, cell := range observations {
			distance := squaredDistance(point, coordinates(c, cell.Parameters))
			weight := math.Exp(-distance / .08)
			total += weight
			if cell.Feasible && !cell.Failed {
				pass += weight
			}
		}
		ei *= pass / total
		if ei > score {
			score = ei
			chosen = p
		}
	}
	return chosen
}
func coordinates(c Config, p map[string]string) []float64 {
	x := make([]float64, len(c.Dimensions))
	for i, d := range c.Dimensions {
		for j, v := range d.Values {
			if p[d.Name] == v {
				if len(d.Values) > 1 {
					x[i] = float64(j) / float64(len(d.Values)-1)
				}
				break
			}
		}
	}
	return x
}
func squaredDistance(a, b []float64) float64 {
	d := 0.0
	for i := range a {
		v := a[i] - b[i]
		d += v * v
	}
	return d
}
func kernel(a, b []float64) float64 { return math.Exp(-squaredDistance(a, b) / (.3 * .3 * 2)) }

type gaussianProcess struct {
	x     [][]float64
	l     [][]float64
	alpha []float64
}

func newGP(x [][]float64, y []float64) *gaussianProcess {
	n := len(y)
	l := make([][]float64, n)
	for i := range l {
		l[i] = make([]float64, n)
	}
	for i := 0; i < n; i++ {
		for j := 0; j <= i; j++ {
			v := kernel(x[i], x[j])
			if i == j {
				v += 1e-6
			}
			for k := 0; k < j; k++ {
				v -= l[i][k] * l[j][k]
			}
			if i == j {
				if v <= 0 || !finite(v) {
					return nil
				}
				l[i][j] = math.Sqrt(v)
			} else {
				l[i][j] = v / l[j][j]
			}
		}
	}
	z := make([]float64, n)
	for i := 0; i < n; i++ {
		v := y[i]
		for j := 0; j < i; j++ {
			v -= l[i][j] * z[j]
		}
		z[i] = v / l[i][i]
	}
	alpha := make([]float64, n)
	for i := n - 1; i >= 0; i-- {
		v := z[i]
		for j := i + 1; j < n; j++ {
			v -= l[j][i] * alpha[j]
		}
		alpha[i] = v / l[i][i]
	}
	return &gaussianProcess{x, l, alpha}
}
func (g *gaussianProcess) predict(x []float64) (float64, float64) {
	n := len(g.x)
	k := make([]float64, n)
	mean := 0.0
	for i := range k {
		k[i] = kernel(x, g.x[i])
		mean += k[i] * g.alpha[i]
	}
	v := make([]float64, n)
	variance := 1.0
	for i := 0; i < n; i++ {
		r := k[i]
		for j := 0; j < i; j++ {
			r -= g.l[i][j] * v[j]
		}
		v[i] = r / g.l[i][i]
		variance -= v[i] * v[i]
	}
	return mean, math.Max(0, variance)
}
