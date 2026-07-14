package llmperf

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

type prefillLimiters struct {
	mu       sync.Mutex
	limiters map[prefillLimiterKey]*prefillLimiter
}

type prefillLimiterKey struct {
	baseURL string
	model   string
	limit   int
}

type prefillLimiter struct {
	ch chan struct{}
}

func (ls *prefillLimiters) acquire(ctx context.Context, config Config) (func(), error) {
	if config.PrefillConcurrency <= 0 {
		return func() {}, nil
	}
	endpoints := normalizedBaseURLs(config)
	baseURL := ""
	if len(endpoints) > 0 {
		baseURL = strings.Join(endpoints, ",")
	}
	key := prefillLimiterKey{
		baseURL: baseURL,
		model:   config.Model,
		limit:   config.PrefillConcurrency,
	}

	limiter := ls.limiter(key)
	select {
	case limiter.ch <- struct{}{}:
		var once sync.Once
		return func() {
			once.Do(func() {
				<-limiter.ch
			})
		}, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("wait for prefill slot: %w", ctx.Err())
	}
}

func (ls *prefillLimiters) limiter(key prefillLimiterKey) *prefillLimiter {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	if ls.limiters == nil {
		ls.limiters = make(map[prefillLimiterKey]*prefillLimiter)
	}
	limiter := ls.limiters[key]
	if limiter == nil {
		limiter = &prefillLimiter{ch: make(chan struct{}, key.limit)}
		ls.limiters[key] = limiter
	}
	return limiter
}
