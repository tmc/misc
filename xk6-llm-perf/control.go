package llmperf

import (
	"context"
	"fmt"
	"sync"
)

// ControlLimits applies process-wide ceilings to clients naming one group.
// Zero disables a ceiling. A decrease waits for existing holders to drain.
type ControlLimits struct {
	Request int `json:"request"`
	Prefill int `json:"prefill"`
}

// ControlSnapshot reports ceilings and current holders, including queued work
// that holds a request slot while waiting for prefill admission.
type ControlSnapshot struct {
	Limits   ControlLimits `json:"limits"`
	Requests int           `json:"requests"`
	Prefills int           `json:"prefills"`
}

type controlGroup struct {
	state   ControlSnapshot
	changed chan struct{}
}

type controlGroups struct {
	mu     sync.Mutex
	groups map[string]*controlGroup
}

func (cs *controlGroups) set(name string, limits ControlLimits) error {
	if name == "" || len(name) > 256 || limits.Request < 0 || limits.Request > 100000 || limits.Prefill < 0 || limits.Prefill > 100000 {
		return fmt.Errorf("invalid control group or ceilings")
	}
	cs.mu.Lock()
	defer cs.mu.Unlock()
	if cs.groups == nil {
		cs.groups = make(map[string]*controlGroup)
	}
	g := cs.groups[name]
	if g == nil {
		if len(cs.groups) >= 256 {
			return fmt.Errorf("control group budget exceeded")
		}
		g = &controlGroup{changed: make(chan struct{})}
		cs.groups[name] = g
	}
	g.state.Limits = limits
	close(g.changed)
	g.changed = make(chan struct{})
	return nil
}

func (cs *controlGroups) snapshot(name string) (ControlSnapshot, error) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	g := cs.groups[name]
	if g == nil {
		return ControlSnapshot{}, fmt.Errorf("unknown control group")
	}
	return g.state, nil
}

func (cs *controlGroups) acquire(ctx context.Context, name string, prefill bool) (func(), error) {
	for {
		cs.mu.Lock()
		g := cs.groups[name]
		if g == nil {
			cs.mu.Unlock()
			return nil, fmt.Errorf("unknown control group")
		}
		if err := ctx.Err(); err != nil {
			cs.mu.Unlock()
			return nil, err
		}
		used, limit := g.state.Requests, g.state.Limits.Request
		if prefill {
			used, limit = g.state.Prefills, g.state.Limits.Prefill
		}
		if limit == 0 || used < limit {
			if prefill {
				g.state.Prefills++
			} else {
				g.state.Requests++
			}
			cs.mu.Unlock()
			var once sync.Once
			return func() {
				once.Do(func() {
					cs.mu.Lock()
					defer cs.mu.Unlock()
					if prefill {
						g.state.Prefills--
					} else {
						g.state.Requests--
					}
					close(g.changed)
					g.changed = make(chan struct{})
				})
			}, nil
		}
		changed := g.changed
		cs.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("control admission: %w", ctx.Err())
		case <-changed:
		}
	}
}
