package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"rsc.io/script"
	"rsc.io/script/scripttest"
)

var debug = flag.Bool("debug", false, "Enable debug mode for tests")

func TestCodeToGPT(t *testing.T) {
	engine := script.NewEngine()
	engine.Cmds["code-to-gpt"] = codeToGPTCmd
	engine.Cmds["create-binary-file"] = createBinaryFileCmd
	engine.Conds["has"] = condHas
	ctx := context.Background()
	env := []string{
		"USER=" + os.Getenv("USER"),
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + filepath.Dir(filepath.Dir(t.TempDir())),
	}

	if *debug {
		env = append(env, "SHELLOPTS=xtrace")
		env = append(env, `PS4='+ $(basename ${BASH_SOURCE[0]}):${LINENO}: ${FUNCNAME[0]:+${FUNCNAME[0]}(): }`)
		env = append(env, "BASH_VERBOSE=1")
		env = append(env, "BASH_XTRACEFD=2")
	}
	scripttest.Test(t, ctx, engine, env, "testdata/*.txt")
}

var codeToGPTCmd = script.Command(
	script.CmdUsage{
		Summary: "Run code-to-gpt.sh script",
		Args:    "[args...]",
	}, func(s *script.State, args ...string) (script.WaitFunc, error) {
		scriptPath, _ := filepath.Abs("./code-to-gpt.sh")
		cmd := exec.CommandContext(s.Context(), "bash", "-c")
		if *debug {
			cmd.Args = append(cmd.Args, "set -x; "+scriptPath+" "+strings.Join(args, " "))
		} else {
			cmd.Args = append(cmd.Args, scriptPath+" "+strings.Join(args, " "))
		}
		fmt.Println("args", cmd.Args)
		cmd.Dir = s.Getwd()
		cmd.Env = s.Environ()
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Start(); err != nil {
			return nil, err
		}
		return func(s *script.State) (string, string, error) {
			err := cmd.Wait()
			return stdout.String(), stderr.String(), err
		}, nil
	})

// extra command to create a binary file for testing
var createBinaryFileCmd = script.Command(
	script.CmdUsage{Summary: "Create a binary file"},
	func(s *script.State, args ...string) (script.WaitFunc, error) {
		if len(args) != 1 {
			return nil, script.ErrUsage
		}
		data := []byte{0, 1, 2, 3}
		err := os.WriteFile(args[0], data, 0644)
		return nil, err
	},
)
var condHas = script.PrefixCondition("has", func(s *script.State, arg string) (bool, error) {
	p := s.ExpandEnv("$PATH", false)
	for _, dir := range filepath.SplitList(p) {
		// check if the file exists and is executable:
		if fi, err := os.Stat(filepath.Join(dir, arg)); err == nil && fi.Mode().IsRegular() && fi.Mode().Perm()&0111 != 0 {
			return true, nil
		}
	}
	return false, nil
})
