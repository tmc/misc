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
	format := flag.String("input-format", "llm-telemetry-v1", "input schema: llm-telemetry-v1, nvtelemetry-v1 or nvtelemetry-v2")
	start := flag.Float64("start", 0, "window start in Unix milliseconds")
	end := flag.Float64("end", 0, "window end in Unix milliseconds")
	gap := flag.Float64("max-gap", 0, "maximum adjacent sample gap in seconds")
	mapping := flag.String("mapping", "", "hash-bound native admission mapping JSON")
	uncertainty := flag.Float64("max-clock-uncertainty", 0, "maximum clock uncertainty in milliseconds")
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
	if *format == "nvtelemetry-v1" || *format == "nvtelemetry-v2" {
		if (*start != 0 || *end != 0 || *gap != 0 || *uncertainty != 0) && *mapping == "" {
			fail(fmt.Errorf("native window join unavailable: sensor time and clock mapping are unknown or unqualified"))
		}
		raw, err := llmperf.ReadNativeTelemetry(f)
		if err != nil {
			fail(err)
		}
		wantVersion := 1
		if *format == "nvtelemetry-v2" {
			wantVersion = 2
		}
		if raw.Header.SchemaVersion != wantVersion {
			fail(fmt.Errorf("native schema version %d does not match input format %q", raw.Header.SchemaVersion, *format))
		}
		if *mapping != "" {
			proof, err := os.Open(*mapping)
			if err != nil {
				fail(err)
			}
			defer proof.Close()
			m, err := llmperf.ReadNativeTelemetryMapping(proof)
			if err != nil {
				fail(err)
			}
			window, fields, err := llmperf.ReplayNativeTelemetry(raw, m, *start, *end, llmperf.EnergyOptions{MaxGap: *gap, MaxClockUncertainty: *uncertainty})
			if err != nil {
				fail(err)
			}
			if err = json.NewEncoder(os.Stdout).Encode(struct {
				Window llmperf.TelemetryWindow         `json:"window"`
				Fields []llmperf.TelemetryFieldSummary `json:"fields"`
			}{window, fields}); err != nil {
				fail(err)
			}
			return
		}
		err = json.NewEncoder(os.Stdout).Encode(struct {
			SourceHash  string                           `json:"source_hash"`
			Header      llmperf.NativeTelemetryManifest  `json:"header"`
			Final       *llmperf.NativeTelemetryManifest `json:"final"`
			Samples     int                              `json:"samples"`
			Phases      []llmperf.NativeTelemetryPhase   `json:"phases"`
			Diagnostics []string                         `json:"diagnostics"`
			Admission   string                           `json:"benchmark_admission"`
		}{raw.SourceHash, raw.Header, raw.Final, len(raw.Samples), raw.Phases, raw.Diagnostics, "unavailable: sensor timing, clock mapping and entity scope require qualified evidence"})
		if err != nil {
			fail(err)
		}
		return
	}
	if *mapping != "" {
		fail(fmt.Errorf("native mapping requires native input format"))
	}
	if *format != "llm-telemetry-v1" {
		fail(fmt.Errorf("unsupported input format %q", *format))
	}
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
