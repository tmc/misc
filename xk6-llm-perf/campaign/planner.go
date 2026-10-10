package campaign

import "strconv"

func grid(dimensions []Dimension) []map[string]string {
	cells := []map[string]string{{}}
	for _, d := range dimensions {
		var next []map[string]string
		for _, p := range cells {
			for _, v := range d.Values {
				q := map[string]string{}
				for k, v := range p {
					q[k] = v
				}
				q[d.Name] = v
				next = append(next, q)
			}
		}
		cells = next
	}
	return cells
}
func radical(n uint64, base uint64) float64 {
	f, r := 1.0, 0.0
	for n > 0 {
		f /= float64(base)
		r += f * float64(n%base)
		n /= base
	}
	return r
}
func next(c Config, cells []map[string]string, m *Manifest) map[string]string {
	used := map[string]bool{}
	for _, v := range m.Candidates {
		used[v.ID] = true
	}
	if c.Mode == "bayesian" {
		if p := bayesian(c, cells, m, used); p != nil {
			return p
		}
	}
	if c.Mode == "scale" {
		return scale(c, m, used)
	}
	if c.Mode == "adaptive" && m.BestID != "" {
		var best map[string]string
		for _, v := range m.Candidates {
			if v.ID == m.BestID {
				best = v.Parameters
			}
		}
		// Explore immediate neighboring cells around the best feasible converged cell.
		for _, d := range c.Dimensions {
			for i, v := range d.Values {
				if best[d.Name] != v {
					continue
				}
				for _, j := range []int{i + 1, i - 1} {
					if j < 0 || j >= len(d.Values) {
						continue
					}
					p := map[string]string{}
					for k, v := range best {
						p[k] = v
					}
					p[d.Name] = d.Values[j]
					if !used[cellID(p)] {
						return p
					}
				}
			}
		}
	}
	if c.Mode == "halton" || c.Mode == "adaptive" || c.Mode == "bayesian" {
		primes := []uint64{2, 3, 5, 7, 11, 13, 17, 19, 23, 29, 31, 37}
		n := c.Seed%1000003 + 1
		for sample := 0; sample < len(cells)*16; sample++ {
			p := map[string]string{}
			for i, d := range c.Dimensions {
				index := int(radical(n, primes[i]) * float64(len(d.Values)))
				p[d.Name] = d.Values[index]
			}
			if !used[cellID(p)] {
				return p
			}
			n++
		}
	}
	// Finite fallback ensures exhausted/discretized Halton sequences still cover
	// every cell exactly once, while honoring the declared candidate budget.
	for _, p := range cells {
		if !used[cellID(p)] {
			return p
		}
	}
	return nil
}
func scale(c Config, m *Manifest, used map[string]bool) map[string]string {
	d := c.Dimensions[0]
	pass, fail := -1, len(d.Values)
	for _, cell := range m.Candidates {
		index := -1
		for i, v := range d.Values {
			if cell.Parameters[d.Name] == v {
				index = i
				break
			}
		}
		if !cell.Converged || cell.Failed {
			return nil
		}
		if cell.Feasible {
			if index > pass {
				pass = index
			}
		} else if index < fail {
			fail = index
		}
	}
	index := 0
	if fail < len(d.Values) {
		if fail-pass <= 1 {
			return nil
		}
		index = pass + (fail-pass)/2
	} else if pass >= 0 {
		current, _ := strconv.ParseFloat(d.Values[pass], 64)
		index = pass + 1
		for index+1 < len(d.Values) {
			v, _ := strconv.ParseFloat(d.Values[index+1], 64)
			if v > current*2 {
				break
			}
			index++
		}
		if index >= len(d.Values) {
			return nil
		}
	}
	if index < 0 || index >= len(d.Values) {
		return nil
	}
	p := map[string]string{d.Name: d.Values[index]}
	if used[cellID(p)] {
		return nil
	}
	return p
}
