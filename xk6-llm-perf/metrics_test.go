package llmperf

import (
	"math"
	"testing"

	"go.k6.io/k6/v2/metrics"
)

func TestRecordMetricSkipsNonFiniteValues(t *testing.T) {
	m := testMetrics(t, metrics.NewRegistry())
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

func testMetrics(t *testing.T, registry *metrics.Registry) *LLMPerfMetrics {
	t.Helper()
	m, err := newMetrics(registry)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestMetricRegistrationConflict(t *testing.T) {
	registry := metrics.NewRegistry()
	if _, err := registry.NewMetric("llm_requests", metrics.Trend); err != nil {
		t.Fatal(err)
	}
	if _, err := newMetrics(registry); err == nil {
		t.Fatal("conflicting metric accepted")
	}
}
