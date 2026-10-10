package llmperf

import (
	"fmt"
	"strings"
)

// streamChoices retains choice identity without allocating request-level usage
// to individual choices. The stream byte limit bounds accumulated text.
type streamChoice struct {
	choice    Choice
	content   strings.Builder
	reasoning strings.Builder
}

type streamChoices map[int]*streamChoice

func (s streamChoices) add(chunk ChatCompletionChunk) error {
	seen := make(map[int]bool)
	for _, delta := range chunk.Choices {
		if delta.Index < 0 || delta.Index >= 128 || seen[delta.Index] {
			return fmt.Errorf("invalid or duplicate stream choice index")
		}
		seen[delta.Index] = true
		c := s[delta.Index]
		if c == nil {
			c = &streamChoice{choice: Choice{Index: delta.Index, Message: Message{Role: "assistant"}}}
			s[delta.Index] = c
		}
		c.content.WriteString(delta.Delta.Content)
		c.reasoning.WriteString(delta.Delta.ReasoningContent)
		if delta.FinishReason != "" {
			c.choice.FinishReason = delta.FinishReason
		}
		for _, part := range delta.Delta.ToolCalls {
			if part.Index < 0 || part.Index >= 128 {
				return fmt.Errorf("tool call index exceeds limit")
			}
			var call *ToolCall
			for i := range c.choice.Message.ToolCalls {
				if c.choice.Message.ToolCalls[i].Index == part.Index {
					call = &c.choice.Message.ToolCalls[i]
					break
				}
			}
			if call == nil {
				c.choice.Message.ToolCalls = append(c.choice.Message.ToolCalls, ToolCall{Index: part.Index})
				call = &c.choice.Message.ToolCalls[len(c.choice.Message.ToolCalls)-1]
			}
			if part.ID != "" {
				call.ID = part.ID
			}
			if part.Type != "" {
				call.Type = part.Type
			}
			call.Function.Name += part.Function.Name
			call.Function.Arguments += part.Function.Arguments
			if len(call.Function.Arguments) > 4*1024*1024 {
				return fmt.Errorf("tool arguments exceed 4 MiB")
			}
		}
	}
	return nil
}

func (s streamChoices) choices() []Choice {
	var result []Choice
	for i := 0; i < 128; i++ {
		if c := s[i]; c != nil {
			choice := c.choice
			choice.Message.Content = c.content.String()
			choice.Message.ReasoningContent = c.reasoning.String()
			result = append(result, choice)
		}
	}
	return result
}
