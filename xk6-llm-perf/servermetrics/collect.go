package servermetrics

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// CollectOptions bounds a serial recording. Targets are explicit configured
// URLs. Authentication belongs to Client. Duration includes query time, and
// client/context cancellation must bound every HTTP operation.
type CollectOptions struct {
	Targets    []string
	Interval   time.Duration
	Duration   time.Duration
	MaxRecords int
	MaxBytes   int64
}

// Observation preserves one scrape attempt, including failures and scheduling
// lag. Failed observations cannot be treated as valid zero readings.
type Observation struct {
	SchemaVersion int       `json:"schema_version"`
	ScheduledAt   time.Time `json:"scheduled_at"`
	Target        string    `json:"target"`
	Scrape        *Scrape   `json:"scrape"`
	Error         string    `json:"error,omitempty"`
}

// Recording reports completeness separately from successful scrape count.
type Recording struct {
	SchemaVersion    int       `json:"schema_version"`
	StartedAt        time.Time `json:"started_at"`
	FinishedAt       time.Time `json:"finished_at"`
	Attempts         int       `json:"attempts"`
	Failures         int       `json:"failures"`
	Bytes            int64     `json:"bytes"`
	MissedTicks      int64     `json:"missed_ticks"`
	Complete         bool      `json:"complete"`
	IncompleteReason string    `json:"incomplete_reason,omitempty"`
}

// Collect writes bounded version-1 observations as JSONL without retaining the
// full stream. Queries never overlap. A slow target skips ticks rather than
// growing a work queue. Record/byte limits, failed targets, canceled contexts
// and sink errors retain an explicit incomplete recording result.
func Collect(ctx context.Context, client *http.Client, opts CollectOptions, out io.Writer) (result Recording, err error) {
	if client == nil || out == nil || len(opts.Targets) == 0 || len(opts.Targets) > 16 || opts.Interval < 10*time.Millisecond || opts.Interval > time.Hour || opts.Duration <= 0 || opts.Duration > 24*time.Hour || opts.MaxRecords <= 0 || opts.MaxRecords > 100000 || opts.MaxBytes <= 0 || opts.MaxBytes > 1024*1024*1024 {
		return result, fmt.Errorf("invalid server recording bounds")
	}
	seen := map[string]bool{}
	for _, target := range opts.Targets {
		if seen[target] {
			return result, fmt.Errorf("duplicate scrape target")
		}
		seen[target] = true
	}
	ctx, cancel := context.WithTimeout(ctx, opts.Duration)
	defer cancel()
	start := time.Now()
	end := start.Add(opts.Duration)
	result = Recording{SchemaVersion: 1, StartedAt: start}
	defer func() {
		result.FinishedAt = time.Now()
		if result.Failures > 0 && result.IncompleteReason == "" {
			result.Complete = false
			result.IncompleteReason = "scrape failures"
		}
	}()
	next := start
	for {
		if delay := time.Until(next); delay > 0 {
			timer := time.NewTimer(delay)
			select {
			case <-timer.C:
			case <-ctx.Done():
				timer.Stop()
			}
		}
		if ctx.Err() != nil {
			if time.Now().Before(end) || ctx.Err() != context.DeadlineExceeded {
				result.IncompleteReason = "cancelled"
				return result, ctx.Err()
			}
			result.Complete = true
			return result, nil
		}
		for _, target := range opts.Targets {
			if result.Attempts >= opts.MaxRecords {
				result.IncompleteReason = "record budget exhausted"
				return result, nil
			}
			observation := Observation{SchemaVersion: 1, ScheduledAt: next, Target: target}
			scrape, fetchErr := Fetch(ctx, client, target)
			if fetchErr != nil {
				observation.Error = fetchErr.Error()
				result.Failures++
			} else {
				observation.Scrape = &scrape
			}
			result.Attempts++
			data, marshalErr := json.Marshal(observation)
			if marshalErr != nil {
				result.IncompleteReason = "encode failure"
				return result, marshalErr
			}
			data = append(data, '\n')
			if result.Bytes+int64(len(data)) > opts.MaxBytes {
				result.IncompleteReason = "byte budget exhausted"
				return result, nil
			}
			n, writeErr := out.Write(data)
			result.Bytes += int64(n)
			if writeErr != nil || n != len(data) {
				result.IncompleteReason = "sink failure"
				if writeErr == nil {
					writeErr = io.ErrShortWrite
				}
				return result, writeErr
			}
		}
		next = next.Add(opts.Interval)
		if lag := time.Since(next); lag >= opts.Interval {
			missed := int64(lag / opts.Interval)
			result.MissedTicks += missed
			next = next.Add(time.Duration(missed) * opts.Interval)
		}
	}
}
