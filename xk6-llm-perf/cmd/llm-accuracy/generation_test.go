package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/tmc/misc/xk6-llm-perf/accuracy"
)

func TestCommandGeneratesFixture(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	dir := t.TempDir()
	binary := filepath.Join(dir, "llm-accuracy")
	if out, err := exec.CommandContext(ctx, "go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, out)
	}
	a := accuracy.Artifact{SchemaVersion: 1, Task: "fixture", TaskVersion: "1", ReferenceVersion: "1", Evaluator: "exact-match-v1", EvaluatorVersion: "1", Model: "fixture", ModelRevision: "1", Cases: []accuracy.Case{{ID: "one", Reference: "A", Outcome: "success"}, {ID: "two", Reference: "A", Outcome: "success"}}}
	a.ReferenceSHA256, _ = accuracy.ReferenceHash(a.Cases)
	b, _ := json.Marshal(a)
	artifact := filepath.Join(dir, "artifact.json")
	os.WriteFile(artifact, b, 0600)
	prompts := filepath.Join(dir, "prompts.json")
	os.WriteFile(prompts, []byte(`{"one":"independent question","two":"fail fixture"}`), 0600)
	calls := 0
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "Bearer fixture-secret" {
			t.Error("missing auth")
		}
		if calls == 2 {
			w.WriteHeader(503)
			return
		}
		fmt.Fprint(w, `{"choices":[{"index":0,"message":{"content":"A"}}]}`)
	}))
	defer provider.Close()
	cmd := exec.CommandContext(ctx, binary, "-input", artifact, "-prompts", prompts, "-endpoint", provider.URL, "-authorization-env", "FIXTURE_AUTH")
	cmd.Env = append(os.Environ(), "FIXTURE_AUTH=Bearer fixture-secret")
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	var report accuracy.Report
	if err := json.Unmarshal(out, &report); err != nil {
		t.Fatal(err)
	}
	if report.Correct != 1 || report.Failed != 1 || report.Accuracy != .5 || report.PromptSHA256 == "" || calls != 2 {
		t.Fatalf("report %+v", report)
	}
}
