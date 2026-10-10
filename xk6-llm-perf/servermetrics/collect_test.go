package servermetrics

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func ExampleCollect() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, "requests_total 0\n") }))
	defer server.Close()
	var output bytes.Buffer
	r, err := Collect(context.Background(), server.Client(), CollectOptions{Targets: []string{server.URL}, Interval: 10 * time.Millisecond, Duration: time.Second, MaxRecords: 1, MaxBytes: 10000}, &output)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(r.Attempts, r.IncompleteReason, strings.Count(output.String(), "\n"))
	// Output: 1 record budget exhausted 1
}

type shortWriter struct{}

func (shortWriter) Write(p []byte) (int, error) { return 0, nil }
func TestCollectBoundsAndSink(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, "x 1\n") }))
	defer server.Close()
	opts := CollectOptions{Targets: []string{server.URL}, Interval: 10 * time.Millisecond, Duration: time.Second, MaxRecords: 1, MaxBytes: 10000}
	r, err := Collect(context.Background(), server.Client(), opts, shortWriter{})
	if err != io.ErrShortWrite || r.Complete || r.IncompleteReason != "sink failure" {
		t.Fatalf("recording=%+v err=%v", r, err)
	}
	opts.MaxBytes = 1
	r, err = Collect(context.Background(), server.Client(), opts, io.Discard)
	if err != nil || r.Complete || r.IncompleteReason != "byte budget exhausted" {
		t.Fatalf("recording=%+v err=%v", r, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	opts.MaxBytes = 10000
	r, err = Collect(ctx, server.Client(), opts, io.Discard)
	if err != context.Canceled || r.Complete || r.Attempts != 0 {
		t.Fatalf("recording=%+v err=%v", r, err)
	}
}
