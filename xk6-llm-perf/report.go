package llmperf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
)

// ReadRun reads a bounded version-1 run export and checks its record accounting.
func ReadRun(r io.Reader) (RunSummary, error) {
	const limit = 64 * 1024 * 1024
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return RunSummary{}, fmt.Errorf("read run: %w", err)
	}
	if len(data) > limit {
		return RunSummary{}, fmt.Errorf("run exceeds 64 MiB")
	}
	var run RunSummary
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&run); err != nil {
		return run, fmt.Errorf("decode run: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return run, fmt.Errorf("trailing run data")
	}
	return run, validateRun(run)
}

func validateRun(run RunSummary) error {
	if !finite(run.StartedAt) || !finite(run.FinishedAt) || !finite(run.Duration) || run.SchemaVersion != 1 || run.Duration <= 0 || run.FinishedAt <= run.StartedAt || run.Attempts < 0 || run.Successes < 0 || run.Failures < 0 || run.GoodRequests < 0 || run.Successes+run.Failures != run.Attempts || run.GoodRequests > run.Successes || run.InputTokens < 0 || run.OutputTokens < 0 || run.DroppedRecords < 0 || len(run.Records) > 100000 || len(run.Records)+run.DroppedRecords != run.Attempts {
		return fmt.Errorf("invalid run accounting or schema")
	}
	if math.Abs((run.FinishedAt-run.StartedAt)/1000-run.Duration) > .001 {
		return fmt.Errorf("run duration does not match window")
	}
	if run.DroppedRecords == 0 {
		if _, err := AnalyzeTimeline(run); err != nil {
			return err
		}
		var successes, failures, good int
		var input, output int64
		for _, m := range run.Records {
			for _, c := range []TokenCount{m.InputTokens, m.OutputTokens} {
				if c.Value != nil && (*c.Value < 0 || c.Source == "unavailable" || c.Source == "") {
					return fmt.Errorf("invalid token count provenance")
				}
				if c.Value == nil && c.Source != "unavailable" {
					return fmt.Errorf("missing token count provenance")
				}
			}
			switch m.Outcome {
			case "success":
				successes++
				if m.Good {
					good++
				}
				if m.InputTokens.Value != nil {
					input += int64(*m.InputTokens.Value)
				}
				if m.OutputTokens.Value != nil {
					output += int64(*m.OutputTokens.Value)
				}
			case "failure", "timeout", "cancelled":
				failures++
				if m.Good {
					return fmt.Errorf("failed request classified good")
				}
			default:
				return fmt.Errorf("unknown request outcome")
			}
		}
		if successes != run.Successes || failures != run.Failures || good != run.GoodRequests || input != run.InputTokens || output != run.OutputTokens {
			return fmt.Errorf("records disagree with aggregate accounting")
		}
	}
	return nil
}

// SliceSummary counts completions in a half-open time interval. Requests may
// start before the slice; concurrency is clipped at its boundaries.
type SliceSummary struct {
	StartedAt        float64     `json:"started_at"`
	FinishedAt       float64     `json:"finished_at"`
	Attempts         int         `json:"completed_attempts"`
	Successes        int         `json:"successes"`
	Failures         int         `json:"failures"`
	GoodRequests     int         `json:"good_requests"`
	OutputTokens     int64       `json:"output_tokens"`
	Goodput          float64     `json:"goodput"`
	OutputThroughput float64     `json:"output_throughput"`
	Requests         Concurrency `json:"requests"`
	Policy           string      `json:"policy"`
}

// TimeSlices derives non-overlapping slices from a complete retained run.
// width uses seconds. The last slice ends at the declared run end.
func TimeSlices(run RunSummary, width float64) ([]SliceSummary, error) {
	if width <= 0 || math.IsNaN(width) || math.IsInf(width, 0) {
		return nil, fmt.Errorf("slice width must be positive and finite")
	}
	if _, err := AnalyzeTimeline(run); err != nil {
		return nil, err
	}
	if math.Ceil(run.Duration/width) > 10000 {
		return nil, fmt.Errorf("more than 10000 time slices")
	}
	var slices []SliceSummary
	for start := run.StartedAt; start < run.FinishedAt; {
		end := math.Min(run.FinishedAt, start+width*1000)
		if end <= start {
			return nil, fmt.Errorf("slice width below timestamp resolution")
		}
		s := SliceSummary{StartedAt: start, FinishedAt: end, Policy: "completion in [start,end); final run endpoint inclusive; tokens from successful completions"}
		var intervals []interval
		for _, m := range run.Records {
			intervals = append(intervals, interval{m.DispatchedAt, m.FinishedAt})
			if m.FinishedAt < start || m.FinishedAt >= end && !(end == run.FinishedAt && m.FinishedAt == end) {
				continue
			}
			s.Attempts++
			if m.Outcome == "success" {
				s.Successes++
				if m.Good {
					s.GoodRequests++
				}
				if m.OutputTokens.Value != nil {
					s.OutputTokens += int64(*m.OutputTokens.Value)
				}
			} else {
				s.Failures++
			}
		}
		duration := (end - start) / 1000
		s.Goodput = float64(s.GoodRequests) / duration
		s.OutputThroughput = float64(s.OutputTokens) / duration
		s.Requests = concurrency(intervals, start, end)
		slices = append(slices, s)
		start = end
	}
	return slices, nil
}
