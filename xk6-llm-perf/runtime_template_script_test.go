package llmperf

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"rsc.io/script"
	"rsc.io/script/scripttest"
)

func TestRuntimeTemplateRegisteredBinary(t *testing.T) {
	opts := runtimeTemplateFixture(t)
	binary := os.Getenv("K6_BINARY")
	if binary == "" {
		t.Skip("set K6_BINARY to newly built registered extension")
	}
	var mu sync.Mutex
	var inputs []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Input []string `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.Input) != 1 {
			http.Error(w, "invalid", 400)
			return
		}
		mu.Lock()
		inputs = append(inputs, body.Input[0])
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[{"object":"embedding","embedding":[0.1,0.2],"index":0}],"model":"fixture","usage":{"prompt_tokens":1,"total_tokens":1}}`))
	}))
	t.Cleanup(server.Close)
	example, err := filepath.Abs("examples/runtime-template.js")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	engine := &script.Engine{Cmds: scripttest.DefaultCmds(), Conds: scripttest.DefaultConds()}
	env := append([]string{"PATH=/usr/bin:/bin", "HOME=" + t.TempDir(), "NO_COLOR=1"}, "K6="+binary, "EXAMPLE="+example, "BASE_URL="+server.URL, "TEMPLATE_PYTHON="+opts.Python, "TEMPLATE_PYTHON_SHA256="+opts.PythonSHA256, "TEMPLATE_SCRIPT="+opts.Script, "TEMPLATE_SCRIPT_SHA256="+opts.ScriptSHA256)
	scripttest.Test(t, ctx, engine, env, "testdata/runtime-template.txt")
	t.Cleanup(func() {
		mu.Lock()
		defer mu.Unlock()
		if len(inputs) != 2 || inputs[0] != "hello-0" || inputs[1] != "hello-1" {
			t.Errorf("per-request inputs: %v", inputs)
		}
	})
}
