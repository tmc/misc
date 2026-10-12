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
	{Name: "llm_reasoning_tokens", Type: metrics.Counter},
	{Name: "llm_cached_tokens", Type: metrics.Counter},
	{Name: "llm_cache_write_tokens", Type: metrics.Counter},
	{Name: "llm_spec_accepted_tokens", Type: metrics.Counter},
	{Name: "llm_spec_draft_tokens", Type: metrics.Counter},
	{Name: "llm_spec_acceptance_rate", Type: metrics.Trend},
	{Name: "llm_spec_acceptance_length", Type: metrics.Trend},
	{Name: "llm_attempts", Type: metrics.Counter},
	{Name: "llm_content_span_windows", Type: metrics.Counter},
	{Name: "llm_content_span_goodput", Type: metrics.Trend},
	{Name: "llm_content_span_duration_ms", Type: metrics.Trend, ValueType: metrics.Time},
	{Name: "llm_content_span_good_requests", Type: metrics.Trend},
	{Name: "llm_content_span_attempts", Type: metrics.Trend},
	{Name: "llm_work_accepted", Type: metrics.Counter},
	{Name: "llm_work_rejected", Type: metrics.Counter},
	{Name: "llm_work_unavailable", Type: metrics.Counter},
	{Name: "llm_work_acceptance", Type: metrics.Rate},
	{Name: "llm_work_good_requests", Type: metrics.Counter},
	{Name: "llm_successes", Type: metrics.Counter},
	{Name: "llm_failures", Type: metrics.Counter},
	{Name: "llm_good_requests", Type: metrics.Counter},
	{Name: "llm_slo_compliance", Type: metrics.Rate},
	{Name: "llm_slo_ttft_ms", Type: metrics.Trend, ValueType: metrics.Time},
	{Name: "llm_slo_tpot_ms", Type: metrics.Trend, ValueType: metrics.Time},
	{Name: "llm_slo_e2e_ms", Type: metrics.Trend, ValueType: metrics.Time},
	{Name: "llm_dispatch_ttft", Type: metrics.Trend, ValueType: metrics.Time},
	{Name: "llm_dispatch_ttfo", Type: metrics.Trend, ValueType: metrics.Time},
	{Name: "llm_dispatch_ttst", Type: metrics.Trend, ValueType: metrics.Time},
	{Name: "llm_mean_content_interval", Type: metrics.Trend, ValueType: metrics.Time},
	{Name: "llm_max_content_interval", Type: metrics.Trend, ValueType: metrics.Time},
	{Name: "llm_admission_wait", Type: metrics.Trend, ValueType: metrics.Time},
	{Name: "llm_full_response_latency", Type: metrics.Trend, ValueType: metrics.Time},
	{Name: "llm_last_content_latency", Type: metrics.Trend, ValueType: metrics.Time},
	{Name: "llm_inter_token_latency", Type: metrics.Trend, ValueType: metrics.Time},
	{Name: "llm_full_response_itl", Type: metrics.Trend, ValueType: metrics.Time},
	{Name: "llm_request_token_speed", Type: metrics.Trend},
	{Name: "llm_decode_token_speed", Type: metrics.Trend},
	{Name: "llm_usage_prompt_difference", Type: metrics.Trend},
	{Name: "llm_usage_output_difference", Type: metrics.Trend},
	{Name: "llm_usage_discrepant_requests", Type: metrics.Counter},
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
	pipeline pipelineInstrumentation
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
		m.pipeline.push(ctx, samples, metrics.Sample{
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
