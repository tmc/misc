package llmperf

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestBinaryCalibration(t *testing.T) {
	binary := os.Getenv("K6_SMOKE_BINARY")
	if binary == "" {
		t.Skip("K6_SMOKE_BINARY not configured")
	}
	var hits atomic.Int64
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path == "/failed" {
			w.WriteHeader(503)
			return
		}
		fmt.Fprint(w, "healthy")
	}))
	defer s.Close()
	binary, err := filepath.Abs(binary)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "run", "--no-color", "examples/calibration-smoke.js")
	cmd.Env = append(os.Environ(), "ENDPOINT_URL="+s.URL, "K6_NO_USAGE_REPORT=true", "K6_AUTO_EXTENSION_RESOLUTION=false")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("calibration: %v\n%s", err, output)
	}
	if hits.Load() != 4 {
		t.Fatalf("HTTP hits=%d, want 4 (TCP probes send no HTTP)", hits.Load())
	}
}
