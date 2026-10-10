package llmperf

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestBinaryModalities(t *testing.T) {
	binary := os.Getenv("K6_SMOKE_BINARY")
	if binary == "" {
		t.Skip("set K6_SMOKE_BINARY to qualify modality adapters")
	}
	var uploads, polls, downloads atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fixture" {
			t.Error("missing authorization")
		}
		switch r.URL.Path {
		case "/audio/text", "/audio/json", "/images/edits":
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Error(err)
				http.Error(w, "bad multipart", 400)
				return
			}
			defer r.MultipartForm.RemoveAll()
			name := "file"
			if r.URL.Path == "/images/edits" {
				name = "image"
			}
			f, h, err := r.FormFile(name)
			if err != nil {
				t.Error(err)
				return
			}
			f.Close()
			if h.Size != 1 || r.FormValue("model") != "fixture" {
				t.Error("wrong upload")
			}
			uploads.Add(1)
			if r.URL.Path == "/audio/text" {
				fmt.Fprint(w, "spoken words")
			} else if r.URL.Path == "/audio/json" {
				fmt.Fprint(w, `{"text":"spoken words"}`)
			} else {
				fmt.Fprint(w, `{"data":[{"b64_json":"YQ=="}]}`)
			}
		case "/images/generations":
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "data: {\"type\":\"image_generation.partial_image\",\"b64_json\":\"YQ==\"}\n\ndata: {\"type\":\"image_generation.completed\",\"b64_json\":\"Yg==\",\"usage\":{\"input_tokens\":2,\"output_tokens\":3}}\n\n")
		case "/retrieval":
			fmt.Fprint(w, `{"data":[{"embedding":[0.1,0.2]}]}`)
		case "/videos":
			fmt.Fprint(w, `{"id":"job","status":"queued"}`)
		case "/videos/job":
			if r.Method != "GET" {
				t.Error(r.Method)
			}
			polls.Add(1)
			fmt.Fprint(w, `{"id":"job","status":"completed"}`)
		case "/videos/job/content":
			downloads.Add(1)
			fmt.Fprint(w, "video")
		case "/slow-videos":
			fmt.Fprint(w, `{"id":"slow","status":"queued"}`)
		default:
			http.Error(w, "unexpected endpoint", 404)
			t.Error(r.URL.Path)
		}
	}))
	defer server.Close()
	binary, err := filepath.Abs(binary)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "run", "--no-color", "examples/modalities/smoke.js")
	cmd.Env = append(os.Environ(), "ENDPOINT_URL="+server.URL, "K6_NO_USAGE_REPORT=true", "K6_AUTO_EXTENSION_RESOLUTION=false")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("modalities: %v\n%s", err, output)
	}
	if uploads.Load() != 3 || polls.Load() != 1 || downloads.Load() != 1 {
		t.Fatalf("uploads=%d polls=%d downloads=%d", uploads.Load(), polls.Load(), downloads.Load())
	}
}
