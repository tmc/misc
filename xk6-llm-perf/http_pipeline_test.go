package llmperf

import (
	"context"
	"net/http"
	"runtime"
	"testing"
	"time"

	"go.k6.io/k6/v2/js/modulestest"
	"go.k6.io/k6/v2/lib/netext/httpext"
	"go.k6.io/k6/v2/metrics"
)

func awaitPipelineProducer(t *testing.T, c *Client) {
	t.Helper()
	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	for c.metrics.PipelineSnapshot().Inflight == 0 {
		select {
		case <-deadline.C:
			t.Fatal("HTTP trail producer did not enter pipeline")
		default:
			runtime.Gosched()
		}
	}
}

func TestHTTPMetricPipelineCancellation(t *testing.T) {
	for _, scope := range []string{"request", "VU"} {
		t.Run(scope, func(t *testing.T) {
			client := setupTestClient(t, "http://localhost")
			output := make(chan metrics.SampleContainer)
			enableVUState(t, client, output)
			reqCtx, reqCancel := context.WithCancel(context.Background())
			defer reqCancel()
			vuCtx, vuCancel := context.WithCancel(context.Background())
			defer vuCancel()
			client.vu.(*modulestest.VU).CtxField = vuCtx
			req, err := http.NewRequestWithContext(reqCtx, "POST", "http://localhost", nil)
			if err != nil {
				t.Fatal(err)
			}
			done := make(chan struct{})
			go func() {
				client.recordHTTPMetrics(true, req, &http.Response{StatusCode: 200}, &httpext.Tracer{}, nil)
				close(done)
			}()
			awaitPipelineProducer(t, client)
			if scope == "request" {
				reqCancel()
			} else {
				vuCancel()
			}
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("cancelled HTTP trail remained blocked")
			}
			snapshot := client.metrics.PipelineSnapshot()
			if snapshot.Attempted != 1 || snapshot.Cancelled != 1 || snapshot.Delivered != 0 || snapshot.Inflight != 0 || snapshot.WaitNanoseconds == 0 || snapshot.MaxWaitNanoseconds == 0 {
				t.Fatalf("HTTP trail receipt %+v", snapshot)
			}
			select {
			case <-output:
				t.Fatal("cancelled trail delivered without collector")
			default:
			}
		})
	}
}

func TestHTTPMetricPipelineDelivery(t *testing.T) {
	client := setupTestClient(t, "http://localhost")
	output := make(chan metrics.SampleContainer)
	enableVUState(t, client, output)
	req, err := http.NewRequest("POST", "http://localhost", nil)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		client.recordHTTPMetrics(true, req, &http.Response{StatusCode: 200}, &httpext.Tracer{}, nil)
		close(done)
	}()
	awaitPipelineProducer(t, client)
	select {
	case sample := <-output:
		found := false
		for _, value := range sample.GetSamples() {
			if value.Metric.Name == "http_reqs" {
				found = true
				if value.Value != 1 {
					t.Fatalf("HTTP request count %v", value.Value)
				}
			}
		}
		if !found {
			t.Fatal("HTTP request trail lost")
		}
	case <-time.After(time.Second):
		t.Fatal("HTTP trail not delivered")
	}
	<-done
	snapshot := client.metrics.PipelineSnapshot()
	if snapshot.Attempted != 1 || snapshot.Delivered != 1 || snapshot.Cancelled != 0 || snapshot.Inflight != 0 || snapshot.WaitNanoseconds == 0 || snapshot.MaxWaitNanoseconds == 0 {
		t.Fatalf("HTTP delivery receipt %+v", snapshot)
	}
}
