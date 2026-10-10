package llmperf

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestBinaryDistributed(t *testing.T) {
	t.Run("success", func(t *testing.T) { testBinaryDistributed(t, false) })
	t.Run("failed requests", func(t *testing.T) { testBinaryDistributed(t, true) })
}
func testBinaryDistributed(t *testing.T, fail bool) {
	t.Helper()
	binary := os.Getenv("K6_SMOKE_BINARY")
	if binary == "" {
		t.Skip("set K6_SMOKE_BINARY to test real distributed workers")
	}
	binary, err := filepath.Abs(binary)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	var mu sync.Mutex
	seen := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req CompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", 400)
			return
		}
		id, _ := req.Messages[0].Content.(string)
		mu.Lock()
		seen[id]++
		mu.Unlock()
		if fail && (strings.HasSuffix(id, "1") || strings.HasSuffix(id, "3") || strings.HasSuffix(id, "5") || strings.HasSuffix(id, "7")) {
			http.Error(w, "fixture request failure", 503)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"hello"}}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
	}))
	t.Cleanup(server.Close)
	rows, err := os.ReadFile("examples/distributed/rows.json")
	if err != nil {
		t.Fatal(err)
	}
	script, err := os.ReadFile("examples/distributed/worker.js")
	if err != nil {
		t.Fatal(err)
	}
	assets := append(append(script, 0), rows...)
	hash := fmt.Sprintf("sha256:%x", sha256.Sum256(assets))
	epoch := time.Now().Add(5 * time.Second).UnixMilli()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	type workerResult struct {
		id     string
		output []byte
		err    error
	}
	results := make(chan workerResult, 2)
	for i, segment := range []string{"0:1/2", "1/2:1"} {
		id := fmt.Sprintf("worker-%d", i)
		cmd := exec.CommandContext(ctx, binary, "run", "--no-color", "--execution-segment", segment, "--execution-segment-sequence", "0,1/2,1", "examples/distributed/worker.js")
		cmd.Env = append(os.Environ(), "ENDPOINT_URL="+server.URL, "RUN_FILE="+filepath.Join(dir, id+".json"), "WORKER_ID="+id, fmt.Sprintf("WINDOW_START_MS=%d", epoch), "WINDOW_DURATION_MS=3000", "TRACE_HASH="+hash, "K6_NO_USAGE_REPORT=true", "K6_AUTO_EXTENSION_RESOLUTION=false")
		go func() { out, err := cmd.CombinedOutput(); results <- workerResult{id, out, err} }()
	}
	workers := []WorkerRun{}
	files := []map[string]any{}
	zero := 0.0
	for range 2 {
		result := <-results
		var exit *exec.ExitError
		if result.err != nil && !(fail && errors.As(result.err, &exit) && exit.ExitCode() == 99) {
			t.Fatalf("%s: %v\n%s", result.id, result.err, result.output)
		}
		data, err := os.ReadFile(filepath.Join(dir, result.id+".json"))
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, map[string]any{"file": result.id + ".json", "sha256": fmt.Sprintf("%x", sha256.Sum256(data)), "clock": map[string]any{"offset_ms": 0, "uncertainty_ms": 0, "basis": "local host fixture"}})
		var run RunSummary
		if err := json.Unmarshal(data, &run); err != nil {
			t.Fatal(err)
		}
		if run.Attempts != 4 || run.ExcludedBoundaryRequests != 0 {
			t.Fatalf("worker accounting: %+v\n%s", run, result.output)
		}
		workers = append(workers, WorkerRun{Run: run, ClockUncertainty: &zero, ClockBasis: "two local processes share host CLOCK_REALTIME; injected zero bound fixture"})
	}
	merged, err := MergeRuns(workers, MergeOptions{ExpectedWorkers: []string{"worker-0", "worker-1"}})
	if err != nil {
		t.Fatal(err)
	}
	if merged.Attempts != 8 || merged.OutputTokens != int64(8-merged.Failures) || merged.StartedAt != float64(epoch) {
		t.Fatalf("merge accounting: %+v", merged)
	}
	wantFailures := 0
	if fail {
		wantFailures = 4
	}
	if merged.Failures != wantFailures {
		t.Fatalf("failed request accounting: %+v", merged)
	}
	manifestData, err := json.Marshal(map[string]any{"schema_version": 1, "expected_workers": []string{"worker-0", "worker-1"}, "max_clock_uncertainty_ms": 0, "workers": files})
	if err != nil {
		t.Fatal(err)
	}
	manifestFile := filepath.Join(dir, "workers.json")
	if err := os.WriteFile(manifestFile, manifestData, 0600); err != nil {
		t.Fatal(err)
	}
	cli := exec.CommandContext(ctx, "go", "run", "./cmd/llm-merge", "-manifest", manifestFile)
	cli.Env = append(os.Environ(), "GOWORK=off")
	output, err := cli.CombinedOutput()
	if err != nil {
		t.Fatalf("actual worker CLI merge: %v\n%s", err, output)
	}
	replay, err := ReadRun(bytes.NewReader(output))
	if err != nil {
		t.Fatal(err)
	}
	if replay.Attempts != merged.Attempts || replay.Failures != merged.Failures || replay.OutputTokens != merged.OutputTokens {
		t.Fatal("CLI changed actual worker accounting")
	}
	mu.Lock()
	defer mu.Unlock()
	if len(seen) != 8 {
		t.Fatalf("unique rows=%v", seen)
	}
	for id, n := range seen {
		if n != 1 {
			t.Fatalf("row %s executed %d times", id, n)
		}
	}
}
