package llmperf

import "testing"

func TestTGIGenerationAndIncrementalFinalToken(t *testing.T) {
	for _, body := range []string{`{"generated_text":"hello","details":{"generated_tokens":1}}`, `[{"generated_text":"hello","details":{"generated_tokens":1}}]`} {
		r, err := decodeResponse([]byte(body), "tgi")
		if err != nil {
			t.Fatal(err)
		}
		if r.Choices[0].Message.Content != "hello" || !r.Usage.completionPresent || r.Usage.CompletionTokens != 1 {
			t.Fatalf("response=%+v", r)
		}
	}
	chunk, complete, err := decodeChunk([]byte(`{"token":{"text":"llo"},"generated_text":"hello","details":{"generated_tokens":2}}`), "tgi")
	if err != nil {
		t.Fatal(err)
	}
	if !complete || chunk.Choices[0].Delta.Content != "llo" || chunk.Usage.CompletionTokens != 2 {
		t.Fatalf("final chunk duplicated text: %+v", chunk)
	}
	if _, err := decodeResponse([]byte(`{"error":"fixture"}`), "tgi"); err == nil {
		t.Fatal("accepted provider error")
	}
}
