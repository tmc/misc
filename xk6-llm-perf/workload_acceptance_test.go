package llmperf

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// Tests both the pinned offline tokenizer receipt and the real built k6 runtime.
func TestBinaryWorkloadAcceptance(t *testing.T) {
	binary := os.Getenv("K6_SMOKE_BINARY")
	if binary == "" {
		t.Skip("set K6_SMOKE_BINARY to test built workload adapters")
	}
	binary, err := filepath.Abs(binary)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	dir := t.TempDir()
	input := filepath.Join(dir, "input.json")
	if err := os.WriteFile(input, []byte(`{"text":"hello world"}`), 0600); err != nil {
		t.Fatal(err)
	}
	token := exec.CommandContext(ctx, "go", "run", ".", "-tokenizer", "testdata/tokenizer.json", "-tokenizer-sha256", "1d5a2116200e41511a30659d8bd90a2ba916a141582e3888b4178b743ca5ad8b", "-model", "fixture", "-revision", "fixture-v1", "-input", input)
	token.Dir = "tools/llm-tokenize"
	var tokenStderr bytes.Buffer
	token.Stderr = &tokenStderr
	receipt, err := token.Output()
	if err != nil {
		t.Fatalf("offline tokenizer: %v\n%s\n%s", err, receipt, tokenStderr.Bytes())
	}
	receiptPath := filepath.Join(dir, "receipt.json")
	if err := os.WriteFile(receiptPath, receipt, 0600); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	hooks := []string{}
	counts := [2]int{}
	contexts := []int{}
	servers := make([]*httptest.Server, 2)
	for i := range servers {
		i := i
		servers[i] = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			defer mu.Unlock()
			if r.URL.Path != "/chat/completions" {
				hooks = append(hooks, r.URL.Path)
				if r.URL.Path == "/reject" {
					w.WriteHeader(503)
					return
				}
				w.WriteHeader(204)
				return
			}
			var req CompletionRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			counts[i]++
			if len(req.Messages) > 1 {
				contexts = append(contexts, len(req.Messages))
			}
			w.Header().Set("Content-Type", "application/json")
			if req.Stream {
				w.Header().Set("Content-Type", "text/event-stream")
				w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\ndata: [DONE]\n\n"))
				return
			}
			w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"hello"}}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
		}))
		defer servers[i].Close()
	}
	cmd := exec.CommandContext(ctx, binary, "run", "--no-color", "examples/workload-acceptance.js")
	cmd.Env = append(os.Environ(), "ENDPOINT_URL="+servers[0].URL, "SECOND_ENDPOINT_URL="+servers[1].URL, "TOKEN_RECEIPT="+receiptPath, "K6_NO_USAGE_REPORT=true", "K6_AUTO_EXTENSION_RESOLUTION=false")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("workload k6: %v\n%s", err, out)
	}
	mu.Lock()
	defer mu.Unlock()
	if counts != [2]int{3, 2} {
		t.Fatalf("routing/context calls: %v", counts)
	}
	if len(contexts) != 2 || contexts[0] != 2 || contexts[1] != 4 {
		t.Fatalf("conversation context sizes: %v", contexts)
	}
	if len(hooks) != 4 || hooks[0] != "/reject" || hooks[1] != "/reset_prefix_cache" || hooks[2] != "/start_profile" || hooks[3] != "/stop_profile" {
		t.Fatalf("hooks: %v", hooks)
	}
}
