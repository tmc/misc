package llmperf

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const modalityLimit = 16 << 20

// RawMultipart describes an authored multipart request. File data is base64,
// decoded locally; paths and URLs are never fetched by this adapter.
type RawMultipart struct {
	Fields map[string]string `json:"fields,omitempty"`
	Files  []RawFile         `json:"files"`
}

// RawFile is a bounded upload. Name is the multipart field name, for example file
// for transcription or image for image edits.
type RawFile struct {
	Name        string `json:"name"`
	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
	Base64      string `json:"base64"`
}

func encodeMultipart(m *RawMultipart) ([]byte, string, error) {
	if m == nil || len(m.Files) == 0 || len(m.Files) > 16 || len(m.Fields) > 64 {
		return nil, "", fmt.Errorf("multipart requires 1 to 16 files and at most 64 fields")
	}
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	keys := make([]string, 0, len(m.Fields))
	for k := range m.Fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := m.Fields[k]
		if !safePartName(k) || len(v) > 1<<20 || !utf8.ValidString(v) {
			return nil, "", fmt.Errorf("invalid multipart field")
		}
		if err := w.WriteField(k, v); err != nil {
			return nil, "", err
		}
		if b.Len() > modalityLimit {
			return nil, "", fmt.Errorf("multipart exceeds 16 MiB limit")
		}
	}
	for _, f := range m.Files {
		if !safePartName(f.Name) || !safePartName(f.Filename) || strings.ContainsAny(f.Filename, "/\\") {
			return nil, "", fmt.Errorf("invalid multipart file name")
		}
		media, params, err := mime.ParseMediaType(f.ContentType)
		if err != nil || len(params) != 0 || len(media) > 128 {
			return nil, "", fmt.Errorf("invalid multipart content type")
		}
		if len(f.Base64) > base64.StdEncoding.EncodedLen(modalityLimit) {
			return nil, "", fmt.Errorf("multipart file exceeds size limit")
		}
		data, err := base64.StdEncoding.Strict().DecodeString(f.Base64)
		if err != nil || len(data) == 0 {
			return nil, "", fmt.Errorf("invalid or empty multipart base64")
		}
		if b.Len()+len(data)+1024 > modalityLimit {
			return nil, "", fmt.Errorf("multipart exceeds 16 MiB limit")
		}
		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{"name": f.Name, "filename": f.Filename}))
		h.Set("Content-Type", media)
		part, err := w.CreatePart(h)
		if err != nil {
			return nil, "", err
		}
		if _, err = part.Write(data); err != nil {
			return nil, "", err
		}
	}
	if err := w.Close(); err != nil {
		return nil, "", err
	}
	if b.Len() > modalityLimit {
		return nil, "", fmt.Errorf("multipart exceeds 16 MiB limit")
	}
	return b.Bytes(), w.FormDataContentType(), nil
}

func safePartName(s string) bool {
	return s != "" && len(s) <= 256 && utf8.ValidString(s) && !strings.ContainsAny(s, "\r\n\x00")
}

func decodeAudioResponse(data []byte) (CompletionResponse, error) {
	var r CompletionResponse
	if len(data) > modalityLimit || !utf8.Valid(data) {
		return r, fmt.Errorf("invalid audio transcript")
	}
	text := string(data)
	if json.Valid(data) {
		var v struct {
			Text  *string         `json:"text"`
			Usage json.RawMessage `json:"usage"`
			Error json.RawMessage `json:"error"`
		}
		if err := json.Unmarshal(data, &v); err != nil {
			return r, fmt.Errorf("decode audio transcript: %w", err)
		}
		if len(v.Error) > 0 && string(v.Error) != "null" {
			return r, fmt.Errorf("provider audio error")
		}
		if v.Text == nil {
			return r, fmt.Errorf("audio JSON response has no transcript")
		}
		text = *v.Text
		var err error
		r.Usage, err = canonicalUsage(v.Usage, "openai-responses")
		if err != nil {
			return r, err
		}
	}
	r.Body = append(json.RawMessage(nil), data...)
	// Plain text is stored as a JSON string so the response remains serializable.
	if !json.Valid(data) {
		r.Body, _ = json.Marshal(text)
	}
	r.Choices = []Choice{{Message: Message{Role: "assistant", Content: text}}}
	return r, nil
}

