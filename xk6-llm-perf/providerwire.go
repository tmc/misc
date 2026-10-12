package llmperf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/tmc/misc/xk6-llm-perf/internal/jsoncheck"
)

// providerwireObject rejects unknown fields and ambiguous JSON before adapting
// the pinned text/function-tool dialect. Opaque metadata is not measurement data.
func providerwireObject(data []byte, required, optional string) (map[string]json.RawMessage, error) {
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("invalid providerwire UTF-8")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	if err := jsoncheck.Check(d, jsoncheck.Options{}); err != nil {
		return nil, err
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil || m == nil {
		return nil, fmt.Errorf("providerwire object required")
	}
	allowed := strings.Fields(required + " " + optional)
	for k := range m {
		found := false
		for _, a := range allowed {
			if k == a {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("unsupported providerwire field")
		}
	}
	for _, k := range strings.Fields(required) {
		if _, ok := m[k]; !ok {
			return nil, fmt.Errorf("missing providerwire field %q", k)
		}
	}
	return m, nil
}
func providerwireString(m map[string]json.RawMessage, k string, nonempty bool) (string, error) {
	var s string
	raw, ok := m[k]
	if !ok || bytes.Equal(raw, []byte("null")) || json.Unmarshal(raw, &s) != nil || nonempty && s == "" {
		return "", fmt.Errorf("invalid providerwire %s", k)
	}
	return s, nil
}
func providerwireMetadata(m map[string]json.RawMessage, key string) error {
	raw, ok := m[key]
	if !ok {
		return nil
	}
	var v map[string]map[string]json.RawMessage
	if bytes.Equal(raw, []byte("null")) || json.Unmarshal(raw, &v) != nil || v == nil {
		return fmt.Errorf("invalid providerwire %s", key)
	}
	for _, obj := range v {
		if obj == nil {
			return fmt.Errorf("invalid providerwire %s", key)
		}
	}
	return nil
}
func providerwireWarnings(raw json.RawMessage) error {
	var list []json.RawMessage
	if bytes.Equal(raw, []byte("null")) || json.Unmarshal(raw, &list) != nil {
		return fmt.Errorf("invalid providerwire warnings")
	}
	for _, b := range list {
		var tag struct{ Type string }
		if json.Unmarshal(b, &tag) != nil {
			return fmt.Errorf("invalid providerwire warning")
		}
		req, opt := "", ""
		switch tag.Type {
		case "unsupported", "compatibility":
			req = "type feature"
			opt = "details"
		case "deprecated":
			req = "type setting message"
		case "other":
			req = "type message"
		default:
			return fmt.Errorf("unsupported providerwire warning")
		}
		m, e := providerwireObject(b, req, opt)
		if e != nil {
			return e
		}
		for k := range m {
			if _, e = providerwireString(m, k, false); e != nil {
				return e
			}
		}
	}
	return nil
}
func providerwireFinish(raw json.RawMessage) (string, error) {
	m, e := providerwireObject(raw, "unified", "raw")
	if e != nil {
		return "", e
	}
	s, e := providerwireString(m, "unified", true)
	if e != nil {
		return "", e
	}
	switch s {
	case "stop", "length", "content-filter", "tool-calls", "other":
	case "error":
		return "", fmt.Errorf("providerwire error finish")
	default:
		return "", fmt.Errorf("invalid providerwire finish reason")
	}
	if _, ok := m["raw"]; ok {
		if _, e = providerwireString(m, "raw", false); e != nil {
			return "", e
		}
	}
	return s, nil
}
func providerwireUsage(raw json.RawMessage) (Usage, error) {
	var u Usage
	m, e := providerwireObject(raw, "inputTokens outputTokens", "raw")
	if e != nil {
		return u, e
	}
	if b, ok := m["raw"]; ok {
		var obj map[string]json.RawMessage
		if json.Unmarshal(b, &obj) != nil || obj == nil {
			return u, fmt.Errorf("invalid providerwire raw usage")
		}
	}
	in, e := providerwireCounts(m["inputTokens"], "total noCache cacheRead cacheWrite")
	if e != nil {
		return u, e
	}
	out, e := providerwireCounts(m["outputTokens"], "total text reasoning")
	if e != nil {
		return u, e
	}
	if in["total"] != nil {
		u.PromptTokens = *in["total"]
		u.promptPresent = true
	}
	if out["total"] != nil {
		u.CompletionTokens = *out["total"]
		u.completionPresent = true
	}
	u.CachedTokens = in["cacheRead"]
	u.CacheWriteTokens = in["cacheWrite"]
	u.ReasoningTokens = out["reasoning"]
	for _, counts := range []map[string]*int{in, out} {
		sum := int64(0)
		complete := true
		for key, n := range counts {
			if key == "total" {
				continue
			}
			if n == nil {
				complete = false
				continue
			}
			sum += int64(*n)
		}
		if total := counts["total"]; total != nil {
			if sum > int64(*total) || complete && sum != int64(*total) {
				return Usage{}, fmt.Errorf("inconsistent providerwire token buckets")
			}
		}
	}
	return u, nil
}
func providerwireCounts(raw json.RawMessage, keys string) (map[string]*int, error) {
	m, e := providerwireObject(raw, "", keys)
	if e != nil {
		return nil, e
	}
	counts := make(map[string]*int)
	for _, k := range strings.Fields(keys) {
		counts[k] = nil
		if b, ok := m[k]; ok {
			var n int64
			if bytes.Equal(b, []byte("null")) || json.Unmarshal(b, &n) != nil || n < 0 || n > 9007199254740991 {
				return nil, fmt.Errorf("invalid providerwire token count")
			}
			v := int(n)
			if int64(v) != n {
				return nil, fmt.Errorf("providerwire token count overflow")
			}
			counts[k] = &v
		}
	}
	return counts, nil
}

// A stream owns its identifiers, boundaries, and pending tool inputs. Completed
// tool-call events confirm streamed input without adding the arguments twice.
type providerwireStream struct {
	started, finished           bool
	metadataSeen, outputStarted bool
	blocks                      map[string]bool
	tools                       map[string]*providerwireTool
}
type providerwireTool struct {
	index         int
	name, input   string
	ended, called bool
}

func (s *providerwireStream) decode(data []byte) (ChatCompletionChunk, bool, error) {
	var c ChatCompletionChunk
	var tag struct{ Type string }
	if err := json.Unmarshal(data, &tag); err != nil {
		return c, false, err
	}
	req, opt := "type", "providerMetadata"
	switch tag.Type {
	case "stream-start":
		req += " warnings"
		opt = ""
	case "response-metadata":
		opt = "id modelId timestamp"
	case "text-start", "text-end", "reasoning-start", "reasoning-end", "tool-input-end":
		req += " id"
	case "text-delta", "reasoning-delta", "tool-input-delta":
		req += " id delta"
	case "tool-input-start":
		req += " id toolName"
		opt += " providerExecuted dynamic"
	case "tool-call":
		req += " toolCallId toolName input"
		opt += " providerExecuted dynamic"
	case "finish":
		req += " usage finishReason"
	case "error":
		return c, false, fmt.Errorf("providerwire provider error")
	default:
		return c, false, fmt.Errorf("unsupported providerwire event")
	}
	m, e := providerwireObject(data, req, opt)
	if e != nil {
		return c, false, e
	}
	if e = providerwireMetadata(m, "providerMetadata"); e != nil {
		return c, false, e
	}
	for _, k := range []string{"providerExecuted", "dynamic"} {
		if b, ok := m[k]; ok {
			var v bool
			if bytes.Equal(b, []byte("null")) || json.Unmarshal(b, &v) != nil || v {
				return c, false, fmt.Errorf("unsupported providerwire %s", k)
			}
		}
	}
	if s.finished {
		return c, false, fmt.Errorf("providerwire event after finish")
	}
	if !s.started && tag.Type != "stream-start" {
		return c, false, fmt.Errorf("providerwire missing stream-start")
	}
	if s.blocks == nil {
		s.blocks = make(map[string]bool)
		s.tools = make(map[string]*providerwireTool)
	}
	var content, reasoning, finish string
	var tools []ToolCall
	switch tag.Type {
	case "stream-start":
		if s.started {
			return c, false, fmt.Errorf("duplicate providerwire stream-start")
		}
		if e = providerwireWarnings(m["warnings"]); e != nil {
			return c, false, e
		}
		s.started = true
	case "response-metadata":
		if s.metadataSeen || s.outputStarted {
			return c, false, fmt.Errorf("providerwire duplicate or late response metadata")
		}
		for _, k := range []string{"id", "modelId", "timestamp"} {
			if _, ok := m[k]; ok {
				v, e := providerwireString(m, k, false)
				if e != nil {
					return c, false, e
				}
				switch k {
				case "id":
					c.ID = v
				case "modelId":
					c.Model = v
				case "timestamp":
					if _, e = time.Parse(time.RFC3339Nano, v); e != nil {
						return c, false, fmt.Errorf("invalid providerwire timestamp")
					}
				}
			}
		}
		s.metadataSeen = true
	case "text-start", "reasoning-start", "text-end", "reasoning-end", "text-delta", "reasoning-delta":
		id, e := providerwireString(m, "id", true)
		if e != nil {
			return c, false, e
		}
		kind := strings.Split(tag.Type, "-")[0]
		key := kind + ":" + id
		switch {
		case strings.HasSuffix(tag.Type, "-start"):
			s.outputStarted = true
			if _, ok := s.blocks[key]; ok || len(s.blocks) >= 128 {
				return c, false, fmt.Errorf("duplicate or excessive providerwire block")
			}
			s.blocks[key] = true
		case strings.HasSuffix(tag.Type, "-end"):
			if !s.blocks[key] {
				return c, false, fmt.Errorf("providerwire block not open")
			}
			s.blocks[key] = false
		default:
			if !s.blocks[key] {
				return c, false, fmt.Errorf("providerwire block not open")
			}
			v, e := providerwireString(m, "delta", false)
			if e != nil {
				return c, false, e
			}
			if kind == "text" {
				content = v
			} else {
				reasoning = v
			}
		}
	case "tool-input-start", "tool-input-delta", "tool-input-end", "tool-call":
		key := "id"
		if tag.Type == "tool-call" {
			key = "toolCallId"
		}
		id, e := providerwireString(m, key, true)
		if e != nil {
			return c, false, e
		}
		t := s.tools[id]
		switch tag.Type {
		case "tool-input-start":
			s.outputStarted = true
			if t != nil || len(s.tools) >= 128 {
				return c, false, fmt.Errorf("duplicate or excessive providerwire tool")
			}
			name, e := providerwireString(m, "toolName", true)
			if e != nil {
				return c, false, e
			}
			t = &providerwireTool{index: len(s.tools), name: name}
			s.tools[id] = t
			tools = append(tools, normalizedTool(t.index, id, name, ""))
		case "tool-input-delta":
			if t == nil || t.ended {
				return c, false, fmt.Errorf("providerwire tool input not open")
			}
			v, e := providerwireString(m, "delta", false)
			if e != nil {
				return c, false, e
			}
			if len(t.input)+len(v) > 4*1024*1024 {
				return c, false, fmt.Errorf("providerwire tool input exceeds limit")
			}
			t.input += v
			tools = append(tools, normalizedTool(t.index, "", "", v))
		case "tool-input-end":
			if t == nil || t.ended {
				return c, false, fmt.Errorf("providerwire tool input not open")
			}
			t.ended = true
		case "tool-call":
			s.outputStarted = true
			name, e := providerwireString(m, "toolName", true)
			if e != nil {
				return c, false, e
			}
			input, e := providerwireString(m, "input", false)
			if e != nil {
				return c, false, e
			}
			if len(input) > 4*1024*1024 {
				return c, false, fmt.Errorf("providerwire tool input exceeds limit")
			}
			if t != nil {
				if !t.ended || t.called || t.name != name || t.input != input {
					return c, false, fmt.Errorf("inconsistent providerwire completed tool")
				}
				t.called = true
			} else {
				if len(s.tools) >= 128 {
					return c, false, fmt.Errorf("providerwire tool count exceeds limit")
				}
				t = &providerwireTool{index: len(s.tools), name: name, input: input, ended: true, called: true}
				s.tools[id] = t
				tools = append(tools, normalizedTool(t.index, id, name, input))
			}
			if e = validateCompletedTools([]Choice{{Message: Message{ToolCalls: []ToolCall{normalizedTool(t.index, id, name, input)}}}}); e != nil {
				return c, false, e
			}
		}
	case "finish":
		for _, open := range s.blocks {
			if open {
				return c, false, fmt.Errorf("providerwire unfinished block")
			}
		}
		for _, t := range s.tools {
			if !t.called {
				return c, false, fmt.Errorf("providerwire unfinished tool")
			}
		}
		finish, e = providerwireFinish(m["finishReason"])
		if e != nil {
			return c, false, e
		}
		c.Usage, e = providerwireUsage(m["usage"])
		if e != nil {
			return c, false, e
		}
		s.finished = true
	}
	if content != "" || reasoning != "" || len(tools) > 0 || finish != "" {
		b, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": content, "reasoning_content": reasoning, "tool_calls": tools}, "finish_reason": finish}}})
		var delta ChatCompletionChunk
		if e = json.Unmarshal(b, &delta); e != nil {
			return c, false, e
		}
		c.Choices = delta.Choices
	}
	return c, s.finished, nil
}

func decodeProviderwireResponse(data []byte) (CompletionResponse, error) {
	var r CompletionResponse
	m, e := providerwireObject(data, "content finishReason usage warnings", "response providerMetadata")
	if e != nil {
		return r, e
	}
	if e = providerwireMetadata(m, "providerMetadata"); e != nil {
		return r, e
	}
	if e = providerwireWarnings(m["warnings"]); e != nil {
		return r, e
	}
	finish, e := providerwireFinish(m["finishReason"])
	if e != nil {
		return r, e
	}
	r.Usage, e = providerwireUsage(m["usage"])
	if e != nil {
		return CompletionResponse{}, e
	}
	if b, ok := m["response"]; ok {
		obj, e := providerwireObject(b, "", "id modelId timestamp")
		if e != nil {
			return r, e
		}
		for k := range obj {
			v, e := providerwireString(obj, k, false)
			if e != nil {
				return r, e
			}
			switch k {
			case "id":
				r.ID = v
			case "modelId":
				r.Model = v
			case "timestamp":
				if _, e = time.Parse(time.RFC3339Nano, v); e != nil {
					return r, fmt.Errorf("invalid providerwire timestamp")
				}
			}
		}
	}
	var parts []json.RawMessage
	if bytes.Equal(m["content"], []byte("null")) || json.Unmarshal(m["content"], &parts) != nil {
		return r, fmt.Errorf("invalid providerwire content")
	}
	var text, reasoning strings.Builder
	var tools []ToolCall
	seen := make(map[string]bool)
	for _, b := range parts {
		var tag struct{ Type string }
		if json.Unmarshal(b, &tag) != nil {
			return r, fmt.Errorf("invalid providerwire content")
		}
		req := "type text"
		opt := "providerMetadata"
		switch tag.Type {
		case "text", "reasoning":
		case "tool-call":
			req = "type toolCallId toolName input"
			opt += " providerExecuted dynamic"
		default:
			return r, fmt.Errorf("unsupported providerwire content")
		}
		obj, e := providerwireObject(b, req, opt)
		if e != nil {
			return r, e
		}
		if e = providerwireMetadata(obj, "providerMetadata"); e != nil {
			return r, e
		}
		if tag.Type == "tool-call" {
			for _, k := range []string{"providerExecuted", "dynamic"} {
				if raw, ok := obj[k]; ok {
					var v bool
					if bytes.Equal(raw, []byte("null")) || json.Unmarshal(raw, &v) != nil || v {
						return r, fmt.Errorf("unsupported providerwire %s", k)
					}
				}
			}
			id, e := providerwireString(obj, "toolCallId", true)
			if e != nil {
				return r, e
			}
			name, e := providerwireString(obj, "toolName", true)
			if e != nil {
				return r, e
			}
			input, e := providerwireString(obj, "input", false)
			if e != nil {
				return r, e
			}
			if seen[id] || len(tools) >= 128 || len(input) > 4*1024*1024 {
				return r, fmt.Errorf("duplicate or excessive providerwire tool")
			}
			seen[id] = true
			tools = append(tools, normalizedTool(len(tools), id, name, input))
		} else {
			v, e := providerwireString(obj, "text", false)
			if e != nil {
				return r, e
			}
			if tag.Type == "text" {
				text.WriteString(v)
			} else {
				reasoning.WriteString(v)
			}
		}
	}
	r.Choices = []Choice{{Index: 0, Message: Message{Role: "assistant", Content: text.String(), ReasoningContent: reasoning.String(), ToolCalls: tools}, FinishReason: finish}}
	if e = validateCompletedTools(r.Choices); e != nil {
		return CompletionResponse{}, e
	}
	return r, nil
}

// validateProviderwireBody admits authored text prompts and function declarations.
// File/custom parts, provider tools, header overrides and raw chunks are outside
// this adapter's measurement contract and fail before dispatch.
func validateProviderwireBody(body map[string]any) error {
	b, e := json.Marshal(body)
	if e != nil {
		return e
	}
	m, e := providerwireObject(b, "prompt", "maxOutputTokens temperature stopSequences topP topK presencePenalty frequencyPenalty seed tools toolChoice reasoning providerOptions responseFormat")
	if e != nil {
		return e
	}
	var messages []json.RawMessage
	if bytes.Equal(m["prompt"], []byte("null")) || json.Unmarshal(m["prompt"], &messages) != nil {
		return fmt.Errorf("invalid providerwire prompt")
	}
	if len(messages) > 1024 {
		return fmt.Errorf("providerwire prompt exceeds message limit")
	}
	for _, b := range messages {
		obj, e := providerwireObject(b, "role content", "providerOptions")
		if e != nil {
			return e
		}
		if e = providerwireMetadata(obj, "providerOptions"); e != nil {
			return e
		}
		role, e := providerwireString(obj, "role", true)
		if e != nil {
			return e
		}
		if role == "system" {
			if _, e = providerwireString(obj, "content", false); e != nil {
				return e
			}
			continue
		}
		if role != "user" && role != "assistant" {
			return fmt.Errorf("unsupported providerwire prompt role")
		}
		var parts []json.RawMessage
		if bytes.Equal(obj["content"], []byte("null")) || json.Unmarshal(obj["content"], &parts) != nil {
			return fmt.Errorf("invalid providerwire prompt content")
		}
		for _, p := range parts {
			part, e := providerwireObject(p, "type text", "providerOptions")
			if e != nil {
				return e
			}
			kind, e := providerwireString(part, "type", true)
			if e != nil {
				return e
			}
			if kind != "text" && (kind != "reasoning" || role != "assistant") {
				return fmt.Errorf("unsupported providerwire prompt part")
			}
			if _, e = providerwireString(part, "text", false); e != nil {
				return e
			}
			if e = providerwireMetadata(part, "providerOptions"); e != nil {
				return e
			}
		}
	}
	for _, k := range []string{"maxOutputTokens", "topK", "seed"} {
		if b, ok := m[k]; ok {
			var n int64
			if bytes.Equal(b, []byte("null")) || json.Unmarshal(b, &n) != nil {
				return fmt.Errorf("invalid providerwire %s", k)
			}
		}
	}
	for _, k := range []string{"temperature", "topP", "presencePenalty", "frequencyPenalty"} {
		if b, ok := m[k]; ok {
			var n float64
			if bytes.Equal(b, []byte("null")) || json.Unmarshal(b, &n) != nil {
				return fmt.Errorf("invalid providerwire %s", k)
			}
		}
	}
	if b, ok := m["stopSequences"]; ok {
		var list []json.RawMessage
		if bytes.Equal(b, []byte("null")) || json.Unmarshal(b, &list) != nil {
			return fmt.Errorf("invalid providerwire stopSequences")
		}
		for _, v := range list {
			if _, e = providerwireString(map[string]json.RawMessage{"value": v}, "value", false); e != nil {
				return e
			}
		}
	}
	if _, ok := m["reasoning"]; ok {
		s, e := providerwireString(m, "reasoning", true)
		if e != nil {
			return e
		}
		switch s {
		case "provider-default", "none", "minimal", "low", "medium", "high", "xhigh":
		default:
			return fmt.Errorf("invalid providerwire reasoning")
		}
	}
	if e = providerwireMetadata(m, "providerOptions"); e != nil {
		return e
	}
	if b, ok := m["tools"]; ok {
		var tools []json.RawMessage
		if bytes.Equal(b, []byte("null")) || json.Unmarshal(b, &tools) != nil || len(tools) > 128 {
			return fmt.Errorf("invalid providerwire tools")
		}
		for _, b := range tools {
			obj, e := providerwireObject(b, "type name inputSchema", "description strict providerOptions")
			if e != nil {
				return e
			}
			kind, e := providerwireString(obj, "type", true)
			if e != nil || kind != "function" {
				return fmt.Errorf("unsupported providerwire tool")
			}
			if _, e = providerwireString(obj, "name", true); e != nil {
				return e
			}
			if _, ok := obj["description"]; ok {
				if _, e = providerwireString(obj, "description", false); e != nil {
					return e
				}
			}
			var schema map[string]json.RawMessage
			if json.Unmarshal(obj["inputSchema"], &schema) != nil || schema == nil {
				return fmt.Errorf("invalid providerwire tool schema")
			}
			if b, ok := obj["strict"]; ok {
				var v bool
				if bytes.Equal(b, []byte("null")) || json.Unmarshal(b, &v) != nil {
					return fmt.Errorf("invalid providerwire strict")
				}
			}
			if e = providerwireMetadata(obj, "providerOptions"); e != nil {
				return e
			}
		}
	}
	if b, ok := m["toolChoice"]; ok {
		obj, e := providerwireObject(b, "type", "toolName")
		if e != nil {
			return e
		}
		kind, e := providerwireString(obj, "type", true)
		if e != nil {
			return e
		}
		switch kind {
		case "auto", "none", "required":
			if _, ok := obj["toolName"]; ok {
				return fmt.Errorf("invalid providerwire toolChoice")
			}
		case "tool":
			if _, e = providerwireString(obj, "toolName", true); e != nil {
				return e
			}
		default:
			return fmt.Errorf("invalid providerwire toolChoice")
		}
	}
	if b, ok := m["responseFormat"]; ok {
		obj, e := providerwireObject(b, "type", "schema name description")
		if e != nil {
			return e
		}
		kind, e := providerwireString(obj, "type", true)
		if e != nil {
			return e
		}
		switch kind {
		case "text":
			if len(obj) != 1 {
				return fmt.Errorf("invalid providerwire responseFormat")
			}
		case "json":
			if raw, ok := obj["schema"]; ok {
				var schema map[string]json.RawMessage
				if json.Unmarshal(raw, &schema) != nil || schema == nil {
					return fmt.Errorf("invalid providerwire response schema")
				}
			}
			for _, k := range []string{"name", "description"} {
				if _, ok := obj[k]; ok {
					if _, e = providerwireString(obj, k, false); e != nil {
						return e
					}
				}
			}
		default:
			return fmt.Errorf("invalid providerwire responseFormat")
		}
	}
	return nil
}
