// Command llm-scrape records configured Prometheus targets or replays JSONL.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/tmc/misc/xk6-llm-perf/servermetrics"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("llm-scrape", flag.ContinueOnError)
	targets := fs.String("targets", "", "configured target-array JSON file, reloaded each tick")
	pods := fs.String("pods", "", "configured Kubernetes PodList JSON file, reloaded each tick")
	namespace := fs.String("namespace", "", "explicit namespace for -pods")
	input := fs.String("input", "", "JSONL recording to replay")
	window := fs.String("window", "", "analysis Window JSON file (max_gap in nanoseconds)")
	duration := fs.Duration("duration", time.Minute, "recording duration")
	interval := fs.Duration("interval", time.Second, "scrape cadence")
	timeout := fs.Duration("timeout", 5*time.Second, "HTTP timeout")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected arguments")
	}
	if *input != "" {
		if *window == "" || *targets != "" || *pods != "" {
			return fmt.Errorf("replay requires only -input and -window")
		}
		f, err := os.Open(*window)
		if err != nil {
			return err
		}
		defer f.Close()
		var w servermetrics.Window
		d := json.NewDecoder(io.LimitReader(f, 1024*1024))
		d.DisallowUnknownFields()
		if err := d.Decode(&w); err != nil {
			return err
		}
		if d.Decode(new(any)) != io.EOF {
			return fmt.Errorf("trailing window data")
		}
		r, err := os.Open(*input)
		if err != nil {
			return err
		}
		defer r.Close()
		a, err := servermetrics.Replay(r, w)
		if err != nil {
			return err
		}
		return json.NewEncoder(out).Encode(a)
	}
	if (*targets == "") == (*pods == "") || *timeout <= 0 || *timeout > time.Minute {
		return fmt.Errorf("choose -targets or -pods and bounded timeout")
	}
	discover := func(context.Context) ([]string, error) {
		path := *targets
		if *pods != "" {
			path = *pods
		}
		// #nosec G304 -- The CLI explicitly reads a caller-selected, bounded discovery file.
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		if *pods != "" {
			return servermetrics.DiscoverPods(f, *namespace, nil)
		}
		return servermetrics.DiscoverTargets(f)
	}
	client := &http.Client{Timeout: *timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	receipt, err := servermetrics.Collect(context.Background(), client, servermetrics.CollectOptions{Discover: discover, Interval: *interval, Duration: *duration, MaxRecords: 100000, MaxBytes: 256 * 1024 * 1024}, out)
	data, encodeErr := json.Marshal(receipt)
	if encodeErr == nil {
		fmt.Fprintln(os.Stderr, string(data))
	}
	if err != nil {
		return err
	}
	if !receipt.Complete {
		return fmt.Errorf("incomplete recording: %s", receipt.IncompleteReason)
	}
	return encodeErr
}
