package openai

import (
    "encoding/json"
)

type OpenAIMetadata struct {
    FinishReason      string   `json:"finish_reason"`
    SystemFingerprint string   `json:"system_fingerprint,omitempty"`
    CompletionTokens  int      `json:"completion_tokens"`
    PromptTokens      int      `json:"prompt_tokens"`
    TotalTokens       int      `json:"total_tokens"`
    Model            string   `json:"model"`
    FirstTokenTime   float64  `json:"first_token_time,omitempty"`
    TokensPerSecond  float64  `json:"tokens_per_second,omitempty"`
    ToolCalls       []ToolCall `json:"tool_calls,omitempty"`
}

type ToolCall struct {
    Type     string          `json:"type"`
    Name     string          `json:"name"`
    Duration float64         `json:"duration"`
    Result   json.RawMessage `json:"result,omitempty"`
}

func (m *OpenAIMetadata) ToJSON() (json.RawMessage, error) {
    return json.Marshal(m)
}

func (m *OpenAIMetadata) GetMetricValues() map[string]float64 {
    return map[string]float64{
        "llm_openai_completion_tokens": float64(m.CompletionTokens),
        "llm_openai_prompt_tokens":     float64(m.PromptTokens),
        "llm_openai_total_tokens":      float64(m.TotalTokens),
        "llm_openai_first_token_time":  m.FirstTokenTime,
        "llm_openai_tokens_per_second": m.TokensPerSecond,
    }
}
