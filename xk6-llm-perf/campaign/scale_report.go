package campaign

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
)

// ReadScale imports a completed scale receipt and independently reproduces each
// assessment from retained records. It refuses incomplete or contradictory
// records. Asset/summary hashes remain provenance, not independent signatures.
func ReadScale(r io.Reader) (*ScaleManifest, error) {
	b, e := io.ReadAll(io.LimitReader(r, (64<<20)+1))
	if e != nil {
		return nil, e
	}
	if len(b) > 64<<20 {
		return nil, fmt.Errorf("scale manifest exceeds 64MiB")
	}
	var m ScaleManifest
	if e = json.Unmarshal(b, &m); e != nil {
		return nil, e
	}
	if e = m.Config.Validate(); e != nil {
		return nil, e
	}
	if m.SchemaVersion != 1 || m.Identity == "" || m.FinishedAt.IsZero() || m.FinishedAt.Before(m.StartedAt) || len(m.Windows) > m.Config.Windows || len(m.Workers) != m.Config.Workers {
		return nil, fmt.Errorf("incomplete scale manifest")
	}
	workers := map[int]bool{}
	for _, w := range m.Workers {
		if w.ID < 0 || w.ID >= m.Config.Workers || workers[w.ID] {
			return nil, fmt.Errorf("invalid worker identity")
		}
		workers[w.ID] = true
	}
	seen := map[string]bool{}
	retained := 0
	previousEnd := int64(0)
	for i, w := range m.Windows {
		if w.Epoch != i+1 || w.StartedAt < previousEnd || w.FinishedAt <= w.StartedAt || w.RequestLimit < 1 || w.PrefillLimit < 1 {
			return nil, fmt.Errorf("invalid scale window")
		}
		previousEnd = w.FinishedAt
		request, prefill := m.Config.FixedRequest, m.Config.FixedPrefill
		if m.Config.Variable == "request" {
			request = w.RequestLimit
			if request < m.Config.Minimum || request > m.Config.Maximum {
				return nil, fmt.Errorf("request cap out of bounds")
			}
		} else {
			prefill = w.PrefillLimit
			if prefill < m.Config.Minimum || prefill > m.Config.Maximum {
				return nil, fmt.Errorf("prefill cap out of bounds")
			}
		}
		if request != w.RequestLimit || prefill != w.PrefillLimit {
			return nil, fmt.Errorf("fixed cap changed")
		}
		for _, s := range w.Samples {
			if s.ID == "" || seen[s.ID] || !workers[s.Worker] || s.Epoch != w.Epoch || !finite(s.DispatchedAt) || !finite(s.FinishedAt) || s.DispatchedAt < float64(w.StartedAt) || s.FinishedAt > float64(w.FinishedAt) || s.FinishedAt < s.DispatchedAt || !finite(s.Latency) || s.Latency < 0 || (s.Outcome != "success" && s.Outcome != "failure" && s.Outcome != "cancelled") {
				return nil, fmt.Errorf("invalid retained scale sample")
			}
			seen[s.ID] = true
			retained++
		}
		if w.Excluded < 0 || w.Dropped < 0 || w.Duplicates < 0 {
			return nil, fmt.Errorf("negative scale accounting")
		}
		retained += w.Excluded
		if retained > m.Config.MaxRecords {
			return nil, fmt.Errorf("scale retention exceeds budget")
		}
		participating := make([]bool, m.Config.Workers)
		for _, id := range w.ParticipatingWorkers {
			if !workers[id] || participating[id] {
				return nil, fmt.Errorf("invalid participating worker")
			}
			participating[id] = true
		}
		recomputed := w
		recomputed.Completed = 0
		recomputed.Failures = 0
		recomputed.P95 = nil
		recomputed.ParticipatingWorkers = nil
		reason := ""
		if w.Duplicates > 0 {
			reason = "duplicate request identity"
		}
		assess(&recomputed, participating, m.Config, reason)
		exceptional := w.Reason == "cancelled" || w.Reason == "worker_failed"
		if w.Reason == "assessment_in_progress" || w.Completed != recomputed.Completed || w.Failures != recomputed.Failures || !equalFloat(w.P95, recomputed.P95) || !exceptional && (w.Passed != recomputed.Passed || w.Reason != recomputed.Reason) || exceptional && w.Passed {
			return nil, fmt.Errorf("scale assessment does not reproduce")
		}
	}
	if e = verifyScaleDecisions(&m); e != nil {
		return nil, e
	}
	return &m, nil
}
func equalFloat(a, b *float64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return finite(*a) && math.Abs(*a-*b) <= 1e-9
}

func verifyScaleDecisions(m *ScaleManifest) error {
	value, lastPassing := m.Config.Minimum, 0
	sustaining := false
	remaining := m.Config.SustainWindows
	expectedStop := ""
	for i, w := range m.Windows {
		control := w.RequestLimit
		if m.Config.Variable == "prefill" {
			control = w.PrefillLimit
		}
		if expectedStop != "" || control != value || w.Sustaining != sustaining {
			return fmt.Errorf("scale control sequence does not reproduce")
		}
		switch {
		case w.Reason == "cancelled":
			expectedStop = "cancelled"
		case w.Reason == "worker_failed" || w.Reason == "missing_worker" || w.Reason == "incomplete_records" || w.Reason == "duplicate request identity":
			expectedStop = "assessment_failed"
		case sustaining:
			if !w.Passed {
				expectedStop = "sustain_failed"
			} else {
				remaining--
				if remaining == 0 {
					expectedStop = "sustain_complete"
				}
			}
		case w.Passed:
			lastPassing = value
			if value == m.Config.Maximum {
				sustaining = true
			} else {
				value *= 2
				if value > m.Config.Maximum {
					value = m.Config.Maximum
				}
			}
		default:
			if lastPassing == 0 {
				expectedStop = "no_passing_boundary"
			} else {
				value = lastPassing
				sustaining = true
			}
		}
		if expectedStop != "" && i != len(m.Windows)-1 {
			return fmt.Errorf("windows continued after stopping decision")
		}
	}
	if expectedStop == "" {
		switch m.StopReason {
		case "cancelled", "assets_changed", "checkpoint_failed", "worker_failed":
			return nil
		case "startup_failed":
			if len(m.Windows) == 0 {
				return nil
			}
		case "window_budget":
			if len(m.Windows) == m.Config.Windows {
				return nil
			}
		}
		return fmt.Errorf("scale stopping reason does not reproduce")
	}
	if expectedStop == "sustain_complete" && m.StopReason == "worker_failed" {
		for _, worker := range m.Workers {
			if worker.ExitCode != 0 {
				return nil
			}
		}
	}
	if m.StopReason != expectedStop {
		return fmt.Errorf("scale stopping reason does not reproduce")
	}
	return nil
}
