package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	os_exec "os/exec"
	"path/filepath"
	"strings"
	"testing"

	"rsc.io/script"
)

var update = flag.Bool("update", false, "update golden files in testdata")

func TestScript(t *testing.T) {
	flag.Parse()

	engine := &script.Engine{
		Conds: script.DefaultConds(),
		Cmds:  script.DefaultCmds(),
	}

	engine.Cmds["oairt"] = scriptOairt()
	engine.Cmds["mkwav"] = scriptMakeWAV()
	engine.Cmds["assert"] = scriptAssert()
	engine.Cmds["contains"] = scriptContains()
	engine.Cmds["notcontains"] = scriptNotContains()

	testFiles, err := filepath.Glob(filepath.Join("testdata", "*.txt"))
	if err != nil {
		t.Fatal(err)
	}

	for _, file := range testFiles {
		file := file
		name := strings.TrimSuffix(filepath.Base(file), ".txt")
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			state, err := script.NewState(ctx, t.TempDir(), os.Environ())
			if err != nil {
				t.Fatal(err)
			}

			f, err := os.Open(file)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()

			var logBuf bytes.Buffer
			if err := engine.Execute(state, file, bufio.NewReader(f), &logBuf); err != nil {
				t.Logf("Script output:\n%s", logBuf.String())
				t.Fatal(err)
			}

			if testing.Verbose() {
				t.Logf("Script output:\n%s", logBuf.String())
			}

			if *update {
				updateGoldenFiles(t, file, state)
			}
		})
	}
}

func scriptOairt() script.Cmd {
	return script.Command(
		script.CmdUsage{
			Summary: "run oairt with the given arguments",
			Args:    "args...",
			Async:   true,
		},
		func(s *script.State, args ...string) (script.WaitFunc, error) {
			// Find the oairt binary in the project root
			oairtPath := filepath.Join(os.Getenv("PWD"), "oairt")
			if _, err := os.Stat(oairtPath); err != nil {
				// Try relative to test file
				oairtPath = "./oairt"
			}

			cmd := os_exec.Command(oairtPath, args...)
			cmd.Dir = s.Getwd()
			cmd.Env = s.Environ()

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err := cmd.Start()
			if err != nil {
				return nil, err
			}

			return func(*script.State) (string, string, error) {
				err := cmd.Wait()
				return stdout.String(), stderr.String(), err
			}, nil
		},
	)
}

func scriptMakeWAV() script.Cmd {
	return script.Command(
		script.CmdUsage{
			Summary: "create a minimal WAV file for testing",
			Args:    "filename [duration_ms]",
		},
		func(s *script.State, args ...string) (script.WaitFunc, error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("mkwav: requires filename")
			}

			filename := args[0]
			durationMs := 1000
			if len(args) > 1 {
				fmt.Sscanf(args[1], "%d", &durationMs)
			}

			sampleRate := 16000
			numSamples := sampleRate * durationMs / 1000
			dataSize := numSamples * 2

			var buf bytes.Buffer

			buf.WriteString("RIFF")
			writeUint32LE(&buf, uint32(36+dataSize))
			buf.WriteString("WAVE")

			buf.WriteString("fmt ")
			writeUint32LE(&buf, 16)
			writeUint16LE(&buf, 1)
			writeUint16LE(&buf, 1)
			writeUint32LE(&buf, uint32(sampleRate))
			writeUint32LE(&buf, uint32(sampleRate*2))
			writeUint16LE(&buf, 2)
			writeUint16LE(&buf, 16)

			buf.WriteString("data")
			writeUint32LE(&buf, uint32(dataSize))

			for i := 0; i < numSamples; i++ {
				writeUint16LE(&buf, 0)
			}

			path := s.Path(filename)
			if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
				return nil, err
			}

			s.Logf("created WAV file: %s (%d ms, %d Hz)", filename, durationMs, sampleRate)
			return nil, nil
		},
	)
}

