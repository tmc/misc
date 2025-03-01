// Package mcpscripttest provides testing support for MCP commands.
package mcpscripttest

import (
    "context"
    "os"
    "os/exec"
    "strings"
    "testing"
    "io"

    "rsc.io/script"
    "rsc.io/script/scripttest"
)

var defaultInheritEnv = []string{"USER", "HOME", "PATH"}

func getTestEnvironment() []string {
    env := make(map[string]string)
    for _, key := range defaultInheritEnv {
        if val, ok := os.LookupEnv(key); ok {
            env[key] = val
        }
    }
    if inherit := os.Getenv("MCPSCRIPTTEST_ENV_INHERIT"); inherit != "" {
        for _, key := range strings.Split(inherit, ",") {
            key = strings.TrimSpace(key)
            if val, ok := os.LookupEnv(key); ok {
                env[key] = val
            }
        }
    }
    var result []string
    for k, v := range env {
        result = append(result, k+"="+v)
    }
    return result
}

// Test runs script tests with MCP commands configured.
func Test(t *testing.T, pattern string) {
    t.Helper()
    scripttest.Test(t, 
        context.Background(),
        NewEngine(),
        getTestEnvironment(),
        pattern)
}

// NewEngine returns a script engine configured with MCP commands.
func NewEngine() *script.Engine {
    e := script.NewEngine()
    
    // Add all MCP commands with mcp- prefix
    e.Cmds["mcp-replay"] = mcpReplayCmd
    e.Cmds["mcp-spy"] = mcpSpyCmd
    e.Cmds["mcp-start"] = mcpStartCmd
    e.Cmds["mcp-test"] = mcpTestCmd
    e.Cmds["mcp-verify"] = mcpVerifyCmd
    
    return e
}

// Command definitions
var mcpReplayCmd = script.Command(
    script.CmdUsage{
        Summary: "replay MCP recordings",
        Args:    "recording [flags]",
    },
    execCmd("mcp-replay"),
)

var mcpSpyCmd = script.Command(
    script.CmdUsage{
        Summary: "spy on MCP traffic",
        Args:    "[flags]",
    },
    execCmd("mcp-spy"),
)

var mcpStartCmd = script.Command(
    script.CmdUsage{
        Summary: "start MCP components",
        Args:    "[flags]",
        Detail:  []string{"Starts MCP components in the background"},
        Async:   true,
    },
    func(s *script.State, args ...string) (script.WaitFunc, error) {
        // Handle --help synchronously
        if len(args) > 0 && args[0] == "--help" {
            return func(*script.State) (string, string, error) {
                return "Usage: mcp-start [options]\n", "", nil
            }, nil
        }
        return execCmdAsync("mcp-start")(s, args...)
    },
)

var mcpTestCmd = script.Command(
    script.CmdUsage{
        Summary: "run MCP tests",
        Args:    "[flags]",
    },
    execCmd("mcp-test"),
)

var mcpVerifyCmd = script.Command(
    script.CmdUsage{
        Summary: "verify MCP recordings",
        Args:    "recording [flags]",
    },
    execCmd("mcp-verify"),
)

// execCmd returns a standard command runner
func execCmd(name string) func(*script.State, ...string) (script.WaitFunc, error) {
    return func(s *script.State, args ...string) (script.WaitFunc, error) {
        path, err := exec.LookPath(name)
        if err != nil {
            return nil, err
        }
        cmd := exec.CommandContext(s.Context(), path, args...)
        cmd.Dir = s.Getwd()
        cmd.Env = s.Environ()

        stdout, err := cmd.Output()
        if err != nil {
            return nil, err
        }

        return func(*script.State) (string, string, error) {
            return string(stdout), "", nil
        }, nil
    }
}

// execCmdAsync returns an async command runner
func execCmdAsync(name string) func(*script.State, ...string) (script.WaitFunc, error) {
    return func(s *script.State, args ...string) (script.WaitFunc, error) {
        path, err := exec.LookPath(name)
        if err != nil {
            return nil, err
        }
        cmd := exec.CommandContext(s.Context(), path, args...)
        cmd.Dir = s.Getwd()
        cmd.Env = s.Environ()

        stdout, err := cmd.StdoutPipe()
        if err != nil {
            return nil, err
        }
        stderr, err := cmd.StderrPipe()
        if err != nil {
            return nil, err
        }

        if err := cmd.Start(); err != nil {
            return nil, err
        }

        return func(s *script.State) (string, string, error) {
            outBytes, err := io.ReadAll(stdout)
            if err != nil {
                return "", "", err
            }
            errBytes, err := io.ReadAll(stderr)
            if err != nil {
                return "", "", err
            }
            err = cmd.Wait()
            return string(outBytes), string(errBytes), err
        }, nil
    }
}

