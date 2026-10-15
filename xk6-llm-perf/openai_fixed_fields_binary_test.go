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

func TestBinaryOpenAIFixedFieldAliases(t *testing.T) {
	binary := os.Getenv("K6_SMOKE_BINARY")
	if binary == "" {
		t.Skip("set K6_SMOKE_BINARY")
	}
	cases := append(append(ambiguousOpenAIFields[:0:0], ambiguousOpenAIFields...), unambiguousOpenAIFields...)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var index int
		fmt.Sscanf(r.URL.Path, "/case/%d", &index)
		tt := cases[index]
		if tt.stream {
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", tt.body)
		} else {
			fmt.Fprint(w, tt.body)
		}
	}))
	defer server.Close()
	var list string
	for i, tt := range cases {
		list += fmt.Sprintf("[%d,%q,%v,%v],", i, tt.protocol, tt.stream, i < len(ambiguousOpenAIFields))
	}
	script := fmt.Sprintf(`import llm from 'k6/x/llm-perf';import {check} from 'k6';
export const options={vus:1,iterations:1,thresholds:{checks:['rate==1']}};
const c=new llm.Client({baseURL:%q});
export default function(){for(const [index,protocol,stream,ambiguous] of [%s]){
const r=c.request({protocol,path:'/case/'+index,stream,body:{messages:[{role:'user',content:'hello'}]}});
check(r,{'fixed field alias admission':r=>(r instanceof Error)===ambiguous&&(!ambiguous||String(r).includes('duplicate'))});}}`, server.URL, list)
	path := filepath.Join(t.TempDir(), "fields.js")
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
