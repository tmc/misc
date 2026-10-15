package llmperf

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/grafana/sobek"
	"go.k6.io/k6/v2/metrics"
)

func TestCompletionExtensionsWire(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		writeCompletion(t, w)
	}))
	defer server.Close()
	client := setupTestClient(t, server.URL)
	client.metrics.samples = make(chan metrics.SampleContainer, 200)
	rt := client.vu.Runtime()
	request, err := rt.RunString(`({model:'fixture',messages:[{role:'user',content:'hi'}],max_tokens:8,extensions:{ignore_eos:true,min_tokens:8,chat_template_kwargs:{enable_thinking:false}}})`)
	if err != nil {
		t.Fatal(err)
	}
	result := client.Complete(sobek.FunctionCall{Arguments: []sobek.Value{request}})
	if err, ok := result.Export().(error); ok {
		t.Fatal(err)
	}
	if body["ignore_eos"] != true || body["min_tokens"] != float64(8) || body["max_tokens"] != float64(8) {
		t.Fatalf("body = %v; want extension and typed fields", body)
	}
	if kwargs, _ := body["chat_template_kwargs"].(map[string]any); kwargs["enable_thinking"] != false {
		t.Fatalf("chat_template_kwargs = %v", body["chat_template_kwargs"])
	}
	if _, ok := body["extensions"]; ok {
		t.Fatalf("body carries the extensions envelope: %v", body)
	}
	if messages, _ := body["messages"].([]any); len(messages) != 1 {
		t.Fatalf("messages = %v", body["messages"])
	}
}

func TestCompletionExtensionsRejected(t *testing.T) {
	tests := []struct {
		name    string
		request string
		want    string
	}{
		{"messages", `{messages:[{role:'user',content:'hi'}],extensions:{messages:[]}}`, `extension field "messages"`},
		{"stream", `{messages:[{role:'user',content:'hi'}],extensions:{stream:true}}`, `extension field "stream"`},
		{"model", `{messages:[{role:'user',content:'hi'}],extensions:{model:'other'}}`, `extension field "model"`},
		{"case alias", `{messages:[{role:'user',content:'hi'}],extensions:{Max_Tokens:4}}`, `extension field "Max_Tokens"`},
		{"long s alias", `{messages:[{role:'user',content:'hi'}],extensions:{'\u017ftream':false}}`, "names a typed request field"},
		{"kelvin alias", `{messages:[{role:'user',content:'hi'}],extensions:{'max_to\u212aens':9}}`, "names a typed request field"},
		{"undefined", `{messages:[{role:'user',content:'hi'}],extensions:{ignore_eos:undefined}}`, `extension field "ignore_eos" is null or undefined`},
		{"null", `{messages:[{role:'user',content:'hi'}],extensions:{ignore_eos:null}}`, `extension field "ignore_eos" is null or undefined`},
		{"control field", `{messages:[{role:'user',content:'hi'}],extensions:{work:{}}}`, `extension field "work"`},
		{"self", `{messages:[{role:'user',content:'hi'}],extensions:{extensions:{}}}`, `extension field "extensions"`},
		{"empty key", `{messages:[{role:'user',content:'hi'}],extensions:{'':1}}`, `extension field ""`},
		{"raw", `{raw:{path:'/chat/completions',protocol:'openai-chat',body:{}},extensions:{ignore_eos:true}}`, "extensions require a typed request"},
		{"not an object", `{messages:[{role:'user',content:'hi'}],extensions:[true]}`, "invalid completion request"},
		{"unknown typed field", `{messages:[{role:'user',content:'hi'}],ignore_eos:true}`, "invalid completion request"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requests atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				writeCompletion(t, w)
			}))
			defer server.Close()
			client := setupTestClient(t, server.URL)
			client.metrics.samples = make(chan metrics.SampleContainer, 200)
			rt := client.vu.Runtime()
			request, err := rt.RunString("(" + tt.request + ")")
			if err != nil {
				t.Fatal(err)
			}
			err, ok := client.Complete(sobek.FunctionCall{Arguments: []sobek.Value{request}}).Export().(error)
			if !ok || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v; want substring %q", err, tt.want)
			}
			if n := requests.Load(); n != 0 {
				t.Fatalf("server received %d requests; want 0", n)
			}
		})
	}
}
