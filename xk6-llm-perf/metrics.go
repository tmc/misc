package llmperf

import (
	"context"
	"fmt"
	"math"
	"time"

	"go.k6.io/k6/v2/js/modules"
	"go.k6.io/k6/v2/metrics"
)

// MetricDefinitions holds all metric definitions
var MetricDefinitions = []struct {
	Name      string
	Type      metrics.MetricType
	ValueType metrics.ValueType
}{
	{Name: "llm_ttft", Type: metrics.Trend, ValueType: metrics.Time},
	{Name: "llm_ttfo", Type: metrics.Trend, ValueType: metrics.Time},
	{Name: "llm_token_latency", Type: metrics.Trend, ValueType: metrics.Time},
	{Name: "llm_inter_chunk_latency", Type: metrics.Trend, ValueType: metrics.Time},
	{Name: "llm_completion_time", Type: metrics.Trend, ValueType: metrics.Time},
	{Name: "llm_request_latency", Type: metrics.Trend, ValueType: metrics.Time},
	{Name: "llm_ttft_adjusted", Type: metrics.Trend, ValueType: metrics.Time},
	{Name: "llm_ttfo_adjusted", Type: metrics.Trend, ValueType: metrics.Time},
	{Name: "llm_request_latency_adjusted", Type: metrics.Trend, ValueType: metrics.Time},
	{Name: "llm_input_sequence_length", Type: metrics.Trend, ValueType: metrics.Default},
	{Name: "llm_output_sequence_length", Type: metrics.Trend, ValueType: metrics.Default},
	{Name: "llm_prompt_token_discrepancy", Type: metrics.Trend, ValueType: metrics.Default},
	{Name: "llm_completion_token_discrepancy", Type: metrics.Trend, ValueType: metrics.Default},
	{Name: "llm_tokens_per_second", Type: metrics.Gauge, ValueType: metrics.Default},
	{Name: "llm_total_tokens", Type: metrics.Counter, ValueType: metrics.Default},
	{Name: "llm_prompt_tokens", Type: metrics.Counter, ValueType: metrics.Default},
	{Name: "llm_completion_tokens", Type: metrics.Counter, ValueType: metrics.Default},
	{Name: "llm_usage_discrepancies", Type: metrics.Counter, ValueType: metrics.Default},
	{Name: "llm_requests", Type: metrics.Counter, ValueType: metrics.Default},
	{Name: "llm_good_request", Type: metrics.Rate, ValueType: metrics.Default},
	{Name: "llm_errors", Type: metrics.Counter, ValueType: metrics.Default},
}

type LLMPerfMetrics struct {
	vu       modules.VU
	registry *metrics.Registry
	samples  chan<- metrics.SampleContainer
	metrics  map[string]*metrics.Metric
}

func newMetrics(registry *metrics.Registry) (*LLMPerfMetrics, error) {
	m := &LLMPerfMetrics{
		registry: registry,
		metrics:  make(map[string]*metrics.Metric),
	}

	// Register all metrics
	for _, def := range MetricDefinitions {
		metric, err := registry.NewMetric(def.Name, def.Type, def.ValueType)
		if err != nil {
			return nil, fmt.Errorf("register metric %s: %w", def.Name, err)
		}
		m.metrics[def.Name] = metric
	}

	return m, nil
}

func (m *LLMPerfMetrics) RecordMetric(name string, value float64) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return
	}
	ctx := context.Background()
	samples := m.samples
	tags := m.registry.RootTagSet()
	var metadata map[string]string
	if m.vu != nil {
		state := m.vu.State()
		if state == nil {
			return
		}
		ctx = m.vu.Context()
		samples = state.Samples
		if state.Tags != nil {
			values := state.Tags.GetCurrentValues()
			tags = values.Tags
			metadata = values.Metadata
		}
	}
	if metric, ok := m.metrics[name]; ok && samples != nil {
		metrics.PushIfNotDone(ctx, samples, metrics.Sample{
			TimeSeries: metrics.TimeSeries{
				Metric: metric,
				Tags:   tags,
			},
			Time:     time.Now(),
			Value:    value,
			Metadata: metadata,
		})
	}
}
