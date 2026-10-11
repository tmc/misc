package llmperf

import "fmt"

const contentSpanPolicy = "single-process-successful-content-span-v1"

// ObservationReceipt retains goodput over the successful content span.
// Timestamps are Unix milliseconds; Duration is seconds. Attempts includes
// failed requests, which do not define the content span.
type ObservationReceipt struct {
	Policy       string  `json:"policy"`
	StartedAt    float64 `json:"started_at"`
	FinishedAt   float64 `json:"finished_at"`
	Duration     float64 `json:"duration"`
	Attempts     int     `json:"attempts"`
	GoodRequests int     `json:"good_requests"`
	Goodput      float64 `json:"goodput"`
}

// AnalyzeObservation derives goodput from complete validated run records.
// The span starts at the earliest successful request with content and ends at
// the final successful content response. It excludes usage-only trailers and
// idle time outside that span. Missing or incomplete observations are errors.
func AnalyzeObservation(run RunSummary) (ObservationReceipt, error) {
	if err := validateRun(run); err != nil {
		return ObservationReceipt{}, err
	}
	return observationReceipt(run)
}

func observationReceipt(run RunSummary) (ObservationReceipt, error) {
	if len(run.Workers) != 0 || len(run.WorkerClocks) != 0 || len(run.MissingWorkers) != 0 || run.WorkerID == "aggregate" {
		return ObservationReceipt{}, fmt.Errorf("content observation requires a single process clock domain")
	}
	if run.Attempts == 0 || run.DroppedRecords != 0 || run.ExcludedBoundaryRequests != 0 || run.InflightAtEnd != 0 {
		return ObservationReceipt{}, fmt.Errorf("observation requires complete attempted records")
	}
	result := ObservationReceipt{Policy: contentSpanPolicy, Attempts: run.Attempts, GoodRequests: run.GoodRequests}
	found := false
	for _, m := range run.Records {
		if m.Outcome != "success" {
			continue
		}
		if m.FullResponseLatency <= 0 {
			if m.Good {
				return ObservationReceipt{}, fmt.Errorf("good request lacks a positive response interval")
			}
			continue
		}
		if m.LastContentLatency == nil {
			if m.Good {
				return ObservationReceipt{}, fmt.Errorf("good request lacks content observation")
			}
			continue
		}
		end := m.DispatchedAt + *m.LastContentLatency
		if !finite(end) || end < m.DispatchedAt {
			return ObservationReceipt{}, fmt.Errorf("content observation span is unavailable or nonpositive")
		}
		if !found || m.DispatchedAt < result.StartedAt {
			result.StartedAt = m.DispatchedAt
		}
		if !found || end > result.FinishedAt {
			result.FinishedAt = end
		}
		found = true
	}
	if !found {
		return ObservationReceipt{}, fmt.Errorf("successful content observation is unavailable")
	}
	result.Duration = (result.FinishedAt - result.StartedAt) / 1000
	if !finite(result.Duration) || result.Duration <= 0 {
		return ObservationReceipt{}, fmt.Errorf("observation duration is unavailable or nonpositive")
	}
	result.Goodput = float64(result.GoodRequests) / result.Duration
	if !finite(result.Goodput) {
		return ObservationReceipt{}, fmt.Errorf("observation goodput is not finite")
	}
	return result, nil
}
