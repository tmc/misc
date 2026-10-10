// Command llm-merge validates pinned worker files and reconstructs one run.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	llmperf "github.com/tmc/misc/xk6-llm-perf"
)

type manifest struct {
	SchemaVersion       int      `json:"schema_version"`
	ExpectedWorkers     []string `json:"expected_workers"`
	MaxClockUncertainty float64  `json:"max_clock_uncertainty_ms"`
	Workers             []worker `json:"workers"`
}
type worker struct {
	File   string `json:"file"`
	SHA256 string `json:"sha256"`
	Clock  clock  `json:"clock"`
}

type clock struct {
	Offset      *float64 `json:"offset_ms"`
	Uncertainty *float64 `json:"uncertainty_ms"`
	Basis       string   `json:"basis"`
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "llm-merge:", err)
		os.Exit(1)
	}
}
func run(args []string, out io.Writer) error {
	flags := flag.NewFlagSet("llm-merge", flag.ContinueOnError)
	input := flags.String("manifest", "", "portable worker-file and clock manifest")
	partial := flags.Bool("allow-partial", false, "retain explicitly missing workers")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *input == "" || flags.NArg() != 0 {
		return fmt.Errorf("usage: llm-merge -manifest workers.json [-allow-partial]")
	}
	f, err := os.Open(*input)
	if err != nil {
		return err
	}
	defer f.Close()
	manifestData, err := io.ReadAll(io.LimitReader(f, 1024*1024+1))
	if err != nil {
		return err
	}
	if len(manifestData) > 1024*1024 {
		return fmt.Errorf("manifest exceeds 1 MiB")
	}
	d := json.NewDecoder(bytes.NewReader(manifestData))
	d.DisallowUnknownFields()
	var m manifest
	if err := d.Decode(&m); err != nil {
		return fmt.Errorf("manifest: %w", err)
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return fmt.Errorf("manifest has trailing data")
	}
	if m.SchemaVersion != 1 || len(m.Workers) == 0 || len(m.Workers) > 256 {
		return fmt.Errorf("invalid manifest schema or worker count")
	}
	workers := make([]llmperf.WorkerRun, 0, len(m.Workers))
	var totalBytes, totalRecords int
	for _, w := range m.Workers {
		if !filepath.IsLocal(w.File) {
			return fmt.Errorf("worker file must be a relative local path")
		}
		want, err := hex.DecodeString(w.SHA256)
		if err != nil || len(want) != sha256.Size {
			return fmt.Errorf("invalid worker sha256")
		}
		// OpenRoot prevents symlink escapes as well as lexical path traversal.
		root, err := os.OpenRoot(filepath.Dir(*input))
		if err != nil {
			return err
		}
		f, err := root.Open(w.File)
		_ = root.Close()
		if err != nil {
			return err
		}
		data, err := io.ReadAll(io.LimitReader(f, 64*1024*1024+1))
		_ = f.Close()
		if err != nil {
			return err
		}
		if len(data) > 64*1024*1024 {
			return fmt.Errorf("worker file exceeds 64 MiB")
		}
		totalBytes += len(data)
		if totalBytes > 128*1024*1024 {
			return fmt.Errorf("worker files exceed 128 MiB total")
		}
		got := sha256.Sum256(data)
		if !bytes.Equal(got[:], want) {
			return fmt.Errorf("worker sha256 mismatch: %s", w.File)
		}
		r, err := llmperf.ReadRun(bytes.NewReader(data))
		if err != nil {
			return fmt.Errorf("worker %s: %w", w.File, err)
		}
		totalRecords += len(r.Records)
		if totalRecords > 100000 {
			return fmt.Errorf("merged record budget exhausted")
		}
		if w.Clock.Offset == nil || w.Clock.Uncertainty == nil {
			return fmt.Errorf("worker clock fields required")
		}
		workers = append(workers, llmperf.WorkerRun{Run: r, ClockOffset: *w.Clock.Offset, ClockUncertainty: w.Clock.Uncertainty, ClockBasis: w.Clock.Basis})
	}
	merged, err := llmperf.MergeRuns(workers, llmperf.MergeOptions{ExpectedWorkers: m.ExpectedWorkers, AllowPartial: *partial, MaxClockUncertainty: m.MaxClockUncertainty})
	if err != nil {
		return err
	}
	return json.NewEncoder(out).Encode(merged)
}
