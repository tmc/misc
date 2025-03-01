package proxy

import (
    "bytes"
    "encoding/json"
    "io"
    "net/http"
)

type Handler struct {
    recorder *Recorder
}

func NewHandler(recorder *Recorder) *Handler {
    return &Handler{recorder: recorder}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // Read and store request body
    body, err := io.ReadAll(r.Body)
    if err != nil {
        http.Error(w, "Error reading request", http.StatusBadRequest)
        return
    }
    r.Body.Close()
    r.Body = io.NopCloser(bytes.NewReader(body))

    // Generate key from request
    var req map[string]interface{}
    if err := json.Unmarshal(body, &req); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }
    key := req["model"].(string)

    // Try to load existing recording
    rec, err := h.recorder.Load(key)
    if err == nil {
        // Found recording, replay it
        w.Header().Set("Content-Type", "application/json")
        w.Header().Set("X-Proxy-Cache", "HIT")
        w.Write(rec.Response)
        return
    }

    // No recording found, proxy to real API and save
    resp, err := http.DefaultClient.Do(r)
    if err != nil {
        http.Error(w, "Error calling API", http.StatusInternalServerError)
        return
    }
    defer resp.Body.Close()

    respBody, err := io.ReadAll(resp.Body)
    if err != nil {
        http.Error(w, "Error reading response", http.StatusInternalServerError)
        return
    }

    // Save recording
    if err := h.recorder.Save(key, body, respBody); err != nil {
        http.Error(w, "Error saving recording", http.StatusInternalServerError)
        return
    }

    // Return response
    w.Header().Set("Content-Type", "application/json")
    w.Header().Set("X-Proxy-Cache", "MISS")
    w.Write(respBody)
}

