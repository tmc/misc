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

func TestBinaryResponsesTerminalAdmission(t *testing.T) {
	binary := os.Getenv("K6_SMOKE_BINARY")
	if binary == "" {
		t.Skip("set K6_SMOKE_BINARY")
	}
	fixtures := map[string]string{
		"/nested-error": `{"type":"response.completed","response":{"status":"completed","error":{"message":"provider secret"}}}`,
		"/null":         `{"type":"response.completed","response":null}`,
		"/missing":      `{"type":"response.completed"}`,
		"/number":       `{"type":"response.completed","response":7}`,
		"/array":        `{"type":"response.completed","response":[]}`,
		"/string":       `{"type":"response.completed","response":"provider secret"}`,
		"/boolean":      `{"type":"response.completed","response":true}`,
		"/compatible":   `{"type":"response.completed","response":{"usage":{"input_tokens":2,"output_tokens":3}}}`,
		"/valid":        `{"type":"response.completed","response":{"status":"completed","error":null,"usage":{"input_tokens":2,"output_tokens":3}}}`,
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		event, ok := fixtures[r.URL.Path]
		if !ok {
			http.Error(w, "unknown fixture", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"partial\"}\n\ndata: %s\n\n", event)
	}))
	defer server.Close()
	for _, negative := range []bool{true, false} {
		t.Run(fmt.Sprintf("negative=%t", negative), func(t *testing.T) {
			script := fmt.Sprintf(`import llm from 'k6/x/llm-perf';
import {check} from 'k6';
const negative=%t;
const paths=negative?['/nested-error','/null','/missing','/number','/array','/string','/boolean']:['/valid','/compatible'];
export const options={vus:1,iterations:1,thresholds:{checks:['rate==1'],llm_successes:[negative?'count==0':'count==2'],llm_good_requests:[negative?'count==0':'count==2'],llm_failures:[negative?'count==7':'count==0']}};
const client=new llm.Client({baseURL:%q,strictMeasurement:true,model:'fixture'});
export default function(){
 llm.beginMeasurement({runId:negative?'negative':'positive',maxRecords:10});
 for(const path of paths){
  const r=client.request({path,protocol:'openai-responses',body:{model:'fixture',stream:true}});
  if(negative){check(r,{[path]:r=>r instanceof Error&&r.measurement.outcome==='failure'&&!r.measurement.good&&String(r).length<256&&!String(r).includes('provider secret')});}
  else{check(r,{[path]:r=>!(r instanceof Error)&&r.measurement.outcome==='success'&&r.measurement.good&&r.provider_status==='completed'&&r.choices[0].message.content==='partial'&&r.measurement.input_tokens.value===2&&r.measurement.output_tokens.value===3});}
 }
 const run=llm.endMeasurement();
 check(run,{'accounting agrees':s=>s.attempts===paths.length&&s.successes===(negative?0:2)&&s.good_requests===(negative?0:2)&&s.failures===(negative?7:0)});
}`, negative, server.URL)
			path := filepath.Join(t.TempDir(), "terminal.js")
			if err := os.WriteFile(path, []byte(script), 0600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, binary, "run", "--no-color", path)
			cmd.Env = append(os.Environ(), "K6_NO_USAGE_REPORT=true", "K6_AUTO_EXTENSION_RESOLUTION=false")
			out, err := cmd.CombinedOutput()
			t.Logf("%s", out)
			if err != nil {
				t.Fatalf("terminal admission: %v", err)
			}
		})
	}
}
