package llmperf

import (
	"encoding/json"
	"fmt"
	"strings"
)

// RawRequest sends an authored JSON payload. Protocol selects a response
// adapter; json leaves token metrics unavailable without declared counts.
type RawRequest struct {
	Path     string         `json:"path"`
	Protocol string         `json:"protocol"`
	Body     map[string]any `json:"body"`
}

func validateRaw(r *RawRequest) error {
	if len(r.Path) > 2048 {
		return fmt.Errorf("raw path exceeds size limit")
	}
	if r.Body == nil {
		return fmt.Errorf("raw body is required")
	}
	if !strings.HasPrefix(r.Path, "/") || strings.HasPrefix(r.Path, "//") || strings.ContainsAny(r.Path, "?#\\") || strings.Contains(r.Path, "..") {
		return fmt.Errorf("raw path must be an absolute API path without query or traversal")
	}
	switch r.Protocol {
	case "openai-chat", "openai-completions", "openai-responses", "anthropic", "json":
	default:
		return fmt.Errorf("unsupported response protocol")
	}
	if stream, ok := r.Body["stream"]; ok {
		if _, ok := stream.(bool); !ok {
			return fmt.Errorf("body.stream must be boolean")
		}
	}
	return nil
}

func canonicalUsage(raw json.RawMessage, protocol string) (Usage, error) {
	var u Usage
	if len(raw) == 0 || string(raw) == "null" {
		return u, nil
	}
	if protocol != "anthropic" && protocol != "openai-responses" {
		return u, json.Unmarshal(raw, &u)
	}
	var v map[string]json.RawMessage
	if err := json.Unmarshal(raw, &v); err != nil {
		return u, err
	}
	if n, ok := v["input_tokens"]; ok {
		v["prompt_tokens"] = n
	}
	if n, ok := v["output_tokens"]; ok {
		v["completion_tokens"] = n
	}
	if n, ok := v["input_tokens_details"]; ok {
		v["prompt_tokens_details"] = n
	}
	if n, ok := v["output_tokens_details"]; ok {
		v["completion_tokens_details"] = n
	}
	data, _ := json.Marshal(v)
	return u, json.Unmarshal(data, &u)
}

