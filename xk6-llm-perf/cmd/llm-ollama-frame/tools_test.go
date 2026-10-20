package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/tmc/misc/xk6-llm-perf/internal/jsoncheck"
)

// The tool-framing fixture holds OpenAI requests and the prompts emitted for
// them by the actual pinned openai.FromChatRequest, ChatMiddleware JSON round
// trip, qwen3 filterThinkTags order and template.Execute with Tools. Refusal
// rows retain the oracle's own result: the prompt pinned Ollama renders after
// silently changing the request, or its conversion error.
func TestPinnedSourceToolFraming(t *testing.T) {
	data, err := os.ReadFile("testdata/tool-framing.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != "2268def02ea646e4567238f8143ed1231a02d872cfe8eaca9b7993ebbe068b29" {
		t.Fatalf("tool fixture sha256 %s", got)
	}
	tmpl, err := os.ReadFile("testdata/qwen3-ollama-0.12.7.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	ids := loadPromptIDs(t, "testdata/qwen3-tool-prompt-ids.jsonl", "5cdab02686f477a2cf7e7ecc4c52ce20f6e06261c75c47642cab759a7738154d")
	refusals := map[string]error{
		"refuse-additional-properties":    errToolSchema,
		"refuse-strict":                   errToolSchema,
		"refuse-single-type-array":        errToolSchema,
		"refuse-missing-required":         errToolSchema,
		"refuse-nested-properties":        errToolSchema,
		"refuse-format":                   errToolSchema,
		"refuse-empty-description":        errToolSchema,
		"refuse-folded-key":               errToolSchema,
		"refuse-inexact-enum":             errInexactNumber,
		"refuse-tool-type":                errToolType,
		"refuse-tool-items":               errToolSchema,
		"refuse-content-with-calls":       errContentWithCalls,
		"refuse-merge-before-calls":       errMergedToolCalls,
		"refuse-merge-after-calls":        errMergedToolCalls,
		"refuse-user-calls":               errToolCallRole,
		"refuse-tool-calls-on-tool":       errToolCallRole,
		"refuse-inexact-argument":         errInexactNumber,
		"refuse-large-integer-argument":   errInexactNumber,
		"refuse-empty-arguments":          errToolArguments,
		"refuse-null-arguments":           errToolArguments,
		"refuse-array-arguments":          errToolArguments,
		"refuse-duplicate-arguments":      errToolArguments,
		"refuse-surrogate-arguments":      errLoneSurrogate,
		"refuse-call-type":                errToolCallType,
		"refuse-empty-call-name":          errToolCallName,
		"refuse-null-content-no-calls":    jsoncheck.ErrNull,
		"refuse-null-content-empty-calls": jsoncheck.ErrNull,
		"refuse-null-tool-content":        jsoncheck.ErrNull,
		"refuse-refusal-string":           errEchoField,
		"refuse-name-on-user":             errEchoField,
		"refuse-annotations-on-user":      errEchoField,
		"refuse-duplicate-property":       jsoncheck.ErrDuplicate,
		"refuse-folded-tools-member":      jsoncheck.ErrDuplicate,
	}
	var supported, refused int
	shas := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		var tt struct {
			Name               string
			System             string
			Defaults           json.RawMessage
			Request            json.RawMessage
			Prompt             *string
			OraclePromptSHA256 string `json:"oracle_prompt_sha256"`
			OracleError        string `json:"oracle_error"`
		}
		if err := json.Unmarshal([]byte(line), &tt); err != nil {
			t.Fatal(err)
		}
		t.Run(tt.Name, func(t *testing.T) {
			system, _ := json.Marshal(tt.System)
			s := show{Template: string(tmpl), Capabilities: []string{"completion", "tools", "thinking"}, System: system, Messages: tt.Defaults}
			var r request
			err := decodeRequest(tt.Request, &r)
			var got string
			if err == nil {
				got, _, err = render(s, r)
			}
			if want, ok := refusals[tt.Name]; ok {
				refused++
				if !errors.Is(err, want) {
					t.Fatalf("err = %v, want %v", err, want)
				}
				if tt.Prompt != nil || tt.OraclePromptSHA256 == "" && tt.OracleError == "" {
					t.Fatal("refusal row lacks oracle result")
				}
				return
			}
			supported++
			if err != nil || tt.Prompt == nil || got != *tt.Prompt {
				t.Fatalf("got %q err=%v; want %q", got, err, *tt.Prompt)
			}
			if h := fmt.Sprintf("%x", sha256.Sum256([]byte(got))); h != tt.OraclePromptSHA256 {
				t.Fatalf("prompt sha256 %s, oracle %s", h, tt.OraclePromptSHA256)
			}
			checkPromptIDs(t, got, ids[tt.Name])
			shas[tt.Name] = tt.OraclePromptSHA256
		})
	}
	// OpenAI SDK echo fields leave the prompt unchanged.
	for _, think := range []string{"default", "none"} {
		if shas["sdk-echo-"+think] != shas["call-result-"+think] {
			t.Errorf("sdk-echo-%s prompt differs from call-result-%s", think, think)
		}
	}
	if supported != len(ids) || refused != len(refusals) {
		t.Fatalf("ran %d supported and %d refused, want %d and %d", supported, refused, len(ids), len(refusals))
	}
}

