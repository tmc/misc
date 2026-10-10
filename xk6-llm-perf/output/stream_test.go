package output

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func ExampleStream() {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "{}") }))
	defer s.Close()
	b, _ := json.Marshal(runFixture().Records[0])
	receipt, err := Stream(context.Background(), bytes.NewReader(append(b, '\n')), s.Client(), StreamOptions{Format: "otlp", Endpoint: s.URL + "/v1/metrics", RunID: "run"})
	fmt.Println(receipt.RecordsAccepted, receipt.Complete, err)
	// Output: 1 true <nil>
}
func TestStreamingFailStop(t *testing.T) {
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 2 {
			w.WriteHeader(503)
		}
		fmt.Fprint(w, "{}")
	}))
	defer s.Close()
	b, _ := json.Marshal(runFixture().Records[0])
	input := bytes.Repeat(append(b, '\n'), 3)
	receipt, err := Stream(context.Background(), bytes.NewReader(input), s.Client(), StreamOptions{Format: "otlp", Endpoint: s.URL + "/v1/metrics", RunID: "run"})
	if err == nil || calls != 2 || receipt.RecordsAccepted != 1 || receipt.Complete {
		t.Fatal(receipt, err)
	}
}
