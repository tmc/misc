package llmperf

import (
	"encoding/json"
	"fmt"
	"strings"
)

// RawRequest sends an authored JSON payload. Protocol selects a response
// adapter; json leaves token metrics unavailable without declared counts.
type RawRequest struct {
	Multipart *RawMultipart  `json:"multipart,omitempty"`
	Poll      *VideoPolling  `json:"poll,omitempty"`
	Stream    *bool          `json:"stream,omitempty"`
	Path      string         `json:"path"`
	Protocol  string         `json:"protocol"`
	Body      map[string]any `json:"body"`
}

func generatedTokenProtocol(protocol string) bool {
	switch protocol {
	case "", "openai-chat", "openai-completions", "openai-responses", "anthropic", "tgi", "solido-rag":
		return true
	}
	return false
}

func validateRaw(r *RawRequest) error {
	if len(r.Path) > 2048 {
		return fmt.Errorf("raw path exceeds size limit")
	}
	if (r.Body == nil) == (r.Multipart == nil) {
		return fmt.Errorf("exactly one raw body or multipart is required")
	}
	if !strings.HasPrefix(r.Path, "/") || strings.HasPrefix(r.Path, "//") || strings.ContainsAny(r.Path, "?#\\") || strings.Contains(r.Path, "..") {
		return fmt.Errorf("raw path must be an absolute API path without query or traversal")
	}
	switch r.Protocol {
	case "openai-chat", "openai-completions", "openai-responses", "anthropic", "tgi", "json", "audio-transcription", "image", "video", "retrieval", "openai-embeddings", "cohere-rankings", "nim-rankings", "hf-tei-rankings", "nim-image-retrieval", "nv-embedqa", "chat-embeddings", "nim-embeddings", "solido-rag":
	default:
		return fmt.Errorf("unsupported response protocol")
	}
	if err := validateEndpointBody(r.Protocol, r.Body); err != nil {
		return err
	}
	if r.Poll != nil && r.Protocol != "video" {
		return fmt.Errorf("poll requires video protocol")
	}
	if r.Protocol == "video" {
		if _, _, err := videoPollPolicy(r.Poll); err != nil {
			return err
		}
	}
	if r.Stream != nil && *r.Stream && (r.Protocol == "audio-transcription" || r.Protocol == "video" || r.Protocol == "retrieval") {
		return fmt.Errorf("protocol does not support streaming")
	}
	if stream, ok := r.Body["stream"]; ok {
		value, ok := stream.(bool)
		if !ok {
			return fmt.Errorf("body.stream must be boolean")
		}
		if r.Stream != nil && *r.Stream != value {
			return fmt.Errorf("stream conflicts with body.stream")
		}
		if value && (r.Protocol == "audio-transcription" || r.Protocol == "video" || r.Protocol == "retrieval" || nonGenerationProtocol(r.Protocol)) {
			return fmt.Errorf("protocol does not support streaming")
		}
	}
	if nonGenerationProtocol(r.Protocol) && r.Stream != nil && *r.Stream {
		return fmt.Errorf("protocol does not support streaming")
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
	if protocol == "anthropic" {
		if cached, ok := v["cache_read_input_tokens"]; ok {
			details, _ := json.Marshal(map[string]json.RawMessage{"cached_tokens": cached})
			v["prompt_tokens_details"] = details
		}
	}
	data, _ := json.Marshal(v)
	return u, json.Unmarshal(data, &u)
}

func decodeResponse(data []byte, protocol string) (CompletionResponse, error) {
	if protocol == "solido-rag" {
		return decodeSolidoResponse(data)
	}
	if nonGenerationProtocol(protocol) {
		return decodeNonGeneration(data, protocol)
	}
	if protocol == "audio-transcription" {
		return decodeAudioResponse(data)
	}
	if protocol == "image" || protocol == "retrieval" {
		return decodeMediaResponse(data, protocol)
	}
	if protocol == "video" {
		protocol = "json"
	}
	if protocol == "tgi" {
		return decodeTGI(data)
	}
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
			Index        int    `json:"index"`
			Text         string `json:"text"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Output []struct {
			Type      string `json:"type"`
			ID        string `json:"id"`
			CallID    string `json:"call_id"`
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
			Summary   []struct {
				Text string `json:"text"`
			} `json:"summary"`
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
		Content []struct {
			Type     string          `json:"type"`
			Text     string          `json:"text"`
			Thinking string          `json:"thinking"`
			ID       string          `json:"id"`
			Name     string          `json:"name"`
			Input    json.RawMessage `json:"input"`
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
	var vendor CompletionResponse
	if err := json.Unmarshal(data, &vendor); err != nil {
		return result, err
	}
	result.Metrics = vendor.Metrics
	var text, reasoning strings.Builder
	var tools []ToolCall
	switch protocol {
	case "openai-completions":
		for i, c := range raw.Choices {
			choice := Choice{Index: c.Index, Message: Message{Role: "assistant", Content: c.Text}, FinishReason: c.FinishReason}
			if i < len(vendor.Choices) {
				choice.SpeculativeDecoding = vendor.Choices[i].SpeculativeDecoding
			}
			result.Choices = append(result.Choices, choice)
		}
	case "openai-responses":
		for _, item := range raw.Output {
			if item.Type == "function_call" {
				tools = append(tools, normalizedTool(len(tools), item.CallID, item.Name, item.Arguments))
			}
			for _, part := range item.Summary {
				reasoning.WriteString(part.Text)
			}
			for _, part := range item.Content {
				text.WriteString(part.Text)
			}
		}
	case "anthropic":
		for _, part := range raw.Content {
			if part.Type == "tool_use" {
				tools = append(tools, normalizedTool(len(tools), part.ID, part.Name, string(part.Input)))
			}
			text.WriteString(part.Text)
			reasoning.WriteString(part.Thinking)
		}
	}
	if protocol != "openai-completions" && (text.Len() > 0 || reasoning.Len() > 0 || len(tools) > 0) {
		result.Choices = []Choice{{Message: Message{Role: "assistant", Content: text.String(), ReasoningContent: reasoning.String(), ToolCalls: tools}}}
	}
	result.Body = append(json.RawMessage(nil), data...)
	return result, nil
}

func decodeChunk(data []byte, protocol string) (ChatCompletionChunk, bool, error) {
	if protocol == "solido-rag" {
		return decodeSolidoChunk(data)
	}
	var result ChatCompletionChunk
	if protocol == "" || protocol == "openai-chat" {
		return result, false, json.Unmarshal(data, &result)
	}
	var vendor ChatCompletionChunk
	if err := json.Unmarshal(data, &vendor); err != nil {
		return result, false, err
	}
	result.Metrics = vendor.Metrics
	var raw struct {
		Type    string          `json:"type"`
		ID      string          `json:"id"`
		Model   string          `json:"model"`
		Error   json.RawMessage `json:"error"`
		Usage   json.RawMessage `json:"usage"`
		Choices []struct {
			Index        int    `json:"index"`
			Text         string `json:"text"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Index       int `json:"index"`
		OutputIndex int `json:"output_index"`
		Item        struct {
			Type   string `json:"type"`
			ID     string `json:"id"`
			CallID string `json:"call_id"`
			Name   string `json:"name"`
		} `json:"item"`
		ContentBlock struct {
			Type  string          `json:"type"`
			ID    string          `json:"id"`
			Name  string          `json:"name"`
			Input json.RawMessage `json:"input"`
		} `json:"content_block"`
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
	var tools []ToolCall
	switch protocol {
	case "openai-completions":
		for i, choice := range raw.Choices {
			encoded, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"index": choice.Index, "delta": map[string]any{"content": choice.Text}, "finish_reason": choice.FinishReason}}})
			var normalized ChatCompletionChunk
			if err := json.Unmarshal(encoded, &normalized); err != nil {
				return result, false, err
			}
			if i < len(vendor.Choices) {
				normalized.Choices[0].SpeculativeDecoding = vendor.Choices[i].SpeculativeDecoding
			}
			result.Choices = append(result.Choices, normalized.Choices[0])
		}
		u, err := canonicalUsage(raw.Usage, protocol)
		result.Usage = u
		return result, false, err
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
		case "response.output_item.added":
			if raw.Item.Type == "function_call" {
				tools = append(tools, normalizedTool(raw.OutputIndex, raw.Item.CallID, raw.Item.Name, ""))
			}
		case "response.function_call_arguments.delta":
			var args string
			if err := json.Unmarshal(raw.Delta, &args); err != nil {
				return result, false, err
			}
			tools = append(tools, normalizedTool(raw.OutputIndex, "", "", args))
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
		if raw.Type == "content_block_start" && raw.ContentBlock.Type == "tool_use" {
			args := string(raw.ContentBlock.Input)
			// Empty initial input is a placeholder; argument deltas follow.
			if args == "{}" || args == "null" {
				args = ""
			}
			tools = append(tools, normalizedTool(raw.Index, raw.ContentBlock.ID, raw.ContentBlock.Name, args))
		}
		if raw.Type == "content_block_delta" {
			var delta struct {
				Text        string `json:"text"`
				Thinking    string `json:"thinking"`
				Type        string `json:"type"`
				PartialJSON string `json:"partial_json"`
			}
			if err := json.Unmarshal(raw.Delta, &delta); err != nil {
				return result, false, err
			}
			content, reasoning = delta.Text, delta.Thinking
			if delta.Type == "input_json_delta" {
				tools = append(tools, normalizedTool(raw.Index, "", "", delta.PartialJSON))
			}
		}
		complete = raw.Type == "message_stop"
	case "tgi":
		var v tgiResponse
		if err := json.Unmarshal(data, &v); err != nil {
			return result, false, err
		}
		if v.Token != nil {
			content = v.Token.Text
		}
		if v.GeneratedText != nil {
			complete = true
			finish = v.Details.FinishReason
			raw.Usage = tgiUsage(v)
		}
	case "json":
		return result, false, fmt.Errorf("json protocol does not support SSE; select a streaming adapter")
	}
	u, err := canonicalUsage(raw.Usage, protocol)
	if err != nil {
		return result, false, err
	}
	result.Usage = u
	// Use the chat wire shape internally without exposing a second chunk model.
	encoded, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"delta": map[string]any{"content": content, "reasoning_content": reasoning, "tool_calls": tools}, "finish_reason": finish}}})
	var normalized ChatCompletionChunk
	if err := json.Unmarshal(encoded, &normalized); err != nil {
		return result, false, err
	}
	result.Choices = normalized.Choices
	if len(vendor.Choices) > 0 && len(result.Choices) > 0 {
		result.Choices[0].SpeculativeDecoding = vendor.Choices[0].SpeculativeDecoding
		result.Choices[0].Index = vendor.Choices[0].Index
	}
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

func normalizedTool(index int, id, name, args string) ToolCall {
	t := ToolCall{Index: index, ID: id, Type: "function"}
	t.Function.Name, t.Function.Arguments = name, args
	return t
}

// validateCompletedTools checks the terminal assembled tool payload. Partial
// arguments remain strings until completion and are never executed.
func validateCompletedTools(choices []Choice) error {
	for _, c := range choices {
		if len(c.Message.ToolCalls) > 128 {
			return fmt.Errorf("tool call count exceeds limit")
		}
		for _, t := range c.Message.ToolCalls {
			if t.Index < 0 || t.Index >= 128 || t.Function.Name == "" || len(t.Function.Name) > 1024 || len(t.ID) > 1024 || len(t.Function.Arguments) > 4<<20 {
				return fmt.Errorf("invalid completed tool call")
			}
			if t.Function.Arguments == "" {
				continue
			} // A no-argument call is valid.
			var args map[string]json.RawMessage
			if err := json.Unmarshal([]byte(t.Function.Arguments), &args); err != nil || args == nil {
				return fmt.Errorf("invalid completed tool arguments")
			}
		}
	}
	return nil
}
