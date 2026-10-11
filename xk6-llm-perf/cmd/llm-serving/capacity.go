package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tmc/misc/xk6-llm-perf/serving"
)

func runCapacity(args []string) error {
	flags := flag.NewFlagSet("capacity", flag.ContinueOnError)
	pid := flags.Int("pid", 0, "Linux client PID to sample instead of evaluating a cell")
	interval := flags.Duration("interval", 100*time.Millisecond, "resource sampling cadence")
	samples := flags.Int("samples", 100, "bounded resource sample count")
	input := flags.String("input", "", "capacity cell and preregistered bounds JSON")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *pid != 0 {
		if *input != "" || flags.NArg() != 0 {
			return fmt.Errorf("capacity: pid and input are mutually exclusive")
		}
		if *interval < time.Millisecond || *interval > time.Second || *samples < 2 || *samples > 10000 || time.Duration(*samples)**interval > time.Minute {
			return fmt.Errorf("capacity: sampling budget exceeds one minute")
		}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		observations, err := serving.SampleProcessResources(ctx, *pid, *interval, *samples)
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(observations)
	}
	if *input == "" || flags.NArg() != 0 {
		return fmt.Errorf("capacity: -input is required")
	}
	f, err := os.Open(*input)
	if err != nil {
		return err
	}
	defer f.Close()
	var cell struct {
		Evidence serving.CapacityEvidence `json:"evidence"`
		Bounds   serving.CapacityBounds   `json:"bounds"`
	}
	data, err := io.ReadAll(io.LimitReader(f, (1<<20)+1))
	if err != nil {
		return err
	}
	if len(data) > 1<<20 {
		return fmt.Errorf("capacity: input exceeds byte limit")
	}
	if err = serving.ValidateCapacityJSON(data); err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err = d.Decode(&cell); err != nil {
		return fmt.Errorf("capacity: decode cell: %w", err)
	}
	var extra any
	if err = d.Decode(&extra); err != io.EOF {
		return fmt.Errorf("capacity: trailing JSON")
	}
	result, err := serving.QualifyCapacity(cell.Evidence, cell.Bounds)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(result)
}
