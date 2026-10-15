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
	"sync"
	"testing"
	"time"
)

// TestBinaryConversationExtensions proves that conversation() puts engine
// extension fields on the wire for every turn, appends the live reply, and
// that shadowing, case-folded and undefined extensions are refused before
// dispatch.
func TestBinaryConversationExtensions(t *testing.T) {
	binary := os.Getenv("K6_SMOKE_BINARY")
	if binary == "" {
		t.Skip("set K6_SMOKE_BINARY")
	}
	binary, err := filepath.Abs(binary)
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var bodies []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		mu.Lock()
		bodies = append(bodies, body)
		turn := len(bodies)
		mu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"reply%d\"}}]}\n\n", turn)
		fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"length\"}],\"usage\":{\"prompt_tokens\":3,\"completion_tokens\":4,\"total_tokens\":7}}\n\ndata: [DONE]\n\n")
	}))
	defer server.Close()
	dir := t.TempDir()
	lib, err := os.ReadFile("examples/lib/workload.js")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "workload.js"), lib, 0600); err != nil {
		t.Fatal(err)
	}
	script := `import {Client} from 'k6/x/llm-perf';import {Counter} from 'k6/metrics';
import {conversation} from './workload.js';
const passed=new Counter('fixture_passed');
export const options={vus:1,iterations:1,thresholds:{fixture_passed:['count==1'],llm_successes:['count==2']}};
export default function(){const c=new Client({baseURL:__ENV.ENDPOINT_URL,strictMeasurement:true,model:'fixture'});
const shadow=c.chat.completions.create({messages:[{role:'user',content:'x'}],extensions:{stream:false}});
if(!(shadow instanceof Error))throw Error('shadowing extension accepted');
const alias=c.chat.completions.create({messages:[{role:'user',content:'x'}],extensions:{'\u017ftream':false}});
if(!(alias instanceof Error))throw Error('folded alias extension accepted');
const unset=c.chat.completions.create({messages:[{role:'user',content:'x'}],extensions:{ignore_eos:undefined}});
if(!(unset instanceof Error))throw Error('undefined extension accepted');
let early=false;try{conversation(c,[{prompt:'a'},{prompt:'b',request:{Extensions:{stream:false}}}]);}catch(e){early=true;}
if(!early)throw Error('case-variant extensions passed conversation validation');
const rows=conversation(c,[{prompt:'one',request:{max_tokens:4,extensions:{ignore_eos:true}}},{prompt:'two',request:{max_tokens:4,extensions:{ignore_eos:true}}}]);
if(rows.length!==2||rows.some(r=>r instanceof Error))throw Error('turn failed');
if(rows[1].choices[0].message.content!=='reply2'||rows.some(r=>r.measurement.output_tokens.value!==4))throw Error('measurement');
passed.add(1);
}`
	path := filepath.Join(dir, "conversation.js")
	if err := os.WriteFile(path, []byte(script), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "run", "--no-color", path)
	cmd.Env = append(os.Environ(), "ENDPOINT_URL="+server.URL, "K6_NO_USAGE_REPORT=true", "K6_AUTO_EXTENSION_RESOLUTION=false")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("conversation extensions: %v\n%s", err, output)
	}
	if len(bodies) != 2 {
		t.Fatalf("server received %d requests; want 2 (shadowing request must not dispatch)", len(bodies))
	}
	for i, body := range bodies {
		if body["ignore_eos"] != true || body["stream"] != true {
			t.Fatalf("turn %d body = %v; want ignore_eos and stream true", i, body)
		}
		if _, ok := body["extensions"]; ok {
			t.Fatalf("turn %d body carries the extensions envelope", i)
		}
	}
	messages, _ := bodies[1]["messages"].([]any)
	if len(messages) != 3 {
		t.Fatalf("second turn messages = %v; want user, assistant, user", messages)
	}
	if reply, _ := messages[1].(map[string]any); reply["role"] != "assistant" || reply["content"] != "reply1" {
		t.Fatalf("second turn did not append the live reply: %v", messages[1])
	}
}
