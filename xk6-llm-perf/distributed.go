package llmperf

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// WorkerClock retains a worker-to-benchmark clock mapping receipt.
type WorkerClock struct {
	Offset      float64 `json:"offset_ms"`
	Uncertainty float64 `json:"uncertainty_ms"`
	Basis       string  `json:"basis"`
}

// WorkerRun supplies an explicit clock mapping from a worker to the shared
// benchmark clock. Offset is added to worker timestamps; uncertainty uses ms.
// A constant mapping cannot repair wall-clock steps during a recording.
type WorkerRun struct {
	Run              RunSummary
	ClockOffset      float64
	ClockUncertainty *float64
	ClockBasis       string
}

// MergeOptions identifies required workers and admits partial loss explicitly.
// Workers must share the same mapped window, workload, seed and trace hash.
type MergeOptions struct {
	ExpectedWorkers     []string
	AllowPartial        bool
	MaxClockUncertainty float64
}

// MergeRuns reconstructs aggregate accounting from complete retained worker
// records. It refuses duplicate worker/request identities, unknown clocks,
// incompatible windows, dropped records and more than 100000 merged records.
// Missing workers are visible even when AllowPartial admits their loss.
func MergeRuns(workers []WorkerRun, opts MergeOptions) (RunSummary, error) {
	var result RunSummary
	if len(workers) == 0 || len(workers) > 256 || len(opts.ExpectedWorkers) == 0 || len(opts.ExpectedWorkers) > 256 || !finite(opts.MaxClockUncertainty) || opts.MaxClockUncertainty < 0 {
		return result, fmt.Errorf("invalid distributed merge bounds")
	}
	expected := map[string]bool{}
	for _, id := range opts.ExpectedWorkers {
		if id == "" || expected[id] {
			return result, fmt.Errorf("invalid expected worker identity")
		}
		expected[id] = true
	}
	seen, requests := map[string]bool{}, map[string]bool{}
	for i, w := range workers {
		r := w.Run
		if r.WorkerID == "" || seen[r.WorkerID] || !expected[r.WorkerID] {
			return result, fmt.Errorf("duplicate or unexpected worker identity")
		}
		seen[r.WorkerID] = true
		if !finite(w.ClockOffset) || w.ClockBasis == "" || w.ClockUncertainty == nil || !finite(*w.ClockUncertainty) || *w.ClockUncertainty < 0 || *w.ClockUncertainty > opts.MaxClockUncertainty {
			return result, fmt.Errorf("worker clock mapping unavailable or outside bound")
		}
		if err := validateRun(r); err != nil {
			return result, fmt.Errorf("worker accounting: %w", err)
		}
		if r.TraceHash == "" || r.Workload == "" {
			return result, fmt.Errorf("worker workload and trace identities required")
		}
		if _, err := AnalyzeTimeline(r); err != nil {
			return result, fmt.Errorf("worker %q: %w", r.WorkerID, err)
		}
		start, end := r.StartedAt+w.ClockOffset, r.FinishedAt+w.ClockOffset
		if i == 0 {
			result = RunSummary{SchemaVersion: 1, RunID: r.RunID, WorkerID: "aggregate", WorkerClocks: map[string]WorkerClock{}, StartedAt: start, FinishedAt: end, Duration: r.Duration, Seed: r.Seed, Workload: r.Workload, TraceHash: r.TraceHash, Versions: map[string]string{}, WindowPolicy: "identical clock-mapped whole-contained worker windows; successful token totals", WarmupPolicy: r.WarmupPolicy, Records: []Measurement{}}
		} else if r.RunID != result.RunID || math.Abs(start-result.StartedAt) > .001 || math.Abs(end-result.FinishedAt) > .001 || r.Duration != result.Duration || r.Seed != result.Seed || r.Workload != result.Workload || r.TraceHash != result.TraceHash {
			return result, fmt.Errorf("incompatible worker workload or window")
		}
		for name, version := range r.Versions {
			if old, ok := result.Versions[name]; ok && old != version {
				return result, fmt.Errorf("worker runtime versions differ")
			}
			result.Versions[name] = version
		}
		result.WorkerClocks[r.WorkerID] = WorkerClock{Offset: w.ClockOffset, Uncertainty: *w.ClockUncertainty, Basis: w.ClockBasis}
		result.Workers = append(result.Workers, r.WorkerID)
		result.ExcludedBoundaryRequests += r.ExcludedBoundaryRequests
		result.InflightAtEnd += r.InflightAtEnd
		for _, m := range r.Records {
			if !strings.HasPrefix(m.RequestID, r.WorkerID+":") || requests[m.RequestID] {
				return result, fmt.Errorf("missing or duplicate request identity")
			}
			requests[m.RequestID] = true
			if len(result.Records) >= 100000 {
				return result, fmt.Errorf("merged record budget exhausted")
			}
			m.StartedAt += w.ClockOffset
			m.DispatchedAt += w.ClockOffset
			m.FinishedAt += w.ClockOffset
			if m.ScheduledAt != nil {
				v := *m.ScheduledAt + w.ClockOffset
				m.ScheduledAt = &v
			}
			result.Attempts++
			switch m.Outcome {
			case "success":
				result.Successes++
				if m.Good {
					result.GoodRequests++
				}
				for input, c := range []TokenCount{m.InputTokens, m.OutputTokens} {
					if c.Value != nil && *c.Value < 0 {
						return result, fmt.Errorf("negative worker token count")
					}
					if input == 0 {
						if c.Value == nil {
							result.MissingInputCounts++
						} else {
							result.InputTokens += int64(*c.Value)
							if c.Source == "estimated" {
								result.EstimatedInputCounts++
							}
						}
					} else {
						if c.Value == nil {
							result.MissingOutputCounts++
						} else {
							result.OutputTokens += int64(*c.Value)
							if c.Source == "estimated" {
								result.EstimatedOutputCounts++
							}
						}
					}
				}
			case "failure", "timeout", "cancelled":
				result.Failures++
				if m.Good {
					return result, fmt.Errorf("failed worker request classified good")
				}
			default:
				return result, fmt.Errorf("unknown worker request outcome")
			}
			result.Records = append(result.Records, m)
		}
	}
	for id := range expected {
		if !seen[id] {
			result.MissingWorkers = append(result.MissingWorkers, id)
		}
	}
	sort.Strings(result.Workers)
	sort.Strings(result.MissingWorkers)
	if len(result.MissingWorkers) > 0 && !opts.AllowPartial {
		return result, fmt.Errorf("required workers are missing")
	}
	if result.Duration > 0 {
		result.Goodput = float64(result.GoodRequests) / result.Duration
		result.InputThroughput = float64(result.InputTokens) / result.Duration
		result.OutputThroughput = float64(result.OutputTokens) / result.Duration
	}
	timeline, err := AnalyzeTimeline(result)
	if err != nil {
		return result, err
	}
	result.Timeline = &timeline
	return result, nil
}
