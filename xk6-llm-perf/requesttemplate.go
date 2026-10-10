package llmperf

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
)

const maxTemplateBytes = 1 << 20

// RequestTemplate substitutes whole JSON values, preserving their JSON types.
// A variable reference is an object containing only {"$var": "name"}.
// Name selects a built-in shape instead of Body. Missing variables are errors.
type RequestTemplate struct {
	Name      string         `json:"name,omitempty"`
	Body      map[string]any `json:"body,omitempty"`
	Variables map[string]any `json:"variables"`
}

// TemplateProvenance identifies the template and rendered payload without
// retaining variable contents. Hashes use canonical encoding/json bytes.
type TemplateProvenance struct {
	SchemaVersion   int    `json:"schemaVersion"`
	Scope           string `json:"scope"`
	Name            string `json:"name,omitempty"`
	TemplateSHA256  string `json:"templateSHA256"`
	VariablesSHA256 string `json:"variablesSHA256"`
	PayloadSHA256   string `json:"payloadSHA256"`
}

// RenderedRequest contains an authored JSON body and its reproducibility receipt.
type RenderedRequest struct {
	Body       map[string]any     `json:"body"`
	Provenance TemplateProvenance `json:"provenance"`
}

// RenderRequestTemplate renders a bounded structured JSON template. It neither
// loads files nor executes code. Strings and object keys remain literal.
func RenderRequestTemplate(spec RequestTemplate) (RenderedRequest, error) {
	var result RenderedRequest
	if (spec.Name == "") == (spec.Body == nil) {
		return result, fmt.Errorf("provide exactly one template name or body")
	}
	if spec.Name == "nim-embeddings" {
		variables := make(map[string]any, len(spec.Variables)+1)
		for key, value := range spec.Variables {
			variables[key] = value
		}
		raw, err := boundedTemplateJSON(variables)
		if err != nil {
			return result, fmt.Errorf("variables: %w", err)
		}
		if err := json.Unmarshal(raw, &variables); err != nil {
			return result, err
		}
		input, err := nimEmbeddingInputs(variables["texts"], variables["images"])
		if err != nil {
			return result, err
		}
		variables["input"] = input
		spec.Variables = variables
	}
	body := spec.Body
	if spec.Name != "" {
		var err error
		body, err = namedRequestTemplate(spec.Name)
		if err != nil {
			return result, err
		}
	}
	source, err := boundedTemplateJSON(body)
	if err != nil {
		return result, fmt.Errorf("template: %w", err)
	}
	variables, err := boundedTemplateJSON(spec.Variables)
	if err != nil {
		return result, fmt.Errorf("variables: %w", err)
	}
	// Round-trip isolates the returned payload from caller-owned mutable objects.
	var normalized map[string]any
	if err := json.Unmarshal(source, &normalized); err != nil {
		return result, err
	}
	var vars map[string]any
	if err := json.Unmarshal(variables, &vars); err != nil {
		return result, err
	}
	nodes := 0
	rendered, err := substituteRequestTemplate(normalized, vars, 0, &nodes)
	if err != nil {
		return result, err
	}
	var ok bool
	result.Body, ok = rendered.(map[string]any)
	if !ok || result.Body == nil {
		return RenderedRequest{}, fmt.Errorf("rendered payload must be an object")
	}
	payload, err := boundedTemplateJSON(result.Body)
	if err != nil {
		return RenderedRequest{}, fmt.Errorf("rendered payload: %w", err)
	}
	result.Provenance = TemplateProvenance{SchemaVersion: 1, Scope: "structured-json-whole-values-v1", Name: spec.Name, TemplateSHA256: templateHash(source), VariablesSHA256: templateHash(variables), PayloadSHA256: templateHash(payload)}
	return result, nil
}

func boundedTemplateJSON(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("encode JSON: %w", err)
	}
	if len(b) > maxTemplateBytes {
		return nil, fmt.Errorf("JSON exceeds 1 MiB limit")
	}
	return b, nil
}

func templateHash(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }

