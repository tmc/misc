package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"testing"

	"rsc.io/script"
	"rsc.io/script/scripttest"
)

func TestMain(m *testing.M) {
	// Set up test environment
	os.Exit(m.Run())
}

func TestXQScripts(t *testing.T) {
	engine := script.NewEngine()
	engine.Cmds = scripttest.DefaultCmds()
	engine.Conds = scripttest.DefaultConds()

	// Add custom command for xq that calls the run function directly
	engine.Cmds["xq"] = script.Command(
		script.CmdUsage{
			Summary: "Run xq command",
			Args:    "[args...]",
		},
		func(s *script.State, args ...string) (script.WaitFunc, error) {
			var stdout, stderr bytes.Buffer
			var stdin io.Reader

			return func(*script.State) (string, string, error) {
				exitCode := run(append([]string{"xq"}, args...), stdin, &stdout, &stderr)
				if exitCode != 0 {
					return stdout.String(), stderr.String(), fmt.Errorf("xq exited with code %d", exitCode)
				}
				return stdout.String(), stderr.String(), nil
			}, nil
		},
	)

	ctx := context.Background()
	env := os.Environ()

	scripttest.Test(t, ctx, engine, env, "testdata/xq_test.txt")
}
