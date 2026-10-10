package llmperf

import "testing"

func TestStreamChoices(t *testing.T) {
	s := streamChoices{}
	for _, raw := range []string{
		`{"choices":[{"index":1,"delta":{"content":"other"}},{"index":0,"delta":{"content":"first"}}]}`,
		`{"choices":[{"index":1,"delta":{"content":" choice"},"finish_reason":"stop"},{"index":0,"delta":{"content":" choice"},"finish_reason":"length"}]}`,
	} {
		chunk, _, err := decodeChunk([]byte(raw), "openai-chat")
		if err != nil {
			t.Fatal(err)
		}
		if err := s.add(chunk); err != nil {
			t.Fatal(err)
		}
	}
	got := s.choices()
	if len(got) != 2 || got[0].Index != 0 || got[1].Index != 1 || got[0].Message.Content != "first choice" || got[1].Message.Content != "other choice" || got[1].FinishReason != "stop" {
		t.Fatalf("choices = %+v", got)
	}
	for _, raw := range []string{`{"choices":[{"index":128}]}`, `{"choices":[{"index":0},{"index":0}]}`} {
		chunk, _, _ := decodeChunk([]byte(raw), "openai-chat")
		if err := s.add(chunk); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}