func substituteRequestTemplate(value any, vars map[string]any, depth int, nodes *int) (any, error) {
	*nodes++
	if depth > 32 || *nodes > 100000 {
		return nil, fmt.Errorf("template exceeds nesting or node limit")
	}
	switch v := value.(type) {
	case map[string]any:
		if ref, ok := v["$var"]; ok {
			name, ok := ref.(string)
			if !ok || len(v) != 1 || name == "" || len(name) > 128 {
				return nil, fmt.Errorf("invalid variable reference")
			}
			item, ok := vars[name]
			if !ok {
				return nil, fmt.Errorf("missing template variable %q", name)
			}
			// Variables are data: their $var objects are never recursively evaluated.
			if err := validateTemplateValue(item, depth+1, nodes); err != nil {
				return nil, err
			}
			return item, nil
		}
		for key, item := range v {
			rendered, err := substituteRequestTemplate(item, vars, depth+1, nodes)
			if err != nil {
				return nil, err
			}
			v[key] = rendered
		}
	case []any:
		for i, item := range v {
			rendered, err := substituteRequestTemplate(item, vars, depth+1, nodes)
			if err != nil {
				return nil, err
			}
			v[i] = rendered
		}
	}
	return value, nil
}

func validateTemplateValue(v any, depth int, nodes *int) error {
	*nodes++
	if depth > 32 || *nodes > 100000 {
		return fmt.Errorf("template exceeds nesting or node limit")
	}
	switch value := v.(type) {
	case map[string]any:
		for _, child := range value {
			if err := validateTemplateValue(child, depth+1, nodes); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range value {
			if err := validateTemplateValue(child, depth+1, nodes); err != nil {
				return err
			}
		}
	}
	return nil
}

func namedRequestTemplate(name string) (map[string]any, error) {
	ref := func(name string) any { return map[string]any{"$var": name} }
	switch name {
	case "nv-embedqa":
		return map[string]any{"text": ref("texts")}, nil
	case "openai-embeddings":
		return map[string]any{"model": ref("model"), "input": ref("texts")}, nil
	case "nim-embeddings":
		return map[string]any{"model": ref("model"), "input": ref("input")}, nil
	case "chat-embeddings":
		return map[string]any{"model": ref("model"), "messages": ref("messages")}, nil
	case "solido-rag":
		return map[string]any{"query": ref("queries"), "filters": map[string]any{"family": "Solido", "tool": "SDE"}, "inference_model": ref("model")}, nil
	case "cohere-rankings":
		return map[string]any{"model": ref("model"), "query": ref("query"), "documents": ref("passages")}, nil
	case "hf-tei-rankings":
		return map[string]any{"query": ref("query"), "texts": ref("passages")}, nil
	case "nim-rankings":
		return map[string]any{"model": ref("model"), "query": map[string]any{"text": ref("query")}, "passages": ref("passages")}, nil
	case "nim-image-retrieval":
		return map[string]any{"input": ref("images")}, nil
	default:
		return nil, fmt.Errorf("unsupported named template %q", name)
	}
}

func nonGenerationProtocol(protocol string) bool {
	switch protocol {
	case "chat-embeddings", "nim-embeddings", "nv-embedqa", "openai-embeddings", "cohere-rankings", "nim-rankings", "hf-tei-rankings", "nim-image-retrieval":
		return true
	}
	return false
}

func validateEndpointBody(protocol string, body map[string]any) error {
	if !nonGenerationProtocol(protocol) && protocol != "solido-rag" {
		return nil
	}
	if stream, _ := body["stream"].(bool); stream && protocol != "solido-rag" {
		return fmt.Errorf("endpoint does not support streaming")
	}
	nonempty := func(value any) bool { s, ok := value.(string); return ok && s != "" }
	list := func(value any, object bool, key string) bool {
		values, ok := value.([]any)
		if !ok || len(values) == 0 || len(values) > 4096 {
			return false
		}
		for _, v := range values {
			if object {
				item, ok := v.(map[string]any)
				if !ok || !nonempty(item[key]) {
					return false
				}
			} else if !nonempty(v) {
				return false
			}
		}
		return true
	}
	valid := false
	switch protocol {
	case "openai-embeddings", "nim-embeddings":
		valid = nonempty(body["model"]) && validEmbeddingInput(body["input"])
		if format, present := body["encoding_format"]; present && format != "float" && format != "base64" {
			valid = false
		}
	case "nv-embedqa":
		valid = list(body["text"], false, "")
	case "chat-embeddings":
		valid = nonempty(body["model"]) && validEmbeddingMessages(body["messages"])
	case "solido-rag":
		_, filters := body["filters"].(map[string]any)
		valid = nonempty(body["inference_model"]) && list(body["query"], false, "") && filters
	case "cohere-rankings":
		valid = nonempty(body["model"]) && nonempty(body["query"]) && list(body["documents"], false, "")
	case "hf-tei-rankings":
		valid = nonempty(body["query"]) && list(body["texts"], false, "")
	case "nim-rankings":
		query, ok := body["query"].(map[string]any)
		valid = nonempty(body["model"]) && ok && nonempty(query["text"]) && list(body["passages"], true, "text")
	case "nim-image-retrieval":
		valid = list(body["input"], true, "url")
		if valid {
			for _, item := range body["input"].([]any) {
				if item.(map[string]any)["type"] != "image_url" {
					valid = false
				}
			}
		}
	}
	if !valid {
		return fmt.Errorf("invalid %s request shape", protocol)
	}
	return nil
}

func decodeNonGeneration(data []byte, protocol string) (CompletionResponse, error) {
	var result CompletionResponse
	if len(data) > 16<<20 {
		return result, fmt.Errorf("response exceeds size limit")
	}
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return result, err
	}
	obj, _ := value.(map[string]any)
	if obj != nil {
		if errValue := obj["error"]; errValue != nil {
			return result, fmt.Errorf("provider error response")
		}
		if id, ok := obj["id"].(string); ok {
			result.ID = id
		}
		if model, ok := obj["model"].(string); ok {
			result.Model = model
		}
		if usage, ok := obj["usage"]; ok {
			raw, err := json.Marshal(usage)
			if err != nil {
				return result, err
			}
			result.Usage, err = canonicalUsage(raw, "json")
			if err != nil {
				return result, err
			}
		}
	}
	var rows []any
	switch protocol {
	case "openai-embeddings", "nim-embeddings":
		rows, _ = obj["data"].([]any)
	case "nv-embedqa", "chat-embeddings":
		if err := validateNVEmbedQA(obj); err != nil {
			return result, err
		}
		result.Body = append(json.RawMessage(nil), data...)
		return result, nil
	case "nim-rankings":
		rows, _ = obj["rankings"].([]any)
	case "cohere-rankings":
		rows, _ = obj["results"].([]any)
	case "hf-tei-rankings":
		rows, _ = value.([]any)
		if rows == nil {
			rows, _ = obj["results"].([]any)
		}
	case "nim-image-retrieval":
		rows, _ = obj["data"].([]any)
	default:
		return result, fmt.Errorf("unsupported non-generation protocol")
	}
	if len(rows) == 0 || len(rows) > 4096 {
		return result, fmt.Errorf("missing or oversized %s results", protocol)
	}
	indices := map[int]bool{}
	for _, row := range rows {
		item, ok := row.(map[string]any)
		if !ok {
			return result, fmt.Errorf("invalid %s result item", protocol)
		}
		if protocol == "nim-image-retrieval" {
			continue
		} // Retrieval data remains authored JSON, not generated text.
		index, ok := item["index"].(float64)
		if !ok || index < 0 || index > 100000 || index != math.Trunc(index) || indices[int(index)] {
			return result, fmt.Errorf("invalid or duplicate result index")
		}
		indices[int(index)] = true
		if protocol == "openai-embeddings" || protocol == "nim-embeddings" {
			if item["object"] != "embedding" {
				return result, fmt.Errorf("invalid embedding result")
			}
			if err := validateEmbeddingVector(item["embedding"]); err != nil {
				return result, err
			}
		} else {
			key := "score"
			if protocol == "cohere-rankings" {
				key = "relevance_score"
			}
			if _, ok := item[key].(float64); !ok {
				return result, fmt.Errorf("missing ranking score")
			}
		}
	}
	result.Body = append(json.RawMessage(nil), data...)
	return result, nil
}

