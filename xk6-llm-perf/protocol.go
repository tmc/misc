package llmperf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/tmc/misc/xk6-llm-perf/internal/jsoncheck"
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
	case "", "openai-chat", "openai-completions", "openai-responses", "anthropic", "tgi", "solido-rag", "providerwire-v4":
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
	case "providerwire-v4", "openai-chat", "openai-completions", "openai-responses", "anthropic", "tgi", "json", "audio-transcription", "image", "video", "retrieval", "openai-embeddings", "cohere-rankings", "nim-rankings", "hf-tei-rankings", "nim-image-retrieval", "nv-embedqa", "chat-embeddings", "nim-embeddings", "solido-rag":
	default:
		return fmt.Errorf("unsupported response protocol")
	}
	if r.Protocol == "providerwire-v4" {
		if r.Multipart != nil {
			return fmt.Errorf("providerwire requires a JSON body")
		}
		if err := validateProviderwireBody(r.Body); err != nil {
			return err
		}
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
	if protocol == "openai-responses" && len(raw) > 0 {
		if err := checkOpenAIUsage(raw); err != nil {
			return u, err
		}
		var aliases struct {
			Input        *int `json:"input_tokens"`
			Prompt       *int `json:"prompt_tokens"`
			Output       *int `json:"output_tokens"`
			Completion   *int `json:"completion_tokens"`
			InputDetails struct {
				Cached *int `json:"cached_tokens"`
			} `json:"input_tokens_details"`
			PromptDetails struct {
				Cached *int `json:"cached_tokens"`
			} `json:"prompt_tokens_details"`
			OutputDetails struct {
				Reasoning *int `json:"reasoning_tokens"`
			} `json:"output_tokens_details"`
			CompletionDetails struct {
				Reasoning *int `json:"reasoning_tokens"`
			} `json:"completion_tokens_details"`
		}
		if err := json.Unmarshal(raw, &aliases); err != nil {
			return u, err
		}
		for _, pair := range [][2]*int{{aliases.Input, aliases.Prompt}, {aliases.Output, aliases.Completion}, {aliases.InputDetails.Cached, aliases.PromptDetails.Cached}, {aliases.OutputDetails.Reasoning, aliases.CompletionDetails.Reasoning}} {
			if pair[0] != nil && *pair[0] < 0 || pair[1] != nil && *pair[1] < 0 {
				return u, fmt.Errorf("negative usage count")
			}
			if pair[0] != nil && pair[1] != nil && *pair[0] != *pair[1] {
				return u, fmt.Errorf("responses usage aliases disagree")
			}
		}
	}
	if len(raw) == 0 || string(raw) == "null" {
		return u, nil
	}
	if protocol != "anthropic" && protocol != "openai-responses" {
		return u, json.Unmarshal(raw, &u)
	}
	if protocol == "anthropic" {
		return anthropicUsage(raw)
	}
	var v map[string]json.RawMessage
	if err := json.Unmarshal(raw, &v); err != nil {
		return u, err
	}
	// Typed alias admission uses encoding/json's field identity; preserve that
	// same identity when converting Responses native names through a map.
	for name, value := range v {
		for _, canonical := range []string{"input_tokens", "output_tokens", "prompt_tokens", "completion_tokens", "input_tokens_details", "output_tokens_details", "prompt_tokens_details", "completion_tokens_details"} {
			if name != canonical && strings.EqualFold(name, canonical) {
				delete(v, name)
				v[canonical] = value
				break
			}
		}
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

// checkProviderJSON retains member identity before typed or map decoding can
// replace duplicate evidence. Null stays available to the usage presence policy.
func checkProviderJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	return jsoncheck.Check(d, jsoncheck.Options{})
}

// providerFields checks case aliases only for fields decoded into fixed
// structs and returns their values by canonical name. Opaque tool inputs and
// metadata keep their case-sensitive identities.
func providerFields(data []byte, fields ...string) (map[string]json.RawMessage, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return nil, err
	}
	values := make(map[string]json.RawMessage, len(fields))
	for _, field := range fields {
		found := false
		for name, value := range object {
			if strings.EqualFold(name, field) {
				if found {
					return nil, fmt.Errorf("duplicate provider JSON field %q", field)
				}
				found = true
				values[field] = value
			}
		}
	}
	return values, nil
}

