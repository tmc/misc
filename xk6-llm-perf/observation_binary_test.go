package llmperf

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestBinaryObservationContentWindow(t *testing.T) {
	binary := os.Getenv("K6_SMOKE_BINARY")
	if binary == "" {
		t.Skip("set K6_SMOKE_BINARY")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/error" {
			http.Error(w, "provider failure", 500)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		w.(http.Flusher).Flush()
		time.Sleep(20 * time.Millisecond)
		fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"x\"}}]}\n\n")
		w.(http.Flusher).Flush()
		time.Sleep(20 * time.Millisecond)
		fmt.Fprint(w, "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":2,\"total_tokens\":3}}\n\ndata: [DONE]\n\n")
	}))
	defer server.Close()
	dir := t.TempDir()
	summaryPath := filepath.Join(dir, "summary.json")
	runPath := filepath.Join(dir, "run.json")
	script := fmt.Sprintf(`import llm from 'k6/x/llm-perf';import {sleep,check} from 'k6';
export const options={vus:1,iterations:1,thresholds:{checks:['rate==1']}};
export default function(){
 const c=new llm.Client({baseURL:%q,strictMeasurement:true,warmupCount:1,maxTTFT:5000,maxTPOT:.25,maxE2E:5000});
 const request=path=>c.request({path,protocol:'openai-chat',body:{messages:[{role:'user',content:'x'}],stream:true}});
 sleep(.1);request('/warmup');
 llm.beginMeasurement({runId:'observation-window',maxRecords:8});
 sleep(.1);request('/good');request('/error');sleep(.1);
 const run=llm.endMeasurement();
 check(run,{'observed window':r=>r.observation.policy==='single-process-successful-content-span-v1'&&r.observation.attempts===2&&r.observation.good_requests===1&&r.observation.duration<r.duration/2&&r.observation.goodput>r.goodput*2});
 sleep(.1);
}
export function handleSummary(data){return {[%q]:JSON.stringify(data),[%q]:JSON.stringify(llm.measurementSummary())};}
`, server.URL, summaryPath, runPath)
	scriptPath := filepath.Join(dir, "observation.js")
	if err := os.WriteFile(scriptPath, []byte(script), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary, "run", "--no-color", scriptPath)
	command.Env = append(os.Environ(), "K6_NO_USAGE_REPORT=true", "K6_AUTO_EXTENSION_RESOLUTION=false")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, output)
	}
	data, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatal(err)
	}
	var summary struct {
		Metrics map[string]struct {
			Values map[string]float64 `json:"values"`
		} `json:"metrics"`
	}
	if err := json.Unmarshal(data, &summary); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(runPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	run, err := ReadRun(f)
	if err != nil {
		t.Fatal(err)
	}
	observed := run.Observation
	if observed == nil {
		t.Fatal("observation receipt absent")
	}
	for name, expected := range map[string]struct {
		stat  string
		value float64
	}{
		"llm_content_span_windows": {"count", 1}, "llm_content_span_goodput": {"avg", observed.Goodput}, "llm_content_span_duration_ms": {"avg", observed.Duration * 1000}, "llm_content_span_good_requests": {"avg", 1}, "llm_content_span_attempts": {"avg", 2}, "llm_attempts": {"count", 2},
	} {
		got, ok := summary.Metrics[name].Values[expected.stat]
		if !ok || math.Abs(got-expected.value) > 1e-12*math.Max(1, math.Abs(expected.value)) {
			t.Fatalf("%s=%g present=%v want=%g", name, got, ok, expected.value)
		}
	}
	engineRate, ok := summary.Metrics["llm_good_requests"].Values["rate"]
	if !ok || engineRate >= observed.Goodput/2 {
		t.Fatal("whole engine rate falsely used for content span", engineRate, observed.Goodput)
	}
	t.Logf("observed %.9gs / %.9g requests/s; declared %.9gs / %.9g requests/s; engine %.9g requests/s", observed.Duration, observed.Goodput, run.Duration, run.Goodput, engineRate)
}

