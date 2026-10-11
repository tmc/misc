package llmperf

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.k6.io/k6/v2/js/modulestest"
)

func ExampleEvaluateRuntimeTemplate() {
	_, err := EvaluateRuntimeTemplate(context.Background(), RuntimeTemplateOptions{})
	fmt.Println(err)
	// Output: runtime template requires absolute runtime and script paths and sha256 pin
}

func runtimeTemplateFixture(t *testing.T) RuntimeTemplateOptions {
	t.Helper()
	python := os.Getenv("RUNTIME_TEMPLATE_PYTHON")
	if python == "" {
		t.Skip("set RUNTIME_TEMPLATE_PYTHON to the pinned template-compat virtualenv interpreter")
	}
	script, err := filepath.Abs("scripts/template-compat.py")
	if err != nil {
		t.Fatal(err)
	}
	pin := func(path string) string {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256(data)
		return hex.EncodeToString(hash[:])
	}
	return RuntimeTemplateOptions{Python: python, PythonSHA256: pin(python), Script: script, ScriptSHA256: pin(script)}
}

func TestRuntimeTemplateActualLibraries(t *testing.T) {
	opts := runtimeTemplateFixture(t)
	for _, tt := range []struct {
		name, input, want string
		fail              bool
	}{
		{"render", `{"schema_version":1,"operation":"render","template":"{\"input\": [{% for text in texts %}{{ text|upper|tojson }}{% if not loop.last %},{% endif %}{% endfor %}],\"model\":{{ model|tojson }}}","variables":{"texts":["hello","world"],"model":"original"},"extra":{"model":"endpoint"},"turn_extra":{"model":"turn"}}`, `"value":{"input":["HELLO","WORLD"],"model":"turn"}`, false},
		{"query", `{"schema_version":1,"operation":"select","expression":"join(' ', results[].text)","response":{"results":[{"text":"hello"},{"text":"world"}]}}`, `"value":"hello world"`, false},
		{"reflection", `{"schema_version":1,"operation":"render","template":"{\"x\":{{ cycler.__init__.__globals__.os.name|tojson }}}","variables":{},"extra":{},"turn_extra":{}}`, "", true},
		{"duplicate", `{"schema_version":1,"operation":"select","expression":"x","response":{"x":1,"x":2}}`, "", true},
		{"nonfinite", `{"schema_version":1,"operation":"select","expression":"x","response":{"x":NaN}}`, "", true},
		{"missing", `{"schema_version":1,"operation":"select","expression":"missing","response":{}}`, "", true},
		{"null", `{"schema_version":1,"operation":"select","expression":"x","response":{"x":null}}`, "", true},
		{"expansion", `{"schema_version":1,"operation":"render","template":"{\"x\":{{ ('a' * 2000000)|tojson }}}","variables":{},"extra":{},"turn_extra":{}}`, "", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			opts.Request = json.RawMessage(tt.input)
			r, err := EvaluateRuntimeTemplate(context.Background(), opts)
			if tt.fail {
				if err == nil {
					t.Fatal("accepted invalid evaluation")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if r.ElapsedMS <= 0 || r.ScriptSHA256 != opts.ScriptSHA256 || !strings.Contains(string(r.Result), tt.want) {
				t.Fatalf("unexpected result: %+v", r)
			}
		})
	}
	opts.Request = json.RawMessage(`{"schema_version":1,"operation":"select","expression":"x","response":{"x":"ok"}}`)
	for _, field := range []string{"script", "python"} {
		bad := opts
		if field == "script" {
			bad.ScriptSHA256 = strings.Repeat("0", 64)
		} else {
			bad.PythonSHA256 = strings.Repeat("0", 64)
		}
		if _, err := EvaluateRuntimeTemplate(context.Background(), bad); err == nil {
			t.Fatal("accepted wrong " + field + " hash")
		}
	}
}

func TestRuntimeTemplateJavaScriptContract(t *testing.T) {
	opts := runtimeTemplateFixture(t)
	opts.Request = json.RawMessage(`{"schema_version":1,"operation":"select","expression":"x","response":{"x":"hello"}}`)
	encoded, err := json.Marshal(opts)
	if err != nil {
		t.Fatal(err)
	}
	rt := modulestest.NewRuntime(t)
	if err := rt.SetupModuleSystem(map[string]any{importPath: new(rootModule)}, nil, nil); err != nil {
		t.Fatal(err)
	}
	_, err = rt.RunOnEventLoop(fmt.Sprintf(`
 const {evaluateRuntimeTemplate}=require(%q);
 const opts=%s;
 const result=evaluateRuntimeTemplate(opts);
 if (result instanceof Error || result.result.value!=='hello' || result.elapsed_ms<=0 || result.script_sha256!==opts.scriptSHA256) throw new Error('runtime result');
 if (!(evaluateRuntimeTemplate({...opts,unknownOption:1}) instanceof Error)) throw new Error('unknown option accepted');
 if (!(evaluateRuntimeTemplate({...opts,scriptSHA256:'0'.repeat(64)}) instanceof Error)) throw new Error('pin ignored');
 `, importPath, encoded))
	if err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeTemplateCancellation(t *testing.T) {
	opts := runtimeTemplateFixture(t)
	opts.Request = json.RawMessage(`{"schema_version":1,"operation":"select","expression":"x","response":{"x":"ok"}}`)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	if _, err := EvaluateRuntimeTemplate(ctx, opts); err == nil {
		t.Fatal("accepted canceled evaluation")
	}
	if time.Since(start) > time.Second {
		t.Fatal("cancellation was not prompt")
	}
}

func ExampleRuntimeTemplateOptions() {
	opts := RuntimeTemplateOptions{Request: json.RawMessage(`{"schema_version":1,"operation":"select","expression":"text","response":{"text":"hello"}}`)}
	var request map[string]any
	json.Unmarshal(opts.Request, &request)
	fmt.Println(request["operation"], request["expression"])
	// Output: select text
}

func ExampleRuntimeTemplateResult() {
	result := RuntimeTemplateResult{Result: json.RawMessage(`{"value":"hello"}`), ElapsedMS: 12.5}
	var envelope struct {
		Value string `json:"value"`
	}
	json.Unmarshal(result.Result, &envelope)
	fmt.Println(envelope.Value, result.ElapsedMS)
	// Output: hello 12.5
}

func TestRuntimeTemplateStrictJSON(t *testing.T) {
	for _, data := range [][]byte{
		[]byte(`{"x":1,"x":2}`), []byte(`{"x":NaN}`), []byte("{\"x\":\"\xff\"}"), []byte(`{} {}`), []byte(strings.Repeat("[", 66) + "0" + strings.Repeat("]", 66)),
	} {
		if strictRuntimeJSON(data) == nil {
			t.Fatal("accepted invalid JSON")
		}
	}
	if err := strictRuntimeJSON([]byte(`{"x":[0,false,null,"hello"]}`)); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeTemplateCopyLimit(t *testing.T) {
	for _, size := range []int{runtimeTemplateLimit, runtimeTemplateLimit + 1} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "output")
			if err := os.WriteFile(path, bytes.Repeat([]byte("x"), size), 0600); err != nil {
				t.Fatal(err)
			}
			input, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer input.Close()
			var output runtimeTemplateOutput
			n, err := io.Copy(&output, input)
			if (err == nil) != (size == runtimeTemplateLimit) || output.Len() > runtimeTemplateLimit {
				t.Fatalf("size=%d copied=%d retained=%d error=%v", size, n, output.Len(), err)
			}
			if err == nil && !bytes.Equal(output.Bytes(), bytes.Repeat([]byte("x"), size)) {
				t.Fatal("output differs")
			}
		})
	}
}
