package llmperf

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestBinaryOpenAIChatUsageBuckets(t *testing.T) {
	binary := os.Getenv("K6_SMOKE_BINARY")
	if binary == "" {
		t.Skip("set K6_SMOKE_BINARY")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		usage := `{"prompt_tokens":5,"completion_tokens":4,"total_tokens":9,"reasoning_tokens":3,"prompt_tokens_details":{"cached_tokens":2}}`
		if r.URL.Path == "/disagree" {
			usage = `{"prompt_tokens":5,"completion_tokens":4,"reasoning_tokens":3,"completion_tokens_details":{"reasoning_tokens":2}}`
		}
		if r.URL.Path == "/sync" {
			fmt.Fprint(w, openAIAdmissionBody("openai-chat", usage, false))
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hello\"},\"finish_reason\":\"stop\"}]}\n\n")
		fmt.Fprintf(w, "data: {\"choices\":[],\"usage\":%s}\n\ndata: [DONE]\n\n", usage)
	}))
	defer server.Close()
	script := fmt.Sprintf(`import llm from 'k6/x/llm-perf';import {check} from 'k6';
export const options={vus:1,iterations:1,thresholds:{checks:['rate==1'],llm_reasoning_tokens:['count==6'],llm_cached_tokens:['count==4']}};
const c=new llm.Client({baseURL:%q,strictMeasurement:true});
export default function(){
for(const path of ['/sync','/stream']){const r=c.request({protocol:'openai-chat',path,stream:path==='/stream',body:{messages:[{role:'user',content:'hello'}]}});check(r,{'usage buckets':r=>!(r instanceof Error)&&r.measurement.reasoning_tokens===3&&r.measurement.cached_tokens===2&&r.usage.reasoning_tokens===3&&r.usage.cached_tokens===2});}
const r=c.request({protocol:'openai-chat',path:'/disagree',stream:true,body:{messages:[{role:'user',content:'hello'}]}});check(r,{'reasoning disagreement refused':r=>r instanceof Error});}`, server.URL)
	path := filepath.Join(t.TempDir(), "buckets.js")
	if err := os.WriteFile(path, []byte(script), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "run", "--no-color", path)
	cmd.Env = append(os.Environ(), "K6_NO_USAGE_REPORT=true", "K6_AUTO_EXTENSION_RESOLUTION=false")
	data, err := cmd.CombinedOutput()
	t.Log(string(data))
	if err != nil {
		t.Fatal(err)
	}
}
