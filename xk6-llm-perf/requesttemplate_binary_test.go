package llmperf

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

func TestBinaryTemplateEndpoints(t *testing.T) {
	binary := os.Getenv("K6_SMOKE_BINARY")
	if binary == "" {
		t.Skip("set K6_SMOKE_BINARY to test structured templates and endpoint shapes")
	}
	binary, err := filepath.Abs(binary)
	if err != nil {
		t.Fatal(err)
	}
	fixtures := endpointFixtures()
	fixtures = append(fixtures, struct{ protocol, name, body, response string }{"single-token", "", `{"model":"fixture","input":[1,2],"encoding_format":"base64"}`, `{"data":[{"object":"embedding","index":0,"embedding":"AACAPwAAAEA="}],"usage":{"prompt_tokens":2,"total_tokens":2}}`}, struct{ protocol, name, body, response string }{"batch-token", "", `{"model":"fixture","input":[[1,2],[3]],"encoding_format":"base64"}`, `{"data":[{"object":"embedding","index":0,"embedding":"AACAPwAAAEA="},{"object":"embedding","index":1,"embedding":"AACAPwAAAEA="}],"usage":{"prompt_tokens":3,"total_tokens":3}}`})
	for _, fixture := range endpointFixtures() {
		if fixture.protocol == "solido-rag" {
			stream := fixture
			stream.protocol = "solido-rag-stream"
			fixtures = append(fixtures, stream)
		}
	}
	var mu sync.Mutex
	seen := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, fixture := range fixtures {
			if r.URL.Path != "/"+fixture.protocol {
				continue
			}
			var got, want map[string]any
			if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
				t.Errorf("decode %s: %v", r.URL.Path, err)
				http.Error(w, "bad", 400)
				return
			}
			if err := json.Unmarshal([]byte(fixture.body), &want); err != nil {
				t.Error(err)
				return
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("%s body got %v want %v", r.URL.Path, got, want)
				http.Error(w, "bad", 400)
				return
			}
			mu.Lock()
			seen[fixture.protocol]++
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			if fixture.protocol == "solido-rag-stream" {
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprint(w, "data: {\"content\":\"hello \",\"sources\":[{\"document\":\"old.pdf\"}]}\n\ndata: {\"content\":\"world\"}\n\ndata: {\"sources\":[{\"document\":\"fixture.pdf\"}],\"usage\":{\"prompt_tokens\":2,\"completion_tokens\":3,\"total_tokens\":5}}\n\ndata: [DONE]\n\n")
				return
			}
			fmt.Fprint(w, fixture.response)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	script, err := filepath.Abs("examples/template-endpoints.js")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary, "run", "--quiet", "--no-color", script)
	cmd.Dir = t.TempDir()
	cmd.Env = append(os.Environ(), "BASE_URL="+server.URL, "K6_NO_USAGE_REPORT=true")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("template binary: %v\n%s", err, output)
	}
	mu.Lock()
	defer mu.Unlock()
	for _, fixture := range fixtures {
		if seen[fixture.protocol] != 1 {
			t.Errorf("%s calls: %d\n%s", fixture.protocol, seen[fixture.protocol], output)
		}
	}
}
