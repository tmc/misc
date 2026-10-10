// Command llm-telemetry replays a local versioned GPU adapter stream.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	llmperf "github.com/tmc/misc/xk6-llm-perf"
)

func main() {
	input := flag.String("input", "", "raw adapter JSONL file")
	start := flag.Float64("start", 0, "window start in Unix milliseconds")
	end := flag.Float64("end", 0, "window end in Unix milliseconds")
	gap := flag.Float64("max-gap", 0, "maximum adjacent sample gap in seconds")
	flag.Parse()
	if *input == "" || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "input is required")
		os.Exit(2)
	}
	f, err := os.Open(*input)
	if err != nil {
		fail(err)
	}
	defer f.Close()
	raw, err := llmperf.ReadTelemetryReplay(f)
	if err != nil {
		fail(err)
	}
	window, fields, err := llmperf.ReplayTelemetry(raw, *start, *end, llmperf.EnergyOptions{MaxGap: *gap})
	if err != nil {
		fail(err)
	}
	err = json.NewEncoder(os.Stdout).Encode(struct {
		Window llmperf.TelemetryWindow         `json:"window"`
		Fields []llmperf.TelemetryFieldSummary `json:"fields"`
	}{window, fields})
	if err != nil {
		fail(err)
	}
}
func fail(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
