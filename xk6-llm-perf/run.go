package llmperf

import (
	"encoding/json"
	"fmt"
	"runtime/debug"
	"sync"
	"time"
)

// RunOptions configures an explicit process-local measurement window.
type RunOptions struct {
	MaxRecordBytes int64  `json:"maxRecordBytes"`
	WorkerID       string `json:"workerId"`
	RunID          string `json:"runId"`
	MaxRecords     int    `json:"maxRecords"`
	TraceHash      string `json:"traceHash"`
	Seed           int64  `json:"seed"`
	Workload       string `json:"workload"`
	StartDelay     string `json:"startDelay"`
	Duration       string `json:"duration"`
}

// RunSummary contains a versioned manifest and aggregates for requests wholly
// inside the declared window. Dropped records affect replay, not counters.
type RunSummary struct {
	RetainedRecordBytes      int64                  `json:"retained_record_bytes"`
	RecordLimit              int                    `json:"record_limit"`
	RecordByteLimit          int64                  `json:"record_byte_limit"`
	WorkerClocks             map[string]WorkerClock `json:"worker_clocks,omitempty"`
	WorkerID                 string                 `json:"worker_id"`
	Workers                  []string               `json:"workers,omitempty"`
	MissingWorkers           []string               `json:"missing_workers,omitempty"`
	SchemaVersion            int                    `json:"schema_version"`
	RunID                    string                 `json:"run_id"`
	StartedAt                float64                `json:"started_at"`
	FinishedAt               float64                `json:"finished_at"`
	Duration                 float64                `json:"duration"`
	Attempts                 int                    `json:"attempts"`
	Successes                int                    `json:"successes"`
	Failures                 int                    `json:"failures"`
	GoodRequests             int                    `json:"good_requests"`
	InputTokens              int64                  `json:"input_tokens"`
	OutputTokens             int64                  `json:"output_tokens"`
	Goodput                  float64                `json:"goodput"`
	OutputThroughput         float64                `json:"output_throughput"`
	InputThroughput          float64                `json:"input_throughput"`
	MissingInputCounts       int                    `json:"missing_input_counts"`
	MissingOutputCounts      int                    `json:"missing_output_counts"`
	EstimatedInputCounts     int                    `json:"estimated_input_counts"`
	EstimatedOutputCounts    int                    `json:"estimated_output_counts"`
	ExcludedBoundaryRequests int                    `json:"excluded_boundary_requests"`
	DroppedRecords           int                    `json:"dropped_records"`
	InflightAtEnd            int                    `json:"inflight_at_end"`
	Timeline                 *Timeline              `json:"timeline"`
	TimelineUnavailable      string                 `json:"timeline_unavailable,omitempty"`
	TraceHash                string                 `json:"trace_hash"`
	Seed                     int64                  `json:"seed"`
	Workload                 string                 `json:"workload"`
	WindowPolicy             string                 `json:"window_policy"`
	WarmupPolicy             string                 `json:"warmup_policy"`
	Versions                 map[string]string      `json:"versions"`
	Records                  []Measurement          `json:"records"`
	active                   bool
	start                    time.Time
	end                      time.Time
}

type runState struct {
	mu         sync.Mutex
	summary    RunSummary
	maxRecords int
	maxBytes   int64
	generation uint64
	sequence   uint64
	inflight   int
}

func (r *runState) snapshot() (RunSummary, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.summary.SchemaVersion == 0 || r.summary.active {
		return RunSummary{}, fmt.Errorf("measurement window has not finished")
	}
	return r.summary, nil
}

