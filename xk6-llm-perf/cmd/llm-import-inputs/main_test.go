package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/grafana/sobek"
	"rsc.io/script"
	"rsc.io/script/scripttest"
)

const fixture = `{"data":[{"session_id":"first","payloads":[{"messages":[{"role":"user","content":"hello"}],"stream":false,"temperature":0.1,"nested":{"a":null}}, {"messages":[{"role":"user","content":"independent"}],"max_tokens":2}]},{"session_id":"second","payloads":[{"input":"third","literal":"\ud83d\ude00","n":9007199254740991}]}]}`

func TestConvert(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"-path", "/chat/completions", "-protocol", "json"}, strings.NewReader(fixture), &out); err != nil {
		t.Fatal(err)
	}
	var got []session
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != "first" || got[1].ID != "second" || len(got[0].Turns) != 2 || got[0].Turns[1].ID != "1" || len(got[0].SourceSHA256) != 64 {
		t.Fatalf("identities: %+v", got)
	}
	var input struct {
		Data []struct{ Payloads []json.RawMessage }
	}
	if err := json.Unmarshal([]byte(fixture), &input); err != nil {
		t.Fatal(err)
	}
	for i, s := range got {
		for j, turn := range s.Turns {
			var a, b bytes.Buffer
			if err := json.Compact(&a, input.Data[i].Payloads[j]); err != nil {
				t.Fatal(err)
			}
			if err := json.Compact(&b, turn.Request.Body); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(a.Bytes(), b.Bytes()) {
				t.Fatalf("payload %d/%d changed: %s", i, j, b.Bytes())
			}
		}
	}
}

