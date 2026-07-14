package llmperf

import (
	"math"
	"testing"

	"go.k6.io/k6/metrics"
)

func TestRecordMetricSkipsNonFiniteValues(t *testing.T) {
	m := NewMetrics(metrics.NewRegistry())
	samples := make(chan metrics.SampleContainer, 10)
	m.samples = samples

	m.RecordMetric("llm_requests", math.NaN())
	m.RecordMetric("llm_requests", math.Inf(1))
	m.RecordMetric("llm_requests", 1)

	values := metricValues(samples)
	if got := len(values["llm_requests"]); got != 1 {
		t.Fatalf("llm_requests samples = %d; want 1", got)
	}
	if got := values["llm_requests"][0]; got != 1 {
		t.Fatalf("llm_requests sample = %v; want 1", got)
	}
}
