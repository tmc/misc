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

func TestBinaryWorkUsageConsistency(t *testing.T) {
	binary := os.Getenv("K6_SMOKE_BINARY")
	if binary == "" {
		t.Skip("set K6_SMOKE_BINARY")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		if _, ok := request["work"]; ok {
			t.Error("work leaked to provider")
		}
		usage := map[string]int{"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2}
		if r.URL.Path == "/missing" {
			delete(usage, "total_tokens")
		}
		if r.URL.Path == "/contradictory" {
			usage["total_tokens"] = 99
		}
		body := map[string]any{"model": "served", "choices": []any{map[string]any{"index": 0, "message": map[string]any{"role": "assistant", "content": "hello"}, "finish_reason": "stop"}}, "usage": usage}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(body); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	runPath := filepath.Join(t.TempDir(), "run.json")
	script := fmt.Sprintf(`import llm from 'k6/x/llm-perf';
import {check} from 'k6';
export const options={vus:1,iterations:1,thresholds:{checks:['rate==1'],llm_successes:['count==3'],llm_good_requests:['count==3'],llm_work_accepted:['count==1'],llm_work_rejected:['count==1'],llm_work_unavailable:['count==1'],llm_work_good_requests:['count==1']}};
const c=new llm.Client({baseURL:%q,strictMeasurement:true});
export default function(){
 llm.beginMeasurement({runId:'usage-matrix',maxRecords:10});
 for(const [path,status,reason] of [['/valid','accepted',null],['/missing','unavailable','exact_usage_unavailable'],['/contradictory','rejected','exact_usage_inconsistent']]){
  const r=c.request({path,protocol:'openai-chat',body:{messages:[{role:'user',content:'hello'}]},work:{requireExactUsage:true,expectedModel:'served',allowedFinishReasons:['stop'],minOutputTokens:1,maxOutputTokens:1}});
  check(r,{[path]:r=>!(r instanceof Error)&&r.measurement.outcome==='success'&&r.measurement.good&&r.measurement.work.status===status&&(reason===null||r.measurement.work.reasons.includes(reason))});
 }
 const run=llm.endMeasurement();
 check(run,{'population':s=>s.successes===3&&s.good_requests===3&&s.work_accepted===1&&s.work_rejected===1&&s.work_unavailable===1&&s.work_good_requests===1});
}
export function handleSummary(){return {[%q]:JSON.stringify(llm.measurementSummary())};}
`, server.URL, runPath)
	path := filepath.Join(t.TempDir(), "usage.js")
	if err := os.WriteFile(path, []byte(script), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary, "run", "--no-color", path)
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
	if run.WorkAccepted != 1 || run.WorkRejected != 1 || run.WorkUnavailable != 1 {
		t.Fatal(run)
	}
}
