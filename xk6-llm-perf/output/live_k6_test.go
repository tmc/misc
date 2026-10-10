package output

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestK6LiveJSONLConsumer(t *testing.T) {
	k6 := os.Getenv("K6_SMOKE_BINARY")
	if k6 == "" {
		k6 = os.Getenv("K6_BINARY")
	}
	if k6 == "" {
		t.Skip("set K6_BINARY for signed extension integration")
	}
	k6, err := filepath.Abs(k6)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	fifo := filepath.Join(dir, "live.jsonl")
	if out, err := exec.Command("mkfifo", fifo).CombinedOutput(); err != nil {
		t.Fatalf("mkfifo: %v %s", err, out)
	}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"choices":[{"index":0,"message":{"content":"A"}}],"usage":{"prompt_tokens":0,"completion_tokens":1,"total_tokens":1}}`)
	}))
	defer provider.Close()
	accepted := 0
	collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { accepted++; fmt.Fprint(w, "{}") }))
	defer collector.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		f, err := os.Open(fifo)
		if err != nil {
			done <- err
			return
		}
		defer f.Close()
		receipt, err := Stream(ctx, f, collector.Client(), StreamOptions{Format: "otlp", Endpoint: collector.URL + "/v1/metrics", RunID: "k6-fixture"})
		if err == nil && (receipt.RecordsAccepted != 3 || !receipt.Complete) {
			err = fmt.Errorf("bad receipt %+v", receipt)
		}
		done <- err
	}()
	cmd := exec.CommandContext(ctx, k6, "run", "--quiet", "--log-format", "raw", "--console-output", fifo, "../examples/output/live-load.js")
	cmd.Env = append(os.Environ(), "ENDPOINT_URL="+provider.URL)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("k6: %v %s", err, out)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("consumer timeout")
	}
	if accepted != 3 {
		t.Fatal("accepted", accepted)
	}
}
