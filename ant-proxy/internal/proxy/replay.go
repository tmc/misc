package proxy

import (
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "os"
    "path/filepath"
    "time"

    "github.com/tmc/misc/ant-proxy/internal/har"
)

type ReplayProxy struct {
    dir string
}

func NewReplayProxy(dir string) *ReplayProxy {
    return &ReplayProxy{dir: dir}
}

func (p *ReplayProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // Read request body
    body, err := io.ReadAll(r.Body)
    if err != nil {
        http.Error(w, "Error reading request", http.StatusBadRequest)
        return
    }
    r.Body.Close()

    // Generate key from request body
    hash := sha256.Sum256(body)
    key := hex.EncodeToString(hash[:])
    path := filepath.Join(p.dir, key+".har")

    // Try to load existing recording
    if entry, err := p.loadEntry(path); err == nil {
        // Replay response
        for k, v := range entry.Response.Headers {
            w.Header().Set(k, v)
        }
        w.Header().Set("X-Proxy-Cache", "HIT")
        w.WriteHeader(entry.Response.Status)
        w.Write([]byte(entry.Response.Body))
        return
    }

    // No recording found, forward request
    resp, err := http.DefaultClient.Do(r)
    if err != nil {
        http.Error(w, "Error forwarding request", http.StatusBadGateway)
        return
    }
    defer resp.Body.Close()

    // Read response body
    respBody, err := io.ReadAll(resp.Body)
    if err != nil {
        http.Error(w, "Error reading response", http.StatusInternalServerError)
        return
    }

    // Create HAR entry
    entry := &har.Entry{
        StartedAt: time.Now(),
        Request: har.Request{
            Method:  r.Method,
            URL:     r.URL.String(),
            Headers: make(map[string]string),
            Body:    string(body),
        },
        Response: har.Response{
            Status:  resp.StatusCode,
            Headers: make(map[string]string),
            Body:    string(respBody),
        },
        CompletedAt: time.Now(),
    }

    // Copy headers
    for k, v := range r.Header {
        entry.Request.Headers[k] = v[0]
    }
    for k, v := range resp.Header {
        entry.Response.Headers[k] = v[0]
    }

    // Save entry
    if err := p.saveEntry(path, entry); err != nil {
        fmt.Printf("Error saving entry: %v\n", err)
    }

    // Return response
    for k, v := range resp.Header {
        w.Header()[k] = v
    }
    w.Header().Set("X-Proxy-Cache", "MISS")
    w.WriteHeader(resp.StatusCode)
    w.Write(respBody)
}

func (p *ReplayProxy) saveEntry(path string, entry *har.Entry) error {
    if err := os.MkdirAll(p.dir, 0755); err != nil {
        return fmt.Errorf("create dir: %w", err)
    }

    f, err := os.Create(path)
    if err != nil {
        return fmt.Errorf("create file: %w", err)
    }
    defer f.Close()

    return json.NewEncoder(f).Encode(entry)
}

func (p *ReplayProxy) loadEntry(path string) (*har.Entry, error) {
    f, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer f.Close()

    var entry har.Entry
    if err := json.NewDecoder(f).Decode(&entry); err != nil {
        return nil, err
    }

    return &entry, nil
}

