package main

import (
	"bytes"
	"context"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
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

	gitBin, err := exec.LookPath("git")
	if err != nil {
		t.Fatal("git not found in PATH")
	}
	goBin := filepath.Join(os.Getenv("HOME"), "go", "bin")

	env := []string{
		"USER=" + os.Getenv("USER"),
		"PATH=" + filepath.Dir(gitBin) + ":" + goBin + ":" + defaultPATH(),
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

// defaultPATH returns the default PATH for the system.
func defaultPATH() string {
	return "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
}

var codeToGPTCmd = script.Command(
	script.CmdUsage{
		Summary: "Run code-to-gpt.sh script",
		Args:    "[args...]",
	}, func(s *script.State, args ...string) (script.WaitFunc, error) {
		// Find the script relative to the working directory
		scriptPath := filepath.Join(".", "code-to-gpt.sh")
		scriptPath, err := filepath.Abs(scriptPath)
		if err != nil {
			return nil, err
		}

		cmd := exec.CommandContext(s.Context(), "bash", scriptPath)
		cmd.Args = append(cmd.Args, args...)
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
		path := filepath.Join(s.Getwd(), args[0])
		err := os.WriteFile(path, data, 0644)
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
