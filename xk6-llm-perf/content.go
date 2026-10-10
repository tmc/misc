package llmperf

import "strings"

func messageText(content any) string {
	if s, ok := content.(string); ok {
		return s
	}
	var text strings.Builder
	if parts, ok := content.([]any); ok {
		for _, part := range parts {
			if p, ok := part.(map[string]any); ok {
				if kind, _ := p["type"].(string); kind == "text" || kind == "input_text" {
					if s, ok := p["text"].(string); ok {
						text.WriteString(s)
						text.WriteByte(' ')
					}
				}
			}
		}
	}
	return text.String()
}

func hasMedia(content any) bool {
	if parts, ok := content.([]any); ok {
		for _, part := range parts {
			if p, ok := part.(map[string]any); ok {
				if kind, _ := p["type"].(string); kind != "text" && kind != "input_text" && kind != "" {
					return true
				}
			}
		}
	}
	return false
}
