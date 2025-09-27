package main

import (
	"context"
	"flag"
	"os"
	"testing"

	"github.com/tmc/misc/md2html/internal/scripttestutil"
	"rsc.io/script"
)

var borderline = flag.Bool("include-borderline-tests", false, "run borderline tests that may be slow or push limits")

func TestScripts(t *testing.T) {
	exe, _ := os.Executable()
	engine := script.NewEngine()
	engine.Cmds["md2html"] = scripttestutil.BackgroundCmd(exe, nil, 0)
	engine.Cmds["curl"] = script.Program("curl", nil, 0)
	// remove Exec:
	delete(engine.Cmds, "exec")

	env := []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
		"TMPDIR=/tmp",
	}
	if gcd := os.Getenv("GOCOVERDIR"); gcd != "" {
		env = append(env, "GOCOVERDIR="+gcd)
	}
	// Use TestWithOptions which respects the -scripttest-sequential flag
	scripttestutil.TestWithOptions(t, context.Background(), engine, env, "testdata/*.txt")
	if *borderline {
		scripttestutil.TestWithOptions(t, context.Background(), engine, env, "testdata/borderline/*.txt")
	}
}
