package main

import (
	"context"
	"flag"
	"os"
	"testing"

	"github.com/tmc/misc/md2html/internal/scripttestutil"
	"rsc.io/script"
	"rsc.io/script/scripttest"
)

var borderline = flag.Bool("include-borderline-tests", false, "run borderline tests that may be slow or push limits")

func TestScripts(t *testing.T) {
	exe, _ := os.Executable()
	engine := script.NewEngine()
	engine.Cmds["md2html"] = scripttestutil.BackgroundCmd(exe)
	engine.Cmds["curl"] = script.Program("curl", nil, 0)

	env := []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
		"TMPDIR=/tmp",
	}
	if gcd := os.Getenv("GOCOVERDIR"); gcd != "" {
		env = append(env, "GOCOVERDIR="+gcd)
	}
	scripttest.Test(t, context.Background(), engine, env, "testdata/*.txt")

	if *borderline {
		scripttest.Test(t, context.Background(), engine, env, "testdata/borderline/*.txt")
	}
}
