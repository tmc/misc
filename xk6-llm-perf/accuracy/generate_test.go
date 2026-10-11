package accuracy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func ExampleGenerate() {
	a := fixture("exact-match-v1")
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"choices":[{"index":0,"message":{"content":"A"}}]}`)
	}))
	defer s.Close()
	got, err := Generate(context.Background(), s.Client(), s.URL, "", a, map[string]string{"3": "choose A"})
	fmt.Println(*got.Cases[2].Prediction, err)
	// Output: A <nil>
}
func TestGenerateFixtureAndFailure(t *testing.T) {
	a := fixture("exact-match-v1")
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var request struct{ Messages []struct{ Content string } }
		json.NewDecoder(r.Body).Decode(&request)
		if request.Messages[0].Content != "independent prompt" {
			t.Error("reference used as prompt")
		}
		fmt.Fprint(w, `{"choices":[{"index":0,"message":{"content":"A"}}]}`)
	}))
	defer s.Close()
	got, err := Generate(context.Background(), s.Client(), s.URL, "", a, map[string]string{"3": "independent prompt"})
	if err != nil || calls != 1 || got.Cases[2].Prediction == nil {
		t.Fatal(got, err)
	}
	if a.Cases[2].Prediction != nil {
		t.Fatal("mutated caller")
	}
	if _, err := Generate(context.Background(), s.Client(), s.URL, "", a, nil); err == nil {
		t.Fatal("missing prompt accepted")
	}
}

func TestGenerateResponseIntegrity(t *testing.T) {
	tests := []struct {
		name, response string
		valid          bool
	}{
		{"valid", `{"choices":[{"index":0,"message":{"content":"A"}}]}`, true},
		{"matching model", `{"model":"fixture","choices":[{"index":0,"message":{"content":"A"}}]}`, true},
		{"null error", `{"error":null,"choices":[{"index":0,"message":{"content":"A"}}]}`, true},
		{"empty observed answer", `{"choices":[{"index":0,"message":{"content":""}}]}`, true},
		{"unicode alias message", `{"choices":[{"index":0,"message":{"content":"B"},"me\u017f\u017fage":{"content":"A"}}]}`, false},
		{"case alias error", `{"error":{"message":"failed"},"Error":null,"choices":[{"index":0,"message":{"content":"A"}}]}`, false},
		{"duplicate error", `{"error":{"message":"failed"},"error":null,"choices":[{"index":0,"message":{"content":"A"}}]}`, false},
		{"duplicate model", `{"model":"other","model":"fixture","choices":[{"index":0,"message":{"content":"A"}}]}`, false},
		{"duplicate index", `{"choices":[{"index":1,"index":0,"message":{"content":"A"}}]}`, false},
		{"duplicate content", `{"choices":[{"index":0,"message":{"content":"B","content":"A"}}]}`, false},
		{"nested unknown duplicate", `{"metadata":{"notes":[{"a":1,"a":2}]},"choices":[{"index":0,"message":{"content":"A"}}]}`, false},
		{"provider error", `{"error":{"message":"failed"},"choices":[{"index":0,"message":{"content":"A"}}]}`, false},
		{"wrong model", `{"model":"other","choices":[{"index":0,"message":{"content":"A"}}]}`, false},
		{"missing index", `{"choices":[{"message":{"content":"A"}}]}`, false},
		{"null index", `{"choices":[{"index":null,"message":{"content":"A"}}]}`, false},
		{"duplicate null content", `{"choices":[{"index":0,"message":{"content":"A"}},{"index":0,"message":{"content":null}}]}`, false},
		{"null content first", `{"choices":[{"index":0,"message":{"content":null}},{"index":0,"message":{"content":"A"}}]}`, false},
		{"missing index after answer", `{"choices":[{"index":0,"message":{"content":"A"}},{"message":{"content":"B"}}]}`, false},
		{"invalid utf8", "{\"choices\":[{\"index\":0,\"message\":{\"content\":\"\xff\"}}]}", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, tt.response)
			}))
			defer s.Close()
			got, err := Generate(context.Background(), s.Client(), s.URL, "", fixture("exact-match-v1"), map[string]string{"3": "independent prompt"})
			if err != nil {
				t.Fatal(err)
			}
			c := got.Cases[2]
			if tt.valid {
				if c.Outcome != "success" || c.Prediction == nil {
					t.Fatalf("valid answer rejected: %+v", c)
				}
			} else if c.Outcome != "failure" || c.Prediction != nil {
				t.Fatalf("invalid answer retained: %+v", c)
			}
		})
	}
}