func TestBinaryObservationSharedMultiVU(t *testing.T) {
	binary := os.Getenv("K6_SMOKE_BINARY")
	if binary == "" {
		t.Skip("set K6_SMOKE_BINARY")
	}
	var arrived atomic.Int32
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if arrived.Add(1) == 2 {
			close(release)
		}
		select {
		case <-release:
		case <-time.After(3 * time.Second):
			http.Error(w, "missing concurrent request", 500)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"x\"}}]}\n\n")
		w.(http.Flusher).Flush()
		time.Sleep(10 * time.Millisecond)
		fmt.Fprint(w, "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":2,\"total_tokens\":3}}\n\ndata: [DONE]\n\n")
	}))
	defer server.Close()
	dir := t.TempDir()
	summaryPath := filepath.Join(dir, "summary.json")
	runPath := filepath.Join(dir, "run.json")
	script := fmt.Sprintf(`import llm from 'k6/x/llm-perf';import {check} from 'k6';
export const options={scenarios:{two:{executor:'per-vu-iterations',vus:2,iterations:1}},thresholds:{checks:['rate==1']}};
export function setup(){llm.beginMeasurement({runId:'shared-observation',maxRecords:8});}
export default function(){
 const c=new llm.Client({baseURL:%q,strictMeasurement:true,maxTTFT:5000,maxTPOT:.25,maxE2E:5000});
 const r=c.request({path:'/good',protocol:'openai-chat',body:{messages:[{role:'user',content:'x'}],stream:true}});
 check(r,{'measured good':r=>!(r instanceof Error)&&r.measurement.good});
}
export function teardown(){const run=llm.endMeasurement();check(run,{'shared complete window':r=>r.attempts===2&&r.good_requests===2&&r.observation.attempts===2&&r.observation.good_requests===2});}
export function handleSummary(data){return {[%q]:JSON.stringify(data),[%q]:JSON.stringify(llm.measurementSummary())};}
`, server.URL, summaryPath, runPath)
	scriptPath := filepath.Join(dir, "shared.js")
	if err := os.WriteFile(scriptPath, []byte(script), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary, "run", "--no-color", scriptPath)
	command.Env = append(os.Environ(), "K6_NO_USAGE_REPORT=true", "K6_AUTO_EXTENSION_RESOLUTION=false")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, output)
	}
	if arrived.Load() != 2 {
		t.Fatal("concurrent VU request population", arrived.Load())
	}
	f, err := os.Open(runPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	run, err := ReadRun(f)
	if err != nil {
		t.Fatal(err)
	}
	if run.Observation == nil || run.Attempts != 2 || run.Observation.GoodRequests != 2 {
		t.Fatal("shared observation unavailable")
	}
	start, end := math.Inf(1), math.Inf(-1)
	for _, m := range run.Records {
		start = math.Min(start, m.DispatchedAt)
		end = math.Max(end, m.DispatchedAt+*m.LastContentLatency)
	}
	if run.Observation.StartedAt != start || run.Observation.FinishedAt != end {
		t.Fatal("global span not reconstructed")
	}
	data, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatal(err)
	}
	var summary struct {
		Metrics map[string]struct {
			Values map[string]float64 `json:"values"`
		} `json:"metrics"`
	}
	if err := json.Unmarshal(data, &summary); err != nil {
		t.Fatal(err)
	}
	if n, ok := summary.Metrics["llm_content_span_windows"].Values["count"]; !ok || n != 1 {
		t.Fatal("teardown aggregate metric not one window", n, ok)
	}
	for name, want := range map[string]float64{"llm_content_span_attempts": 2, "llm_content_span_good_requests": 2, "llm_content_span_goodput": run.Observation.Goodput} {
		got, ok := summary.Metrics[name].Values["avg"]
		if !ok || math.Abs(got-want) > 1e-12*math.Max(1, math.Abs(want)) {
			t.Fatal("shared aggregate mismatch", name, got, ok, want)
		}
	}
	t.Logf("shared two-VU actual observation=%+v", *run.Observation)
}
