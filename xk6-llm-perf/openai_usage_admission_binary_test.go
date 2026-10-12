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

func TestBinaryOpenAIUsageAdmission(t *testing.T) {
	binary := os.Getenv("K6_SMOKE_BINARY")
	if binary == "" {
		t.Skip("set K6_SMOKE_BINARY")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		protocol := parts[0]
		var stream, index int
		fmt.Sscanf(parts[1], "%d", &stream)
		fmt.Sscanf(parts[2], "%d", &index)
		usage := `{"prompt_tokens":0,"completion_tokens":0,"metadata":{"A":1,"a":2}}`
		if index < len(ambiguousOpenAIUsage) {
			usage = ambiguousOpenAIUsage[index]
		}
		body := openAIAdmissionBody(protocol, usage, stream == 1)
		if index == 7 {
			body = `{"usage":{"prompt_tokens":-1},"USAGE":{"prompt_tokens":0}}`
		}
		if protocol == "openai-responses" && index == 8 {
			body = `{"type":"response.completed","response":{"usage":{"input_tokens":-1},"Usage":{"input_tokens":0}}}`
		}
		if protocol == "openai-responses" && index == 9 {
			body = openAIAdmissionBody(protocol, `{"input_tokens":1,"Prompt_Tokens":9}`, stream == 1)
		}
		if protocol == "openai-responses" && index == 10 {
			body = openAIAdmissionBody(protocol, `{"input_tokens_details":{"cached_tokens":1},"prompt_tokens_details":{"cached_tokens":9}}`, stream == 1)
		}
		if stream == 1 {
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprintf(w, "data: %s\n\n", body)
			if protocol != "openai-responses" {
				fmt.Fprint(w, "data: [DONE]\n\n")
			}
		} else {
			fmt.Fprint(w, body)
		}
	}))
	defer server.Close()
	script := fmt.Sprintf(`import llm from 'k6/x/llm-perf';import {check} from 'k6';export const options={vus:1,iterations:1,thresholds:{checks:['rate==1']}};
 const clients={};for(const protocol of ['openai-chat','openai-completions','openai-responses']){for(const stream of [0,1]){for(let index=0;index<(protocol==='openai-responses'?11:8);index++){const key=protocol+stream+index;clients[key]=new llm.Client({baseURL:%q,strictMeasurement:true});}}}
 export default function(){for(const protocol of ['openai-chat','openai-completions','openai-responses']){for(const stream of [0,1]){for(let index=0;index<(protocol==='openai-responses'?11:8);index++){const c=clients[protocol+stream+index];const r=c.request({protocol,path:'/'+protocol+'/'+stream+'/'+index,stream:stream===1,body:{messages:[{role:'user',content:'hello'}]}});check(r,{'usage admission':r=>(r instanceof Error)===(index!==6)});}}}}`, server.URL)
	path := filepath.Join(t.TempDir(), "admission.js")
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
