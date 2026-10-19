package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/tmc/misc/xk6-llm-perf/accuracy"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"rsc.io/script"
	"rsc.io/script/scripttest"
)

func TestCommand(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "llm-accuracy")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, "go", "build", "-o", binary, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	engine := &script.Engine{Cmds: scripttest.DefaultCmds(), Conds: scripttest.DefaultConds()}
	scripttest.Test(t, ctx, engine, append(os.Environ(), "CLI="+binary), "testdata/*.txt")
}

func TestRecipeRequestsCLI(t *testing.T) {
	directory := t.TempDir()
	cases := []accuracy.Case{{ID: "0", Reference: "A", Outcome: "success"}}
	hash, _ := accuracy.ReferenceHash(cases)
	artifact := accuracy.Artifact{SchemaVersion: 1, Task: "fixture", TaskVersion: "1", ReferenceVersion: "1", ReferenceSHA256: hash, Evaluator: "exact-match-v1", EvaluatorVersion: "1", Model: "fixture", ModelRevision: "1", Cases: cases}
	data, _ := json.Marshal(artifact)
	input := filepath.Join(directory, "artifact.json")
	requests := filepath.Join(directory, "requests.json")
	if err := os.WriteFile(input, data, 0600); err != nil {
		t.Fatal(err)
	}
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		fmt.Fprint(w, `{"choices":[{"index":0,"message":{"content":"A"}}]}`)
	}))
	defer server.Close()
	valid := `{"0":{"messages":[{"role":"system","content":"instruction"},{"role":"user","content":"question"}],"max_tokens":5,"stop":["\n"]}}`
	for _, text := range []string{
		`{"0":{"messages":[],"max_tokens":5,"Max_tokens":10}}`,
		`{"0":{"messages":[],"max_tokens":5,"unknown":true}}`,
		`{"0":{"messages":null,"max_tokens":5}}`,
		valid + ` {}`,
	} {
		if err := os.WriteFile(requests, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
		if err := run(input, server.URL, "", requests, "", accuracy.WorkerOptions{}); err == nil {
			t.Fatalf("accepted %s", text)
		}
	}
	if calls != 0 {
		t.Fatal("invalid request dispatched")
	}
	if err := os.WriteFile(requests, []byte(valid), 0600); err != nil {
		t.Fatal(err)
	}
	if err := run(input, server.URL, "", requests, "", accuracy.WorkerOptions{}); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("calls=%d", calls)
	}
	if err := run(input, server.URL, requests, requests, "", accuracy.WorkerOptions{}); err == nil {
		t.Fatal("accepted both input formats")
	}
}
