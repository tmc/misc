package llmperf

type ClientConfig struct {
	APIKey   string `json:"apiKey"`
	BaseURL  string `json:"baseURL"`
	Model    string `json:"model"`
	Timeout  string `json:"timeout"`
	MaxRetries int  `json:"maxRetries"`
}

type CompletionRequest struct {
	BaseURL     string    `json:"baseURL"`
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type CompletionResponse struct {
	ID      string    `json:"id"`
	Object  string    `json:"object"`
	Created int64     `json:"created"`
	Model   string    `json:"model"`
	Choices []Choice  `json:"choices"`
	Usage   Usage     `json:"usage"`
	Status  int       `json:"status"`
}

type Choice struct {
	Message       Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}