func scriptAssert() script.Cmd {
	return script.Command(
		script.CmdUsage{
			Summary: "assert that a condition is true",
			Args:    "condition message",
		},
		func(s *script.State, args ...string) (script.WaitFunc, error) {
			if len(args) < 2 {
				return nil, fmt.Errorf("assert: requires condition and message")
			}

			condition := args[0]
			message := strings.Join(args[1:], " ")

			switch condition {
			case "true":
				return nil, nil
			case "false":
				return nil, fmt.Errorf("assertion failed: %s", message)
			case "exists":
				if _, err := os.Stat(s.Path(message)); err != nil {
					return nil, fmt.Errorf("assertion failed: file does not exist: %s", message)
				}
				return nil, nil
			case "not-exists":
				if _, err := os.Stat(s.Path(message)); err == nil {
					return nil, fmt.Errorf("assertion failed: file exists: %s", message)
				}
				return nil, nil
			default:
				return nil, fmt.Errorf("unknown assertion: %s", condition)
			}
		},
	)
}

func scriptContains() script.Cmd {
	return script.Command(
		script.CmdUsage{
			Summary: "assert that output contains a string",
			Args:    "string",
		},
		func(s *script.State, args ...string) (script.WaitFunc, error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("contains: requires string to search for")
			}

			searchStr := strings.Join(args, " ")
			stdout := s.Stdout()
			stderr := s.Stderr()

			if !strings.Contains(stdout, searchStr) && !strings.Contains(stderr, searchStr) {
				return nil, fmt.Errorf("output does not contain: %q", searchStr)
			}

			return nil, nil
		},
	)
}

func scriptNotContains() script.Cmd {
	return script.Command(
		script.CmdUsage{
			Summary: "assert that output does not contain a string",
			Args:    "string",
		},
		func(s *script.State, args ...string) (script.WaitFunc, error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("notcontains: requires string to search for")
			}

			searchStr := strings.Join(args, " ")
			stdout := s.Stdout()
			stderr := s.Stderr()

			if strings.Contains(stdout, searchStr) || strings.Contains(stderr, searchStr) {
				return nil, fmt.Errorf("output contains: %q", searchStr)
			}

			return nil, nil
		},
	)
}

func writeUint16LE(buf *bytes.Buffer, v uint16) {
	buf.WriteByte(byte(v))
	buf.WriteByte(byte(v >> 8))
}

func writeUint32LE(buf *bytes.Buffer, v uint32) {
	buf.WriteByte(byte(v))
	buf.WriteByte(byte(v >> 8))
	buf.WriteByte(byte(v >> 16))
	buf.WriteByte(byte(v >> 24))
}

func updateGoldenFiles(t *testing.T, scriptFile string, state *script.State) {
	goldenFile := strings.TrimSuffix(scriptFile, ".txt") + ".golden"
	stdout := state.Stdout()

	if stdout != "" {
		if err := os.WriteFile(goldenFile, []byte(stdout), 0644); err != nil {
			t.Errorf("failed to update golden file: %v", err)
		}
	}
}

func scriptBase64() script.Cmd {
	return script.Command(
		script.CmdUsage{
			Summary: "encode or decode base64 data",
			Args:    "encode|decode [data]",
		},
		func(s *script.State, args ...string) (script.WaitFunc, error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("base64: requires encode or decode")
			}

			var output string
			switch args[0] {
			case "encode":
				if len(args) < 2 {
					return nil, fmt.Errorf("base64 encode: requires data")
				}
				output = base64.StdEncoding.EncodeToString([]byte(args[1]))
			case "decode":
				if len(args) < 2 {
					return nil, fmt.Errorf("base64 decode: requires data")
				}
				decoded, err := base64.StdEncoding.DecodeString(args[1])
				if err != nil {
					return nil, err
				}
				output = string(decoded)
			default:
				return nil, fmt.Errorf("base64: unknown command %q", args[0])
			}

			return func(*script.State) (string, string, error) {
				return output, "", nil
			}, nil
		},
	)
}