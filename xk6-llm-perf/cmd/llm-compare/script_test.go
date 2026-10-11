package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"rsc.io/script"
	"rsc.io/script/scripttest"
	"testing"
	"time"
)

func TestCommand(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "llm-compare")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, "go", "build", "-o", binary, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	engine := &script.Engine{Cmds: scripttest.DefaultCmds(), Conds: scripttest.DefaultConds()}
	scripttest.Test(t, ctx, engine, append(os.Environ(), "CLI="+binary), "testdata/*.txt")
}
