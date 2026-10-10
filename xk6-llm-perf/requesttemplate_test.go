package llmperf

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func ExampleRenderRequestTemplate() {
	rendered, err := RenderRequestTemplate(RequestTemplate{Name: "openai-embeddings", Variables: map[string]any{"model": "local", "texts": []string{"hello"}}})
	if err != nil {
		panic(err)
	}
	data, _ := json.Marshal(rendered.Body)
	fmt.Println(string(data))
	fmt.Println(rendered.Provenance.Scope)
	// Output:
	// {"input":["hello"],"model":"local"}
	// structured-json-whole-values-v1
}

func TestRequestTemplateSubstitution(t *testing.T) {
	spec := RequestTemplate{Body: map[string]any{"text": map[string]any{"$var": "text"}, "number": map[string]any{"$var": "number"}, "literal": "{{ text }}", "nested": []any{map[string]any{"$var": "data"}}}, Variables: map[string]any{"text": "quotes \"\n<&", "number": 0, "data": map[string]any{"$var": "untouched"}}}
	got, err := RenderRequestTemplate(spec)
	if err != nil {
		t.Fatal(err)
	}
	if got.Body["text"] != spec.Variables["text"] || got.Body["number"] != float64(0) || got.Body["literal"] != "{{ text }}" {
		t.Fatalf("type or literal loss: %v", got.Body)
	}
	nested := got.Body["nested"].([]any)[0].(map[string]any)
	if nested["$var"] != "untouched" {
		t.Fatal("variable data executed")
	}
	got.Body["text"] = "changed"
	again, err := RenderRequestTemplate(spec)
	if err != nil || again.Provenance != got.Provenance {
		t.Fatalf("unstable receipt: %v", err)
	}
	spec.Variables["number"] = 1
	changed, err := RenderRequestTemplate(spec)
	if err != nil || changed.Provenance.TemplateSHA256 != got.Provenance.TemplateSHA256 || changed.Provenance.PayloadSHA256 == got.Provenance.PayloadSHA256 || changed.Provenance.VariablesSHA256 == got.Provenance.VariablesSHA256 {
		t.Fatalf("receipt did not distinguish variables: %v", err)
	}
}

func TestRequestTemplateRejects(t *testing.T) {
	deep := any("leaf")
	for i := 0; i < 35; i++ {
		deep = []any{deep}
	}
	tests := []struct {
		name string
		spec RequestTemplate
	}{
		{"missing", RequestTemplate{}},
		{"both", RequestTemplate{Name: "nv-embedqa", Body: map[string]any{}}},
		{"unknown name", RequestTemplate{Name: "no-such-template"}},
		{"missing variable", RequestTemplate{Body: map[string]any{"a": map[string]any{"$var": "missing"}}}},
		{"ambiguous reference", RequestTemplate{Body: map[string]any{"a": map[string]any{"$var": "x", "other": 1}}}},
		{"root scalar", RequestTemplate{Body: map[string]any{"$var": "x"}, Variables: map[string]any{"x": 1}}},
		{"oversized", RequestTemplate{Body: map[string]any{"a": strings.Repeat("x", maxTemplateBytes)}}},
		{"deep", RequestTemplate{Body: map[string]any{"a": deep}}},
		{"deep variable", RequestTemplate{Body: map[string]any{"a": map[string]any{"$var": "x"}}, Variables: map[string]any{"x": deep}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := RenderRequestTemplate(tt.spec); err == nil {
				t.Fatal("accepted invalid template")
			}
		})
	}
}

