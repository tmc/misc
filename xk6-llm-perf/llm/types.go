package llm

// CompletionRequest represents an LLM completion request
type CompletionRequest struct {
    Model       string    `json:"model"`
    Messages    []Message `json:"messages"`
    MaxTokens   int       `json:"max_tokens,omitempty"`
    Temperature float64   `json:"temperature,omitempty"`
    TopP        float64   `json:"top_p,omitempty"`
    Stream      bool      `json:"stream,omitempty"`
}

type Message struct {
    Role    string `json:"role"`
    Content string `json:"content"`
}

type CompletionResponse struct {
    Status  int    `json:"status"`
    Text    string `json:"text"`
}

type ChatCompletion struct {
    ID      string `json:"id"`
    Object  string `json:"object"`
    Created int64  `json:"created"`
    Model   string `json:"model"`
    Usage   struct {
        PromptTokens     int `json:"prompt_tokens"`
        CompletionTokens int `json:"completion_tokens"`
        TotalTokens      int `json:"total_tokens"`
    } `json:"usage"`
    Choices []struct {
        Message struct {
            Role    string `json:"role"`
            Content string `json:"content"`
        } `json:"message"`
        FinishReason string `json:"finish_reason"`
    } `json:"choices"`
}

type ChatCompletionChunk struct {
    ID      string `json:"id"`
    Object  string `json:"object"`
    Created int64  `json:"created"`
    Model   string `json:"model"`
    Choices []struct {
        Delta struct {
            Role    string `json:"role,omitempty"`
            Content string `json:"content,omitempty"`
        } `json:"delta"`
        FinishReason string `json:"finish_reason"`
    } `json:"choices"`
}
