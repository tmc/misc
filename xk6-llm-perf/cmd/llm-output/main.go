// llm-output renders a sanitized run from stdin or posts a prepared sink export.
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

	llm "github.com/tmc/misc/xk6-llm-perf"
	"github.com/tmc/misc/xk6-llm-perf/output"
)

func main() {
	aiperf := flag.Bool("aiperf-statistics", false, "linear percentiles for retained successful requests and pooled content intervals")
	prices := flag.String("prices", "", "explicit price table file (format=cost only)")
	stream := flag.Bool("stream", false, "consume measurement JSONL with serial sink backpressure")
	streamRun := flag.String("stream-run", "", "benchmark run ID for OTLP streaming")
	input := flag.String("input", "", "run JSON file; default stdin")
	format := flag.String("format", "json", "json, csv, html, cost, otlp or mlflow")
	endpoint := flag.String("endpoint", "", "explicit HTTP sink URL (otlp/mlflow only)")
	runID := flag.String("mlflow-run", "", "existing MLflow run ID")
	authEnv := flag.String("authorization-env", "", "environment variable holding HTTP Authorization header")
	flag.Parse()
	var reader io.Reader = os.Stdin
	if *input != "" {
		f, err := os.Open(*input)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer f.Close()
		reader = f
	}
	var err error
	if *aiperf && (*stream || (*format != "json" && *format != "csv" && *format != "html")) {
		err = fmt.Errorf("aiperf-statistics requires non-streaming json, csv or html output")
	} else if *prices != "" || *format == "cost" {
		if *stream || *format != "cost" || *prices == "" || *endpoint != "" {
			err = fmt.Errorf("prices require non-streaming cost format without a sink")
		} else {
			err = runCost(reader, *prices)
		}
	} else if *stream {
		id := *streamRun
		if *format == "mlflow" {
			id = *runID
		}
		auth := ""
		if *authEnv != "" {
			var ok bool
			auth, ok = os.LookupEnv(*authEnv)
			if !ok {
				fmt.Fprintln(os.Stderr, "authorization environment variable missing")
				os.Exit(1)
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		receipt, streamErr := output.Stream(ctx, reader, &http.Client{Timeout: 30 * time.Second}, output.StreamOptions{Format: *format, Endpoint: *endpoint, RunID: id, Authorization: auth})
		err = json.NewEncoder(os.Stdout).Encode(receipt)
		if err == nil {
			err = streamErr
		}
	} else {
		err = runWithOptions(reader, *format, *endpoint, *runID, *authEnv, llm.ReportOptions{AIPerf: *aiperf})
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run(reader io.Reader, format, endpoint, runID, authEnv string) error {
	return runWithOptions(reader, format, endpoint, runID, authEnv, llm.ReportOptions{})
}

func runWithOptions(reader io.Reader, format, endpoint, runID, authEnv string, opts llm.ReportOptions) error {
	r, err := llm.ReadRun(reader)
	if err != nil {
		return err
	}
	d, err := output.SummarizeWithOptions(r, opts)
	if err != nil {
		return err
	}
	var b []byte
	switch format {
	case "json":
		b, err = json.Marshal(d)
	case "csv":
		if endpoint != "" {
			return fmt.Errorf("sink requires otlp or mlflow")
		}
		return output.CSV(os.Stdout, d)
	case "html":
		if endpoint != "" {
			return fmt.Errorf("sink requires otlp or mlflow")
		}
		return output.HTML(os.Stdout, d)
	case "otlp":
		b, err = output.OTLP(r)
	case "mlflow":
		b, err = output.MLflowBatch(d, runID, int64(r.FinishedAt))
	default:
		return fmt.Errorf("unknown output format")
	}
	if err != nil {
		return err
	}
	if endpoint != "" {
		if format != "otlp" && format != "mlflow" {
			return fmt.Errorf("sink requires otlp or mlflow")
		}
		auth := ""
		if authEnv != "" {
			var ok bool
			auth, ok = os.LookupEnv(authEnv)
			if !ok {
				return fmt.Errorf("authorization environment variable missing")
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return output.Send(ctx, &http.Client{Timeout: 30 * time.Second}, endpoint, auth, b)
	}
	_, err = os.Stdout.Write(append(b, '\n'))
	return err
}

func runCost(reader io.Reader, path string) error {
	run, err := llm.ReadRun(reader)
	if err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	prices, err := output.ReadPrices(f)
	if err != nil {
		return err
	}
	ledger, err := output.DeriveCost(run, prices)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(ledger)
}
