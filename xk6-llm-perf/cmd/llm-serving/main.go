package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/tmc/misc/xk6-llm-perf/internal/jsoncheck"
	"github.com/tmc/misc/xk6-llm-perf/serving"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "llm-serving:", err)
		os.Exit(1)
	}
}
func run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: llm-serving lifecycle|capacity|paired|compare")
	}
	switch args[0] {
	case "lifecycle":
		return runLifecycle(args[1:])
	case "capacity":
		return runCapacity(args[1:])
	case "compare":
		if runCompare(args[1:], os.Stdout, os.Stderr) != 0 {
			return errors.New("paired analysis failed")
		}
		return nil
	case "paired":
		if runPaired(args[1:], os.Stdout, os.Stderr) != 0 {
			return errors.New("paired execution failed")
		}
		return nil
	default:
		return errors.New("unknown serving subcommand")
	}
}
func runLifecycle(args []string) error {
	fs := flag.NewFlagSet("lifecycle", flag.ContinueOnError)
	config := fs.String("config", "", "lifecycle configuration JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *config == "" || fs.NArg() != 0 {
		return errors.New("lifecycle requires -config")
	}
	f, err := os.Open(*config)
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, (1<<20)+1))
	if err != nil {
		return err
	}
	if len(b) > 1<<20 {
		return errors.New("lifecycle config exceeds limit")
	}
	if err = uniqueJSON(json.NewDecoder(bytes.NewReader(b))); err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var c serving.LifecycleConfig
	if err = dec.Decode(&c); err != nil {
		return err
	}
	var extra any
	if err = dec.Decode(&extra); err != io.EOF {
		return errors.New("trailing configuration data")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	receipt, runErr := serving.Run(ctx, c)
	if err = json.NewEncoder(os.Stdout).Encode(receipt); err != nil {
		return err
	}
	return runErr
}

// uniqueJSON refuses duplicate keys and nulls before typed decoding.
func uniqueJSON(d *json.Decoder) error {
	return jsoncheck.Value(d, jsoncheck.Options{RejectNull: true})
}
