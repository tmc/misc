package output

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	llm "github.com/tmc/misc/xk6-llm-perf"
)

// StreamOptions selects an explicit consumer. Authorization stays outside receipts.
type StreamOptions struct {
	Format        string
	Endpoint      string
	RunID         string
	Authorization string
	MaxRecords    int
	MaxBytes      int64
}

// StreamReceipt preserves the accepted prefix on failures. No automatic retry is performed.
type StreamReceipt struct {
	SchemaVersion   int    `json:"schema_version"`
	RunID           string `json:"run_id"`
	RecordsRead     int    `json:"records_read"`
	RecordsAccepted int    `json:"records_accepted"`
	BytesRead       int64  `json:"bytes_read"`
	Complete        bool   `json:"complete"`
	Policy          string `json:"policy"`
}

// Stream consumes bounded measurement JSONL with synchronous backpressure and
// posts one request at a time to OTLP or an existing MLflow run. Callers can feed
// a live pipe; slow sinks slow the reader. On failure the returned receipt gives
// the accepted prefix, so callers must not replay it blindly.
func Stream(ctx context.Context, reader io.Reader, client *http.Client, opts StreamOptions) (StreamReceipt, error) {
	r := StreamReceipt{SchemaVersion: 1, RunID: opts.RunID, Policy: "serial per-record posts; fail-stop; no retries; individual gauges"}
	if opts.MaxRecords == 0 {
		opts.MaxRecords = 100000
	}
	if opts.MaxBytes == 0 {
		opts.MaxBytes = 64 << 20
	}
	if opts.MaxRecords < 1 || opts.MaxRecords > 100000 || opts.MaxBytes < 1 || opts.MaxBytes > 64<<20 || opts.RunID == "" || len(opts.RunID) > 256 || (opts.Format != "otlp" && opts.Format != "mlflow") {
		return r, fmt.Errorf("invalid streaming sink or limits")
	}
	scanner := bufio.NewScanner(io.LimitReader(reader, opts.MaxBytes+1))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	for scanner.Scan() {
		line := scanner.Bytes()
		r.BytesRead += int64(len(line) + 1)
		if r.BytesRead > opts.MaxBytes || r.RecordsRead >= opts.MaxRecords {
			return r, fmt.Errorf("stream budget exceeded")
		}
		if err := ctx.Err(); err != nil {
			return r, err
		}
		if len(bytes.TrimSpace(line)) == 0 {
			return r, fmt.Errorf("empty measurement line")
		}
		var m llm.Measurement
		d := json.NewDecoder(bytes.NewReader(line))
		d.DisallowUnknownFields()
		if err := d.Decode(&m); err != nil {
			return r, fmt.Errorf("decode measurement: %w", err)
		}
		if err := d.Decode(new(any)); err != io.EOF {
			return r, fmt.Errorf("trailing measurement data")
		}
		r.RecordsRead++
		run := llm.RunSummary{SchemaVersion: 1, RunID: opts.RunID, StartedAt: m.StartedAt, FinishedAt: m.FinishedAt + 1, Duration: (m.FinishedAt + 1 - m.StartedAt) / 1000, Attempts: 1, Records: []llm.Measurement{m}}
		if m.Outcome == "success" {
			run.Successes = 1
			if m.Good {
				run.GoodRequests = 1
			}
			if m.InputTokens.Value != nil {
				run.InputTokens = int64(*m.InputTokens.Value)
			}
			if m.OutputTokens.Value != nil {
				run.OutputTokens = int64(*m.OutputTokens.Value)
			}
		} else {
			run.Failures = 1
		}
		if m.Outcome == "success" {
			if m.InputTokens.Value == nil {
				run.MissingInputCounts = 1
			}
			if m.OutputTokens.Value == nil {
				run.MissingOutputCounts = 1
			}
			if m.InputTokens.Source == "estimated" {
				run.EstimatedInputCounts = 1
			}
			if m.OutputTokens.Source == "estimated" {
				run.EstimatedOutputCounts = 1
			}
		}
		run.Goodput = float64(run.GoodRequests) / run.Duration
		run.InputThroughput = float64(run.InputTokens) / run.Duration
		run.OutputThroughput = float64(run.OutputTokens) / run.Duration
		var payload []byte
		var err error
		if opts.Format == "otlp" {
			payload, err = OTLP(run)
		} else {
			payload, err = liveMLflow(run, opts.RunID)
		}
		if err != nil {
			return r, err
		}
		if err := Send(ctx, client, opts.Endpoint, opts.Authorization, payload); err != nil {
			return r, err
		}
		r.RecordsAccepted++
	}
	if err := scanner.Err(); err != nil {
		return r, fmt.Errorf("read measurement stream: %w", err)
	}
	r.Complete = true
	return r, nil
}
func liveMLflow(run llm.RunSummary, runID string) ([]byte, error) {
	if _, err := Summarize(run); err != nil {
		return nil, err
	}
	m := run.Records[0]
	type point struct {
		Key       string  `json:"key"`
		Value     float64 `json:"value"`
		Timestamp int64   `json:"timestamp"`
		Step      int64   `json:"step"`
	}
	metrics := []point{{"live.gen_ai.client.operation.duration", m.FullResponseLatency / 1000, int64(m.FinishedAt), 0}}
	for _, item := range []struct {
		k string
		v *float64
	}{{"live.gen_ai.client.operation.time_to_first_chunk", m.TTFT}, {"live.llmperf.client.time_per_output_token", m.ITL}} {
		if item.v != nil {
			metrics = append(metrics, point{item.k, *item.v / 1000, int64(m.FinishedAt), 0})
		}
	}
	if m.TTFO != nil {
		metrics = append(metrics, point{"live.llmperf.client.time_to_first_output", *m.TTFO / 1000, int64(m.FinishedAt), 0})
	}
	for _, field := range timingFields() {
		if v := field.value(m); v != nil {
			metrics = append(metrics, point{"live.llmperf.client." + field.name, *v / 1000, int64(m.FinishedAt), 0})
		}
	}
	for _, item := range []struct {
		k string
		v *int
	}{{"live.gen_ai.client.input.token.usage", m.InputTokens.Value}, {"live.gen_ai.client.output.token.usage", m.OutputTokens.Value}} {
		if item.v != nil {
			metrics = append(metrics, point{item.k, float64(*item.v), int64(m.FinishedAt), 0})
		}
	}
	return json.Marshal(struct {
		RunID   string  `json:"run_id"`
		Metrics []point `json:"metrics"`
	}{runID, metrics})
}
