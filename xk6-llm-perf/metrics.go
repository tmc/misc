package llmperf

import (
	"time"

	"go.k6.io/k6/metrics"
)

// MetricDefinitions holds all metric definitions
var MetricDefinitions = []struct {
	Name      string
	Type      metrics.MetricType
	ValueType metrics.ValueType
}{
	// Common metrics
	{Name: "llm_ttft", Type: metrics.Trend, ValueType: metrics.Time},
	{Name: "llm_token_latency", Type: metrics.Trend, ValueType: metrics.Time},
	{Name: "llm_tokens_per_second", Type: metrics.Gauge, ValueType: metrics.Default},
	{Name: "llm_completion_time", Type: metrics.Trend, ValueType: metrics.Time},
	{Name: "llm_total_tokens", Type: metrics.Counter, ValueType: metrics.Default},
	{Name: "llm_cost_total", Type: metrics.Counter, ValueType: metrics.Default},
	{Name: "llm_errors", Type: metrics.Counter, ValueType: metrics.Default},
}

type LLMPerfMetrics struct {
	registry *metrics.Registry
	samples  chan<- metrics.SampleContainer
}

func NewMetrics(registry *metrics.Registry) *LLMPerfMetrics {
	return &LLMPerfMetrics{
		registry: registry,
	}
}

func (m *LLMPerfMetrics) RecordProviderMetrics(metadata ProviderMetadata) {
	if metadata == nil {
		return
	}

	metricValues := metadata.GetMetricValues()
	now := time.Now()

	samples := make([]metrics.Sample, 0, len(metricValues))
	for name, value := range metricValues {
		if metric := m.registry.Get(name); metric != nil {
			samples = append(samples, metrics.Sample{
				TimeSeries: metrics.TimeSeries{
					Metric: metric,
				},
				Time:  now,
				Value: value,
			})
		}
	}

	if len(samples) > 0 && m.samples != nil {
		m.samples <- metrics.Samples(samples)
	}
}