func checkAnthropicEnvelope(data []byte) error {
	if err := checkProviderJSON(data); err != nil {
		return err
	}
	if _, err := providerFields(data, "type", "content", "usage", "message", "delta", "content_block", "index", "error", "id", "model", "stop_reason"); err != nil {
		return err
	}
	var envelope struct {
		Message      json.RawMessage   `json:"message"`
		Delta        json.RawMessage   `json:"delta"`
		ContentBlock json.RawMessage   `json:"content_block"`
		Content      []json.RawMessage `json:"content"`
		Error        json.RawMessage   `json:"error"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return err
	}
	if len(envelope.Message) > 0 {
		if _, err := providerFields(envelope.Message, "usage", "id", "model"); err != nil {
			return err
		}
	}
	if len(envelope.Delta) > 0 {
		if _, err := providerFields(envelope.Delta, "type", "text", "thinking", "partial_json", "stop_reason", "stop_sequence"); err != nil {
			return err
		}
	}
	for _, block := range append(envelope.Content, envelope.ContentBlock) {
		if len(block) > 0 {
			if _, err := providerFields(block, "type", "id", "name", "input", "text", "thinking"); err != nil {
				return err
			}
		}
	}
	if len(envelope.Error) > 0 {
		if _, err := providerFields(envelope.Error, "type", "message"); err != nil {
			return err
		}
	}
	return nil
}

// checkOpenAIUsage checks reported count identities without treating opaque
// provider extensions as fixed-schema objects.
func checkOpenAIUsage(data []byte) error {
	if err := checkProviderJSON(data); err != nil {
		return err
	}
	if _, err := providerFields(data, "prompt_tokens", "completion_tokens", "total_tokens", "cache_write_tokens", "reasoning_tokens", "prompt_tokens_details", "completion_tokens_details", "input_tokens", "output_tokens", "input_tokens_details", "output_tokens_details"); err != nil {
		return err
	}
	var v struct {
		Prompt     json.RawMessage `json:"prompt_tokens_details"`
		Completion json.RawMessage `json:"completion_tokens_details"`
		Input      json.RawMessage `json:"input_tokens_details"`
		Output     json.RawMessage `json:"output_tokens_details"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	for _, item := range []struct {
		raw   json.RawMessage
		field string
	}{{v.Prompt, "cached_tokens"}, {v.Input, "cached_tokens"}, {v.Completion, "reasoning_tokens"}, {v.Output, "reasoning_tokens"}} {
		if len(item.raw) > 0 {
			if _, err := providerFields(item.raw, item.field); err != nil {
				return err
			}
		}
	}
	return nil
}

func checkOpenAIEnvelope(data []byte, responses bool) error {
	if err := checkProviderJSON(data); err != nil {
		return err
	}
	fields := []string{"usage", "choices", "error", "id", "model", "metrics"}
	if responses {
		fields = append(fields, "response", "type", "status", "stop_reason", "delta", "item", "output", "output_index", "content_index", "item_id", "text")
	}
	v, err := providerFields(data, fields...)
	if err != nil {
		return err
	}
	if len(v["usage"]) > 0 {
		if err := checkOpenAIUsage(v["usage"]); err != nil {
			return err
		}
	}
	if len(v["choices"]) > 0 {
		if err := checkOpenAIChoices(v["choices"]); err != nil {
			return err
		}
	}
	if err := checkOpenAIMetrics(v["metrics"]); err != nil {
		return err
	}
	if responses {
		return checkResponsesFields(v)
	}
	return nil
}

func checkOpenAIMetrics(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	_, err := providerFields(data, "speculative_decoding")
	return err
}

// checkResponsesFields checks the fixed Responses members that feed content,
// reasoning, tool, status and usage measurements.
func checkResponsesFields(v map[string]json.RawMessage) error {
	if len(v["response"]) > 0 {
		r, err := providerFields(v["response"], "usage", "metrics", "status", "id", "model", "error")
		if err != nil {
			return err
		}
		if len(r["usage"]) > 0 {
			if err := checkOpenAIUsage(r["usage"]); err != nil {
				return err
			}
		}
		if err := checkOpenAIMetrics(r["metrics"]); err != nil {
			return err
		}
	}
	if len(v["item"]) > 0 {
		if _, err := providerFields(v["item"], "type", "id", "call_id", "name"); err != nil {
			return err
		}
	}
	if len(v["output"]) == 0 {
		return nil
	}
	items, err := providerFieldsEach(v["output"], "type", "id", "call_id", "name", "arguments", "summary", "content")
	if err != nil {
		return err
	}
	for _, item := range items {
		if len(item["summary"]) > 0 {
			if _, err := providerFieldsEach(item["summary"], "text"); err != nil {
				return err
			}
		}
		if len(item["content"]) > 0 {
			if _, err := providerFieldsEach(item["content"], "type", "text"); err != nil {
				return err
			}
		}
	}
	return nil
}

// providerFieldsEach applies providerFields to each element of a JSON array.
func providerFieldsEach(data []byte, fields ...string) ([]map[string]json.RawMessage, error) {
	var elems []json.RawMessage
	if err := json.Unmarshal(data, &elems); err != nil {
		return nil, err
	}
	values := make([]map[string]json.RawMessage, len(elems))
	for i, elem := range elems {
		v, err := providerFields(elem, fields...)
		if err != nil {
			return nil, err
		}
		values[i] = v
	}
	return values, nil
}

// checkOpenAIChoices checks the fixed chat and completions choice members
// that feed content, finish and tool measurements. Function arguments, content
// parts and extension objects are opaque.
func checkOpenAIChoices(data []byte) error {
	choices, err := providerFieldsEach(data, "index", "delta", "message", "finish_reason", "text", "speculative_decoding")
	if err != nil {
		return err
	}
	for _, c := range choices {
		for _, raw := range []json.RawMessage{c["delta"], c["message"]} {
			if len(raw) == 0 {
				continue
			}
			m, err := providerFields(raw, "role", "content", "reasoning_content", "reasoning", "tool_calls", "refusal", "tool_call_id", "name")
			if err != nil {
				return err
			}
			if len(m["tool_calls"]) == 0 {
				continue
			}
			tools, err := providerFieldsEach(m["tool_calls"], "index", "id", "type", "function")
			if err != nil {
				return err
			}
			for _, t := range tools {
				if len(t["function"]) > 0 {
					if _, err := providerFields(t["function"], "name", "arguments"); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func openAIUsageProtocol(protocol string) bool {
	return protocol == "" || protocol == "openai-chat" || protocol == "openai-completions" || protocol == "openai-responses"
}

// anthropicUsage accepts only native count sources. Normalized aliases cannot
// supply missing native counts; when both are reported they must agree.
func anthropicUsage(raw json.RawMessage) (Usage, error) {
	if err := checkProviderJSON(raw); err != nil {
		return Usage{}, err
	}
	if _, err := providerFields(raw, "input_tokens", "output_tokens", "cache_read_input_tokens", "cache_creation_input_tokens", "total_tokens", "prompt_tokens", "completion_tokens"); err != nil {
		return Usage{}, err
	}
	var v struct {
		Input           *int `json:"input_tokens"`
		Output          *int `json:"output_tokens"`
		Read            *int `json:"cache_read_input_tokens"`
		Write           *int `json:"cache_creation_input_tokens"`
		Total           *int `json:"total_tokens"`
		PromptAlias     *int `json:"prompt_tokens"`
		CompletionAlias *int `json:"completion_tokens"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return Usage{}, err
	}
	for _, n := range []*int{v.Input, v.Output, v.Read, v.Write, v.Total, v.PromptAlias, v.CompletionAlias} {
		if n != nil && *n < 0 {
			return Usage{}, fmt.Errorf("negative usage count")
		}
	}
	u := Usage{anthropicInput: v.Input, CachedTokens: v.Read, CacheWriteTokens: v.Write}
	if err := u.normalizeAnthropicPrompt(); err != nil {
		return Usage{}, err
	}
	if u.promptPresent && v.PromptAlias != nil && *v.PromptAlias != u.PromptTokens {
		return Usage{}, fmt.Errorf("anthropic prompt token aliases disagree")
	}
	if v.Output != nil {
		u.CompletionTokens, u.completionPresent = *v.Output, true
		if v.CompletionAlias != nil && *v.CompletionAlias != *v.Output {
			return Usage{}, fmt.Errorf("anthropic completion token aliases disagree")
		}
	}
	if v.Total != nil {
		u.TotalTokens, u.totalPresent = *v.Total, true
	}
	return u, nil
}

func decodeResponse(data []byte, protocol string) (CompletionResponse, error) {
	if openAIUsageProtocol(protocol) {
		if err := checkOpenAIEnvelope(data, protocol == "openai-responses"); err != nil {
			return CompletionResponse{}, err
		}
	}
	if protocol == "anthropic" {
		if err := checkAnthropicEnvelope(data); err != nil {
			return CompletionResponse{}, err
		}
	}
	if protocol == "providerwire-v4" {
		return decodeProviderwireResponse(data)
	}
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
		var envelope struct {
			Error json.RawMessage `json:"error"`
		}
		if err := json.Unmarshal(data, &envelope); err != nil {
			return result, err
		}
		if len(envelope.Error) > 0 && string(envelope.Error) != "null" {
			return result, fmt.Errorf("provider error response")
		}
		return result, json.Unmarshal(data, &result)
	}
	var raw struct {
		Status     string          `json:"status"`
		StopReason string          `json:"stop_reason"`
		ID         string          `json:"id"`
		Model      string          `json:"model"`
		Error      json.RawMessage `json:"error"`
		Usage      json.RawMessage `json:"usage"`
		Choices    []struct {
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
				Type string `json:"type"`
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
	var vendor struct {
		Metrics struct {
			SpeculativeDecoding json.RawMessage `json:"speculative_decoding"`
		} `json:"metrics"`
		Choices []struct {
			SpeculativeDecoding json.RawMessage `json:"speculative_decoding"`
		} `json:"choices"`
	}
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
		result.ProviderStatus = raw.Status
		if err := responsesStatus(raw.Status); err != nil {
			return result, err
		}
		for _, item := range raw.Output {
			if item.Type == "function_call" {
				tools = append(tools, normalizedTool(len(tools), item.CallID, item.Name, item.Arguments))
			}
			for _, part := range item.Summary {
				reasoning.WriteString(part.Text)
			}
			for _, part := range item.Content {
				if item.Type == "reasoning" || part.Type == "reasoning_text" {
					reasoning.WriteString(part.Text)
				} else {
					text.WriteString(part.Text)
				}
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
		result.Choices = []Choice{{Message: Message{Role: "assistant", Content: text.String(), ReasoningContent: reasoning.String(), ToolCalls: tools}, FinishReason: raw.StopReason}}
	}
	result.Body = append(json.RawMessage(nil), data...)
	return result, nil
}

func decodeChunk(data []byte, protocol string) (ChatCompletionChunk, bool, error) {
	if openAIUsageProtocol(protocol) {
		if err := checkOpenAIEnvelope(data, protocol == "openai-responses"); err != nil {
			return ChatCompletionChunk{}, false, err
		}
	}
	if protocol == "anthropic" {
		if err := checkAnthropicEnvelope(data); err != nil {
			return ChatCompletionChunk{}, false, err
		}
	}
	if protocol == "solido-rag" {
		return decodeSolidoChunk(data)
	}
	var result ChatCompletionChunk
	if protocol == "" || protocol == "openai-chat" {
		return result, false, json.Unmarshal(data, &result)
	}
	var vendor struct {
		Metrics struct {
			SpeculativeDecoding json.RawMessage `json:"speculative_decoding"`
		} `json:"metrics"`
		Choices []struct {
			Index               int             `json:"index"`
			SpeculativeDecoding json.RawMessage `json:"speculative_decoding"`
		} `json:"choices"`
	}
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
	if raw.Type == "error" || len(raw.Error) > 0 && string(raw.Error) != "null" {
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
		case "response.output_text.done":
			var done struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal(data, &done); err != nil {
				return result, false, err
			}
			content = done.Text
		case "response.output_text.delta":
			if err := json.Unmarshal(raw.Delta, &content); err != nil {
				return result, false, err
			}
		case "response.reasoning_text.delta", "response.reasoning_summary_text.delta":
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
		case "response.completed", "response.incomplete", "response.failed":
			var response *struct {
				Metrics struct {
					SpeculativeDecoding json.RawMessage `json:"speculative_decoding"`
				} `json:"metrics"`
				Status string          `json:"status"`
				ID     string          `json:"id"`
				Model  string          `json:"model"`
				Usage  json.RawMessage `json:"usage"`
				Error  json.RawMessage `json:"error"`
			}
			if err := json.Unmarshal(raw.Response, &response); err != nil || response == nil {
				return result, false, fmt.Errorf("response terminal snapshot must be a non-null object")
			}
			if len(response.Error) > 0 && strings.TrimSpace(string(response.Error)) != "null" {
				return result, false, fmt.Errorf("provider terminal error response")
			}
			result.ID, result.Model, raw.Usage = response.ID, response.Model, response.Usage
			result.Metrics.SpeculativeDecoding, result.specDecodeUnavailable = terminalSpecDecode(result.Metrics.SpeculativeDecoding, response.Metrics.SpeculativeDecoding)
			result.ProviderStatus = response.Status
			if result.ProviderStatus == "" {
				result.ProviderStatus = strings.TrimPrefix(raw.Type, "response.")
			}
			if result.ProviderStatus != strings.TrimPrefix(raw.Type, "response.") {
				return result, false, fmt.Errorf("response status conflicts with terminal event")
			}
			if err := responsesStatus(result.ProviderStatus); err != nil {
				return result, false, err
			}
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
		if raw.Type == "message_delta" {
			var delta struct {
				StopReason string `json:"stop_reason"`
			}
			// Usage snapshots may omit a content or stop-reason delta.
			if len(raw.Delta) > 0 {
				if err := json.Unmarshal(raw.Delta, &delta); err != nil {
					return result, false, err
				}
			}
			finish = delta.StopReason
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

func mergeUsage(previous, next Usage) (Usage, error) {
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
	if next.CacheWriteTokens != nil {
		previous.CacheWriteTokens = next.CacheWriteTokens
	}
	if next.anthropicInput != nil {
		previous.anthropicInput = next.anthropicInput
	}
	if err := previous.normalizeAnthropicPrompt(); err != nil {
		return Usage{}, err
	}
	return previous, nil
}

// normalizeAnthropicPrompt sums reported buckets only. Missing input leaves the
// prompt unavailable; absent cache buckets remain unavailable, rather than zero.
func (u *Usage) normalizeAnthropicPrompt() error {
	if u.anthropicInput == nil {
		return nil
	}
	n := *u.anthropicInput
	for _, bucket := range []*int{u.CachedTokens, u.CacheWriteTokens} {
		if bucket != nil {
			if *bucket > int(^uint(0)>>1)-n {
				return fmt.Errorf("anthropic prompt token count overflows int")
			}
			n += *bucket
		}
	}
	u.PromptTokens, u.promptPresent = n, true
	return nil
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

func responsesStatus(status string) error {
	switch status {
	case "", "completed":
		return nil
	case "failed", "incomplete", "cancelled":
		return fmt.Errorf("provider response status: %s", status)
	default:
		return fmt.Errorf("nonterminal or unknown provider response status")
	}
}

// A terminal snapshot must agree with a same-event root snapshot. Neither is
// an increment; malformed statistics never make generated text unusable.
func terminalSpecDecode(root, terminal json.RawMessage) (json.RawMessage, string) {
	present := func(raw json.RawMessage) bool { return len(raw) != 0 && string(raw) != "null" }
	if !present(root) {
		return terminal, ""
	}
	if !present(terminal) {
		return root, ""
	}
	a, err := parseSpecDecode(root)
	if err != nil {
		return nil, "root: " + err.Error()
	}
	b, err := parseSpecDecode(terminal)
	if err != nil {
		return nil, "terminal: " + err.Error()
	}
	if !sameSpecDecode(a, b) {
		return nil, "conflicting root and terminal speculative snapshots"
	}
	return terminal, ""
}

// responsesStream admits each text part once. The response byte limit bounds
// stored text; a separate part limit bounds empty-event state.
type responsesPart struct {
	text   strings.Builder
	itemID string
	done   bool
}

type responsesStream struct {
	parts     map[[2]int]*responsesPart
	textBytes int
	unindexed bool
}

func responsesIndex(raw json.RawMessage) (int, error) {
	if len(raw) == 0 {
		return -1, nil
	}
	var index int
	if string(raw) == "null" {
		return 0, fmt.Errorf("null responses part index")
	}
	if err := json.Unmarshal(raw, &index); err != nil || index < 0 {
		return 0, fmt.Errorf("invalid responses part index")
	}
	return index, nil
}

func (s *responsesStream) decode(data []byte) (ChatCompletionChunk, bool, error) {
	if len(data) > 16*1024*1024 {
		return ChatCompletionChunk{}, false, fmt.Errorf("responses event exceeds 16 MiB")
	}
	chunk, terminal, err := decodeChunk(data, "openai-responses")
	if err != nil {
		return chunk, terminal, err
	}
	var event struct {
		Type    string          `json:"type"`
		Output  json.RawMessage `json:"output_index"`
		Content json.RawMessage `json:"content_index"`
		ItemID  string          `json:"item_id"`
	}
	if err := json.Unmarshal(data, &event); err != nil {
		return chunk, false, err
	}
	if event.Type != "response.output_text.delta" && event.Type != "response.output_text.done" {
		if event.Type == "response.reasoning_text.delta" || event.Type == "response.function_call_arguments.delta" {
			var raw struct {
				Delta string `json:"delta"`
			}
			if err := json.Unmarshal(data, &raw); err != nil {
				return chunk, false, err
			}
			chunk.aiperfContent = raw.Delta != ""
		}
		return chunk, terminal, nil
	}
	output, err := responsesIndex(event.Output)
	if err != nil {
		return chunk, false, err
	}
	content, err := responsesIndex(event.Content)
	if err != nil {
		return chunk, false, err
	}
	if (output < 0) != (content < 0) {
		return chunk, false, fmt.Errorf("incomplete responses part identity")
	}
	key := [2]int{output, content}
	if s.parts == nil {
		s.parts = make(map[[2]int]*responsesPart)
	}
	part := s.parts[key]
	if part == nil {
		if len(s.parts) >= 4096 {
			return chunk, false, fmt.Errorf("responses text part limit exceeded")
		}
		// Missing indices are a separate single-part dialect, never a fallback for
		// an indexed part whose identity was lost on a later event.
		if len(s.parts) > 0 && s.unindexed != (output < 0) {
			return chunk, false, fmt.Errorf("responses part identity changed")
		}
		s.unindexed = output < 0
		part = &responsesPart{}
		s.parts[key] = part
	}
	if part.done {
		return chunk, false, fmt.Errorf("responses text after part done")
	}
	if len(event.ItemID) > 2048 {
		return chunk, false, fmt.Errorf("responses item ID exceeds 2048 bytes")
	}
	if part.itemID != "" && event.ItemID != "" && part.itemID != event.ItemID {
		return chunk, false, fmt.Errorf("responses item identity changed")
	}
	if event.ItemID != "" {
		part.itemID = event.ItemID
	}
	text := chunk.Choices[0].Delta.Content
	chunk.aiperfContent = text != ""
	if event.Type == "response.output_text.delta" {
		if s.textBytes+len(text) > 16*1024*1024 {
			return chunk, false, fmt.Errorf("responses text exceeds 16 MiB")
		}
		s.textBytes += len(text)
		part.text.WriteString(text)
	} else {
		if part.text.Len() == 0 {
			if s.textBytes+len(text) > 16*1024*1024 {
				return chunk, false, fmt.Errorf("responses text exceeds 16 MiB")
			}
			s.textBytes += len(text)
		}
		part.done = true
		if part.text.Len() > 0 {
			if text != part.text.String() {
				return chunk, false, fmt.Errorf("responses done text disagrees with deltas")
			}
			chunk.Choices[0].Delta.Content = ""
		}
	}
	return chunk, terminal, nil
}

const responsesContentPolicy = "aiperf-responses-content/f10eb50a"

func recordResponsesTiming(m *Measurement, now, fallback time.Time) {
	anchor := m.requestStart
	if anchor.IsZero() {
		anchor = fallback
	}
	value := ms(now.Sub(anchor))
	if m.AIPerfTTFT == nil {
		first := value
		m.AIPerfTTFT = &first
	}
	m.AIPerfLastContentLatency = &value
	*m.AIPerfContentEvents++
}