func endpointFixtures() []struct{ protocol, name, body, response string } {
	return []struct{ protocol, name, body, response string }{
		{"chat-embeddings", "chat-embeddings", `{"model":"fixture","messages":[{"role":"user","content":[{"type":"text","text":"hello"},{"type":"image_url","image_url":{"url":"data:image/png;base64,aGVsbG8="}}]}]}`, `{"embedding":[0.1,0.2]}`},
		{"nim-embeddings", "nim-embeddings", `{"model":"fixture","input":["hello data:image/png;base64,aGVsbG8="]}`, `{"data":[{"object":"embedding","index":0,"embedding":[0.1,0.2]}]}`},
		{"solido-rag", "solido-rag", `{"query":["hello"],"filters":{"family":"Solido","tool":"SDE"},"inference_model":"fixture"}`, `{"content":"hello world","sources":[{"document":"fixture.pdf"}],"usage":{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}}`},
		{"nv-embedqa", "nv-embedqa", `{"text":["hello"]}`, `{"embeddings":[[0.1,0.2]]}`},
		{"openai-embeddings", "openai-embeddings", `{"model":"fixture","input":["hello"]}`, `{"data":[{"object":"embedding","index":0,"embedding":[0.1,0.2]}],"usage":{"prompt_tokens":2,"total_tokens":2}}`},
		{"cohere-rankings", "cohere-rankings", `{"model":"fixture","query":"hello","documents":["world"]}`, `{"results":[{"index":0,"relevance_score":0.9}]}`},
		{"nim-rankings", "nim-rankings", `{"model":"fixture","query":{"text":"hello"},"passages":[{"text":"world"}]}`, `{"rankings":[{"index":0,"score":0.9}]}`},
		{"hf-tei-rankings", "hf-tei-rankings", `{"query":"hello","texts":["world"]}`, `[{"index":0,"score":0.9}]`},
		{"nim-image-retrieval", "nim-image-retrieval", `{"input":[{"type":"image_url","url":"data:image/png;base64,aGVsbG8="}]}`, `{"data":[{"index":0,"embedding":[0.1,0.2]}]}`},
	}
}

func TestNamedEndpointShapes(t *testing.T) {
	for _, fixture := range endpointFixtures() {
		t.Run(fixture.protocol, func(t *testing.T) {
			var body map[string]any
			if err := json.Unmarshal([]byte(fixture.body), &body); err != nil {
				t.Fatal(err)
			}
			if err := validateEndpointBody(fixture.protocol, body); err != nil {
				t.Fatal(err)
			}
			var response CompletionResponse
			var err error
			if fixture.protocol == "solido-rag" {
				response, err = decodeSolidoResponse([]byte(fixture.response))
			} else {
				response, err = decodeNonGeneration([]byte(fixture.response), fixture.protocol)
			}
			if err != nil {
				t.Fatal(err)
			}
			if fixture.protocol != "solido-rag" && (len(response.Choices) != 0 || response.Usage.completionPresent) {
				t.Fatal("invented generated tokens")
			}
			if len(response.Body) == 0 {
				t.Fatal("lost raw response")
			}
			body["stream"] = true
			if err := validateEndpointBody(fixture.protocol, body); err == nil && fixture.protocol != "solido-rag" {
				t.Fatal("accepted streaming")
			}
			if _, err := decodeNonGeneration([]byte(`{"unexpected":[]}`), fixture.protocol); err == nil {
				t.Fatal("accepted absent result")
			}
		})
	}
}

func TestNonGenerationRejects(t *testing.T) {
	for _, response := range []string{
		`{"results":[{"index":0,"relevance_score":0.9},{"index":0,"relevance_score":0.2}]}`,
		`{"results":[{"index":-1,"relevance_score":0.9}]}`,
		`{"results":[{"index":0,"relevance_score":"0.9"}]}`,
		`{"error":{"message":"bad"},"results":[{"index":0,"relevance_score":0.9}]}`,
	} {
		if _, err := decodeNonGeneration([]byte(response), "cohere-rankings"); err == nil {
			t.Fatalf("accepted %s", response)
		}
	}
	if _, err := decodeNonGeneration([]byte(`{"data":[{"index":0,"object":"embedding","embedding":["0.1"]}]}`), "openai-embeddings"); err == nil {
		t.Fatal("accepted nonnumeric embedding")
	}
}

