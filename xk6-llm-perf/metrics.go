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
    {Name: "llm_ttft", Type: metrics.Trend, ValueType: metrics.Time},
    {Name: "llm_token_latency", Type: metrics.Trend, ValueType: metrics.Time},
    {Name: "llm_tokens_per_second", Type: metrics.Gauge, ValueType: metrics.Default},
    {Name: "llm_total_tokens", Type: metrics.Counter, ValueType: metrics.Default},
    {Name: "llm_errors", Type: metrics.Counter, ValueType: metrics.Default},
}

type LLMPerfMetrics struct {
    registry *metrics.Registry
    samples  chan<- metrics.SampleContainer
    metrics  map[string]*metrics.Metric
}

func NewMetrics(registry *metrics.Registry) *LLMPerfMetrics {
    m := &LLMPerfMetrics{
        registry: registry,
        metrics:  make(map[string]*metrics.Metric),
    }

    // Register all metrics
    for _, def := range MetricDefinitions {
        metric, err := registry.NewMetric(def.Name, def.Type, def.ValueType)
        if err != nil {
            panic(err)
        }
        m.metrics[def.Name] = metric
    }

    return m
}

func (m *LLMPerfMetrics) RecordMetric(name string, value float64) {
    if metric, ok := m.metrics[name]; ok && m.samples != nil {
        m.samples <- metrics.Sample{
            TimeSeries: metrics.TimeSeries{
                Metric: metric,
            },
            Time:  time.Now(),
            Value: value,
        }
    }
}
