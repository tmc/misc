package main

import (
	"bufio"
	"context"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/txtar"
	"rsc.io/script"
)

var borderline = flag.Bool("borderline", false, "run borderline tests that may be slow or push limits")

func TestScripts(t *testing.T) {
	// Find all .txt files in testdata
	matches, err := filepath.Glob("testdata/*.txt")
	if err != nil {
		t.Fatal(err)
	}

	// Add borderline tests if flag is set
	if *borderline {
		borderlineMatches, err := filepath.Glob("testdata/borderline/*.txt")
		if err != nil {
			t.Fatal(err)
		}
		matches = append(matches, borderlineMatches...)
	}

	engine := script.NewEngine()

	for _, file := range matches {
		t.Run(filepath.Base(file), func(t *testing.T) {
			content, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}

			workdir, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}

			state, err := script.NewState(context.Background(), workdir, os.Environ())
			if err != nil {
				t.Fatal(err)
			}

			// Parse as txtar archive and extract files if it contains txtar content
			archive := txtar.Parse(content)
			if len(archive.Files) > 0 {
				err = state.ExtractFiles(archive)
				if err != nil {
					t.Fatal(err)
				}
			}

			// Execute the script (comment section of txtar, or entire file if not txtar)
			script := string(archive.Comment)
			if strings.TrimSpace(script) == "" {
				script = string(content) // fallback for non-txtar files
			}

			reader := bufio.NewReader(strings.NewReader(script))
			err = engine.Execute(state, filepath.Base(file), reader, os.Stdout)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}