// validEmbeddingInput accepts the four OpenAI input shapes. A token-ID array
// describes one input; a nested token-ID array describes a batch.
func validEmbeddingInput(value any) bool {
	if text, ok := value.(string); ok {
		return text != ""
	}
	items, ok := value.([]any)
	if !ok || len(items) == 0 || len(items) > 4096 {
		return false
	}
	switch items[0].(type) {
	case string:
		for _, item := range items {
			if text, ok := item.(string); !ok || text == "" {
				return false
			}
		}
	case float64:
		return validEmbeddingTokens(items)
	case []any:
		for _, item := range items {
			tokens, ok := item.([]any)
			if !ok || !validEmbeddingTokens(tokens) {
				return false
			}
		}
	default:
		return false
	}
	return true
}

func validEmbeddingTokens(tokens []any) bool {
	if len(tokens) == 0 || len(tokens) > 4096 {
		return false
	}
	for _, item := range tokens {
		token, ok := item.(float64)
		if !ok || token < 0 || token > math.MaxInt32 || token != math.Trunc(token) {
			return false
		}
	}
	return true
}

func validateEmbeddingVector(value any) error {
	if text, ok := value.(string); ok {
		if len(text) == 0 || len(text) > base64.StdEncoding.EncodedLen(65536*4) {
			return fmt.Errorf("invalid base64 embedding size")
		}
		for _, c := range text {
			if c == '\n' || c == '\r' {
				return fmt.Errorf("invalid base64 embedding whitespace")
			}
		}
		data, err := base64.StdEncoding.Strict().DecodeString(text)
		if err != nil || len(data) == 0 || len(data)%4 != 0 {
			return fmt.Errorf("invalid base64 float32 embedding")
		}
		for i := 0; i < len(data); i += 4 {
			number := math.Float32frombits(binary.LittleEndian.Uint32(data[i:]))
			if math.IsNaN(float64(number)) || math.IsInf(float64(number), 0) {
				return fmt.Errorf("nonfinite embedding coordinate")
			}
		}
		return nil
	}
	values, ok := value.([]any)
	if !ok || len(values) == 0 || len(values) > 65536 {
		return fmt.Errorf("invalid embedding result")
	}
	for _, coordinate := range values {
		number, ok := coordinate.(float64)
		if !ok || math.IsNaN(number) || math.IsInf(number, 0) {
			return fmt.Errorf("invalid embedding coordinate")
		}
	}
	return nil
}

