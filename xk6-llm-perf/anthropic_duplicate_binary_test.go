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

func TestBinaryAnthropicDuplicateAdmission(t *testing.T) {
	binary := os.Getenv("K6_SMOKE_BINARY")
	if binary == "" {
		t.Skip("set K6_SMOKE_BINARY")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/part/") {
			var index int
			fmt.Sscanf(r.URL.Path, "/part/%d", &index)
			part := ambiguousAnthropicParts[index]
			if part.stream {
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprintf(w, "data: %s\n\ndata: {\"type\":\"message_stop\"}\n\n", part.body)
			} else {
				fmt.Fprint(w, part.body)
			}
			return
		}
		usage := `{"input_tokens":2,"output_tokens":1,"cache_creation_input_tokens":0}`
		for i, raw := range ambiguousAnthropicUsage {
			if strings.HasSuffix(r.URL.Path, fmt.Sprint(i)) {
				usage = raw
			}
		}
		if strings.HasPrefix(r.URL.Path, "/stream") {
			w.Header().Set("Content-Type", "text/event-stream")
			if strings.HasSuffix(r.URL.Path, "container") {
				fmt.Fprint(w, "data: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":-1},\"USAGE\":{\"input_tokens\":2}}}\n\n")
			} else {
				fmt.Fprintf(w, "data: {\"type\":\"message_start\",\"message\":{\"usage\":%s}}\n\n", usage)
			}
			if strings.HasSuffix(r.URL.Path, "opaque") {
				fmt.Fprint(w, "data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"tool_use\",\"id\":\"call\",\"name\":\"lookup\",\"input\":{\"A\":1,\"a\":2}}}\n\n")
			}
			fmt.Fprint(w, "data: {\"type\":\"message_stop\"}\n\n")
			return
		}
		if strings.HasSuffix(r.URL.Path, "container") {
			fmt.Fprint(w, `{"usage":{"input_tokens":-1},"USAGE":{"input_tokens":2}}`)
			return
		}
		content := `{"type":"text","text":"hello"}`
		if strings.HasSuffix(r.URL.Path, "opaque") {
			content = `{"type":"tool_use","id":"call","name":"lookup","input":{"A":1,"a":2}}`
		}
		fmt.Fprintf(w, `{"content":[%s],"stop_reason":"end_turn","usage":%s}`, content, usage)
	}))
	defer server.Close()
	script := fmt.Sprintf(`import llm from 'k6/x/llm-perf';import {check} from 'k6';
export const options={vus:1,iterations:1,thresholds:{checks:['rate==1']}};
const c=new llm.Client({baseURL:%q,strictMeasurement:true});
export default function(){for(const stream of [false,true]){for(const suffix of ['0','1','2','3','4','5','container','valid','opaque']){
const r=c.request({protocol:'anthropic',path:(stream?'/stream/':'/sync/')+suffix,stream,body:{messages:[{role:'user',content:'hello'}]}});
check(r,{'duplicate admission':r=>(r instanceof Error)===(suffix!=='valid'&&suffix!=='opaque')});}}
for(const [index,stream] of [[0,true],[1,true],[2,true],[3,false]]){const r=c.request({protocol:'anthropic',path:'/part/'+index,stream,body:{messages:[{role:'user',content:'hello'}]}});check(r,{'nested fixed aliases refuse':r=>r instanceof Error});}}`, server.URL)
	path := filepath.Join(t.TempDir(), "duplicates.js")
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
