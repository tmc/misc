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

func TestBinarySpecDecode(t *testing.T) {
	binary := os.Getenv("K6_SMOKE_BINARY")
	if binary == "" {
		t.Skip("set K6_SMOKE_BINARY")
	}
	binary, err := filepath.Abs(binary)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request CompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "request", 400)
			return
		}
		trt := request.Model == "trt" || request.Model == "trt-zero"
		zero := request.Model == "vllm-zero" || request.Model == "trt-zero"
		stats := map[string]any{"num_spec_steps": 2, "acceptance_histogram": []int{0, 1, 1}}
		accepted, drafted := 3, 6
		if zero {
			accepted, drafted = 0, 0
			stats["num_spec_steps"] = 0
			stats["acceptance_histogram"] = []int{0}
		}
		if trt {
			stats["total_accepted_draft_tokens"] = accepted
			stats["total_draft_tokens"] = drafted
		} else {
			stats["num_accepted_draft_tokens"] = accepted
			stats["num_draft_tokens"] = drafted
		}
		usage := map[string]int{"prompt_tokens": 2, "completion_tokens": 3, "total_tokens": 5}
		if request.Stream {
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hi\"}}]}\n\n")
			choice := map[string]any{"index": 0, "finish_reason": "stop"}
			final := map[string]any{"choices": []any{choice}, "usage": usage}
			if trt {
				choice["speculative_decoding"] = stats
			} else {
				final["metrics"] = map[string]any{"speculative_decoding": stats}
			}
			data, _ := json.Marshal(final)
			fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", data)
		} else {
			choice := map[string]any{"index": 0, "message": map[string]string{"role": "assistant", "content": "hi"}}
			final := map[string]any{"choices": []any{choice}, "usage": usage}
			if trt {
				choice["speculative_decoding"] = stats
			} else {
				final["metrics"] = map[string]any{"speculative_decoding": stats}
			}
			json.NewEncoder(w).Encode(final)
		}
	}))
	defer server.Close()
	script := `import {Client} from 'k6/x/llm-perf';import {Counter} from 'k6/metrics';
const passed=new Counter('fixture_passed');
export const options={vus:1,iterations:1,thresholds:{fixture_passed:['count==1'],llm_successes:['count==8']}};
export default function(){const c=new Client({baseURL:__ENV.ENDPOINT_URL,strictMeasurement:true});
for(const model of ['vllm','trt','vllm-zero','trt-zero']) for(const stream of [false,true]){
 const r=c.chat.completions.create({model,messages:[{role:'user',content:'hi'}],stream});
 if(r instanceof Error || r.measurement.outcome!=='success')throw Error('completion '+model);
 const s=r.measurement.spec_decode;
 if(model==='trt-zero'){if(s!==null || !r.measurement.spec_decode_unavailable)throw Error('TRT zero');}
 else if(model==='vllm-zero'){if(!s || s.acceptance_rate!==null || s.steps!==0)throw Error('vLLM zero');}
 else if(!s || s.accepted!==3 || s.drafted!==6 || s.acceptance_rate!==0.5 || s.mean_acceptance_length!==2.5)throw Error('stats '+model);
}passed.add(1);}
`
	path := filepath.Join(t.TempDir(), "spec.js")
	if err := os.WriteFile(path, []byte(script), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "run", "--no-color", path)
	cmd.Env = append(os.Environ(), "ENDPOINT_URL="+server.URL, "K6_NO_USAGE_REPORT=true", "K6_AUTO_EXTENSION_RESOLUTION=false")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("spec fixtures: %v\n%s", err, output)
	}
	t.Log(string(output))
}
