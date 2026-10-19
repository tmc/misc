package accuracy_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/tmc/misc/xk6-llm-perf/accuracy"
)

func ExampleGenerateRequests() {
	cases := []accuracy.Case{{ID: "0", Reference: "A", Outcome: "success"}}
	hash, _ := accuracy.ReferenceHash(cases)
	a := accuracy.Artifact{SchemaVersion: 1, Task: "example", TaskVersion: "1", ReferenceVersion: "1", ReferenceSHA256: hash, Evaluator: "exact-match-v1", EvaluatorVersion: "1", Model: "example", ModelRevision: "1", Cases: cases}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"choices":[{"index":0,"message":{"content":"A"}}]}`)
	}))
	defer server.Close()
	requests := map[string]accuracy.GenerationRequest{"0": {Messages: []accuracy.Message{{Role: "user", Content: "Choose A"}}, MaxTokens: 5, Stop: []string{"\n"}}}
	got, err := accuracy.GenerateRequests(context.Background(), server.Client(), server.URL, "", a, requests)
	fmt.Println(*got.Cases[0].Prediction, err)
	// Output: A <nil>
}
