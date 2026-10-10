package llmperf

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type tgiResponse struct {
	GeneratedText *string         `json:"generated_text"`
	Error         json.RawMessage `json:"error"`
	Token         *struct {
		Text string `json:"text"`
	} `json:"token"`
	Details struct {
		GeneratedTokens *int   `json:"generated_tokens"`
		FinishReason    string `json:"finish_reason"`
	} `json:"details"`
}

func tgiUsage(v tgiResponse) json.RawMessage {
	if v.Details.GeneratedTokens == nil {
		return nil
	}
	data, _ := json.Marshal(map[string]any{"completion_tokens": *v.Details.GeneratedTokens})
	return data
}

func decodeTGI(data []byte) (CompletionResponse, error) {
	var result CompletionResponse
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		var list []json.RawMessage
		if err := json.Unmarshal(data, &list); err != nil {
			return result, err
		}
		if len(list) != 1 {
			return result, fmt.Errorf("TGI response must contain one generation")
		}
		data = list[0]
	}
	var v tgiResponse
	if err := json.Unmarshal(data, &v); err != nil {
		return result, err
	}
	if len(v.Error) > 0 && string(v.Error) != "null" {
		return result, fmt.Errorf("TGI provider error")
	}
	if v.GeneratedText == nil {
		return result, fmt.Errorf("TGI generated text unavailable")
	}
	u, err := canonicalUsage(tgiUsage(v), "tgi")
	if err != nil {
		return result, err
	}
	result.Usage = u
	result.Choices = []Choice{{Message: Message{Role: "assistant", Content: *v.GeneratedText}, FinishReason: v.Details.FinishReason}}
	return result, nil
}
