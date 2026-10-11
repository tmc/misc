package output

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	llm "github.com/tmc/misc/xk6-llm-perf"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestK6LiveJSONLConsumer(t *testing.T) {
	k6 := os.Getenv("K6_SMOKE_BINARY")
	if k6 == "" {
		k6 = os.Getenv("K6_BINARY")
	}
	if k6 == "" {
		t.Skip("set K6_BINARY for signed extension integration")
	}
	k6, err := filepath.Abs(k6)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	fifo := filepath.Join(dir, "live.jsonl")
	if out, err := exec.Command("mkfifo", fifo).CombinedOutput(); err != nil {
		t.Fatalf("mkfifo: %v %s", err, out)
	}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"choices":[{"index":0,"message":{"content":"A"}}],"usage":{"prompt_tokens":0,"completion_tokens":1,"total_tokens":1}}`)
	}))
	defer provider.Close()
	accepted := 0
	collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { accepted++; fmt.Fprint(w, "{}") }))
	defer collector.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		f, err := os.Open(fifo)
		if err != nil {
			done <- err
			return
		}
		defer f.Close()
		receipt, err := Stream(ctx, f, collector.Client(), StreamOptions{Format: "otlp", Endpoint: collector.URL + "/v1/metrics", RunID: "k6-fixture"})
		if err == nil && (receipt.RecordsAccepted != 3 || !receipt.Complete) {
			err = fmt.Errorf("bad receipt %+v", receipt)
		}
		done <- err
	}()
	cmd := exec.CommandContext(ctx, k6, "run", "--quiet", "--log-format", "raw", "--console-output", fifo, "../examples/output/live-load.js")
	cmd.Env = append(os.Environ(), "ENDPOINT_URL="+provider.URL)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("k6: %v %s", err, out)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("consumer timeout")
	}
	if accepted != 3 {
		t.Fatal("accepted", accepted)
	}
}

func TestK6TrailingUsageExport(t *testing.T) {
	k6 := os.Getenv("K6_SMOKE_BINARY")
	if k6 == "" {
		k6 = os.Getenv("K6_BINARY")
	}
	if k6 == "" {
		t.Skip("set K6_BINARY for signed extension integration")
	}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		f := w.(http.Flusher)
		fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"A\"},\"finish_reason\":null}]}\n\n")
		f.Flush()
		time.Sleep(30 * time.Millisecond)
		fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"B\"},\"finish_reason\":\"stop\"}]}\n\n")
		f.Flush()
		time.Sleep(120 * time.Millisecond)
		fmt.Fprint(w, "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":0,\"completion_tokens\":2,\"total_tokens\":2}}\n\ndata: [DONE]\n\n")
		f.Flush()
	}))
	defer provider.Close()
	dir := t.TempDir()
	script := filepath.Join(dir, "load.js")
	record := filepath.Join(dir, "record.jsonl")
	source := `import llm from 'k6/x/llm-perf';
 export const options = { vus: 1, iterations: 1 };
 const client = new llm.Client({baseURL: __ENV.ENDPOINT_URL, model: 'fixture', strictMeasurement: true});
 export default function() { const r = client.chat.completions.create({messages:[{role:'user',content:'fixture'}],stream:true}); console.log(JSON.stringify(r.measurement)); }
 export function handleSummary() { return {}; }`
	if err := os.WriteFile(script, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, k6, "run", "--quiet", "--log-format", "raw", "--console-output", record, script)
	cmd.Env = append(os.Environ(), "ENDPOINT_URL="+provider.URL)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("k6: %v %s", err, out)
	}
	data, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	var m llm.Measurement
	if err := json.Unmarshal(bytes.TrimSpace(data), &m); err != nil {
		t.Fatalf("record: %v %s", err, data)
	}
	if m.LastContentLatency == nil || m.FullResponseLatency-*m.LastContentLatency < 90 {
		t.Fatalf("delayed trailer boundary lost: %+v", m)
	}
	run := llm.RunSummary{SchemaVersion: 1, StartedAt: m.StartedAt, FinishedAt: m.FinishedAt + 1, Duration: (m.FinishedAt + 1 - m.StartedAt) / 1000, Attempts: 1, Successes: 1, Records: []llm.Measurement{m}, OutputTokens: 2}
	if m.Good {
		run.GoodRequests = 1
	}
	run.Goodput = float64(run.GoodRequests) / run.Duration
	run.OutputThroughput = 2 / run.Duration
	d, err := Summarize(run)
	if err != nil {
		t.Fatal(err)
	}
	if *named(d, "full_response_latency").Mean-*named(d, "last_content_latency").Mean < 90 {
		t.Fatal("export endpoint drift")
	}
}
