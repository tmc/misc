package llmperf

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRootModuleSharedTransport(t *testing.T) {
	var root rootModule
	first := root.sharedTransport()
	second := root.sharedTransport()
	if first == nil {
		t.Fatal("sharedTransport returned nil")
	}
	if first != second {
		t.Fatal("sharedTransport returned different transports")
	}
}

func TestRootModuleBoundedTransport(t *testing.T) {
	var root rootModule
	first := root.transportFor(16)
	if first != root.transportFor(16) || first == root.sharedTransport() || first.MaxConnsPerHost != 16 {
		t.Fatal("connection cap must bind one shared bounded pool")
	}
	if other := root.transportFor(8); other == first || other.MaxConnsPerHost != 8 {
		t.Fatal("different caps must not reuse an incompatible pool")
	}
	for _, cfg := range []Config{{BaseURL: "http://localhost", MaxConnsPerHost: -1}, {BaseURL: "http://localhost", MaxConnsPerHost: 65537}, {BaseURL: "http://localhost", MaxConnsPerHost: 1, UseK6Transport: true}} {
		if err := validateConfig(cfg); err == nil {
			t.Fatal("accepted invalid connection policy", cfg)
		}
	}
}

func TestRootModuleConnectionCapAcrossClients(t *testing.T) {
	var root rootModule
	var active, maximum atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := active.Add(1)
		for old := maximum.Load(); n > old; old = maximum.Load() {
			if maximum.CompareAndSwap(old, n) {
				break
			}
		}
		defer active.Add(-1)
		time.Sleep(10 * time.Millisecond)
		io.WriteString(w, "ok")
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var workers sync.WaitGroup
	for i := 0; i < 4; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			client := &http.Client{Transport: root.transportFor(1)}
			req, _ := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
			resp, err := client.Do(req)
			if err != nil {
				t.Error(err)
				return
			}
			defer resp.Body.Close()
			if _, err := io.Copy(io.Discard, resp.Body); err != nil {
				t.Error(err)
			}
		}()
	}
	workers.Wait()
	if maximum.Load() != 1 {
		t.Fatalf("shared connection limit: max active %d", maximum.Load())
	}
	root.transportFor(1).CloseIdleConnections()
}
