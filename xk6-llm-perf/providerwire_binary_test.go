package llmperf

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBinaryProviderwireV4(t *testing.T) {
	binary := os.Getenv("K6_SMOKE_BINARY")
	if binary == "" {
		t.Skip("set K6_SMOKE_BINARY")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		if string(b) != `{"prompt":[]}` {
			t.Errorf("authored body %s", b)
		}
		for k, v := range map[string]string{"Ai-Language-Model-Id": "fixture", "Ai-Language-Model-Specification-Version": "4", "Ai-Language-Model-Streaming": "true", "Authorization": "Bearer fixture-key"} {
			if r.Header.Get(k) != v {
				t.Errorf("header %s=%q", k, r.Header.Get(k))
			}
		}
		w.Header().Set("Content-Type", "text/event-stream")
		for _, tt := range providerwireMetadataCases() {
			if r.URL.Path == "/"+tt.name {
				for _, b := range tt.events {
					fmt.Fprintf(w, "data: %s\n\n", b)
				}
				return
			}
		}
		fmt.Fprint(w, "data: {\"type\":\"stream-start\",\"warnings\":[]}\n\ndata: {\"type\":\"text-start\",\"id\":\"t\"}\n\ndata: {\"type\":\"text-delta\",\"id\":\"t\",\"delta\":\"answer\"}\n\ndata: {\"type\":\"text-end\",\"id\":\"t\"}\n\n")
		switch r.URL.Path {
		case "/eof":
			return
		case "/done":
			fmt.Fprint(w, "data: [DONE]\n\n")
			return
		case "/error":
			fmt.Fprint(w, "data: {\"type\":\"error\",\"error\":{\"message\":\"secret\"}}\n\n")
			return
		case "/unknown":
			fmt.Fprint(w, "data: {\"type\":\"future-event\"}\n\n")
			return
		}
		usage := `{"inputTokens":{"total":9,"noCache":2,"cacheRead":3,"cacheWrite":4},"outputTokens":{"total":5,"text":2,"reasoning":3}}`
		if r.URL.Path == "/absent" {
			usage = `{"inputTokens":{},"outputTokens":{}}`
		}
		if r.URL.Path == "/zero" {
			usage = `{"inputTokens":{"total":0},"outputTokens":{"total":0}}`
		}
		fmt.Fprintf(w, "data: {\"type\":\"finish\",\"finishReason\":{\"unified\":\"stop\"},\"usage\":%s}\n\n", usage)
	}))
	defer server.Close()
	script := fmt.Sprintf(`import llm from 'k6/x/llm-perf';
import {check} from 'k6';
export const options={vus:1,iterations:1,thresholds:{checks:['rate==1'],llm_successes:['count==5'],llm_failures:['count==10']}};
const client=new llm.Client({baseURL:%q,model:'fixture',apiKey:'fixture-key',strictMeasurement:true});
export default function(){
 llm.beginMeasurement({runId:'providerwire-v4',maxRecords:20});
 for(const path of ['/valid','/absent','/zero','/eof','/done','/error','/unknown','/no-metadata','/single-metadata','/duplicate-metadata','/late-text-start','/late-visible','/late-reasoning','/late-tool','/late-tool-call']){
  const r=client.request({path,protocol:'providerwire-v4',stream:true,body:{prompt:[]}});
  const good=path==='/valid'||path==='/absent'||path==='/zero'||path==='/no-metadata'||path==='/single-metadata';
  check(r,{[path]:r=>good?!(r instanceof Error)&&r.measurement.outcome==='success'&&r.choices[0].message.content==='answer':r instanceof Error&&r.measurement.outcome==='failure'&&!r.measurement.good&&!String(r).includes('secret')});
  if(path==='/single-metadata')check(r,{'identity remains first':r=>r.id==='first'&&r.model==='first-model'});
  if(path==='/no-metadata')check(r,{'identity remains absent':r=>r.id===''&&r.model===''});
  if(path==='/valid')check(r,{'buckets':r=>r.measurement.input_tokens.value===9&&r.measurement.output_tokens.value===5&&r.measurement.cached_tokens===3&&r.measurement.cache_write_tokens===4&&r.measurement.reasoning_tokens===3});
  if(path==='/absent')check(r,{'absent remains unavailable':r=>r.measurement.input_tokens.value===null&&r.measurement.output_tokens.value===null&&r.measurement.cached_tokens===null&&r.measurement.cache_write_tokens===null});
  if(path==='/zero')check(r,{'zero remains measured':r=>r.measurement.input_tokens.value===0&&r.measurement.output_tokens.value===0});
 }
 const s=llm.endMeasurement();check(s,{'accounting':s=>s.attempts===15&&s.successes===5&&s.failures===10});
}`, server.URL)
	path := filepath.Join(t.TempDir(), "providerwire.js")
	if e := os.WriteFile(path, []byte(script), 0600); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "run", "--no-color", path)
	cmd.Env = append(os.Environ(), "K6_NO_USAGE_REPORT=true", "K6_AUTO_EXTENSION_RESOLUTION=false")
	out, e := cmd.CombinedOutput()
	t.Logf("%s", out)
	if e != nil {
		t.Fatalf("providerwire boundary: %v (%s)", e, strings.TrimSpace(string(out)))
	}
}
