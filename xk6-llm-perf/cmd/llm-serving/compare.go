package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/tmc/misc/xk6-llm-perf/serving"
)

// runCompare analyzes a retained preregistered independent-pair experiment.
func runCompare(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("compare", flag.ContinueOnError)
	flags.SetOutput(stderr)
	input := flags.String("input", "", "retained paired experiment JSON")
	seed := flags.Int64("seed", 1, "bootstrap random seed")
	samples := flags.Int("resamples", 10000, "paired bootstrap samples")
	confidence := flags.Float64("confidence", .95, "family confidence level")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *input == "" || flags.NArg() != 0 {
		fmt.Fprintln(stderr, "compare requires -input and no positional arguments")
		return 2
	}
	file, err := os.Open(*input)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	defer file.Close()
	e, err := serving.ReadPairedExperiment(file)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	result, err := serving.AnalyzePaired(e, serving.PairedOptions{Seed: *seed, Resamples: *samples, Confidence: *confidence})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
