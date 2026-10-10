// llm-campaign runs bounded and resumable k6 experiments.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/tmc/misc/xk6-llm-perf/campaign"
)

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, "llm-campaign:", e)
		os.Exit(1)
	}
}
func run() error {
	config := flag.String("config", "", "campaign JSON configuration")
	directory := flag.String("output", "", "private campaign artifact directory (also checkpoint)")
	scale := flag.Bool("scale", false, "run single-run global admission scaling")
	validate := flag.Bool("validate", false, "validate configuration without executing k6")
	flag.Parse()
	if *config == "" {
		return fmt.Errorf("-config is required")
	}
	f, e := os.Open(*config)
	if e != nil {
		return e
	}
	defer f.Close()
	dec := json.NewDecoder(io.LimitReader(f, 1<<20))
	dec.DisallowUnknownFields()
	var c campaign.Config
	var sc campaign.ScaleConfig
	var value any = &c
	if *scale {
		value = &sc
	}
	if e = dec.Decode(value); e != nil {
		return e
	}
	var extra any
	if e = dec.Decode(&extra); e != io.EOF {
		return fmt.Errorf("configuration has trailing data")
	}
	if *scale {
		e = sc.Validate()
	} else {
		e = c.Validate()
	}
	if e != nil {
		return e
	}
	if *validate {
		fmt.Println("configuration valid")
		return nil
	}
	if *directory == "" {
		return fmt.Errorf("-output is required")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if *scale {
		m, err := campaign.RunScale(ctx, sc, *directory)
		if m != nil {
			fmt.Printf("%s: %d windows, %d workers\n", m.StopReason, len(m.Windows), len(m.Workers))
		}
		return err
	}
	m, e := campaign.Run(ctx, c, *directory)
	if m != nil {
		fmt.Printf("%s: %d candidates, best=%s\n", m.StopReason, len(m.Candidates), m.BestID)
	}
	return e
}
