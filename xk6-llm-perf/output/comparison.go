package output

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"math"

	llm "github.com/tmc/misc/xk6-llm-perf"
	"github.com/tmc/misc/xk6-llm-perf/serving"
)

// Comparison retains each run's metric populations without pooling requests.
type Comparison struct {
	SchemaVersion     int                            `json:"schema_version"`
	StatisticalStatus string                         `json:"statistical_status"`
	Limitations       string                         `json:"limitations"`
	Runs              []Document                     `json:"runs"`
	Policies          map[string]*serving.PairPolicy `json:"policies"`
	PolicyComplete    map[string]bool                `json:"policy_complete"`
}

// Compare validates runs and keeps their individual policies and populations.
func Compare(runs []llm.RunSummary) (Comparison, error) {
	if len(runs) < 2 || len(runs) > 64 {
		return Comparison{}, fmt.Errorf("comparison requires 2..64 runs")
	}
	c := Comparison{SchemaVersion: 1, StatisticalStatus: "NOT statistically gated; descriptive saved-run summaries; independence unverified", Limitations: "unavailable policy fields remain unavailable; no policy equality or producer attestation asserted", Policies: make(map[string]*serving.PairPolicy), PolicyComplete: make(map[string]bool)}
	seen := make(map[string]bool)
	for _, run := range runs {
		if run.RunID == "" || seen[run.RunID] {
			return Comparison{}, fmt.Errorf("comparison requires distinct nonempty run IDs")
		}
		seen[run.RunID] = true
		d, err := Summarize(run)
		if err != nil {
			return Comparison{}, err
		}
		p, complete, err := serving.RunPairPolicy(run)
		if err != nil {
			return Comparison{}, err
		}
		c.Policies[run.RunID] = p
		c.PolicyComplete[run.RunID] = complete
		c.Runs = append(c.Runs, d)
	}
	return c, nil
}

// ComparisonSVG plots one metric mean with explicit run populations and units.
// Missing metrics remain unavailable; bars never imply confidence intervals.
func ComparisonSVG(w io.Writer, c Comparison, metric string) error {
	if len(c.Runs) < 2 || len(c.Runs) > 64 {
		return fmt.Errorf("comparison requires 2..64 runs")
	}
	if c.SchemaVersion != 1 {
		return fmt.Errorf("unsupported comparison schema")
	}
	ids := make(map[string]bool)
	for _, d := range c.Runs {
		if d.SchemaVersion != 2 || d.RunID == "" || ids[d.RunID] {
			return fmt.Errorf("invalid comparison run identity")
		}
		ids[d.RunID] = true
		names := make(map[string]bool)
		for _, s := range d.Metrics {
			if s.Name == "" || names[s.Name] || s.Unit == "" || s.Count < 0 || (s.Mean == nil && s.Count != 0) || (s.Mean != nil && s.Count == 0) {
				return fmt.Errorf("invalid comparison metric population")
			}
			names[s.Name] = true
		}
	}
	type row struct {
		id       string
		value    *float64
		n        int
		missing  int
		document Document
	}
	rows := make([]row, 0, len(c.Runs))
	unit := ""
	max := 0.0
	for _, d := range c.Runs {
		r := row{id: d.RunID, missing: d.MissingInputCounts + d.MissingOutputCounts, document: d}
		for _, s := range d.Metrics {
			if s.Name == metric {
				if unit != "" && s.Unit != unit {
					return fmt.Errorf("comparison metric units differ")
				}
				unit = s.Unit
				r.value = s.Mean
				r.n = s.Count
				if s.Mean != nil {
					if math.IsNaN(*s.Mean) || math.IsInf(*s.Mean, 0) || *s.Mean < 0 {
						return fmt.Errorf("invalid comparison mean")
					}
					max = math.Max(max, *s.Mean)
				}
			}
		}
		rows = append(rows, r)
	}
	if unit == "" {
		return fmt.Errorf("comparison metric unavailable in all runs")
	}
	height := 140 + 80*len(rows)
	_, err := fmt.Fprintf(w, `<svg xmlns="http://www.w3.org/2000/svg" width="1100" height="%d" viewBox="0 0 1100 %d" role="img" aria-label="%s comparison"><rect width="100%%" height="100%%" fill="white"/><g font-family="sans-serif" font-size="14" fill="#18202a"><text x="20" y="30">%s (%s): per-run mean</text><text x="20" y="55">NOT statistically gated; no pooled requests or confidence claim</text><text x="20" y="80">Compare observed measurement/window/warmup policies and numeric SLO limits before interpreting goodput.</text>`, height, height, svgText(metric), svgText(metric), svgText(unit))
	if err != nil {
		return err
	}
	for i, r := range rows {
		y := 110 + 80*i
		label := "N/A"
		width := 0.0
		if r.value != nil {
			label = fmt.Sprintf("%.6g %s", *r.value, unit)
			if max > 0 {
				width = 400 * (*r.value / max)
			}
		}
		if _, err = fmt.Fprintf(w, `<text x="20" y="%d">%s</text><rect x="310" y="%d" width="%.3f" height="18" fill="#3569bd"/><text x="730" y="%d">%s; n=%d; missing token counts=%d</text>`, y, svgText(r.id), y-14, width, y, svgText(label), r.n, r.missing); err != nil {
			return err
		}
		d := r.document
		if _, err = fmt.Fprintf(w, `<text x="20" y="%d">attempts=%d; successes=%d; failures=%d; dropped=%d; boundary=%d; inflight=%d</text>`, y+20, d.Attempts, d.Successes, d.Failures, d.DroppedRecords, d.ExcludedBoundaryRequests, d.InflightAtEnd); err != nil {
			return err
		}
		slo := "SLO limits unavailable"
		if p := c.Policies[d.RunID]; p != nil && p.SLO != nil {
			slo = fmt.Sprintf("SLO limits (ms): TTFT=%g; TTFO=%g; max chunk=%g; TPOT=%s; E2E=%s; complete=%t", p.SLO.MaxTTFT, p.SLO.MaxTTFO, p.SLO.MaxChunkLatency, sloLabel(p.SLO.MaxTPOT), sloLabel(p.SLO.MaxE2E), c.PolicyComplete[d.RunID])
		}
		if _, err = fmt.Fprintf(w, `<text x="20" y="%d">%s</text>`, y+40, svgText(slo)); err != nil {
			return err
		}
	}
	_, err = io.WriteString(w, "</g></svg>")
	return err
}

func svgText(s string) string { var b bytes.Buffer; xml.EscapeText(&b, []byte(s)); return b.String() }

func sloLabel(v *float64) string {
	if v == nil {
		return "unavailable"
	}
	return fmt.Sprint(*v)
}
