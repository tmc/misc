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
)

type Recorder struct {
    dir string
}

func NewRecorder(dir string) *Recorder {
    return &Recorder{dir: dir}
}

func (r *Recorder) ServeHTTP(w http.ResponseWriter, req *http.Request) {
    // Read request body
    body, err := io.ReadAll(req.Body)
    if err != nil {
        http.Error(w, "Error reading request", http.StatusBadRequest)
        return
    }
    req.Body.Close()

    // Generate key from request body
    hash := sha256.Sum256(body)
    key := hex.EncodeToString(hash[:])
    path := filepath.Join(r.dir, key+".json")

    // Try to load cached response
    if resp, err := r.load(path); err == nil {
        w.Header().Set("Content-Type", "application/json")
        w.Header().Set("X-Proxy-Cache", "HIT")
        json.NewEncoder(w).Encode(resp)
        return
    }

    // Parse request to validate it's an Anthropic request
    var request Request
    if err := json.Unmarshal(body, &request); err != nil {
        http.Error(w, "Invalid request format", http.StatusBadRequest)
        return
    }

    // Create mock response
    resp := &Response{
        Content: fmt.Sprintf("Mocked response for model %s: %s", 
            request.Model, 
            request.Messages[len(request.Messages)-1].Content),
    }

    // Save response
    if err := r.save(path, resp); err != nil {
        fmt.Printf("Error saving response: %v\n", err)
    }

    // Return response
    w.Header().Set("Content-Type", "application/json")
    w.Header().Set("X-Proxy-Cache", "MISS")
    json.NewEncoder(w).Encode(resp)
}

func (r *Recorder) save(path string, resp *Response) error {
    if err := os.MkdirAll(r.dir, 0755); err != nil {
        return fmt.Errorf("create dir: %w", err)
    }

    f, err := os.Create(path)
    if err != nil {
        return fmt.Errorf("create file: %w", err)
    }
    defer f.Close()

    return json.NewEncoder(f).Encode(resp)
}

func (r *Recorder) load(path string) (*Response, error) {
    f, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer f.Close()

    var resp Response
    if err := json.NewDecoder(f).Decode(&resp); err != nil {
        return nil, err
    }

    return &resp, nil
}

