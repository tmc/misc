// llm-accuracy grades a pinned explicit artifact, optionally generating missing answers.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/tmc/misc/xk6-llm-perf/accuracy"
	"github.com/tmc/misc/xk6-llm-perf/internal/jsoncheck"
)

func main() {
	worker := flag.String("worker", "", "explicit confined Python worker script")
	workerHash := flag.String("worker-sha256", "", "required worker script SHA-256")
	workerPython := flag.String("worker-python", "python3", "trusted worker launcher")
	workerSource := flag.String("aiperf-source", "", "clean pinned AIPerf source")
	workerRuntime := flag.String("worker-runtime", "", "pinned Python virtualenv root")
	workerPins := flag.String("dependency-versions", "", "JSON file mapping packages to exact versions")
	input := flag.String("input", "", "artifact file; default stdin")
	endpoint := flag.String("endpoint", "", "explicit OpenAI chat endpoint to generate missing predictions")
	requests := flag.String("requests", "", "JSON object mapping case IDs to recipe generation requests")
	prompts := flag.String("prompts", "", "JSON object mapping case IDs to independent prompts")
	authEnv := flag.String("authorization-env", "", "environment variable holding Authorization header")
	flag.Parse()
	opts := accuracy.WorkerOptions{Python: *workerPython, Script: *worker, ScriptSHA256: *workerHash, AIPerfSource: *workerSource, RuntimeRoot: *workerRuntime}
	if *worker != "" {
		f, err := os.Open(*workerPins)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		b, err := io.ReadAll(io.LimitReader(f, 65537))
		closeErr := f.Close()
		if err != nil || closeErr != nil || len(b) > 65536 || json.Unmarshal(b, &opts.DependencyVersions) != nil {
			fmt.Fprintln(os.Stderr, "invalid dependency pins")
			os.Exit(1)
		}
	}
	err := run(*input, *endpoint, *prompts, *requests, *authEnv, opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run(input, endpoint, promptFile, requestFile, authEnv string, worker accuracy.WorkerOptions) error {
	var reader io.Reader = os.Stdin
	if input != "" {
		f, err := os.Open(input) // #nosec G304 -- local CLI deliberately reads the caller-selected artifact, bounded and validated below
		if err != nil {
			return err
		}
		defer f.Close()
		reader = f
	}
	data, err := io.ReadAll(io.LimitReader(reader, 16<<20+1))
	if err != nil || len(data) > 16<<20 {
		return fmt.Errorf("artifact exceeds budget or read failed")
	}
	evaluate := func(data []byte) (accuracy.Report, error) {
		if worker.Script != "" {
			return accuracy.EvaluateWithWorker(context.Background(), bytes.NewReader(data), worker)
		}
		return accuracy.Evaluate(bytes.NewReader(data))
	}
	report, err := evaluate(data)
	if err != nil {
		return err
	}
	if endpoint != "" {
		if (promptFile == "") == (requestFile == "") {
			return fmt.Errorf("generation requires exactly one separate prompts or requests file")
		}
		path := promptFile
		if requestFile != "" {
			path = requestFile
		}
		f, err := os.Open(path) // #nosec G304 -- local CLI deliberately reads the caller-selected prompts file, bounded and decoded below
		if err != nil {
			return err
		}
		promptBytes, err := io.ReadAll(io.LimitReader(f, 4<<20+1))
		closeErr := f.Close()
		if err != nil || closeErr != nil || len(promptBytes) > 4<<20 {
			return fmt.Errorf("prompts exceed budget or read failed")
		}
		var artifact accuracy.Artifact
		if err := json.Unmarshal(data, &artifact); err != nil {
			return fmt.Errorf("decode generation artifact: %w", err)
		}
		auth := ""
		if authEnv != "" {
			var ok bool
			auth, ok = os.LookupEnv(authEnv)
			if !ok {
				return fmt.Errorf("authorization environment variable missing")
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		var generated accuracy.Artifact
		if requestFile != "" {
			if err := jsoncheck.Check(json.NewDecoder(bytes.NewReader(promptBytes)), jsoncheck.Options{FoldNames: true, RejectNull: true}); err != nil {
				return fmt.Errorf("decode requests: %w", err)
			}
			var requests map[string]accuracy.GenerationRequest
			decoder := json.NewDecoder(bytes.NewReader(promptBytes))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&requests); err != nil {
				return fmt.Errorf("decode requests: %w", err)
			}
			if err := decoder.Decode(new(any)); err != io.EOF {
				return fmt.Errorf("trailing requests data")
			}
			generated, err = accuracy.GenerateRequests(ctx, &http.Client{Timeout: 30 * time.Second}, endpoint, auth, artifact, requests)
		} else {
			var prompts map[string]string
			if err := json.Unmarshal(promptBytes, &prompts); err != nil {
				return fmt.Errorf("decode prompts: %w", err)
			}
			generated, err = accuracy.Generate(ctx, &http.Client{Timeout: 30 * time.Second}, endpoint, auth, artifact, prompts)
		}
		if err != nil {
			return err
		}
		b, err := json.Marshal(generated)
		if err != nil {
			return err
		}
		report, err = evaluate(b)
		if err != nil {
			return err
		}
	} else if promptFile != "" || requestFile != "" || authEnv != "" {
		return fmt.Errorf("generation options require endpoint")
	}
	return json.NewEncoder(os.Stdout).Encode(report)
}
