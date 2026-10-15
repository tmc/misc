package llmperf

import (
	"context"
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

func TestBinaryProtocolIntegrity(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/responses":
			fmt.Fprint(w, `{"id":"resp_1","status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"hello"}]}],"usage":{"input_tokens":1,"output_tokens":1}}`)
		case "/chat/completions":
			fmt.Fprint(w, `{"error":{"message":"failed"},"choices":[{"message":{"content":"partial"}}]}`)
		case "/reasoning":
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"reasoning\":\"think\"}}]}\n\ndata: {\"choices\":[],\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":2}}\n\ndata: [DONE]\n\n")
		case "/model-switch":
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "data: {\"model\":\"cheap\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"a\"}}]}\n\ndata: {\"model\":\"pricey\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
		case "/oversized-model":
			fmt.Fprintf(w, `{"model":%q,"choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}]}`, strings.Repeat("x", 1025))
		case "/audio":
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprint(w, "42")
		}
	}))
	defer s.Close()
	script := fmt.Sprintf(`import llm from 'k6/x/llm-perf';import {check} from 'k6';export const options={vus:1,iterations:1,thresholds:{checks:['rate==1']}};const client=new llm.Client({baseURL:%q,model:'fixture'});export default function(){const a=client.request({path:'/responses',protocol:'openai-responses',body:{model:'fixture',input:'hi'}});check(a,{'valid Responses completed accepted':r=>!(r instanceof Error)&&r.choices[0].message.content==='hello'});const b=client.chat.completions.create({messages:[{role:'user',content:'hi'}]});check(b,{'provider error rejected':r=>r instanceof Error});const d=client.request({path:'/reasoning',protocol:'openai-chat',stream:true,body:{model:'fixture',messages:[{role:'user',content:'hi'}],stream:true}});check(d,{'reasoning alias retained with TTFT and no TTFO':r=>!(r instanceof Error)&&r.choices[0].message.reasoning_content==='think'&&r.measurement.ttft!==null&&r.measurement.ttfo===null});const e=client.request({path:'/model-switch',protocol:'openai-chat',stream:true,body:{model:'fixture',messages:[{role:'user',content:'hi'}],stream:true}});check(e,{'stream model change rejected':r=>r instanceof Error});const f=client.request({path:'/oversized-model',protocol:'openai-chat',work:{expectedModel:'fixture'},body:{model:'fixture',messages:[{role:'user',content:'hi'}]}});check(f,{'oversized model flagged and rejected':r=>!(r instanceof Error)&&r.measurement.work.observed_model===undefined&&r.measurement.work.observed_model_oversized===true&&r.measurement.work.reasons[0]==='model_mismatch'});const c=client.request({path:'/audio',protocol:'audio-transcription',body:{model:'fixture',input:'fixture',response_format:'text'}});check(c,{'numeric plain text transcript accepted':r=>!(r instanceof Error)&&r.choices[0].message.content==='42'});}`, s.URL)
	path := filepath.Join(t.TempDir(), "contracts.js")
	if e := os.WriteFile(path, []byte(script), 0600); e != nil {
		t.Fatal(e)
	}
	binary := os.Getenv("K6_SMOKE_BINARY")
	if binary == "" {
		t.Skip("set K6_SMOKE_BINARY")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "run", "--no-color", path)
	cmd.Env = append(os.Environ(), "K6_NO_USAGE_REPORT=true", "K6_AUTO_EXTENSION_RESOLUTION=false")
	b, e := cmd.CombinedOutput()
	t.Logf("%s", b)
	if e != nil {
		t.Fatalf("real k6 contracts failed: %v", e)
	}
}
