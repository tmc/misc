package proxy

// Request represents an Anthropic API request
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

