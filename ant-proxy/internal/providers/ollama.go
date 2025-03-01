package providers

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
)

type OllamaProvider struct {
    endpoint string
    client   *http.Client
}

type ollamaRequest struct {
    Model  string `json:"model"`
    Prompt string `json:"prompt"`
}

type ollamaResponse struct {
    Response string `json:"response"`
}

func NewOllamaProvider(endpoint string) *OllamaProvider {
    if endpoint == "" {
        endpoint = "http://localhost:11434/api/generate"
    }
    return &OllamaProvider{
        endpoint: endpoint,
        client:   &http.Client{},
    }
}

func (p *OllamaProvider) Name() string {
    return "ollama"
}

func (p *OllamaProvider) Generate(ctx context.Context, req *Request) (*Response, error) {
    ollamaReq := ollamaRequest{
        Model:  req.Model,
        Prompt: concatMessages(req.Messages),
    }

    body, err := json.Marshal(ollamaReq)
    if err != nil {
        return nil, fmt.Errorf("marshal request: %w", err)
    }

    httpReq, err := http.NewRequestWithContext(ctx, "POST", p.endpoint, bytes.NewReader(body))
    if err != nil {
        return nil, fmt.Errorf("create request: %w", err)
    }
    httpReq.Header.Set("Content-Type", "application/json")

    resp, err := p.client.Do(httpReq)
    if err != nil {
        return nil, fmt.Errorf("do request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("bad status: %s, body: %s", resp.Status, body)
    }

    var ollamaResp ollamaResponse
    if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
        return nil, fmt.Errorf("decode response: %w", err)
    }

    return &Response{Content: ollamaResp.Response}, nil
}

func (p *OllamaProvider) ModifyRequest(req *http.Request) error {
    // No modifications needed for Ollama
    return nil
}

func concatMessages(msgs []Message) string {
    var prompt string
    for _, msg := range msgs {
        if msg.Role == "system" {
            prompt += "[SYSTEM] " + msg.Content + "\n"
        } else {
            prompt += msg.Content + "\n"
        }
    }
    return prompt
}

