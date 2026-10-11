package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"rsc.io/script"
	"rsc.io/script/scripttest"
	"testing"
	"time"
)

func TestCommand(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "llm-ollama-frame")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, "go", "build", "-o", binary, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	templateData, err := os.ReadFile("testdata/qwen3-ollama-0.12.7.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	modelData, err := json.Marshal(show{Template: string(templateData), Capabilities: []string{"thinking"}})
	if err != nil {
		t.Fatal(err)
	}
	modelPath := filepath.Join(t.TempDir(), "model-show.json")
	if err := os.WriteFile(modelPath, modelData, 0600); err != nil {
		t.Fatal(err)
	}
	modelHash := fmt.Sprintf("%x", sha256.Sum256(modelData))
	env := append(os.Environ(), "CLI="+binary, "MODEL_SHOW="+modelPath, "MODEL_SHA="+modelHash)
	for _, fixture := range []struct{ name, system, messages string }{
		{"SYSTEM", `"prior instruction"`, ""}, {"MESSAGES", "", `[{"role":"assistant","content":"prior"}]`},
		{"NULL_SYSTEM", `null`, ""}, {"NULL_MESSAGES", "", `null`}, {"EMPTY", `""`, `[]`},
	} {
		model := show{Template: string(templateData), Capabilities: []string{"thinking"}, System: []byte(fixture.system), Messages: []byte(fixture.messages)}
		data, err := json.Marshal(model)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), fixture.name+".json")
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		env = append(env, "MODEL_"+fixture.name+"="+path, "SHA_"+fixture.name+"="+fmt.Sprintf("%x", sha256.Sum256(data)))
	}
	engine := &script.Engine{Cmds: scripttest.DefaultCmds(), Conds: scripttest.DefaultConds()}
	scripttest.Test(t, ctx, engine, env, "testdata/*.txt")
}
