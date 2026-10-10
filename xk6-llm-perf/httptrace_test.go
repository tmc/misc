package llmperf

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func ExampleHTTPTrace() {
	var t HTTPTrace
	fmt.Println(t.Waiting == nil, t.Reused == nil)
	// Output: true true
}

func TestTransportTraceStages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "fixture") }))
	defer server.Close()
	transport := newTransport()
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport}
	for i := 0; i < 2; i++ {
		req, _ := http.NewRequest("GET", server.URL, nil)
		req, trace := attachTrace(req)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		// Drain for reuse.
		var b [64]byte
		for {
			_, err := resp.Body.Read(b[:])
			if err != nil {
				break
			}
		}
		resp.Body.Close()
		got := trace.finish(time.Now())
		if got.Waiting == nil || got.Sending == nil || got.Receiving == nil || got.Reused == nil {
			t.Fatalf("trace=%+v", got)
		}
		if i == 1 && (!*got.Reused || got.Connect != nil || got.TLS != nil) {
			t.Fatalf("reuse stages=%+v", got)
		}
	}
}
