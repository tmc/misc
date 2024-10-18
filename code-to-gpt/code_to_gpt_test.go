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
	scripttest.Test(t, ctx, engine, env, "testdata/*.txt")
}

var codeToGPTCmd = script.Command(
	script.CmdUsage{
		Summary: "Run code-to-gpt.sh script",
		Args:    "[args...]",
	}, func(s *script.State, args ...string) (script.WaitFunc, error) {
		scriptPath, _ := filepath.Abs("./code-to-gpt.sh")
		cmd := exec.CommandContext(s.Context(), scriptPath, args...)
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
