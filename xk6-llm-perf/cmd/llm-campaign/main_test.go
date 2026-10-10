package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"rsc.io/script"
	"rsc.io/script/scripttest"
	"strings"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if os.Getenv("CLI_HELPER") == "1" {
		path := ""
		for i, a := range os.Args {
			if a == "--summary-export" && i+1 < len(os.Args) {
				path = os.Args[i+1]
			}
		}
		if path != "" {
			if e := os.WriteFile(path, []byte(`{"metrics":{"score":{"count":7}}}`), 0600); e != nil {
				fmt.Fprintln(os.Stderr, e)
				os.Exit(3)
			}
			for _, a := range os.Args {
				if strings.Contains(a, "LOAD=fail") {
					os.Exit(2)
				}
			}
			os.Exit(0)
		}
	}
	os.Exit(m.Run())
}
func TestCommand(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "llm-campaign")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)
	if out, e := exec.CommandContext(ctx, "go", "build", "-o", binary, ".").CombinedOutput(); e != nil {
		t.Fatalf("build: %v\n%s", e, out)
	}
	engine := &script.Engine{Cmds: scripttest.DefaultCmds(), Conds: scripttest.DefaultConds()}
	scripttest.Test(t, ctx, engine, append(os.Environ(), "CLI="+binary, "K6_HELPER="+os.Args[0]), "testdata/*.txt")
}
