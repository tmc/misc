package recorder

import (
	"testing"
	"time"

	"github.com/chromedp/cdproto/har"
)

func TestJQFilter(t *testing.T) {
	tests := []struct {
		name    string
		entry   *har.Entry
		filter  string
		want    bool // whether entry should be included
		wantErr bool
	}{
		{
			name: "filter_status_200",
			entry: &har.Entry{
				Response: &har.Response{
					Status: 200,
				},
			},
			filter: "select(.response.status == 200)",
			want:   true,
		},
		{
			name: "filter_status_404",
			entry: &har.Entry{
				Response: &har.Response{
					Status: 404,
				},
			},
			filter: "select(.response.status == 200)",
			want:   false,
		},
		{
			name: "filter_url_contains",
			entry: &har.Entry{
				Request: &har.Request{
					URL: "https://api.example.com/v1",
				},
			},
			filter: `select(.request.url | contains("api"))`,
			want:   true,
		},
		{
			name: "transform_entry",
			entry: &har.Entry{
				Request: &har.Request{
					URL:    "https://example.com",
					Method: "GET",
				},
				Response: &har.Response{
					Status: 200,
				},
			},
			filter: `{url: .request.url, method: .request.method, status: .response.status}`,
			want:   true,
		},
		{
			name: "invalid_filter",
			entry: &har.Entry{
				Request: &har.Request{
					URL: "https://example.com",
				},
			},
			filter:  "invalid { syntax",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := New(WithFilter(tt.filter))
			if err != nil {
				if !tt.wantErr {
					t.Fatalf("New() error = %v", err)
				}
				return
			}

			got, err := r.applyJQFilter(tt.entry)
			if (err != nil) != tt.wantErr {
				t.Errorf("applyJQFilter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.want && got == nil {
				t.Error("applyJQFilter() = nil, want non-nil")
			} else if !tt.want && got != nil {
				t.Error("applyJQFilter() = non-nil, want nil")
			}
		})
	}
}