func TestNamedRequestTemplates(t *testing.T) {
	variables := map[string]any{"model": "fixture", "texts": []string{"hello"}, "query": "hello", "queries": []string{"hello"}, "messages": []map[string]any{{"role": "user", "content": []map[string]any{{"type": "text", "text": "hello"}, {"type": "image_url", "image_url": map[string]any{"url": "data:image/png;base64,aGVsbG8="}}}}}, "passages": []string{"world"}, "images": []map[string]any{{"type": "image_url", "url": "data:image/png;base64,aGVsbG8="}}}
	for _, fixture := range endpointFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			vars := make(map[string]any)
			for key, value := range variables {
				vars[key] = value
			}
			if fixture.name == "nim-embeddings" {
				vars["images"] = []string{"data:image/png;base64,aGVsbG8="}
			}
			if fixture.name == "nim-rankings" {
				vars["passages"] = []map[string]any{{"text": "world"}}
			}
			got, err := RenderRequestTemplate(RequestTemplate{Name: fixture.name, Variables: vars})
			if err != nil {
				t.Fatal(err)
			}
			var want map[string]any
			if err := json.Unmarshal([]byte(fixture.body), &want); err != nil {
				t.Fatal(err)
			}
			gotJSON, _ := json.Marshal(got.Body)
			wantJSON, _ := json.Marshal(want)
			if string(gotJSON) != string(wantJSON) {
				t.Fatalf("got %s want %s", gotJSON, wantJSON)
			}
		})
	}
	got, err := RenderRequestTemplate(RequestTemplate{Name: "nv-embedqa", Variables: variables})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(got.Body)
	if string(data) != `{"text":["hello"]}` {
		t.Fatalf("nv-embedqa body: %s", data)
	}
}

func TestEmbeddingInputForms(t *testing.T) {
	for _, input := range []string{`"hello"`, `["hello","world"]`, `[0,1,2147483647]`, `[[0,1],[2]]`} {
		var body map[string]any
		if err := json.Unmarshal([]byte(`{"model":"fixture","encoding_format":"base64","input":`+input+`}`), &body); err != nil {
			t.Fatal(err)
		}
		if err := validateEndpointBody("openai-embeddings", body); err != nil {
			t.Fatalf("%s: %v", input, err)
		}
	}
	for _, input := range []string{`[]`, `[[]]`, `[1,"x"]`, `[[1],2]`, `[-1]`, `[1.1]`, `[2147483648]`, `[""]`, `null`} {
		var body map[string]any
		if err := json.Unmarshal([]byte(`{"model":"fixture","input":`+input+`}`), &body); err != nil {
			t.Fatal(err)
		}
		if err := validateEndpointBody("openai-embeddings", body); err == nil {
			t.Fatalf("accepted %s", input)
		}
	}
	if err := validateEndpointBody("openai-embeddings", map[string]any{"model": "fixture", "input": "hello", "encoding_format": "other"}); err == nil {
		t.Fatal("accepted unknown encoding")
	}
}

func TestEmbeddingBase64(t *testing.T) {
	response := `{"data":[{"object":"embedding","index":0,"embedding":"AACAPwAAAEA="}],"usage":{"prompt_tokens":2,"total_tokens":2}}`
	got, err := decodeNonGeneration([]byte(response), "openai-embeddings")
	if err != nil {
		t.Fatal(err)
	}
	if string(got.Body) != response || got.Usage.completionPresent {
		t.Fatal("lost encoded body or invented generation count")
	}
	for _, text := range []string{"", "!", "AQ==", "AACAfw==", "AADAfw==", "AACAPw==\n", "AACAPx=="} {
		data, _ := json.Marshal(map[string]any{"data": []map[string]any{{"object": "embedding", "index": 0, "embedding": text}}})
		if _, err := decodeNonGeneration(data, "openai-embeddings"); err == nil {
			t.Fatalf("accepted %q", text)
		}
	}
}

func TestNVEmbedQAResponses(t *testing.T) {
	for _, response := range []string{`{"embedding":[0.1,0.2]}`, `{"embeddings":[[0.1],[0.2]]}`, `{"data":[{"object":"embedding","index":0,"embedding":[0.1]}]}`} {
		got, err := decodeNonGeneration([]byte(response), "nv-embedqa")
		if err != nil || len(got.Choices) > 0 || got.Usage.completionPresent {
			t.Fatalf("%s: %v", response, err)
		}
	}
	for _, response := range []string{`{"embedding":[]}`, `{"embeddings":[[0.1],["bad"]]}`, `{"data":[{"embedding":[0.1]}]}`, `{"text":"hello"}`} {
		if _, err := decodeNonGeneration([]byte(response), "nv-embedqa"); err == nil {
			t.Fatalf("accepted %s", response)
		}
	}
}