func validateNVEmbedQA(obj map[string]any) error {
	if data, ok := obj["data"].([]any); ok {
		if len(data) == 0 || len(data) > 4096 {
			return fmt.Errorf("invalid nv-embedqa embeddings")
		}
		for _, row := range data {
			item, ok := row.(map[string]any)
			if !ok || item["object"] != "embedding" {
				return fmt.Errorf("invalid nv-embedqa result item")
			}
			if err := validateEmbeddingVector(item["embedding"]); err != nil {
				return err
			}
		}
		return nil
	}
	for _, field := range []string{"embeddings", "embedding"} {
		values, ok := obj[field].([]any)
		if !ok || len(values) == 0 {
			continue
		}
		if _, ok := values[0].(float64); ok {
			return validateEmbeddingVector(values)
		}
		if len(values) > 4096 {
			return fmt.Errorf("invalid nv-embedqa batch size")
		}
		for _, vector := range values {
			if err := validateEmbeddingVector(vector); err != nil {
				return err
			}
		}
		return nil
	}
	return fmt.Errorf("missing nv-embedqa embeddings")
}

func nimEmbeddingInputs(textValue, imageValue any) ([]any, error) {
	strings := func(value any) ([]any, error) {
		if value == nil {
			return nil, nil
		}
		values, ok := value.([]any)
		if !ok || len(values) > 4096 {
			return nil, fmt.Errorf("invalid NIM embedding text or image array")
		}
		for _, item := range values {
			if text, ok := item.(string); !ok || text == "" {
				return nil, fmt.Errorf("invalid NIM embedding text or image")
			}
		}
		return values, nil
	}
	texts, err := strings(textValue)
	if err != nil {
		return nil, err
	}
	images, err := strings(imageValue)
	if err != nil {
		return nil, err
	}
	if len(texts) == 0 && len(images) == 0 {
		return nil, fmt.Errorf("NIM embeddings require texts or images")
	}
	if len(texts) == 0 {
		return images, nil
	}
	if len(images) == 0 {
		return texts, nil
	}
	if len(texts) != len(images) {
		return nil, fmt.Errorf("NIM embedding text and image counts differ")
	}
	input := make([]any, len(texts))
	for i := range texts {
		input[i] = texts[i].(string) + " " + images[i].(string)
	}
	return input, nil
}

