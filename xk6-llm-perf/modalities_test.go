package llmperf

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func ExampleRawMultipart() {
	upload := RawMultipart{Fields: map[string]string{"model": "whisper"}, Files: []RawFile{{Name: "file", Filename: "audio.wav", ContentType: "audio/wav", Base64: "UklGRg=="}}}
	fmt.Println(upload.Files[0].Filename)
	// Output: audio.wav
}

func ExampleRawFile() {
	file := RawFile{Name: "image", Filename: "image.png", ContentType: "image/png", Base64: "iVBORw=="}
	fmt.Println(file.Name, file.ContentType)
	// Output: image image/png
}

func ExampleVideoPolling() {
	policy := VideoPolling{Interval: "1s", MaxPolls: 30, Download: true}
	fmt.Println(policy.Interval, policy.MaxPolls)
	// Output: 1s 30
}

func TestMultipartRoundTrip(t *testing.T) {
	m := &RawMultipart{Fields: map[string]string{"model": "fixture", "prompt": "a image"}, Files: []RawFile{{Name: "image", Filename: "tiny.png", ContentType: "image/png", Base64: base64.StdEncoding.EncodeToString([]byte{0, 1, 2, 255})}}}
	data, ct, err := encodeMultipart(m)
	if err != nil {
		t.Fatal(err)
	}
	_, params, err := mime.ParseMediaType(ct)
	if err != nil {
		t.Fatal(err)
	}
	r := multipart.NewReader(bytes.NewReader(data), params["boundary"])
	fields := map[string]string{}
	for {
		p, err := r.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		v, _ := io.ReadAll(p)
		fields[p.FormName()] = string(v)
		if p.FileName() != "" && p.Header.Get("Content-Type") != "image/png" {
			t.Fatal(p.Header)
		}
	}
	if fields["model"] != "fixture" || fields["image"] != string([]byte{0, 1, 2, 255}) {
		t.Fatal(fields)
	}
}

func TestMultipartLimits(t *testing.T) {
	for _, f := range []RawFile{{Name: "file\r\nBad", Filename: "a.wav", ContentType: "audio/wav", Base64: "YQ=="}, {Name: "file", Filename: "../a.wav", ContentType: "audio/wav", Base64: "YQ=="}, {Name: "file", Filename: "a.wav", ContentType: "audio/wav\r\nBad", Base64: "YQ=="}, {Name: "file", Filename: "a.wav", ContentType: "audio/wav", Base64: "!"}} {
		if _, _, err := encodeMultipart(&RawMultipart{Files: []RawFile{f}}); err == nil {
			t.Fatal("accepted invalid upload", f)
		}
	}
	huge := &RawMultipart{Files: []RawFile{{Name: "file", Filename: "a.wav", ContentType: "audio/wav", Base64: strings.Repeat("A", base64.StdEncoding.EncodedLen(modalityLimit)+1)}}}
	if _, _, err := encodeMultipart(huge); err == nil {
		t.Fatal("accepted oversized upload")
	}
}

func TestAudioNormalization(t *testing.T) {
	for _, tt := range []struct {
		body, text string
		bad        bool
	}{{"spoken text", "spoken text", false}, {`{"text":"hello","usage":{"completion_tokens":0}}`, "hello", false}, {`{"text":""}`, "", false}, {`{}`, "", true}, {`{"error":"no"}`, "", true}, {`{"text":5}`, "", true}} {
		format := "json"
		if tt.body == "spoken text" {
			format = "text"
		}
		r, err := decodeAudioResponseFormat([]byte(tt.body), format, "")
		if (err != nil) != tt.bad {
			t.Fatalf("%s: %v", tt.body, err)
		}
		if !tt.bad && messageText(r.Choices[0].Message.Content) != tt.text {
			t.Fatal(r)
		}
	}
}

func TestVideoLifecycle(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fixture" || r.Method != "GET" {
			t.Error("missing authenticated GET")
		}
		calls++
		switch r.URL.Path {
		case "/videos/job1":
			fmt.Fprint(w, `{"id":"job1","status":"completed","seconds":4}`)
		case "/videos/job1/content":
			fmt.Fprint(w, "video bytes")
		default:
			t.Error(r.URL.Path)
		}
	}))
	defer server.Close()
	req, _ := http.NewRequest("POST", server.URL+"/videos", nil)
	req.Header.Set("Authorization", "Bearer fixture")
	data, err := pollVideo(context.Background(), server.Client(), req, []byte(`{"id":"job1","status":"queued"}`), &VideoPolling{Interval: "10ms", MaxPolls: 2, Download: true})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || !bytes.Contains(data, []byte(`"content_receipt"`)) || !bytes.Contains(data, []byte(`"bytes":11`)) {
		t.Fatalf("%d %s", calls, data)
	}
}

func TestVideoRefusalAndCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://invalid.example/", http.StatusFound)
	}))
	defer server.Close()
	req, _ := http.NewRequest("POST", server.URL+"/videos", nil)
	for _, body := range []string{`{"id":"../escape","status":"queued"}`, `{"id":"job","status":"failed"}`, `{"id":"job","status":"unknown"}`, `{"id":"job","status":"queued"}`} {
		if _, err := pollVideo(context.Background(), server.Client(), req, []byte(body), &VideoPolling{Interval: "10ms", MaxPolls: 1}); err == nil {
			t.Fatal("accepted", body)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	if _, err := pollVideo(ctx, server.Client(), req, []byte(`{"id":"job","status":"queued"}`), &VideoPolling{Interval: "1s"}); err != context.Canceled {
		t.Fatal(err)
	}
	if time.Since(start) > 100*time.Millisecond {
		t.Fatal("cancellation was delayed")
	}
}

func TestMediaResponseUnits(t *testing.T) {
	for _, protocol := range []string{"image", "retrieval"} {
		r, err := decodeMediaResponse([]byte(`{"data":[{"embedding":[0.1],"b64_json":"YQ=="}]}`), protocol)
		if err != nil || r.Usage.completionPresent || r.Usage.promptPresent || len(r.Choices) != 0 {
			t.Fatalf("%s: %+v %v", protocol, r, err)
		}
	}
	if _, err := decodeMediaResponse([]byte(`{"data":[]}`), "retrieval"); err == nil {
		t.Fatal("accepted missing retrieval data")
	}
}

func TestImageStreamLifecycle(t *testing.T) {
	input := `data: {"type":"image_generation.partial_image","b64_json":"YQ==","partial_image_index":0}

data: {"type":"image_generation.completed","b64_json":"Yg==","usage":{"input_tokens":2,"output_tokens":3}}

`
	var arrivals []bool
	r, err := readImageStream(strings.NewReader(input), func(_ time.Time, terminal bool) { arrivals = append(arrivals, terminal) })
	if err != nil {
		t.Fatal(err)
	}
	if len(arrivals) != 2 || arrivals[0] || !arrivals[1] || r.Usage.CompletionTokens != 3 || len(r.Choices) != 0 {
		t.Fatalf("%+v %v", r, arrivals)
	}
	for _, data := range []string{`data: {"b64_json":"YQ=="}` + "\n\n", `data: {"type":"error"}` + "\n\n", `data: {"type":"image_generation.completed","b64_json":"bad"}` + "\n\n"} {
		if _, err := readImageStream(strings.NewReader(data), nil); err == nil {
			t.Fatal("accepted invalid image stream")
		}
	}
}

func TestVideoIdentityAndPollBounds(t *testing.T) {
	for _, tt := range []struct {
		name, body string
		polls      int
	}{{"identity", `{"id":"different","status":"completed"}`, 1}, {"exhaustion", `{"id":"job","status":"queued"}`, 2}} {
		t.Run(tt.name, func(t *testing.T) {
			var calls int
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; fmt.Fprint(w, tt.body) }))
			defer s.Close()
			req, _ := http.NewRequest("POST", s.URL+"/videos", nil)
			if _, err := pollVideo(context.Background(), s.Client(), req, []byte(`{"id":"job","status":"queued"}`), &VideoPolling{Interval: "10ms", MaxPolls: 2}); err == nil {
				t.Fatal("accepted invalid lifecycle")
			}
			if calls != tt.polls {
				t.Fatalf("calls=%d want=%d", calls, tt.polls)
			}
		})
	}
}

func TestVideoCancelInFlight(t *testing.T) {
	started := make(chan struct{})
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(started); <-r.Context().Done() }))
	defer s.Close()
	req, _ := http.NewRequest("POST", s.URL+"/videos", nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := pollVideo(ctx, s.Client(), req, []byte(`{"id":"job","status":"queued"}`), &VideoPolling{Interval: "10ms"})
		done <- err
	}()
	select {
	case <-started:
		cancel()
	case <-time.After(time.Second):
		t.Fatal("poll did not start")
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled poll succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled poll did not join")
	}
}