func TestNIMEmbeddingInputComposition(t *testing.T) {
	tests := []struct {
		name      string
		variables map[string]any
		want      string
	}{
		{"text", map[string]any{"texts": []string{"hello"}}, `{"input":["hello"],"model":"fixture"}`},
		{"image", map[string]any{"images": []string{"data:image/png;base64,aGVsbG8="}}, `{"input":["data:image/png;base64,aGVsbG8="],"model":"fixture"}`},
		{"paired", map[string]any{"texts": []string{"hello"}, "images": []string{"data:image/png;base64,aGVsbG8="}}, `{"input":["hello data:image/png;base64,aGVsbG8="],"model":"fixture"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.variables["model"] = "fixture"
			got, err := RenderRequestTemplate(RequestTemplate{Name: "nim-embeddings", Variables: tt.variables})
			if err != nil {
				t.Fatal(err)
			}
			data, _ := json.Marshal(got.Body)
			if string(data) != tt.want {
				t.Fatalf("got %s want %s", data, tt.want)
			}
		})
	}
	for _, variables := range []map[string]any{
		{"model": "fixture"},
		{"model": "fixture", "texts": []string{"hello", "world"}, "images": []string{"image"}},
		{"model": "fixture", "texts": "hello"},
		{"model": "fixture", "images": []any{1}},
	} {
		if _, err := RenderRequestTemplate(RequestTemplate{Name: "nim-embeddings", Variables: variables}); err == nil {
			t.Fatalf("accepted %v", variables)
		}
	}
}

func TestChatEmbeddingContentShapes(t *testing.T) {
	for _, messages := range []string{`[]`, `[{"role":"user","content":[]}]`, `[{"role":"user","content":[{"type":"image_url","image_url":"url"}]}]`, `[{"content":"hello"}]`, `[{"role":"user","content":[{"type":"unknown"}]}]`} {
		var body map[string]any
		if err := json.Unmarshal([]byte(`{"model":"fixture","messages":`+messages+`}`), &body); err != nil {
			t.Fatal(err)
		}
		if err := validateEndpointBody("chat-embeddings", body); err == nil {
			t.Fatalf("accepted %s", messages)
		}
	}
	for _, response := range []string{`{"embedding":[0.1]}`, `{"embeddings":[[0.1],[0.2]]}`, `{"data":[{"object":"embedding","index":0,"embedding":[0.1]}]}`} {
		result, err := decodeNonGeneration([]byte(response), "chat-embeddings")
		if err != nil || len(result.Choices) != 0 || result.Usage.completionPresent {
			t.Fatalf("response %s: %v", response, err)
		}
	}
}

func TestSolidoResponseAndStream(t *testing.T) {
	result, err := decodeSolidoResponse([]byte(`{"content":"generated","sources":{"fixture.pdf":{"pages":{"1":"context"}}},"usage":{"prompt_tokens":2,"completion_tokens":3}}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Choices) != 1 || result.Choices[0].Message.Content != "generated" || len(result.Sources) == 0 || !result.Usage.completionPresent || result.Usage.CompletionTokens != 3 {
		t.Fatal("lost generated content, usage, or sources")
	}
	chunk, terminal, err := decodeSolidoChunk([]byte(`{"content":"delta","sources":[{"document":"first.pdf"}]}`))
	if err != nil || terminal || len(chunk.Choices) != 1 || chunk.Choices[0].Delta.Content != "delta" || len(chunk.Sources) == 0 {
		t.Fatalf("bad stream delta: %v", err)
	}
	trailer, terminal, err := decodeSolidoChunk([]byte(`{"sources":[{"document":"last.pdf"}],"usage":{"completion_tokens":3}}`))
	if err != nil || terminal || len(trailer.Choices) != 0 || !trailer.Usage.completionPresent {
		t.Fatalf("metadata trailer fabricated content/termination: %v", err)
	}
	for _, response := range []string{`{}`, `{"content":""}`, `{"content":1}`, `{"content":"generated","sources":"bad"}`, `{"content":"generated","error":{"message":"failure"}}`} {
		if _, err := decodeSolidoResponse([]byte(response)); err == nil {
			t.Fatalf("accepted %s", response)
		}
	}
	for _, response := range []string{`{}`, `{"content":1}`, `{"sources":"bad"}`, `{"error":{"message":"failure"}}`} {
		if _, _, err := decodeSolidoChunk([]byte(response)); err == nil {
			t.Fatalf("accepted stream %s", response)
		}
	}
}
