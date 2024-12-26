package recorder

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/har"
	"github.com/chromedp/cdproto/network"
)

func TestRecorderStreaming(t *testing.T) {
	tests := []struct {
		name      string
		streaming bool
		events    []interface{}
		want      int // number of expected JSON outputs
	}{
		{
			name:      "streaming_enabled",
			streaming: true,
			events: []interface{}{
				&network.EventRequestWillBeSent{
					RequestID: "1",
					Request: &network.Request{
						URL:    "https://example.com",
						Method: "GET",
						Headers: map[string]interface{}{
							"User-Agent": "test",
						},
					},
				},
				&network.EventResponseReceived{
					RequestID: "1",
					Response: &network.Response{
						URL:        "https://example.com",
						Status:     200,
						StatusText: "OK",
						Headers: map[string]interface{}{
							"Content-Type": "text/html",
						},
					},
				},
				&network.EventLoadingFinished{
					RequestID: "1",
					Timestamp: network.MonotonicTime(time.Now().UnixNano() / int64(time.Millisecond)),
				},
			},
			want: 1,
		},
		{
			name:      "streaming_disabled",
			streaming: false,
			events:    []interface{}{},
			want:      0,
		},
		{
			name:      "streaming_with_filtered_url",
			streaming: true,
			events: []interface{}{
				&network.EventRequestWillBeSent{
					RequestID: "1",
					Request: &network.Request{
						URL:    "https://filtered.com",
						Method: "GET",
					},
				},
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := []Option{WithStreaming(tt.streaming)}
			if tt.name == "streaming_with_filtered_url" {
				opts = append(opts, WithURLPattern("example\\.com"))
			}

			rec, err := New(opts...)
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}

			ctx := context.Background()
			handler := rec.HandleNetworkEvent(ctx)

			// Capture stdout
			var output strings.Builder
			oldStdout := json.NewEncoder(&output)
			_ = oldStdout // Avoid unused variable warning

			// Process events
			for _, event := range tt.events {
				handler(event)
			}

			// Count JSON objects in output
			outputs := strings.Split(strings.TrimSpace(output.String()), "\n")
			count := 0
			for _, out := range outputs {
				if out != "" {
					var entry har.Entry
					if err := json.Unmarshal([]byte(out), &entry); err == nil {
						count++
					}
				}
			}

			if count != tt.want {
				t.Errorf("got %d JSON outputs, want %d", count, tt.want)
			}
		})
	}
}

func TestCreateHAREntry(t *testing.T) {
	tests := []struct {
		name    string
		req     *network.Request
		resp    *network.Response
		timing  *network.EventLoadingFinished
		wantURL string
		wantErr bool
	}{
		{
			name: "valid_entry",
			req: &network.Request{
				URL:    "https://example.com",
				Method: "GET",
				Headers: map[string]interface{}{
					"User-Agent": "test",
				},
			},
			resp: &network.Response{
				URL:        "https://example.com",
				Status:     200,
				StatusText: "OK",
				Headers: map[string]interface{}{
					"Content-Type": "text/html",
				},
			},
			timing: &network.EventLoadingFinished{
				RequestID: "test1",
				Timestamp: network.MonotonicTime(time.Now().UnixNano() / int64(time.Millisecond)),
			},
			wantURL: "https://example.com",
			wantErr: false,
		},
		{
			name:    "missing_request",
			req:     nil,
			resp:    nil,
			timing:  nil,
			wantURL: "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec, err := New()
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}

			reqID := network.RequestID("test1")
			if tt.req != nil {
				rec.requests[reqID] = tt.req
			}
			if tt.resp != nil {
				rec.responses[reqID] = tt.resp
			}
			if tt.timing != nil {
				rec.timings[reqID] = tt.timing
			}

			entry := rec.createHAREntry(reqID)
			if tt.wantErr {
				if entry != nil {
					t.Error("createHAREntry() returned entry when error expected")
				}
				return
			}

			if entry == nil {
				t.Fatal("createHAREntry() returned nil")
			}

			if entry.Request.URL != tt.wantURL {
				t.Errorf("entry.Request.URL = %v, want %v", entry.Request.URL, tt.wantURL)
			}
		})
	}
}
