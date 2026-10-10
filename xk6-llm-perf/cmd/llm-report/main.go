// Command llm-report derives concurrency and time slices from a retained run.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	llmperf "github.com/tmc/misc/xk6-llm-perf"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "llm-report:", err)
		os.Exit(1)
	}
}

func run(args []string, out io.Writer) error {
	flags := flag.NewFlagSet("llm-report", flag.ContinueOnError)
	input := flags.String("input", "", "versioned run JSON file")
	records := flags.String("records", "", "optional retained JSONL file to reconcile against run")
	width := flags.Duration("slice", 10*time.Second, "time slice width")
	telemetry := flags.String("telemetry", "", "optional benchmark telemetry-window JSON")
	maxGap := flags.Duration("max-telemetry-gap", time.Second, "maximum admitted telemetry sample gap")
	clockUncertainty := flags.Float64("max-clock-uncertainty", 0, "maximum admitted clock uncertainty in milliseconds")
	bootstrap := flags.Int("bootstrap", 0, "optional latency-mean bootstrap replicates (100..10000)")
	seed := flags.Uint64("seed", 0, "bootstrap seed")
	level := flags.Float64("confidence", .95, "bootstrap confidence level")
	independent := flags.Bool("independent-observations", false, "acknowledge independent identically distributed request samples")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *input == "" || flags.NArg() != 0 {
		return fmt.Errorf("usage: llm-report -input outputs/run.json [-slice 10s]")
	}
	f, err := os.Open(*input)
	if err != nil {
		return err
	}
	defer f.Close()
	summary, err := llmperf.ReadRun(f)
	if err != nil {
		return err
	}
	if *records != "" {
		f, err := os.Open(*records)
		if err != nil {
			return err
		}
		rows, err := llmperf.ReadRecords(f)
		f.Close()
		if err != nil {
			return err
		}
		if err := llmperf.ReconcileRecords(summary, rows); err != nil {
			return err
		}
	}
	distributions, err := llmperf.AnalyzeRequests(summary)
	if err != nil {
		return err
	}
	timeline, err := llmperf.AnalyzeTimeline(summary)
	if err != nil {
		return err
	}
	slices, err := llmperf.TimeSlices(summary, width.Seconds())
	if err != nil {
		return err
	}
	var energy *llmperf.EnergySummary
	var confidence *llmperf.Confidence
	if *bootstrap != 0 {
		result, err := llmperf.LatencyConfidence(summary, llmperf.ConfidenceOptions{Seed: *seed, Replicates: *bootstrap, Level: *level, Independent: *independent})
		if err != nil {
			return err
		}
		confidence = &result
	}
	if *telemetry != "" {
		f, err := os.Open(*telemetry)
		if err != nil {
			return err
		}
		window, err := llmperf.ReadTelemetryWindow(f)
		f.Close()
		if err != nil {
			return err
		}
		result, err := llmperf.AnalyzeEnergy(summary, window, llmperf.EnergyOptions{MaxGap: maxGap.Seconds(), MaxClockUncertainty: *clockUncertainty})
		if err != nil {
			return err
		}
		energy = &result
	}
	return json.NewEncoder(out).Encode(struct {
		SchemaVersion int                     `json:"schema_version"`
		Distributions llmperf.RequestAnalysis `json:"request_distributions"`
		Timeline      llmperf.Timeline        `json:"timeline"`
		Slices        []llmperf.SliceSummary  `json:"slices"`
		Energy        *llmperf.EnergySummary  `json:"energy,omitempty"`
		Confidence    *llmperf.Confidence     `json:"latency_mean_confidence,omitempty"`
	}{1, distributions, timeline, slices, energy, confidence})
}
