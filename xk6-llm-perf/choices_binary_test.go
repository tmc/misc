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

func TestBinaryChoices(t *testing.T) {
	binary := os.Getenv("K6_SMOKE_BINARY")
	if binary == "" {
		t.Skip("set K6_SMOKE_BINARY")
	}
	binary, err := filepath.Abs(binary)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"index\":1,\"delta\":{\"content\":\"other\"}},{\"index\":0,\"delta\":{\"reasoning_content\":\"thinking\"}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"selected\",\"tool_calls\":[{\"index\":0,\"id\":\"call0\",\"type\":\"function\",\"function\":{\"name\":\"f\",\"arguments\":\"{\\\"n\\\":\"}}]}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\"1}\"}}]},\"finish_reason\":\"stop\"},{\"index\":1,\"delta\":{\"content\":\" choice\"},\"finish_reason\":\"length\"}],\"usage\":{\"prompt_tokens\":2,\"completion_tokens\":6,\"total_tokens\":8}}\n\ndata: [DONE]\n\n")
	}))
	defer server.Close()
	script := `import {Client} from 'k6/x/llm-perf';import {Counter} from 'k6/metrics';
const passed=new Counter('fixture_passed');
export const options={vus:1,iterations:1,thresholds:{fixture_passed:['count==1'],llm_successes:['count==1']}};
export default function(){const c=new Client({baseURL:__ENV.ENDPOINT_URL,strictMeasurement:true,model:'fixture'});
const r=c.chat.completions.create({messages:[{role:'user',content:'hi'}],stream:true});
if(r instanceof Error)throw r;
if(r.choices.length!==2 || r.choices[0].index!==0 || r.choices[1].index!==1)throw Error('identity');
if(r.choices[0].message.content!=='selected' || r.choices[1].message.content!=='other choice')throw Error('content');
if(r.choices[0].message.reasoning_content!=='thinking' || JSON.parse(r.choices[0].message.tool_calls[0].function.arguments).n!==1)throw Error('tool');
if(r.choices[0].finish_reason!=='stop' || r.choices[1].finish_reason!=='length')throw Error('finish');
if(!r.measurement.multi_choice || r.measurement.itl!==null || r.measurement.output_tokens.value!==6 || r.measurement.ttft===null || r.measurement.ttfo===null)throw Error('attribution');
passed.add(1);
}`
	path := filepath.Join(t.TempDir(), "choices.js")
	if err := os.WriteFile(path, []byte(script), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "run", "--no-color", path)
	cmd.Env = append(os.Environ(), "ENDPOINT_URL="+server.URL, "K6_NO_USAGE_REPORT=true", "K6_AUTO_EXTENSION_RESOLUTION=false")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("choice fixtures: %v\n%s", err, output)
	}
	t.Log(string(output))
}