// VideoPolling bounds the complete submission, polling, and optional content
// download lifecycle by the caller's request context. Interval defaults to 1s,
// MaxPolls to 120. Download retains only a byte count and SHA-256 receipt.
type VideoPolling struct {
	Interval string `json:"interval,omitempty"`
	MaxPolls int    `json:"maxPolls,omitempty"`
	Download bool   `json:"download,omitempty"`
}

func videoPollPolicy(p *VideoPolling) (time.Duration, int, error) {
	if p == nil {
		return 0, 0, fmt.Errorf("video polling policy is required")
	}
	d := time.Second
	if p.Interval != "" {
		var err error
		d, err = time.ParseDuration(p.Interval)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid video poll interval")
		}
	}
	n := p.MaxPolls
	if n == 0 {
		n = 120
	}
	if d < 10*time.Millisecond || d > time.Minute || n < 1 || n > 1000 {
		return 0, 0, fmt.Errorf("video poll bounds exceeded")
	}
	return d, n, nil
}

func pollVideo(ctx context.Context, client *http.Client, submission *http.Request, initial []byte, p *VideoPolling) ([]byte, error) {
	interval, maxPolls, err := videoPollPolicy(p)
	if err != nil {
		return nil, err
	}
	if submission == nil || submission.URL == nil || client == nil {
		return nil, fmt.Errorf("video polling requires original request and client")
	}
	// Even same-origin redirects are refused: endpoint paths are explicit and
	// credentials must never escape through a provider-controlled redirect.
	bounded := *client
	bounded.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	data := initial
	var id string
	totalBytes := len(initial)
	for polls := 0; ; polls++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var job struct {
			ID     string          `json:"id"`
			Status string          `json:"status"`
			Error  json.RawMessage `json:"error"`
		}
		if len(data) > modalityLimit {
			return nil, fmt.Errorf("video response exceeds size limit")
		}
		if err := json.Unmarshal(data, &job); err != nil {
			return nil, fmt.Errorf("decode video job: %w", err)
		}
		if !safeJobID(job.ID) {
			return nil, fmt.Errorf("invalid video job ID")
		}
		if id == "" {
			id = job.ID
		} else if id != job.ID {
			return nil, fmt.Errorf("video job identity changed")
		}
		if len(job.Error) > 0 && string(job.Error) != "null" {
			return nil, fmt.Errorf("provider video error")
		}
		switch job.Status {
		case "completed":
			if !p.Download {
				return data, nil
			}
			body, err := videoGet(ctx, &bounded, submission, id+"/content")
			if err != nil {
				return nil, err
			}
			if totalBytes+len(body) > 64<<20 {
				return nil, fmt.Errorf("video lifecycle exceeds 64 MiB budget")
			}
			var result map[string]json.RawMessage
			if err := json.Unmarshal(data, &result); err != nil {
				return nil, err
			}
			sum := sha256.Sum256(body)
			receipt, _ := json.Marshal(struct {
				Bytes  int    `json:"bytes"`
				SHA256 string `json:"sha256"`
			}{len(body), hex.EncodeToString(sum[:])})
			result["content_receipt"] = receipt
			return json.Marshal(result)
		case "failed", "cancelled", "canceled":
			return nil, fmt.Errorf("video job %s", job.Status)
		case "queued", "in_progress", "processing":
		default:
			return nil, fmt.Errorf("unknown video job status")
		}
		if polls >= maxPolls {
			return nil, fmt.Errorf("video polling exhausted")
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
		data, err = videoGet(ctx, &bounded, submission, id)
		totalBytes += len(data)
		if totalBytes > 64<<20 {
			return nil, fmt.Errorf("video lifecycle exceeds 64 MiB budget")
		}
		if err != nil {
			return nil, err
		}
	}
}

func safeJobID(id string) bool {
	if id == "" || len(id) > 256 {
		return false
	}
	for _, c := range id {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-') {
			return false
		}
	}
	return true
}

