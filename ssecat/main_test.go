// Test ssecat using script testing.
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"rsc.io/script"
	"rsc.io/script/scripttest"
)

func TestScript(t *testing.T) {
	// Install ssecat binary
	cmd := exec.Command("go", "install")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to install ssecat: %v\n%s", err, out)
	}

	engine := script.NewEngine()
	engine.Cmds = scripttest.DefaultCmds()
	engine.Conds = scripttest.DefaultConds()

	// Add ssecat command
	engine.Cmds["ssecat"] = script.Program("ssecat", nil, 0)

	ctx := context.Background()
	env := []string{
		fmt.Sprintf("PATH=%s", os.Getenv("PATH")),
	}
	scripttest.Test(t, ctx, engine, env, "testdata/*.txt")
}
