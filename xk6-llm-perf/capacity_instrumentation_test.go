package llmperf

import (
	"context"
	"testing"
	"time"

	"go.k6.io/k6/v2/metrics"
)

func TestPipelineCancellation(t *testing.T) {
	var m LLMPerfMetrics
	output := make(chan metrics.SampleContainer)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { m.pipeline.push(ctx, output, metrics.Samples{}); close(done) }()
	deadline := time.After(time.Second)
	for m.pipeline.inflight.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("producer did not start")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	if err := m.ResetPipeline(); err == nil {
		t.Fatal("reset accepted active producer")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cancelled producer blocked")
	}
	s := m.PipelineSnapshot()
	if s.Attempted != 1 || s.Cancelled != 1 || s.Delivered != 0 || s.Inflight != 0 || s.WaitNanoseconds == 0 {
		t.Fatalf("snapshot %+v", s)
	}
	if err := m.ResetPipeline(); err != nil {
		t.Fatal(err)
	}
	if m.PipelineSnapshot() != (PipelineSnapshot{Generation: 1}) {
		t.Fatal("reset retained counters")
	}
}

func TestPipelineDelivery(t *testing.T) {
	var m LLMPerfMetrics
	output := make(chan metrics.SampleContainer, 1)
	m.pipeline.push(context.Background(), output, metrics.Samples{})
	if s := m.PipelineSnapshot(); s.Delivered != 1 || s.Cancelled != 0 || s.Attempted != 1 {
		t.Fatalf("snapshot %+v", s)
	}
}

func TestPipelineResetRace(t *testing.T) {
	var m LLMPerfMetrics
	output := make(chan metrics.SampleContainer, 1000)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 1000; i++ {
			m.pipeline.push(context.Background(), output, metrics.Samples{})
		}
	}()
	for i := 0; i < 1000; i++ {
		_ = m.ResetPipeline()
	}
	<-done
	s := m.PipelineSnapshot()
	if s.Attempted != s.Delivered+s.Cancelled || s.Inflight != 0 {
		t.Fatalf("partial reset receipt %+v", s)
	}
}

func TestPipelineSnapshotResetRace(t *testing.T) {
	var m LLMPerfMetrics
	m.pipeline.attempted.Store(100)
	m.pipeline.delivered.Store(100)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 1000; i++ {
			_ = m.ResetPipeline()
		}
	}()
	for i := 0; i < 1000; i++ {
		s := m.PipelineSnapshot()
		if s.Attempted != s.Delivered+s.Cancelled {
			t.Fatalf("inconsistent stopped snapshot %+v", s)
		}
	}
	<-done
}
