package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/grafana/sobek"
	"rsc.io/script"
	"rsc.io/script/scripttest"
)

const authored = `{"session_id":"authored","messages":[{"role":"user","content":"one"}],"output_length":8,"extra":{"temperature":0.1}}` + "\n" + `{"session_id":"authored","messages":[{"role":"user","content":"two"}]}`

var exportArgs = []string{"-format", "mooncake", "-output", "sessions", "-execution-policy", "serial-recorded", "-path", "/chat/completions", "-protocol", "openai-chat", "-model", "fixture"}

func TestExportConsumer(t *testing.T) {
	var out bytes.Buffer
	if err := run(exportArgs, strings.NewReader(authored), &out); err != nil {
		t.Fatal(err)
	}
	var rows []replaySession
	if err := json.Unmarshal(out.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || len(rows[0].Turns) != 2 || rows[0].Turns[0].Source.SourceRow != 1 || !strings.Contains(rows[0].SourcePolicy, "not scheduled") {
		t.Fatal("missing source metadata")
	}
	source, err := os.ReadFile("../../examples/workloads/raw-sessions.js")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(source), "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "import ") {
			lines[i] = ""
		}
	}
	code := strings.ReplaceAll(strings.Join(lines, "\n"), "export const options", "const options")
	code = strings.ReplaceAll(code, "export default function ()", "function runSession()")
	rt := sobek.New()
	if err := rt.Set("input", out.String()); err != nil {
		t.Fatal(err)
	}
	_, err = rt.RunString(`
 const __ENV={SESSIONS_FILE:'fixture',ENDPOINT_URL:'http://fixture'};
 const execution={scenario:{iterationInTest:0}}, calls=[],logs=[];let failed=0;
 function SharedArray(name,fn){return fn();}function open(){return input;}
 function Counter(name){this.add=n=>{};}function check(v,c){for(const f of Object.values(c))if(!f(v))failed++;}
 const console={log:s=>logs.push(JSON.parse(s))};
 const llm={Client:function(){this.request=r=>{calls.push(r);return {status:200};};}};
 ` + code + `
 runSession();
 if(calls.length!==2||calls[0].body.messages[0].content!=='one'||calls[1].body.messages.length!==1||calls[1].body.messages[0].content!=='two'||calls[0].body.temperature!==0.1||calls[0].body.max_tokens!==8)throw new Error('consumer payload mismatch');
 if(logs[0].session_id!=='authored'||logs[0].turn_id!=='1'||logs[1].turn_id!=='2'||failed)throw new Error('consumer identity');
 `)
	if err != nil {
		t.Fatal(err)
	}
}
func TestExecutionRefusals(t *testing.T) {
	for _, tt := range []struct{ name, input string }{
		{"count-only", `{"session_id":"x","input_length":512,"hash_ids":[1]}`},
		{"live-deltas", `{"session_id":"x","messages":[{"role":"user"}],"assistant_responses":"live"}`},
		{"unsafe-payload", `{"session_id":"x","payload":{"n":9007199254740993}}`},
		{"precision-payload", `{"session_id":"x","payload":{"n":0.1234567890123456789}}`},
		{"zero-output", `{"session_id":"x","messages":[{"role":"user"}],"output_length":0}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := run(exportArgs, strings.NewReader(tt.input), &out); err == nil || out.Len() != 0 {
				t.Fatal("invalid execution admitted")
			}
		})
	}
	var out bytes.Buffer
	if err := run([]string{"-format", "mooncake", "-output", "sessions"}, strings.NewReader(authored), &out); err == nil {
		t.Fatal("implicit policy admitted")
	}
}
func TestCommand(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "llm-import-trace")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, "go", "build", "-o", binary, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	engine := &script.Engine{Cmds: scripttest.DefaultCmds(), Conds: scripttest.DefaultConds()}
	scripttest.Test(t, ctx, engine, []string{"PATH=" + os.Getenv("PATH"), "CLI=" + binary}, "testdata/*.txt")
}

func TestBinaryConsumer(t *testing.T) {
	binary := os.Getenv("K6_SMOKE_BINARY")
	if binary == "" {
		t.Skip("set K6_SMOKE_BINARY for bounded registered binary loopback")
	}
	var out bytes.Buffer
	if err := run(exportArgs, strings.NewReader(authored), &out); err != nil {
		t.Fatal(err)
	}
	var expected []replaySession
	if err := json.Unmarshal(out.Bytes(), &expected); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "sessions.json")
	if err := os.WriteFile(file, out.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var requests []map[string]json.RawMessage
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.URL.Path != "/chat/completions" {
			t.Errorf("path: %s", r.URL.Path)
		}
		var body map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		requests = append(requests, body)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"model":"fixture","choices":[{"index":0,"message":{"role":"assistant","content":"ok"}}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "run", "--no-color", "../../examples/workloads/raw-sessions.js")
	cmd.Env = []string{"SESSIONS_FILE=" + file, "ENDPOINT_URL=" + server.URL, "VUS=1", "K6_NO_USAGE_REPORT=true", "K6_AUTO_EXTENSION_RESOLUTION=false"}
	result, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("k6: %v\n%s", err, result)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(requests) != 2 {
		t.Fatalf("requests: %d\n%s", len(requests), result)
	}
	for i, body := range requests {
		var a, b any
		x, _ := json.Marshal(body)
		y, _ := json.Marshal(expected[0].Turns[i].Request.Body)
		json.Unmarshal(x, &a)
		json.Unmarshal(y, &b)
		if !reflect.DeepEqual(a, b) {
			t.Fatalf("wire body %d changed: %s / %s", i, x, y)
		}
	}
	t.Logf("two authored wire bodies equal; exact session/turn receipts follow:\n%s", result)
}

func TestReplayAggregateBudget(t *testing.T) {
	args := append([]string(nil), exportArgs...)
	for i := range args {
		if args[i] == "/chat/completions" {
			args[i] = "/" + strings.Repeat("x", 2047)
		}
		if args[i] == "fixture" {
			args[i] = strings.Repeat("m", 1024)
		}
	}
	var out bytes.Buffer
	input := strings.Repeat(`{"text_input":"literal"}`+"\n", 1000)
	if err := run(args, strings.NewReader(input), &out); err == nil || !strings.Contains(err.Error(), "aggregate replay output budget") || out.Len() != 0 {
		t.Fatalf("replay allocation admitted: %v", err)
	}
}

func TestBlockSizeFlag(t *testing.T) {
	moon := `{"session_id":"s","input_length":8,"hash_ids":[1,2]}`
	lab := `{"session_id":"s","round_index":0,"input_tokens_total":8,"prefix_tokens":0,"newly_append_tokens":8,"timing_events":[{"event_type":"user_message","timestamp":"2026-10-18T00:00:00Z"}]}`
	for _, tt := range []struct {
		name, format, input string
		args                []string
		want                int
	}{
		{"moon default", "mooncake", moon, nil, 512},
		{"moon explicit4", "mooncake", moon, []string{"-block-size", "4"}, 4},
		{"lab default", "tracelab", lab, nil, 64},
		{"lab explicit4", "tracelab", lab, []string{"-block-size", "4"}, 4},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			args := append([]string{"-format", tt.format}, tt.args...)
			if err := run(args, strings.NewReader(tt.input), &out); err != nil {
				t.Fatal(err)
			}
			var got struct{ Sessions []struct{ BlockSize int } }
			if err := json.Unmarshal(out.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if got.Sessions[0].BlockSize != tt.want {
				t.Fatalf("block metadata: %s", out.Bytes())
			}
		})
	}
	for _, value := range []string{"0", "-1", "1000001", "true", "false", "1.5", "bad", "null"} {
		t.Run("refuse-"+value, func(t *testing.T) {
			var out bytes.Buffer
			if err := run([]string{"-format", "mooncake", "-block-size", value}, strings.NewReader(moon), &out); err == nil || out.Len() != 0 {
				t.Fatal("invalid block size admitted")
			}
		})
	}
	var out bytes.Buffer
	if err := run([]string{"-format", "bailian", "-block-size", "4"}, strings.NewReader(`{"chat_id":1,"timestamp":0,"input_length":8,"output_length":4}`), &out); err == nil || !strings.Contains(err.Error(), "unsupported for bailian") {
		t.Fatal("Bailian override ignored")
	}
	out.Reset()
	args := append(append([]string(nil), exportArgs...), "-block-size", "4")
	if err := run(args, strings.NewReader(authored), &out); err != nil {
		t.Fatal(err)
	}
	var exported []replaySession
	if err := json.Unmarshal(out.Bytes(), &exported); err != nil {
		t.Fatal(err)
	}
	if exported[0].BlockSize != 4 || exported[0].HashScope != "file" {
		t.Fatal("authored export dropped block declaration")
	}
}
