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
