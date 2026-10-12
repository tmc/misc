package llmperf

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBinaryMissingUsageThroughput(t *testing.T) {
	binary := os.Getenv("K6_SMOKE_BINARY")
	if binary == "" {
		t.Skip("set K6_SMOKE_BINARY")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		usage := map[string]int{}
		if r.URL.Path == "/input" || r.URL.Path == "/zero" {
			usage["prompt_tokens"] = 0
		}
		if r.URL.Path == "/output" || r.URL.Path == "/zero" {
			usage["completion_tokens"] = 0
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"index": 0, "message": map[string]string{"content": "hello"}, "finish_reason": "stop"}}, "usage": usage})
	}))
	defer server.Close()
	dir := t.TempDir()
	script := fmt.Sprintf(`import llm from 'k6/x/llm-perf';import {check} from 'k6';
export const options={vus:1,iterations:1,thresholds:{checks:['rate==1']}};
const c=new llm.Client({baseURL:%q,strictMeasurement:true});
export default function(){for(const [path,input,output] of [['/missing',null,null],['/input',0,null],['/output',null,0],['/zero',0,0]]){
llm.beginMeasurement({runId:path,maxRecords:1});const r=c.request({protocol:'openai-chat',path,body:{messages:[{role:'user',content:'hello'}]}});const run=llm.endMeasurement();console.log("RATE_RECEIPT "+JSON.stringify(run));
check(r,{'request succeeded':r=>!(r instanceof Error)&&r.measurement.outcome==='success'});
check(run,{'independent rate presence':s=>s.input_throughput===input&&s.output_throughput===output});}}`, server.URL)
	path := filepath.Join(dir, "rates.js")
	if err := os.WriteFile(path, []byte(script), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "run", "--no-color", "--log-format", "raw", path)
	cmd.Env = append(os.Environ(), "K6_NO_USAGE_REPORT=true", "K6_AUTO_EXTENSION_RESOLUTION=false")
	data, err := cmd.CombinedOutput()
	t.Log(string(data))
	if err != nil {
		t.Fatal(err)
	}
	var runs []json.RawMessage
	for _, line := range strings.Split(string(data), "\n") {
		if raw, ok := strings.CutPrefix(line, "RATE_RECEIPT "); ok {
			runs = append(runs, json.RawMessage(raw))
		}
	}
	if len(runs) != 4 {
		t.Fatalf("runs=%d", len(runs))
	}
	for _, raw := range runs {
		if _, err := ReadRun(bytes.NewReader(raw)); err != nil {
			t.Fatal(err)
		}
	}
}
