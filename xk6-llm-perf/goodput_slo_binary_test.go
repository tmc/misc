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
	"testing"
	"time"
)

func TestBinarySimultaneousGoodputSLO(t *testing.T) {
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
		if r.URL.Path == "/cancel" {
			select {
			case <-r.Context().Done():
			case <-time.After(time.Second):
			}
			return
		}
		if r.URL.Path == "/e2e" {
			time.Sleep(20 * time.Millisecond)
		}
		fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"a\"}}]}\n\n")
		w.(http.Flusher).Flush()
		if r.URL.Path == "/multi" {
			time.Sleep(20 * time.Millisecond)
			fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"b\"}}]}\n\n")
			w.(http.Flusher).Flush()
		}
		tokens := 3
		if r.URL.Path == "/one" {
			tokens = 1
		}
		if r.URL.Path != "/missing" {
			fmt.Fprintf(w, "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":%d,\"total_tokens\":%d}}\n\n", tokens, tokens+1)
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()
	runPath := filepath.Join(t.TempDir(), "run.json")
	script := fmt.Sprintf(`import llm from 'k6/x/llm-perf';
import {check} from 'k6';
export const options={vus:1,iterations:1,thresholds:{checks:['rate==1'],llm_attempts:['count==7'],llm_successes:['count==5'],llm_failures:['count==2'],llm_good_requests:['count==1'],llm_slo_compliance:['rate>0.14','rate<0.15'],llm_good_request:['rate>0.14','rate<0.15']}};
export default function(){
 llm.beginMeasurement({runId:'simultaneous-slo',maxRecords:20});
 const base={baseURL:%q,model:'fixture',strictMeasurement:true,maxTTFT:5000,maxTPOT:0.25,maxE2E:5000};
 const c=new llm.Client(base);
 for (const path of ['/good','/multi','/one','/missing','/error']) {
  const r=c.request({path,protocol:'openai-chat',body:{messages:[{role:'user',content:'x'}],stream:true}});
  const m=r.measurement;
  check(m,{[path]:m=>m.attempted&&m.good===(path==='/good')&&m.slo.max_tpot_ms===0.25&&m.slo.max_e2e_ms===5000});
 }
 const d=new llm.Client({...base,maxTPOT:0,maxE2E:0.25});
 const late=d.request({path:'/e2e',protocol:'openai-chat',body:{messages:[{role:'user',content:'x'}],stream:true}});
 check(late,{'E2E bound applied':r=>r.measurement.outcome==='success'&&!r.measurement.good&&r.measurement.slo.max_e2e_ms===0.25});
 const timed=new llm.Client({...base,timeout:'50ms'});
 const cancelled=timed.request({path:'/cancel',protocol:'openai-chat',body:{messages:[{role:'user',content:'x'}],stream:true}});
 check(cancelled,{'timeout not good':r=>r instanceof Error&&!r.measurement.good&&r.measurement.outcome==='timeout'});
 const run=llm.endMeasurement();
 check(run,{'accounting':r=>r.attempts===7&&r.successes===5&&r.failures===2&&r.good_requests===1});
}
export function handleSummary(){return {[%q]:JSON.stringify(llm.measurementSummary())};}
`, server.URL, runPath)
	scriptPath := filepath.Join(t.TempDir(), "goodput.js")
	if err := os.WriteFile(scriptPath, []byte(script), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary, "run", "--no-color", scriptPath)
	command.Env = append(os.Environ(), "K6_NO_USAGE_REPORT=true", "K6_AUTO_EXTENSION_RESOLUTION=false")
	output, err := command.CombinedOutput()
	t.Log(string(output))
	if err != nil {
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
	if run.Attempts != 7 || run.GoodRequests != 1 {
		t.Fatal("replay counts", run)
	}
	for _, m := range run.Records {
		if m.SLO.MaxTPOT == nil || m.SLO.MaxE2E == nil {
			t.Fatal("actual threshold missing")
		}
	}
}

func TestBinaryGoodputExplicitZeroCohorts(t *testing.T) {
	binary := os.Getenv("K6_SMOKE_BINARY")
	if binary == "" {
		t.Skip("set K6_SMOKE_BINARY")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/error" {
			http.Error(w, "provider failure", 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"index":0,"message":{"content":"x"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
	}))
	defer server.Close()
	for _, tc := range []struct {
		name, path             string
		limit                  float64
		good, success, failure float64
	}{
		{"all_good", "/valid", 0, 2, 2, 0},
		{"all_failure", "/error", 0, 0, 0, 2},
		{"zero_good_success", "/valid", .25, 0, 2, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			summaryPath := filepath.Join(dir, "summary.json")
			runPath := filepath.Join(dir, "run.json")
			script := fmt.Sprintf(`import llm from 'k6/x/llm-perf';
export const options={vus:1,iterations:1};
export default function(){
 llm.beginMeasurement({runId:%q,maxRecords:4});
 const c=new llm.Client({baseURL:%q,strictMeasurement:true,maxTPOT:%g});
 for(let i=0;i<2;i++)c.request({path:%q,protocol:'openai-chat',body:{messages:[{role:'user',content:'x'}]}});
 llm.endMeasurement();
}
export function handleSummary(data){return {[%q]:JSON.stringify(data),[%q]:JSON.stringify(llm.measurementSummary())};}
`, tc.name, server.URL, tc.limit, tc.path, summaryPath, runPath)
			scriptPath := filepath.Join(dir, "cohort.js")
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
			for name, want := range map[string]float64{"llm_attempts": 2, "llm_successes": tc.success, "llm_failures": tc.failure, "llm_good_requests": tc.good} {
				metric, ok := summary.Metrics[name]
				if !ok {
					t.Fatalf("missing %s", name)
				}
				got, ok := metric.Values["count"]
				if !ok || got != want {
					t.Fatalf("%s count=%g present=%v want=%g", name, got, ok, want)
				}
				t.Logf("%s actual count=%g", name, got)
			}
			for _, name := range []string{"llm_good_request", "llm_slo_compliance"} {
				got, ok := summary.Metrics[name].Values["rate"]
				if !ok || got != tc.good/2 {
					t.Fatal("fraction absent or wrong", name, got)
				}
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
			if run.Attempts != 2 || float64(run.GoodRequests) != tc.good {
				t.Fatal("zero cohort replay mismatch")
			}
		})
	}
}