func validEmbeddingMessages(value any) bool {
	messages, ok := value.([]any)
	if !ok || len(messages) == 0 || len(messages) > 1000 {
		return false
	}
	for _, value := range messages {
		message, ok := value.(map[string]any)
		if !ok {
			return false
		}
		role, ok := message["role"].(string)
		if !ok || role == "" || len(role) > 128 {
			return false
		}
		if content, ok := message["content"].(string); ok {
			if content == "" {
				return false
			}
			continue
		}
		parts, ok := message["content"].([]any)
		if !ok || len(parts) == 0 || len(parts) > 4096 {
			return false
		}
		for _, value := range parts {
			part, ok := value.(map[string]any)
			if !ok {
				return false
			}
			switch part["type"] {
			case "text":
				text, ok := part["text"].(string)
				if !ok || text == "" {
					return false
				}
			case "image_url":
				image, ok := part["image_url"].(map[string]any)
				if !ok {
					return false
				}
				url, ok := image["url"].(string)
				if !ok || url == "" {
					return false
				}
			default:
				return false
			}
		}
	}
	return true
}

func decodeSolidoResponse(data []byte) (CompletionResponse, error) {
	var result CompletionResponse
	var raw struct {
		ID      string          `json:"id"`
		Model   string          `json:"model"`
		Content string          `json:"content"`
		Sources json.RawMessage `json:"sources"`
		Usage   json.RawMessage `json:"usage"`
		Error   json.RawMessage `json:"error"`
	}
	if len(data) > 16<<20 {
		return result, fmt.Errorf("SOLIDO response exceeds size limit")
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return result, err
	}
	if len(raw.Error) > 0 && string(raw.Error) != "null" {
		return result, fmt.Errorf("SOLIDO provider error")
	}
	if raw.Content == "" {
		return result, fmt.Errorf("missing SOLIDO generated content")
	}
	if err := validateSolidoSources(raw.Sources); err != nil {
		return result, err
	}
	usage, err := canonicalUsage(raw.Usage, "json")
	if err != nil {
		return result, err
	}
	result.ID, result.Model, result.Usage = raw.ID, raw.Model, usage
	result.Choices = []Choice{{Index: 0, Message: Message{Role: "assistant", Content: raw.Content}}}
	result.Body = append(json.RawMessage(nil), data...)
	result.Sources = append(json.RawMessage(nil), raw.Sources...)
	return result, nil
}

func decodeSolidoChunk(data []byte) (ChatCompletionChunk, bool, error) {
	var result ChatCompletionChunk
	var raw struct {
		Content *string         `json:"content"`
		Sources json.RawMessage `json:"sources"`
		Usage   json.RawMessage `json:"usage"`
		Error   json.RawMessage `json:"error"`
	}
	if len(data) > 4<<20 {
		return result, false, fmt.Errorf("SOLIDO chunk exceeds size limit")
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return result, false, err
	}
	if len(raw.Error) > 0 && string(raw.Error) != "null" {
		return result, false, fmt.Errorf("SOLIDO provider stream error")
	}
	if err := validateSolidoSources(raw.Sources); err != nil {
		return result, false, err
	}
	if raw.Content == nil && len(raw.Sources) == 0 && len(raw.Usage) == 0 {
		return result, false, fmt.Errorf("missing SOLIDO stream content or metadata")
	}
	if raw.Content != nil {
		result.Choices = make([]struct {
			Index               int             `json:"index"`
			SpeculativeDecoding json.RawMessage `json:"speculative_decoding,omitempty"`
			Delta               struct {
				Role             string     `json:"role,omitempty"`
				Content          string     `json:"content,omitempty"`
				ReasoningContent string     `json:"reasoning_content,omitempty"`
				ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
			} `json:"delta"`
			FinishReason string `json:"finish_reason,omitempty"`
		}, 1)
		result.Choices[0].Delta.Content = *raw.Content
	}
	usage, err := canonicalUsage(raw.Usage, "json")
	if err != nil {
		return result, false, err
	}
	result.Usage = usage
	result.Sources = append(json.RawMessage(nil), raw.Sources...)
	return result, false, nil // SSE requires the common [DONE] marker; sources alone do not terminate.
}

func validateSolidoSources(raw json.RawMessage) error {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return err
	}
	switch value.(type) {
	case map[string]any, []any:
		return nil
	}
	return fmt.Errorf("SOLIDO sources must be object or array")
}
