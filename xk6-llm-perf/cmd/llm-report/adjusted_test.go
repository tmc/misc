package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestAdjustedLatenciesCommand(t *testing.T) {
	directory := t.TempDir()
	binary := filepath.Join(directory, "llm-report")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, output)
	}
	data := []byte(`{"schema_version":1,"started_at":0,"finished_at":4000,"duration":4,"attempts":3,"successes":2,"failures":1,"input_tokens":4,"output_tokens":8,"records":[{"schema_version":1,"request_id":"0","phase":"measurement","attempted":true,"recorded":true,"dispatched_at":1000,"finished_at":1100,"full_response_latency":100,"last_content_latency":80,"outcome":"success","input_tokens":{"value":2,"source":"reported"},"output_tokens":{"value":4,"source":"reported"}},{"schema_version":1,"request_id":"1","phase":"measurement","attempted":true,"recorded":true,"dispatched_at":1000,"finished_at":1300,"full_response_latency":300,"last_content_latency":250,"outcome":"success","input_tokens":{"value":2,"source":"reported"},"output_tokens":{"value":4,"source":"reported"}},{"schema_version":1,"request_id":"2","phase":"measurement","attempted":true,"recorded":true,"dispatched_at":1000,"finished_at":1900,"full_response_latency":900,"outcome":"timeout","input_tokens":{"value":2,"source":"reported"},"output_tokens":{"value":4,"source":"reported"}}]}`)
	input := filepath.Join(directory, "run.json")
	invoke := func(data []byte, adjusted bool) (map[string]any, []byte, error) {
		t.Helper()
		if err := os.WriteFile(input, data, 0600); err != nil {
			t.Fatal(err)
		}
		args := []string{"-input", input}
		if adjusted {
			args = append(args, "-aiperf-adjusted-latencies")
		}
		command := exec.CommandContext(ctx, binary, args...)
		var out, stderr bytes.Buffer
		command.Stdout = &out
		command.Stderr = &stderr
		err := command.Run()
		if err != nil {
			return nil, append(out.Bytes(), stderr.Bytes()...), err
		}
		if !json.Valid(out.Bytes()) {
			t.Fatalf("invalid JSON: %s", out.Bytes())
		}
		var result map[string]any
		if err := json.Unmarshal(out.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result, out.Bytes(), nil
	}
	ordinary, _, err := invoke(data, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, present := ordinary["aiperf_adjusted_latencies"]; present {
		t.Fatal("default output changed")
	}
	adjusted, _, err := invoke(data, true)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ordinary["request_distributions"], adjusted["request_distributions"]) {
		t.Fatal("success-only distributions changed")
	}
	report := adjusted["aiperf_adjusted_latencies"].(map[string]any)
	metric := report["metrics"].(map[string]any)["adj_request_latency"].(map[string]any)
	if metric["available"] != true || metric["count"] != float64(3) || metric["mean"].(map[string]any)["positive_infinity"] != true {
		t.Fatalf("metric=%+v", metric)
	}
	if metric["mean"].(map[string]any)["value"] != nil || report["percentile_method"] != "numpy-nearest-round-even-v1" {
		t.Fatal("infinity or estimator lost")
	}
	var source map[string]any
	if err := json.Unmarshal(data, &source); err != nil {
		t.Fatal(err)
	}
	records := source["records"].([]any)
	source["records"] = records[:2]
	source["attempts"] = 2
	source["failures"] = 0
	noErrors, _ := json.Marshal(source)
	result, _, err := invoke(noErrors, true)
	if err != nil {
		t.Fatal(err)
	}
	noAdjustment := result["aiperf_adjusted_latencies"].(map[string]any)["metrics"].(map[string]any)["adj_request_latency"].(map[string]any)
	if noAdjustment["available"] != false || noAdjustment["unavailable"] == "" {
		t.Fatal("no-error adjustment fabricated")
	}
	source["attempts"] = 3
	source["failures"] = 1
	source["dropped_records"] = 1
	dropped, _ := json.Marshal(source)
	_, output, err := invoke(dropped, true)
	if err == nil || !strings.Contains(string(output), "timeline requires all attempted request records") || bytes.HasPrefix(output, []byte("{")) {
		t.Fatalf("dropped report not refused: %s %v", output, err)
	}
}