func decodeResponse(data []byte, protocol string) (CompletionResponse, error) {
	var result CompletionResponse
	if protocol == "" || protocol == "openai-chat" {
		return result, json.Unmarshal(data, &result)
	}
	var raw struct {
		ID      string          `json:"id"`
		Model   string          `json:"model"`
		Error   json.RawMessage `json:"error"`
		Usage   json.RawMessage `json:"usage"`
		Choices []struct {
			Text         string `json:"text"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Output []struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
		Content []struct {
			Type     string `json:"type"`
			Text     string `json:"text"`
			Thinking string `json:"thinking"`
		} `json:"content"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return result, err
	}
	if len(raw.Error) > 0 && string(raw.Error) != "null" {
		return result, fmt.Errorf("provider error response")
	}
	result.ID, result.Model = raw.ID, raw.Model
	u, err := canonicalUsage(raw.Usage, protocol)
	if err != nil {
		return result, err
	}
	result.Usage = u
	if protocol == "json" {
		result.Body = append(json.RawMessage(nil), data...)
		return result, nil
	}
	var text, reasoning strings.Builder
	switch protocol {
	case "openai-completions":
		for _, c := range raw.Choices {
			result.Choices = append(result.Choices, Choice{Message: Message{Role: "assistant", Content: c.Text}, FinishReason: c.FinishReason})
		}
	case "openai-responses":
		for _, item := range raw.Output {
			for _, part := range item.Content {
				text.WriteString(part.Text)
			}
		}
	case "anthropic":
		for _, part := range raw.Content {
			text.WriteString(part.Text)
			reasoning.WriteString(part.Thinking)
		}
	}
	if protocol != "openai-completions" && (text.Len() > 0 || reasoning.Len() > 0) {
		result.Choices = []Choice{{Message: Message{Role: "assistant", Content: text.String(), ReasoningContent: reasoning.String()}}}
	}
	result.Body = append(json.RawMessage(nil), data...)
	return result, nil
}

func decodeChunk(data []byte, protocol string) (ChatCompletionChunk, bool, error) {
	var result ChatCompletionChunk
	if protocol == "" || protocol == "openai-chat" {
		return result, false, json.Unmarshal(data, &result)
	}
	var raw struct {
		Type    string          `json:"type"`
		ID      string          `json:"id"`
		Model   string          `json:"model"`
		Error   json.RawMessage `json:"error"`
		Usage   json.RawMessage `json:"usage"`
		Choices []struct {
			Text         string `json:"text"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Delta    json.RawMessage `json:"delta"`
		Response json.RawMessage `json:"response"`
		Message  struct {
			ID    string          `json:"id"`
			Model string          `json:"model"`
			Usage json.RawMessage `json:"usage"`
		} `json:"message"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return result, false, err
	}
	if raw.Type == "error" || raw.Type == "response.failed" || len(raw.Error) > 0 && string(raw.Error) != "null" {
		return result, false, fmt.Errorf("provider stream error")
	}
	result.ID, result.Model = raw.ID, raw.Model
	content, reasoning, finish := "", "", ""
	complete := false
	switch protocol {
	case "openai-completions":
		if len(raw.Choices) > 0 {
			content = raw.Choices[0].Text
			finish = raw.Choices[0].FinishReason
		}
	case "openai-responses":
		switch raw.Type {
		case "response.output_text.delta":
			if err := json.Unmarshal(raw.Delta, &content); err != nil {
				return result, false, err
			}
		case "response.reasoning_summary_text.delta":
			if err := json.Unmarshal(raw.Delta, &reasoning); err != nil {
				return result, false, err
			}
		case "response.completed":
			var response struct {
				ID    string          `json:"id"`
				Model string          `json:"model"`
				Usage json.RawMessage `json:"usage"`
			}
			if err := json.Unmarshal(raw.Response, &response); err != nil {
				return result, false, err
			}
			result.ID, result.Model, raw.Usage = response.ID, response.Model, response.Usage
			complete = true
		}
	case "anthropic":
		if raw.Type == "message_start" {
			result.ID, result.Model, raw.Usage = raw.Message.ID, raw.Message.Model, raw.Message.Usage
		}
		if raw.Type == "content_block_delta" {
			var delta struct {
				Text     string `json:"text"`
				Thinking string `json:"thinking"`
			}
			if err := json.Unmarshal(raw.Delta, &delta); err != nil {
				return result, false, err
			}
			content, reasoning = delta.Text, delta.Thinking
		}
		complete = raw.Type == "message_stop"
	case "json":
		return result, false, fmt.Errorf("json protocol does not support SSE; select a streaming adapter")
	}
	u, err := canonicalUsage(raw.Usage, protocol)
	if err != nil {
		return result, false, err
	}
	result.Usage = u
	// Use the chat wire shape internally without exposing a second chunk model.
	encoded, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"delta": map[string]any{"content": content, "reasoning_content": reasoning}, "finish_reason": finish}}})
	var normalized ChatCompletionChunk
	if err := json.Unmarshal(encoded, &normalized); err != nil {
		return result, false, err
	}
	result.Choices = normalized.Choices
	return result, complete, nil
}

func mergeUsage(previous, next Usage) Usage {
	if next.promptPresent {
		previous.PromptTokens, previous.promptPresent = next.PromptTokens, true
	}
	if next.completionPresent {
		previous.CompletionTokens, previous.completionPresent = next.CompletionTokens, true
	}
	if next.totalPresent {
		previous.TotalTokens, previous.totalPresent = next.TotalTokens, true
	}
	if next.CachedTokens != nil {
		previous.CachedTokens = next.CachedTokens
	}
	if next.ReasoningTokens != nil {
		previous.ReasoningTokens = next.ReasoningTokens
	}
	return previous
}
