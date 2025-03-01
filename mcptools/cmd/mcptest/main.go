package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/tmc/misc/mcptools/internal/mcp"
)

var (
	update = flag.Bool("update", false, "update golden files")
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: mcptest [flags] [files...]\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	// Default to all .txt files in testdata
	patterns := flag.Args()
	if len(patterns) == 0 {
		patterns = []string{"testdata/*.txt"}
	}

	var failed bool
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			log.Fatal(err)
		}
		for _, path := range matches {
			if err := runTest(path); err != nil {
				log.Printf("%s: %v", path, err)
				failed = true
			}
		}
	}
	if failed {
		os.Exit(1)
	}
}

func runTest(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	entries, err := mcp.LoadRecording(f)
	if err != nil {
		return err
	}

	goldenPath := strings.TrimSuffix(path, ".txt") + ".golden"
	if *update {
		f, err := os.Create(goldenPath)
		if err != nil {
			return err
		}
		defer f.Close()
		for _, e := range entries {
			if err := e.WriteTo(f); err != nil {
				return err
			}
		}
		return nil
	}

	golden, err := os.Open(goldenPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no golden file %s (run with -update to create)", goldenPath)
		}
		return err
	}
	defer golden.Close()

	goldenEntries, err := mcp.LoadRecording(golden)
	if err != nil {
		return err
	}

	if len(entries) != len(goldenEntries) {
		return fmt.Errorf("got %d entries, want %d", len(entries), len(goldenEntries))
	}

	for i, got := range entries {
		want := goldenEntries[i]
		if got.Dir != want.Dir {
			return fmt.Errorf("entry %d: got dir %q, want %q", i, got.Dir, want.Dir)
		}
		if string(got.Data) != string(want.Data) {
			return fmt.Errorf("entry %d: got data %q, want %q", i, got.Data, want.Data)
		}
	}
	return nil
}
