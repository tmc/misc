package llmperf

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"go.k6.io/k6/v2/metrics"
)

// PipelineSnapshot describes sample delivery, not request or SSE event counts.
// Take snapshots only after producers stop for an exact run receipt.
type PipelineSnapshot struct {
	Generation         uint64 `json:"generation"`
	Attempted          uint64 `json:"attempted"`
	Delivered          uint64 `json:"delivered"`
	Cancelled          uint64 `json:"cancelled"`
	WaitNanoseconds    uint64 `json:"wait_nanoseconds"`
	MaxWaitNanoseconds uint64 `json:"max_wait_nanoseconds"`
	Inflight           int64  `json:"inflight"`
}

type pipelineInstrumentation struct {
	mu         sync.Mutex
	generation atomic.Uint64
	attempted  atomic.Uint64
	delivered  atomic.Uint64
	cancelled  atomic.Uint64
	wait       atomic.Uint64
	maxWait    atomic.Uint64
	inflight   atomic.Int64
}

func (p *pipelineInstrumentation) push(ctx context.Context, output chan<- metrics.SampleContainer, sample metrics.SampleContainer) {
	p.mu.Lock()
	p.inflight.Add(1)
	p.attempted.Add(1)
	p.mu.Unlock()
	defer p.inflight.Add(-1)
	start := time.Now()
	delivered := false
	if ctx.Err() == nil {
		select {
		case output <- sample:
			delivered = true
		case <-ctx.Done():
		}
	}
	elapsed := uint64(time.Since(start))
	p.wait.Add(elapsed)
	for old := p.maxWait.Load(); elapsed > old; old = p.maxWait.Load() {
		if p.maxWait.CompareAndSwap(old, elapsed) {
			break
		}
	}
	if delivered {
		p.delivered.Add(1)
	} else {
		p.cancelled.Add(1)
	}
}

// PipelineSnapshot returns process-local delivery counters for this VU's metrics.
func (m *LLMPerfMetrics) PipelineSnapshot() PipelineSnapshot {
	p := &m.pipeline
	p.mu.Lock()
	defer p.mu.Unlock()
	return PipelineSnapshot{p.generation.Load(), p.attempted.Load(), p.delivered.Load(), p.cancelled.Load(), p.wait.Load(), p.maxWait.Load(), p.inflight.Load()}
}

// ResetPipeline starts a new receipt window. The caller must stop all producers
// before resetting; this is not an automatic reset on each request.
func (m *LLMPerfMetrics) ResetPipeline() error {
	p := &m.pipeline
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.inflight.Load() != 0 {
		return fmt.Errorf("reset pipeline: producers are active")
	}
	p.generation.Add(1)
	p.attempted.Store(0)
	p.delivered.Store(0)
	p.cancelled.Store(0)
	p.wait.Store(0)
	p.maxWait.Store(0)
	return nil
}
