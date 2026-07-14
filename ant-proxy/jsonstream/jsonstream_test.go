package jsonstream_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tmc/misc/ant-proxy/jsonstream"
)

func TestStreamArray(t *testing.T) {
	data := `[{"id":1,"text":"first"},{"id":2,"text":"second"},{"id":3,"text":"third"}]`
	r := strings.NewReader(data)

	var chunks []map[string]interface{}
	for msg, err := range jsonstream.Stream(r) {
		if err != nil {
			t.Fatal(err)
		}
		var chunk map[string]interface{}
		if err := json.Unmarshal(msg, &chunk); err != nil {
			t.Fatal(err)
		}
		chunks = append(chunks, chunk)
	}

	if len(chunks) != 3 {
		t.Fatalf("got %d chunks, want 3", len(chunks))
	}
	if chunks[0]["id"].(float64) != 1 || chunks[0]["text"] != "first" {
		t.Errorf("first chunk wrong: %v", chunks[0])
	}
}

func TestStreamNDJSON(t *testing.T) {
	data := "{\"id\":1,\"text\":\"first\"}\n{\"id\":2,\"text\":\"second\"}\n{\"id\":3,\"text\":\"third\"}\n"
	r := strings.NewReader(data)

	var chunks []map[string]interface{}
	for msg, err := range jsonstream.Stream(r) {
		t.Logf("got msg=%q err=%v", string(msg), err)
		if err != nil {
			t.Fatal(err)
		}
		var chunk map[string]interface{}
		if err := json.Unmarshal(msg, &chunk); err != nil {
			t.Fatal(err)
		}
		chunks = append(chunks, chunk)
	}

	t.Logf("total chunks: %d", len(chunks))
	if len(chunks) != 3 {
		t.Fatalf("got %d chunks, want 3", len(chunks))
	}
	if chunks[1]["id"].(float64) != 2 || chunks[1]["text"] != "second" {
		t.Errorf("second chunk wrong: %v", chunks[1])
	}
}

func TestStreamSingleObject(t *testing.T) {
	data := `{"id":1,"text":"single"}`
	r := strings.NewReader(data)

	var chunks []map[string]interface{}
	for msg, err := range jsonstream.Stream(r) {
		t.Logf("got msg=%q err=%v", string(msg), err)
		if err != nil {
			t.Fatal(err)
		}
		var chunk map[string]interface{}
		if err := json.Unmarshal(msg, &chunk); err != nil {
			t.Fatal(err)
		}
		chunks = append(chunks, chunk)
	}

	t.Logf("total chunks: %d", len(chunks))
	if len(chunks) != 1 {
		t.Fatalf("got %d chunks, want 1", len(chunks))
	}
	if chunks[0]["id"].(float64) != 1 || chunks[0]["text"] != "single" {
		t.Errorf("chunk wrong: %v", chunks[0])
	}
}

func TestBytes(t *testing.T) {
	data := []byte(`[{"value":1},{"value":2}]`)

	var values []int
	for msg, err := range jsonstream.Bytes(data) {
		if err != nil {
			t.Fatal(err)
		}
		var obj map[string]int
		if err := json.Unmarshal(msg, &obj); err != nil {
			t.Fatal(err)
		}
		values = append(values, obj["value"])
	}

	if len(values) != 2 || values[0] != 1 || values[1] != 2 {
		t.Errorf("got values %v, want [1 2]", values)
	}
}

func TestStreamEmpty(t *testing.T) {
	r := strings.NewReader("")
	count := 0
	for range jsonstream.Stream(r) {
		count++
	}
	if count != 0 {
		t.Errorf("got %d messages from empty stream, want 0", count)
	}
}

func TestStreamEmptyArray(t *testing.T) {
	r := strings.NewReader("[]")
	count := 0
	for range jsonstream.Stream(r) {
		count++
	}
	if count != 0 {
		t.Errorf("got %d messages from empty array, want 0", count)
	}
}

func TestStreamGemini(t *testing.T) {
	data := `[{
  "candidates": [
    {
      "content": {
        "parts": [
          {
            "text": "Hello! I'd be happy"
          }
        ],
        "role": "model"
      },
      "index": 0
    }
  ]
},
{
  "candidates": [
    {
      "content": {
        "parts": [
          {
            "text": " to tell you about Go."
          }
        ],
        "role": "model"
      },
      "finishReason": "MAX_TOKENS",
      "index": 0
    }
  ]
}]`

	type GeminiChunk struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	var chunks []GeminiChunk
	for msg, err := range jsonstream.Bytes([]byte(data)) {
		if err != nil {
			t.Fatal(err)
		}
		var chunk GeminiChunk
		if err := json.Unmarshal(msg, &chunk); err != nil {
			t.Fatal(err)
		}
		chunks = append(chunks, chunk)
	}

	if len(chunks) != 2 {
		t.Fatalf("got %d chunks, want 2", len(chunks))
	}

	text0 := chunks[0].Candidates[0].Content.Parts[0].Text
	text1 := chunks[1].Candidates[0].Content.Parts[0].Text

	if text0 != "Hello! I'd be happy" {
		t.Errorf("first chunk text = %q", text0)
	}
	if text1 != " to tell you about Go." {
		t.Errorf("second chunk text = %q", text1)
	}
}