func TestToolAdmission(t *testing.T) {
	tmpl, err := os.ReadFile("testdata/qwen3-ollama-0.12.7.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	const tools = `[{"type":"function","function":{"name":"f","parameters":{"type":"object","required":[],"properties":{}}}}]`
	call := `{"role":"assistant","content":"","tool_calls":[{"id":"c","type":"function","function":{"name":"f","arguments":"{}"}}]}`
	// A nil want with refuse set accepts any refusal from strict decoding or
	// existing text admission.
	for _, tt := range []struct {
		name, request, defaults string
		capabilities            []string
		refuse                  bool
		want                    error
	}{
		{"tools without capability", `{"model":"qwen3:0.6b","messages":[{"role":"user","content":"hi"}],"tools":` + tools + `}`, "", []string{"thinking"}, true, errToolsCapability},
		{"history without capability", `{"model":"qwen3:0.6b","messages":[{"role":"user","content":"hi"},` + call + `]}`, "", []string{"thinking"}, false, nil},
		{"tool call id on user", `{"model":"qwen3:0.6b","messages":[{"role":"user","content":"hi","tool_call_id":"c"}]}`, "", nil, true, errToolCallID},
		{"content surrogate", `{"model":"qwen3:0.6b","messages":[{"role":"user","content":"\udc00"}]}`, "", nil, true, errLoneSurrogate},
		{"content surrogate pair", `{"model":"qwen3:0.6b","messages":[{"role":"user","content":"\ud83d\ude00"}]}`, "", nil, false, nil},
		{"tool budget", `{"model":"qwen3:0.6b","messages":[{"role":"user","content":"hi"}],"tools":[` + strings.Repeat(tools[1:len(tools)-1]+",", 64) + tools[1:] + `}`, "", nil, true, errToolBudgetExceeded},
		{"argument budget", `{"model":"qwen3:0.6b","messages":[{"role":"user","content":"hi"},{"role":"assistant","content":"","tool_calls":[{"id":"c","type":"function","function":{"name":"f","arguments":"{\"a\":\"` + strings.Repeat("x", 65536) + `\"}"}}]}]}`, "", nil, true, errToolBudgetExceeded},
		{"omitted content", `{"model":"qwen3:0.6b","messages":[{"role":"user"}]}`, "", nil, true, nil},
		{"null tools", `{"model":"qwen3:0.6b","messages":[{"role":"user","content":"hi"}],"tools":null}`, "", nil, true, nil},
		{"call index", `{"model":"qwen3:0.6b","messages":[{"role":"user","content":"hi"},{"role":"assistant","content":"","tool_calls":[{"index":0,"type":"function","function":{"name":"f","arguments":"{}"}}]}]}`, "", nil, false, nil},
		{"tool message name", `{"model":"qwen3:0.6b","messages":[{"role":"tool","content":"r","name":"f"}]}`, "", nil, false, nil},
		{"call index not integer", `{"model":"qwen3:0.6b","messages":[{"role":"user","content":"hi"},{"role":"assistant","content":"","tool_calls":[{"index":1.5,"type":"function","function":{"name":"f","arguments":"{}"}}]}]}`, "", nil, true, nil},
		{"annotations not array", `{"model":"qwen3:0.6b","messages":[{"role":"user","content":"hi"},{"role":"assistant","content":"a","annotations":{}}]}`, "", nil, true, nil},
		{"null annotation entries", `{"model":"qwen3:0.6b","messages":[{"role":"user","content":"hi"},{"role":"assistant","content":"a","annotations":[null]}]}`, "", nil, false, nil},
		{"null refusal on user", `{"model":"qwen3:0.6b","messages":[{"role":"user","content":"hi","refusal":null}]}`, "", nil, true, jsoncheck.ErrNull},
		{"null name on tool", `{"model":"qwen3:0.6b","messages":[{"role":"tool","content":"r","name":null}]}`, "", nil, true, jsoncheck.ErrNull},
		{"null temperature", `{"model":"qwen3:0.6b","messages":[{"role":"user","content":"hi"}],"temperature":null}`, "", nil, true, jsoncheck.ErrNull},
		{"folded content alias", `{"model":"qwen3:0.6b","messages":[{"role":"user","content":"hi","Content":"x"}]}`, "", nil, true, jsoncheck.ErrDuplicate},
		{"folded tool call field", `{"model":"qwen3:0.6b","messages":[{"role":"user","content":"hi"},{"role":"assistant","content":"","tool_calls":[{"type":"function","Type":"function","function":{"name":"f","arguments":"{}"}}]}]}`, "", nil, true, jsoncheck.ErrDuplicate},
		{"folded schema field", `{"model":"qwen3:0.6b","messages":[{"role":"user","content":"hi"}],"tools":[{"type":"function","function":{"name":"f","Name":"g","parameters":{"type":"object","required":[],"properties":{}}}}]}`, "", nil, true, errToolSchema},
		{"default tool role", `{"model":"qwen3:0.6b","messages":[{"role":"user","content":"hi"}]}`, `[{"role":"TOOL","content":"r"}]`, nil, true, nil},
		{"default tool calls", `{"model":"qwen3:0.6b","messages":[{"role":"user","content":"hi"}]}`, `[{"role":"assistant","content":"","tool_calls":[{"function":{"name":"f","arguments":{}}}]}]`, nil, true, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			caps := tt.capabilities
			if caps == nil {
				caps = []string{"completion", "tools", "thinking"}
			}
			s := show{Template: string(tmpl), Capabilities: caps, Messages: json.RawMessage(tt.defaults)}
			var r request
			err := decodeRequest([]byte(tt.request), &r)
			if err == nil {
				_, _, err = render(s, r)
			}
			if !tt.refuse {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || tt.want != nil && !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestExactFloat64(t *testing.T) {
	for _, tt := range []struct {
		in   string
		want bool
	}{
		{"0", true}, {"-0", true}, {"1.0", true}, {"1e3", true}, {"2.5", true}, {"-0.25", true},
		{"9007199254740992", true}, {"9007199254740993", false},
		{"0.1", false}, {"1e400", false}, {"1e-400", false}, {"1e-100000000", false}, {"1e100000000", false},
		{"5e-324", false}, {"4.940656458412465441765687928682213723650598026143247644255856825006755072702087518652998363616359923797965646954457177309266567103559397963987747960107818781263007131903114045278458171678489821036887186360569987307230500063874091535649843873124733972731696151400317153853980741262385655911710266585566867681870395603106249319452715914924553293054565444011274801297099995419319894090804165633245247571478690147267801593552386115501348035264934720193790268107107491703332226844753335720832431936092382893458368060106011506169809753078342277318329247904982524730776375927247874656084778203734469699533647017972677717585125660551199131504891101451037862738167250955837389733598993664809941164205702637090279242767544565229087538682506419718265533447265625e-324", true},
	} {
		if got := exactFloat64(tt.in); got != tt.want {
			t.Errorf("exactFloat64(%s) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestLoneSurrogate(t *testing.T) {
	for _, tt := range []struct {
		in   string
		want bool
	}{
		{`"plain"`, false}, {`"\u00e9"`, false}, {`"\ud83d\ude00"`, false}, {`"\\ud800"`, false},
		{`"\ud800"`, true}, {`"\udc00"`, true}, {`"\ud800\u0041"`, true}, {`"\ud800x"`, true}, {`"\\\ud800"`, true},
	} {
		if got := loneSurrogate([]byte(tt.in)); got != tt.want {
			t.Errorf("loneSurrogate(%s) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

// Tool schemas print through the mirrored pointer String methods, which
// json.Marshal with sorted maps, HTML escaping and single-type strings.
func TestToolFunctionString(t *testing.T) {
	var f toolFunction
	in := `{"name":"f","description":"<a> & b","parameters":{"type":"object","required":["z"],"properties":{"z":{"type":["string","null"]},"a":{"type":"integer","enum":[1,2.5]}}}}`
	if err := json.Unmarshal([]byte(in), &f); err != nil {
		t.Fatal(err)
	}
	want := `{"name":"f","description":"\u003ca\u003e \u0026 b","parameters":{"type":"object","required":["z"],"properties":{"a":{"type":"integer","enum":[1,2.5]},"z":{"type":["string","null"]}}}}`
	if got := f.String(); got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
	args := toolCallFunctionArguments{"b": 1.0, "a": "<x>"}
	if got := args.String(); got != `{"a":"\u003cx\u003e","b":1}` {
		t.Fatalf("arguments %s", got)
	}
	if got := (propertyType{"string"}).String(); got != "string" {
		t.Fatalf("property type %s", got)
	}
}
