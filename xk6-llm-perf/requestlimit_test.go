package llmperf

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/grafana/sobek"
	"go.k6.io/k6/v2/metrics"
)

func TestSharedRequestCeilingAndRelease(t *testing.T) {
	var active, peak atomic.Int32
	ready := make(chan struct{})
	gate := make(chan struct{})
	var once sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := active.Add(1)
		defer active.Add(-1)
		for old := peak.Load(); n > old; old = peak.Load() {
			if peak.CompareAndSwap(old, n) {
				break
			}
		}
		if n == 2 {
			once.Do(func() { close(ready) })
		}
		select {
		case <-gate:
		case <-r.Context().Done():
			return
		}
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"fixture"}}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
	}))
	defer server.Close()
	var limits prefillLimiters
	clients := make([]*Client, 8)
	args := make([]sobek.Value, 8)
	for i := range clients {
		c := setupTestClient(t, server.URL)
		c.config.Model = "fixture"
		c.config.RequestConcurrency = 2
		c.config.Timeout = "2s"
		c.requestLimiters = &limits
		c.metrics.samples = make(chan metrics.SampleContainer, 100)
		clients[i] = c
		value, err := jsonValue(c.vu.Runtime(), CompletionRequest{Messages: []Message{{Role: "user", Content: "fixture"}}})
		if err != nil {
			t.Fatal(err)
		}
		args[i] = value
	}
	var wg sync.WaitGroup
	failures := make(chan error, len(clients))
	for i, c := range clients {
		wg.Go(func() {
			result := c.Complete(sobek.FunctionCall{Arguments: []sobek.Value{args[i]}})
			if err, ok := result.Export().(error); ok {
				failures <- err
			}
		})
	}
	select {
	case <-ready:
	case <-time.After(time.Second):
		close(gate)
		wg.Wait()
		t.Fatal("request ceiling did not admit two workers")
	}
	close(gate)
	wg.Wait()
	close(failures)
	for err := range failures {
		t.Error(err)
	}
	if peak.Load() != 2 || active.Load() != 0 {
		t.Fatalf("peak=%d active=%d", peak.Load(), active.Load())
	}
	config := clients[0].config
	config.PrefillConcurrency = config.RequestConcurrency
	release, err := limits.acquire(clients[0].vu.Context(), config)
	if err != nil {
		t.Fatal(err)
	}
	release()
}
