package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"rsc.io/script"
	"rsc.io/script/scripttest"
)

// TestScript runs the scripts in testdata/script against a freshly built
// calltree binary.
func TestScript(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "calltree")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("building calltree: %v\n%s", err, out)
	}
	e := script.NewEngine()
	e.Cmds = scripttest.DefaultCmds()
	e.Conds = scripttest.DefaultConds()
	e.Cmds["calltree"] = script.Program(bin, nil, 0)
	scripttest.Test(t, context.Background(), e, os.Environ(), "testdata/script/*.txt")
}
