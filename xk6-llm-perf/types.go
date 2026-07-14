package llmperf

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type CompletionRequest struct {
	Model         string         `json:"model"`
	Messages      []Message      `json:"messages"`
	Temperature   float64        `json:"temperature,omitempty"`
	MaxTokens     int            `json:"max_tokens,omitempty"`
	Stream        bool           `json:"stream,omitempty"`
	StreamOptions *StreamOptions `json:"stream_options,omitempty"`
}

type StreamOptions struct {
	IncludeUsage bool `json:"include_usage,omitempty"`
}

type CompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
	Status  int      `json:"-"`
}

type Choice struct {
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason,omitempty"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type ChatCompletionChunk struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Usage   Usage  `json:"usage,omitempty"`
	Choices []struct {
		Delta struct {
			Role             string `json:"role,omitempty"`
			Content          string `json:"content,omitempty"`
			ReasoningContent string `json:"reasoning_content,omitempty"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason,omitempty"`
	} `json:"choices"`
}

type Config struct {
	APIKey             string   `json:"apiKey"`
	BaseURL            string   `json:"baseURL"`
	BaseURLs           []string `json:"baseURLs"`
	Model              string   `json:"model"`
	Timeout            string   `json:"timeout"`
	NetworkRTT         int      `json:"networkRTT"`
	TokenMultiplier    float64  `json:"tokenMultiplier"`
	PrefillConcurrency int      `json:"prefillConcurrency"`
	WarmupCount        int      `json:"warmupCount"`
	MaxTTFT            int      `json:"maxTTFT"`
	MaxTTFO            int      `json:"maxTTFO"`
	MaxTokenLatency    int      `json:"maxTokenLatency"`
}