func videoGet(ctx context.Context, client *http.Client, original *http.Request, suffix string) ([]byte, error) {
	u := *original.URL
	u.Path = strings.TrimRight(u.Path, "/") + "/" + suffix
	u.RawPath = ""
	u.RawQuery = ""
	u.Fragment = ""
	if _, err := url.ParseRequestURI(u.String()); err != nil {
		return nil, fmt.Errorf("invalid video poll URL")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header = original.Header.Clone()
	req.Header.Del("Content-Type")
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("poll video: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("video poll status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, modalityLimit+1))
	if err != nil {
		return nil, err
	}
	if len(data) > modalityLimit {
		return nil, fmt.Errorf("video poll response exceeds 16 MiB limit")
	}
	return data, nil
}

// decodeMediaResponse retains image and retrieval data as authored JSON. Images,
// pixels, audio seconds, and retrieval vectors are never counted as tokens.
func decodeMediaResponse(data []byte, protocol string) (CompletionResponse, error) {
	var result CompletionResponse
	var v struct {
		ID     string            `json:"id"`
		Model  string            `json:"model"`
		Data   []json.RawMessage `json:"data"`
		Base64 string            `json:"b64_json"`
		Usage  json.RawMessage   `json:"usage"`
		Error  json.RawMessage   `json:"error"`
	}
	if len(data) > modalityLimit {
		return result, fmt.Errorf("media response exceeds size limit")
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return result, fmt.Errorf("decode media response: %w", err)
	}
	if len(v.Error) > 0 && string(v.Error) != "null" {
		return result, fmt.Errorf("provider media error")
	}
	if len(v.Data) == 0 && (protocol != "image" || v.Base64 == "") {
		return result, fmt.Errorf("media response has no data")
	}
	result.ID, result.Model = v.ID, v.Model
	result.Body = append(json.RawMessage(nil), data...)
	var err error
	result.Usage, err = canonicalUsage(v.Usage, "openai-responses")
	return result, err
}

// readImageStream preserves partial and final images separately, bounded by the
// complete response budget. Observe receives only actual image arrival times;
// the boolean marks a terminal image, not a token or a byte count.
func readImageStream(r io.Reader, observe func(time.Time, bool)) (CompletionResponse, error) {
	var result CompletionResponse
	reader := NewSSEReader(r)
	events := make([]json.RawMessage, 0)
	total := 0
	terminal := false
	for {
		event, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return result, fmt.Errorf("read image stream: %w", err)
		}
		if event.Data == "[DONE]" {
			terminal = true
			break
		}
		total += len(event.Data)
		if total > modalityLimit || len(events) >= 1024 {
			return result, fmt.Errorf("image stream exceeds response budget")
		}
		var v struct {
			Type   string          `json:"type"`
			Base64 string          `json:"b64_json"`
			Usage  json.RawMessage `json:"usage"`
			Error  json.RawMessage `json:"error"`
		}
		if err := json.Unmarshal([]byte(event.Data), &v); err != nil {
			return result, fmt.Errorf("decode image stream: %w", err)
		}
		if len(v.Error) > 0 && string(v.Error) != "null" || v.Type == "error" {
			return result, fmt.Errorf("provider image stream error")
		}
		kind := v.Type
		if kind == "" {
			kind = event.Event
		}
		done := kind == "image_generation.completed" || kind == "image_edit.completed"
		if v.Base64 != "" {
			if _, err := base64.StdEncoding.Strict().DecodeString(v.Base64); err != nil {
				return result, fmt.Errorf("invalid image response base64")
			}
			events = append(events, json.RawMessage(event.Data))
			if observe != nil {
				observe(time.Now(), done)
			}
		}
		if len(v.Usage) > 0 {
			u, err := canonicalUsage(v.Usage, "openai-responses")
			if err != nil {
				return result, err
			}
			result.Usage = u
		}
		if done {
			terminal = true
			break
		}
	}
	if !terminal {
		return result, fmt.Errorf("image stream ended without terminal event")
	}
	if len(events) == 0 {
		return result, fmt.Errorf("image stream has no images")
	}
	result.Body, _ = json.Marshal(struct {
		Events []json.RawMessage `json:"events"`
	}{events})
	return result, nil
}
