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

type delayedWriter struct{ delay time.Duration }

func (w delayedWriter) Write(p []byte) (int, error) {
	time.Sleep(w.delay)
	return len(p), nil
}

type canceledWriter struct{}

func (canceledWriter) WriteContext(ctx context.Context, _ []byte) (int, error) {
	<-ctx.Done()
	return 0, ctx.Err()
}

func TestCollectSinkDeadline(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, "x 1\n") }))
	defer server.Close()
	opts := CollectOptions{Targets: []string{server.URL}, Interval: 10 * time.Millisecond, Duration: 30 * time.Millisecond, MaxRecords: 100, MaxBytes: 100000}
	for _, contextAware := range []bool{false, true} {
		t.Run(fmt.Sprint(contextAware), func(t *testing.T) {
			var r Recording
			var err error
			if contextAware {
				r, err = CollectContext(context.Background(), server.Client(), opts, canceledWriter{})
			} else {
				r, err = Collect(context.Background(), server.Client(), opts, delayedWriter{100 * time.Millisecond})
			}
			if err != context.DeadlineExceeded || r.Complete || r.IncompleteReason != "sink deadline exceeded" {
				t.Fatalf("recording=%+v err=%v", r, err)
			}
		})
	}
}

func TestCollectContextSinkCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, "x 1\n") }))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	timer := time.AfterFunc(30*time.Millisecond, cancel)
	defer timer.Stop()
	r, err := CollectContext(ctx, server.Client(), CollectOptions{Targets: []string{server.URL}, Interval: 10 * time.Millisecond, Duration: time.Second, MaxRecords: 100, MaxBytes: 100000}, canceledWriter{})
	if err != context.Canceled || r.Complete || r.IncompleteReason != "cancelled" {
		t.Fatalf("recording=%+v err=%v", r, err)
	}
}

func TestCollectSlowHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()
	var out bytes.Buffer
	r, err := Collect(context.Background(), server.Client(), CollectOptions{Targets: []string{server.URL}, Interval: 10 * time.Millisecond, Duration: 30 * time.Millisecond, MaxRecords: 100, MaxBytes: 100000}, &out)
	if err != context.DeadlineExceeded || r.Complete || r.Failures != 1 || r.Attempts != 1 || r.IncompleteReason != "scrape deadline exceeded" {
		t.Fatalf("recording=%+v err=%v", r, err)
	}
	if !strings.Contains(out.String(), `"error":`) {
		t.Fatal("failed scrape receipt missing")
	}
}

type discardContextWriter struct{}

func (discardContextWriter) WriteContext(ctx context.Context, p []byte) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	return len(p), nil
}

func ExampleCollectContext() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, "requests_total 0\n") }))
	defer server.Close()
	r, err := CollectContext(context.Background(), server.Client(), CollectOptions{Targets: []string{server.URL}, Interval: 10 * time.Millisecond, Duration: time.Second, MaxRecords: 1, MaxBytes: 10000}, discardContextWriter{})
	fmt.Println(r.Attempts, err)
	// Output: 1 <nil>
}
