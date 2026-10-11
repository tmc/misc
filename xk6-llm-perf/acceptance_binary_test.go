package llmperf

import (
	"context"
	"encoding/json"
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

func TestBinaryWorkQualification(t *testing.T) {
	binary := os.Getenv("K6_SMOKE_BINARY")
	if binary == "" {
		t.Skip("set K6_SMOKE_BINARY")
	}
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if _, ok := body["work"]; ok {
			t.Error("work policy leaked to provider")
		}
		requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"model":"fixture","choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "run.json")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary, "run", "--no-color", "examples/work-qualification-smoke.js")
	command.Env = append(os.Environ(), "BASE_URL="+server.URL, "RUN_FILE="+path, "K6_NO_USAGE_REPORT=true", "K6_AUTO_EXTENSION_RESOLUTION=false")
	output, err := command.CombinedOutput()
	t.Log(string(output))
	if err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 3 {
		t.Fatalf("requests=%d", requests.Load())
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	run, err := ReadRun(f)
	if err != nil {
		t.Fatal(err)
	}
	if run.WorkAccepted != 1 || run.WorkRejected != 1 || run.WorkUnavailable != 1 || run.WorkGoodRequests != 1 {
		t.Fatal(run)
	}
	for _, m := range run.Records {
		if m.MaxConnsPerHost != 16 || m.TransportProfile != "shared-http2-enabled-v1" || m.RequestBodyBytes <= 0 || m.ResponseBodyBytes <= 0 || m.Stream {
			t.Fatal("client transport or wire evidence unavailable", m)
		}
		if m.SerializationLatency == nil || m.ParsingLatency == nil {
			t.Fatal("client timing unavailable", m)
		}
	}
}
