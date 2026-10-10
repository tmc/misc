package llmperf

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"go.k6.io/k6/v2/js/modulestest"
	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/metrics"
)

func completionFixture(t *testing.T, observe ...func(CompletionRequest)) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(observe) > 0 {
			var req CompletionRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "invalid JSON", 400)
				return
			}
			observe[0](req)
		}
		if r.Header.Get("Accept") == "text/event-stream" {
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\ndata: {\"choices\":[],\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":1,\"total_tokens\":2}}\n\ndata: [DONE]\n\n")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"fixture","model":"fixture","choices":[{"message":{"role":"assistant","content":"hello"}}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
	}))
	t.Cleanup(server.Close)
	return server
}

func TestJavaScriptContract(t *testing.T) {
	server := completionFixture(t)
	rt := modulestest.NewRuntime(t)
	if err := rt.SetupModuleSystem(map[string]any{importPath: new(rootModule)}, nil, nil); err != nil {
		t.Fatal(err)
	}
	_, err := rt.RunOnEventLoop(fmt.Sprintf(`
 const mod = require(%q);
 const client = new mod.Client({baseURL: %q, model: "fixture", warmupCount: 1, timeout: "2s"});
 let rejected = false;
 try { new mod.Client({unknownOption: 1}); } catch (e) { rejected = true; }
 if (!rejected) throw new Error("unknown option accepted");
 `, importPath, server.URL))
	if err != nil {
		t.Fatal(err)
	}
	registry := rt.VU.InitEnvField.Registry
	samples := make(chan metrics.SampleContainer, 200)
	tags := registry.RootTagSet().With("fixture", "eventloop")
	rt.MoveToVUContext(&lib.State{Samples: samples, Tags: lib.NewVUStateTags(tags), BuiltinMetrics: rt.BuiltinMetrics})
	_, err = rt.RunOnEventLoop(`
 for (const stream of [false, true]) {
  const response = client.chat.completions.create({messages: [{role: "user", content: "hello"}], max_tokens: 8, stream});
  if (response.status !== 200 || response.choices[0].message.content !== "hello" || response.usage.completion_tokens !== 1) throw new Error("response contract failed");
 }
 const bad = client.chat.completions.create({unknownRequestOption: 1});
 if (!(bad instanceof Error)) throw new Error("invalid request did not return Error");
 `)
	if err != nil {
		t.Fatal(err)
	}
	values := metricValues(samples)
	if len(values["llm_requests"]) != 1 {
		t.Fatalf("warmup option did not suppress first request: %v", values["llm_requests"])
	}
}

func TestMetricCancellationAndTags(t *testing.T) {
	rt := modulestest.NewRuntime(t)
	m := testMetrics(t, rt.VU.InitEnvField.Registry)
	m.vu = rt.VU
	m.RecordMetric("llm_requests", 1) // Init context has no samples.
	tags := m.registry.RootTagSet().With("fixture", "tagged")
	samples := make(chan metrics.SampleContainer, 1)
	rt.MoveToVUContext(&lib.State{Samples: samples, Tags: lib.NewVUStateTags(tags)})
	m.RecordMetric("llm_requests", 1)
	sample := (<-samples).GetSamples()[0]
	if sample.Tags != tags {
		t.Fatal("VU tags lost")
	}
	rt.CancelContext()
	rt.VU.StateField.Samples = make(chan metrics.SampleContainer)
	done := make(chan struct{})
	go func() { m.RecordMetric("llm_requests", 1); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("metric emission blocked after cancellation")
	}
}

func TestBinarySmoke(t *testing.T) {
	binary := os.Getenv("K6_SMOKE_BINARY")
	if binary == "" {
		t.Skip("set K6_SMOKE_BINARY to test the built extension")
	}
	server := completionFixture(t)
	binary, err := filepath.Abs(binary)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "run", "--no-color", "examples/offline-smoke.js")
	cmd.Env = append(os.Environ(), "ENDPOINT_URL="+server.URL, "K6_NO_USAGE_REPORT=true", "K6_AUTO_EXTENSION_RESOLUTION=false")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("k6 smoke: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "llm_requests") {
		t.Fatalf("extension metrics missing:\n%s", output)
	}
	t.Log(string(output))
}

