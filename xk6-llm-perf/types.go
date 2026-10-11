package llmperf

import "encoding/json"

type Message struct {
	Role             string     `json:"role"`
	Content          any        `json:"content"`
	ReasoningContent string     `json:"reasoning_content,omitempty"`
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string     `json:"tool_call_id,omitempty"`
	Name             string     `json:"name,omitempty"`
}

type ToolCall struct {
	Index    int    `json:"index,omitempty"`
	ID       string `json:"id,omitempty"`
	Type     string `json:"type,omitempty"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type CompletionRequest struct {
	Work                *WorkExpectation  `json:"work,omitempty"`
	ScheduledAt         *float64          `json:"scheduledAt,omitempty"`
	CancelAfter         string            `json:"cancelAfter,omitempty"`
	Raw                 *RawRequest       `json:"raw,omitempty"`
	Model               string            `json:"model"`
	Messages            []Message         `json:"messages"`
	Temperature         *float64          `json:"temperature,omitempty"`
	MaxTokens           int               `json:"max_tokens,omitempty"`
	Stream              bool              `json:"stream,omitempty"`
	StreamOptions       *StreamOptions    `json:"stream_options,omitempty"`
	Measurement         *TokenCounts      `json:"measurement,omitempty"`
	Tools               []json.RawMessage `json:"tools,omitempty"`
	ToolChoice          any               `json:"tool_choice,omitempty"`
	MaxCompletionTokens int               `json:"max_completion_tokens,omitempty"`
}

type StreamOptions struct {
	IncludeUsage         bool `json:"include_usage,omitempty"`
	ContinuousUsageStats bool `json:"continuous_usage_stats,omitempty"`
}

type CompletionResponse struct {
	Sources json.RawMessage `json:"sources,omitempty"`
	Body    json.RawMessage `json:"body,omitempty"`
	Metrics struct {
		SpeculativeDecoding json.RawMessage `json:"speculative_decoding"`
	} `json:"metrics,omitempty"`
	ID             string       `json:"id"`
	Object         string       `json:"object"`
	Created        int64        `json:"created"`
	Model          string       `json:"model"`
	Choices        []Choice     `json:"choices"`
	Usage          Usage        `json:"usage"`
	Status         int          `json:"status"`
	ProviderStatus string       `json:"provider_status,omitempty"`
	Measurement    *Measurement `json:"measurement,omitempty"`
}

type Choice struct {
	Index               int             `json:"index"`
	SpeculativeDecoding json.RawMessage `json:"speculative_decoding,omitempty"`
	Message             Message         `json:"message"`
	FinishReason        string          `json:"finish_reason,omitempty"`
}

type Usage struct {
	PromptTokens      int `json:"prompt_tokens"`
	CompletionTokens  int `json:"completion_tokens"`
	TotalTokens       int `json:"total_tokens"`
	promptPresent     bool
	completionPresent bool
	totalPresent      bool
	ReasoningTokens   *int `json:"reasoning_tokens,omitempty"`
	CachedTokens      *int `json:"cached_tokens,omitempty"`
}

type ChatCompletionChunk struct {
	specDecodeUnavailable string
	ProviderStatus        string          `json:"provider_status,omitempty"`
	Sources               json.RawMessage `json:"sources,omitempty"`
	Metrics               struct {
		SpeculativeDecoding json.RawMessage `json:"speculative_decoding"`
	} `json:"metrics,omitempty"`
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Usage   Usage  `json:"usage,omitempty"`
	Choices []struct {
		Index               int             `json:"index"`
		SpeculativeDecoding json.RawMessage `json:"speculative_decoding,omitempty"`
		Delta               struct {
			Role             string     `json:"role,omitempty"`
			Content          string     `json:"content,omitempty"`
			ReasoningContent string     `json:"reasoning_content,omitempty"`
			ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason,omitempty"`
	} `json:"choices"`
}

type Config struct {
	MaxConnsPerHost      int      `json:"maxConnsPerHost"`
	ControlGroup         string   `json:"controlGroup"`
	APIKey               string   `json:"apiKey"`
	BaseURL              string   `json:"baseURL"`
	BaseURLs             []string `json:"baseURLs"`
	Model                string   `json:"model"`
	Timeout              string   `json:"timeout"`
	NetworkRTT           int      `json:"networkRTT"`
	TokenMultiplier      float64  `json:"tokenMultiplier"`
	RequestConcurrency   int      `json:"requestConcurrency"`
	UseK6Transport       bool     `json:"useK6Transport"`
	PrefillConcurrency   int      `json:"prefillConcurrency"`
	WarmupCount          int      `json:"warmupCount"`
	MaxTTFT              float64  `json:"maxTTFT"`
	MaxTTFO              float64  `json:"maxTTFO"`
	MaxTPOT              float64  `json:"maxTPOT"`
	MaxE2E               float64  `json:"maxE2E"`
	MaxTokenLatency      float64  `json:"maxTokenLatency"`
	StrictMeasurement    bool     `json:"strictMeasurement"`
	Phase                string   `json:"phase"`
	RTTSource            string   `json:"rttSource"`
	DiscrepancyTolerance *float64 `json:"discrepancyTolerance"`
}

// UnmarshalJSON normalizes the reasoning aliases used by compatible providers.
func (m *Message) UnmarshalJSON(data []byte) error {
	type plain Message
	var v struct {
		*plain
		Reasoning string `json:"reasoning"`
	}
	v.plain = (*plain)(m)
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	if m.ReasoningContent == "" {
		m.ReasoningContent = v.Reasoning
	}
	return nil
}

func (c *ChatCompletionChunk) UnmarshalJSON(data []byte) error {
	type plain ChatCompletionChunk
	if err := json.Unmarshal(data, (*plain)(c)); err != nil {
		return err
	}
	var aliases struct {
		Choices []struct {
			Delta struct {
				Reasoning string `json:"reasoning"`
			} `json:"delta"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &aliases); err != nil {
		return err
	}
	for i := range c.Choices {
		if c.Choices[i].Delta.ReasoningContent == "" {
			c.Choices[i].Delta.ReasoningContent = aliases.Choices[i].Delta.Reasoning
		}
	}
	return nil
}
