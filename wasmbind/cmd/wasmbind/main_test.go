package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"rsc.io/script"
	"rsc.io/script/scripttest"
)

func TestUsage(t *testing.T) {
	for _, args := range [][]string{nil, {"unknown"}, {"generate", "."}, {"check", "a", "b"}} {
		var out, err bytes.Buffer
		if got := run(context.Background(), args, &out, &err); got != 2 {
			t.Fatalf("%v: exit %d", args, got)
		}
	}
}
func TestScript(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "wasmbind")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, output)
	}
	engine := &script.Engine{Cmds: scripttest.DefaultCmds(), Conds: scripttest.DefaultConds()}
	scripttest.Test(t, context.Background(), engine, append(os.Environ(), "WASMBIND="+bin, "GOWORK=off"), "testdata/*.txt")
}
