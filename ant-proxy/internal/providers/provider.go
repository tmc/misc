package providers

import (
    "context"
    "net/http"
)

// Provider defines the interface for LLM providers
type Provider interface {
    Name() string
    Generate(ctx context.Context, req *Request) (*Response, error)
    ModifyRequest(req *http.Request) error
}

type Request struct {
    Model       string    `json:"model"`
    Messages    []Message `json:"messages"`
    MaxTokens   int       `json:"max_tokens"`
    Temperature float64   `json:"temperature"`
}

type Message struct {
    Role    string `json:"role"`
    Content string `json:"content"`
}

type Response struct {
    Content string `json:"content"`
}