func TestReject(t *testing.T) {
	tests := []struct{ name, input string }{
		{"empty", `{"data":[]}`}, {"null", `{"data":null}`}, {"missing", `{}`},
		{"unknown", `{"data":[],"extra":1}`}, {"duplicate root", `{"data":[],"data":[]}`},
		{"duplicate id", `{"data":[{"session_id":"x","payloads":[{}]},{"session_id":"x","payloads":[{}]}]}`},
		{"null id", `{"data":[{"session_id":null,"payloads":[{}]}]}`},
		{"numeric id", `{"data":[{"session_id":1,"payloads":[{}]}]}`},
		{"empty turns", `{"data":[{"session_id":"x","payloads":[]}]}`},
		{"null turns", `{"data":[{"session_id":"x","payloads":null}]}`},
		{"null payload", `{"data":[{"session_id":"x","payloads":[null]}]}`},
		{"array payload", `{"data":[{"session_id":"x","payloads":[[]]}]}`},
		{"duplicate member", `{"data":[{"session_id":"x","payloads":[{"nested":{"a":1,"a":2}}]}]}`},
		{"unsafe integer", `{"data":[{"session_id":"x","payloads":[{"n":9007199254740993}]}]}`},
		{"precision", `{"data":[{"session_id":"x","payloads":[{"n":0.1234567890123456789}]}]}`},
		{"underflow", `{"data":[{"session_id":"x","payloads":[{"n":1e-999}]}]}`},
		{"negative zero", `{"data":[{"session_id":"x","payloads":[{"n":-0}]}]}`},
		{"overflow", `{"data":[{"session_id":"x","payloads":[{"n":1e999}]}]}`},
		{"surrogate", `{"data":[{"session_id":"x","payloads":[{"s":"\ud800"}]}]}`},
		{"trailing", fixture + ` {}`}, {"size", strings.Repeat(" ", maxInput+1)},
		{"depth", `{"data":[{"session_id":"x","payloads":[{"v":` + strings.Repeat("[", 65) + `0` + strings.Repeat("]", 65) + `}]}]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := run([]string{"-path", "/test", "-protocol", "json"}, strings.NewReader(tt.input), &out); err == nil {
				t.Fatal("accepted invalid input")
			}
			if out.Len() != 0 {
				t.Fatal("partial output")
			}
		})
	}
}

// Execute the actual runner source against converted output with a request spy.
// This checks the consumer schema, session dispatch, independent bodies and
// failure continuation; it is not a provider or registered-extension test.
func TestRunner(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"-path", "/test", "-protocol", "json"}, strings.NewReader(fixture), &out); err != nil {
		t.Fatal(err)
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
	code := strings.Join(lines, "\n")
	code = strings.ReplaceAll(code, "export const options", "const options")
	code = strings.ReplaceAll(code, "export default function ()", "function runSession()")
	rt := sobek.New()
	if err := rt.Set("input", out.String()); err != nil {
		t.Fatal(err)
	}
	_, err = rt.RunString(`
 const __ENV={SESSIONS_FILE:'fixture',ENDPOINT_URL:'http://fixture'};
 const execution={scenario:{iterationInTest:0}};
 const calls=[], logs=[], counts={}; let failedChecks=0;
 function SharedArray(name,fn){return fn();}
 function open(){return input;}
 function Counter(name){this.add=n=>counts[name]=(counts[name]||0)+n;}
 function check(value,checks){for(const f of Object.values(checks))if(!f(value))failedChecks++;}
 const console={log:s=>logs.push(JSON.parse(s))};
 const llm={Client:function(){this.request=r=>{calls.push(r);return calls.length===1?new Error('fixture failure'):{status:200};};}};
 ` + code + `
 runSession(); execution.scenario.iterationInTest=1; runSession();
 if(calls.length!==3 || logs.length!==3 || failedChecks!==1 || counts.raw_session_failures!==1)throw new Error('failure population');
 if(calls[0].body.messages[0].content!=='hello'||calls[1].body.messages.length!==1||calls[1].body.messages[0].content!=='independent'||calls[2].body.input!=='third')throw new Error('payload/order changed');
 if(logs[0].session_id!=='first'||logs[0].turn_id!=='0'||logs[1].turn_id!=='1'||logs[2].session_id!=='second'||logs[0].outcome!=='failure')throw new Error('session identity');
 if(options.scenarios.sessions.iterations!==2||options.thresholds.checks[0]!=='rate==1')throw new Error('executor/threshold');
 `)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCommand(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "llm-import-inputs")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, "go", "build", "-o", binary, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	engine := &script.Engine{Cmds: scripttest.DefaultCmds(), Conds: scripttest.DefaultConds()}
	scripttest.Test(t, ctx, engine, []string{"PATH=" + os.Getenv("PATH"), "CLI=" + binary}, "testdata/*.txt")
}

// A pinned existing registered binary may run this loopback-only integration
// control. Its source identity must be retained separately from this patch.
func TestBinaryRunner(t *testing.T) {
	binary := os.Getenv("K6_SMOKE_BINARY")
	if binary == "" {
		t.Skip("set K6_SMOKE_BINARY for registered-binary loopback control")
	}
	var converted bytes.Buffer
	if err := run([]string{"-path", "/test", "-protocol", "json"}, strings.NewReader(fixture), &converted); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "sessions.json")
	if err := os.WriteFile(file, converted.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var bodies []json.RawMessage
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		var body json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		bodies = append(bodies, body)
		if r.URL.Path != "/test" {
			t.Errorf("path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		if len(bodies) == 1 {
			w.WriteHeader(503)
			w.Write([]byte(`{"error":"fixture"}`))
			return
		}
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "run", "--no-color", "../../examples/workloads/raw-sessions.js")
	cmd.Env = append(os.Environ(), "SESSIONS_FILE="+file, "ENDPOINT_URL="+server.URL, "VUS=1", "K6_NO_USAGE_REPORT=true", "K6_AUTO_EXTENSION_RESOLUTION=false")
	output, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 99 {
		t.Fatalf("expected k6 threshold exit 99: %v\n%s", err, output)
	}
	t.Logf("fixture k6 output:\n%s", output)
	mu.Lock()
	defer mu.Unlock()
	if len(bodies) != 3 {
		t.Fatalf("attempted %d turns: %s", len(bodies), output)
	}
	var input struct {
		Data []struct{ Payloads []json.RawMessage }
	}
	if err := json.Unmarshal([]byte(fixture), &input); err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, s := range input.Data {
		for _, payload := range s.Payloads {
			var want, got any
			if err := json.Unmarshal(payload, &want); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(bodies[n], &got); err != nil {
				t.Fatal(err)
			}
			a, _ := json.Marshal(want)
			b, _ := json.Marshal(got)
			if !bytes.Equal(a, b) {
				t.Fatalf("wire payload %d changed: %s / %s", n, a, b)
			}
			n++
		}
	}
	if !bytes.Contains(output, []byte("raw_session_failures")) || !bytes.Contains(output, []byte("session_id")) {
		t.Fatalf("missing failure/identity receipt: %s", output)
	}
	t.Logf("registered binary loopback: three wire-equal payloads, first failed, later two attempted; threshold failed as required")
}