func TestBinaryTraceReplay(t *testing.T) {
	binary := os.Getenv("K6_SMOKE_BINARY")
	if binary == "" {
		t.Skip("set K6_SMOKE_BINARY to test trace replay")
	}
	var mu sync.Mutex
	seen := make(map[string]int)
	server := completionFixture(t, func(req CompletionRequest) {
		mu.Lock()
		defer mu.Unlock()
		if len(req.Messages) == 1 {
			seen[messageText(req.Messages[0].Content)]++
		}
	})
	rows := make([]map[string]any, 7)
	for i := range rows {
		rows[i] = map[string]any{"prompt": fmt.Sprintf("row-%d", i)}
	}
	data, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	trace := filepath.Join(t.TempDir(), "trace.json")
	if err := os.WriteFile(trace, data, 0600); err != nil {
		t.Fatal(err)
	}
	binary, err = filepath.Abs(binary)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "run", "--no-color", "examples/trace-replay.js")
	cmd.Env = append(os.Environ(), "ENDPOINT_URL="+server.URL, "TRACE_FILE="+trace, "VUS=3", "MAX_TTFT=0", "MAX_TTFO=0", "MAX_TOKEN_LATENCY=0", "K6_NO_USAGE_REPORT=true", "K6_AUTO_EXTENSION_RESOLUTION=false")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("trace replay: %v\n%s", err, output)
	}
	mu.Lock()
	defer mu.Unlock()
	for i := range rows {
		prompt := fmt.Sprintf("row-%d", i)
		if seen[prompt] != 1 {
			t.Fatalf("%s replayed %d times; want 1", prompt, seen[prompt])
		}
	}
	if len(seen) != len(rows) {
		t.Fatalf("replayed %d unique rows; want %d", len(seen), len(rows))
	}
}

func TestBinaryMeasurement(t *testing.T) {
	binary := os.Getenv("K6_SMOKE_BINARY")
	if binary == "" {
		t.Skip("set K6_SMOKE_BINARY to test measurement exports")
	}
	server := completionFixture(t)
	binary, err := filepath.Abs(binary)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "run.json")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "run", "--no-color", "examples/measurement-smoke.js")
	cmd.Env = append(os.Environ(), "ENDPOINT_URL="+server.URL, "RUN_FILE="+file, "K6_NO_USAGE_REPORT=true", "K6_AUTO_EXTENSION_RESOLUTION=false")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("measurement: %v\n%s", err, output)
	}
	f, err := os.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	run, err := ReadRun(f)
	if err != nil {
		t.Fatal(err)
	}
	if run.Attempts != 6 || run.Successes != 6 || run.Timeline == nil {
		t.Fatalf("run=%+v", run)
	}
	if _, err := TimeSlices(run, 1); err != nil {
		t.Fatal(err)
	}
}

func TestBinaryWorkload(t *testing.T) {
	binary := os.Getenv("K6_SMOKE_BINARY")
	if binary == "" {
		t.Skip("set K6_SMOKE_BINARY to test workload adapters")
	}
	var mu sync.Mutex
	seen := map[string]int{}
	server := completionFixture(t, func(req CompletionRequest) {
		mu.Lock()
		defer mu.Unlock()
		seen[messageText(req.Messages[0].Content)]++
	})
	binary, err := filepath.Abs(binary)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "run", "--no-color", "examples/workload-smoke.js")
	cmd.Env = append(os.Environ(), "ENDPOINT_URL="+server.URL, "K6_NO_USAGE_REPORT=true", "K6_AUTO_EXTENSION_RESOLUTION=false")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("workload: %v\n%s", err, output)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(seen) != 7 {
		t.Fatalf("rows=%v", seen)
	}
	for i := 0; i < 7; i++ {
		if seen[fmt.Sprintf("row-%d", i)] != 1 {
			t.Fatalf("duplicate or missing rows: %v", seen)
		}
	}
}

func TestBinaryPhases(t *testing.T) {
	binary := os.Getenv("K6_SMOKE_BINARY")
	if binary == "" {
		t.Skip("set K6_SMOKE_BINARY to test explicit phases")
	}
	server := completionFixture(t)
	binary, err := filepath.Abs(binary)
	if err != nil {
		t.Fatal(err)
	}
	script, err := filepath.Abs("examples/phased-load-test.js")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "outputs"), 0700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "run", "--no-color", script)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "ENDPOINT_URL="+server.URL, "WARMUP_SECONDS=1", "MEASUREMENT_SECONDS=1", "COOLDOWN_SECONDS=1", "K6_NO_USAGE_REPORT=true", "K6_AUTO_EXTENSION_RESOLUTION=false")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("phases: %v\n%s", err, output)
	}
	f, err := os.Open(filepath.Join(dir, "outputs/run.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	run, err := ReadRun(f)
	if err != nil {
		t.Fatal(err)
	}
	if run.Attempts == 0 || run.Failures != 0 || run.Duration != 1 {
		t.Fatalf("run=%+v", run)
	}
	for _, m := range run.Records {
		if m.Phase != "measurement" || m.HTTPTrace == nil || m.HTTPTrace.Waiting == nil {
			t.Fatalf("record=%+v", m)
		}
	}
}
