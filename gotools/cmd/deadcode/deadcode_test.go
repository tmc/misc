// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// needsGoTool skips t if the 'go' tool is missing
func needsGoTool(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skipf("skipping because 'go' tool not found: %v", err)
	}
}

// Test runs the deadcode command on each scenario
// described by a testdata/*.txtar file.
func Test(t *testing.T) {
	needsGoTool(t)
	if runtime.GOOS == "android" {
		t.Skipf("the dependencies are not available on android")
	}

	exe := buildDeadcode(t)

	matches, err := filepath.Glob("testdata/*.txtar")
	if err != nil {
		t.Fatal(err)
	}
	for _, filename := range matches {
		filename := filename
		t.Run(filename, func(t *testing.T) {
			t.Parallel()

			// Parse txtar file
			data, err := os.ReadFile(filename)
			if err != nil {
				t.Fatal(err)
			}
			ar, err := parseTxtar(data)
			if err != nil {
				t.Fatal(err)
			}

			// Write the archive files to the temp directory.
			tmpdir := t.TempDir()
			for _, f := range ar.files {
				filename := filepath.Join(tmpdir, f.name)
				if err := os.MkdirAll(filepath.Dir(filename), 0777); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filename, f.data, 0666); err != nil {
					t.Fatal(err)
				}
			}

			// Parse archive comment as directives
			type testcase struct {
				linenum int
				args    []string
				wantErr bool
				want    map[string]bool // string -> sense
			}
			var cases []*testcase
			var current *testcase
			for i, line := range strings.Split(ar.comment, "\n") {
				line = strings.TrimSpace(line)
				if line == "" || line[0] == '#' {
					continue // skip blanks and comments
				}

				words, err := words(line)
				if err != nil {
					t.Fatalf("cannot break line into words: %v (%s)", err, line)
				}
				switch kind := words[0]; kind {
				case "deadcode", "!deadcode":
					current = &testcase{
						linenum: i + 1,
						want:    make(map[string]bool),
						args:    words[1:],
						wantErr: kind[0] == '!',
					}
					cases = append(cases, current)
				case "want", "!want":
					if current == nil {
						t.Fatalf("'want' directive must be after 'deadcode'")
					}
					if len(words) != 2 {
						t.Fatalf("'want' directive needs argument <<%s>>", line)
					}
					current.want[words[1]] = kind[0] != '!'
				default:
					t.Fatalf("%s: invalid directive %q", filename, kind)
				}
			}

			for _, tc := range cases {
				t.Run(fmt.Sprintf("L%d", tc.linenum), func(t *testing.T) {
					// Run the command.
					cmd := exec.Command(exe, tc.args...)
					cmd.Dir = tmpdir
					cmd.Env = append(os.Environ(), "GOPROXY=", "GO111MODULE=on")
					var stdout, stderr bytes.Buffer
					cmd.Stdout = &stdout
					cmd.Stderr = &stderr
					err := cmd.Run()

					// Check error expectation
					if err != nil {
						if !tc.wantErr {
							t.Fatalf("deadcode failed: %v\nstderr=\n%s", err, &stderr)
						}
					} else if tc.wantErr {
						t.Fatalf("deadcode succeeded unexpectedly\nstdout=\n%s", &stdout)
					}

					// Check output expectations
					got := stdout.String()
					if err != nil {
						got = stderr.String()
					}
					for pattern, want := range tc.want {
						ok := strings.Contains(got, pattern)
						if ok != want {
							if want {
								t.Errorf("missing %q in output", pattern)
							} else {
								t.Errorf("unwanted %q in output", pattern)
							}
							t.Errorf("got:\n%s", got)
						}
					}
				})
			}
		})
	}
}

// Simple txtar implementation
type txtarFile struct {
	name string
	data []byte
}

type txtarArchive struct {
	comment string
	files   []txtarFile
}

func parseTxtar(data []byte) (*txtarArchive, error) {
	var ar txtarArchive

	// Split into sections
	sections := bytes.Split(data, []byte("\n-- "))

	// First section is comment
	ar.comment = string(bytes.TrimSpace(sections[0]))

	// Remaining sections are files
	for _, section := range sections[1:] {
		if len(section) == 0 {
			continue
		}

		// Split into name and content
		parts := bytes.SplitN(section, []byte(" --\n"), 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("malformed file section")
		}

		name := string(bytes.TrimSpace(parts[0]))
		data := bytes.TrimSpace(parts[1])

		ar.files = append(ar.files, txtarFile{
			name: name,
			data: data,
		})
	}

	return &ar, nil
}

// buildDeadcode builds the deadcode executable.
// It returns its path.
func buildDeadcode(t *testing.T) string {
	bin := filepath.Join(t.TempDir(), "deadcode")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", bin)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Building deadcode: %v\n%s", err, out)
	}
	return bin
}

// words breaks a string into words, respecting
// Go string quotations around words with spaces.
func words(s string) ([]string, error) {
	var words []string
	for s != "" {
		s = strings.TrimSpace(s)
		if s == "" {
			break
		}
		if s[0] == '"' || s[0] == '`' {
			prefix, err := strconv.QuotedPrefix(s)
			if err != nil {
				return nil, err
			}
			s = s[len(prefix):]
			word, _ := strconv.Unquote(prefix)
			words = append(words, word)
		} else {
			i := strings.IndexAny(s, " \t\n\r")
			if i < 0 {
				i = len(s)
			}
			words = append(words, s[:i])
			s = s[i:]
		}
	}
	return words, nil
}