func (r *runState) begin(opts RunOptions) error {
	delay, duration := time.Duration(0), time.Duration(0)
	var err error
	if opts.StartDelay != "" {
		delay, err = time.ParseDuration(opts.StartDelay)
		if err != nil || delay < 0 {
			return fmt.Errorf("startDelay must be a non-negative duration")
		}
	}
	if opts.Duration != "" {
		duration, err = time.ParseDuration(opts.Duration)
		if err != nil || duration <= 0 {
			return fmt.Errorf("duration must be positive")
		}
	}
	if opts.MaxRecordBytes == 0 {
		opts.MaxRecordBytes = 16 * 1024 * 1024
	}
	if opts.MaxRecordBytes < 0 || opts.MaxRecordBytes > 32*1024*1024 {
		return fmt.Errorf("maxRecordBytes must be between 1 and 32 MiB")
	}
	if opts.MaxRecords < 0 || opts.MaxRecords > 100000 {
		return fmt.Errorf("maxRecords must be between 0 and 100000")
	}
	if opts.WorkerID == "" {
		opts.WorkerID = "local"
	}
	if len(opts.WorkerID) > 256 {
		return fmt.Errorf("workerId exceeds size limit")
	}
	if len(opts.RunID) > 256 || len(opts.Workload) > 1024 || len(opts.TraceHash) > 256 {
		return fmt.Errorf("run metadata exceeds size limit")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.summary.active {
		return fmt.Errorf("measurement window already active")
	}
	now := time.Now().Add(delay)
	r.maxRecords = opts.MaxRecords
	r.maxBytes = opts.MaxRecordBytes
	r.generation++
	r.inflight = 0
	r.summary = RunSummary{RecordLimit: opts.MaxRecords, RecordByteLimit: opts.MaxRecordBytes, WorkerID: opts.WorkerID, SchemaVersion: 1, RunID: opts.RunID, StartedAt: millis(now), TraceHash: opts.TraceHash, Seed: opts.Seed, Workload: opts.Workload, WindowPolicy: "attempt started and completed inside window; successful token totals", WarmupPolicy: "phase=measurement and per-client warmup exhausted", Versions: buildVersions(), Records: []Measurement{}, active: true, start: now}
	if duration > 0 {
		r.summary.end = now.Add(duration)
	}
	return nil
}

// Build identity is populated by the Makefile.
var extensionRevision = "unknown"
var extensionDirty = "unknown"
var extensionSourceHash = "unknown"

func buildVersions() map[string]string {
	v := map[string]string{"extension_source_hash": extensionSourceHash, "extension_revision": extensionRevision, "extension_dirty": extensionDirty}
	if info, ok := debug.ReadBuildInfo(); ok {
		v["go"] = info.GoVersion
		for _, d := range info.Deps {
			if d.Path == "go.k6.io/k6/v2" || d.Path == "github.com/tmc/misc/xk6-llm-perf" {
				v[d.Path] = d.Version
			}
		}
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" || s.Key == "vcs.modified" {
				v[s.Key] = s.Value
			}
		}
	}
	return v
}

func (r *runState) add(m Measurement) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := &r.summary
	if m.runGeneration != 0 && m.runGeneration != r.generation {
		return
	}
	if m.runGeneration != 0 {
		r.inflight--
	}
	if !s.active || !m.Recorded || !m.Attempted {
		return
	}
	if m.requestStart.Before(s.start) || !s.end.IsZero() && m.FinishedAt > millis(s.end) {
		s.ExcludedBoundaryRequests++
		return
	}
	s.Attempts++
	if m.Outcome == "success" {
		s.Successes++
		if m.Good {
			s.GoodRequests++
		}
		if m.InputTokens.Value != nil {
			s.InputTokens += int64(*m.InputTokens.Value)
			if m.InputTokens.Source == "estimated" {
				s.EstimatedInputCounts++
			}
		} else {
			s.MissingInputCounts++
		}
		if m.OutputTokens.Value != nil {
			s.OutputTokens += int64(*m.OutputTokens.Value)
			if m.OutputTokens.Source == "estimated" {
				s.EstimatedOutputCounts++
			}
		} else {
			s.MissingOutputCounts++
		}
	} else {
		s.Failures++
	}
	data, err := json.Marshal(m)
	if err == nil && len(s.Records) < r.maxRecords && s.RetainedRecordBytes+int64(len(data))+1 <= r.maxBytes {
		s.RetainedRecordBytes += int64(len(data)) + 1
		s.Records = append(s.Records, m)
	} else {
		s.DroppedRecords++
	}
}

func (r *runState) dispatch(m *Measurement) uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.summary.active || !m.Recorded {
		return 0
	}
	r.inflight++
	r.sequence++
	m.RequestID = fmt.Sprintf("%s:%d:%d", r.summary.WorkerID, r.generation, r.sequence)
	return r.generation
}

func (r *runState) finish() (RunSummary, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := &r.summary
	if !s.active {
		return RunSummary{}, fmt.Errorf("no active measurement window")
	}
	now := time.Now()
	if now.Before(s.start) {
		return RunSummary{}, fmt.Errorf("measurement window has not started")
	}
	if !s.end.IsZero() && now.After(s.end) {
		now = s.end
	}
	s.active = false
	s.FinishedAt = millis(now)
	s.Duration = now.Sub(s.start).Seconds()
	s.InflightAtEnd = r.inflight
	if s.Duration > 0 {
		s.Goodput = float64(s.GoodRequests) / s.Duration
		s.OutputThroughput = float64(s.OutputTokens) / s.Duration
		s.InputThroughput = float64(s.InputTokens) / s.Duration
	}
	if t, err := AnalyzeTimeline(*s); err == nil {
		s.Timeline = &t
	} else {
		s.TimelineUnavailable = err.Error()
	}
	return *s, nil
}
