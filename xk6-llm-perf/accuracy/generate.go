package accuracy

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"
)

// Generate fills missing successful-case predictions through an explicit OpenAI
// chat endpoint. An independent prompts map prevents reference answers from
// accidentally becoming model inputs.
// Each failed request remains a failed case. Calls are serial, bounded by the
// caller context, and credentials are only sent in the Authorization header.
func Generate(ctx context.Context, client *http.Client, endpoint, authorization string, a Artifact, prompts map[string]string) (Artifact, error) {
	requests := make(map[string]GenerationRequest, len(prompts))
	for id, prompt := range prompts {
		requests[id] = GenerationRequest{Messages: []Message{{Role: "user", Content: prompt}}}
	}
	promptBytes, _ := json.Marshal(prompts)
	return generateRequests(ctx, client, endpoint, authorization, a, requests, promptBytes, false)
}

// Message is an authored system, user or assistant chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// GenerationRequest preserves a recipe's chat roles, token limit and stop strings.
// MaxTokens must be positive. Stop may be absent when the recipe has no stops.
type GenerationRequest struct {
	Messages  []Message `json:"messages"`
	MaxTokens int       `json:"max_tokens,omitempty"`
	Stop      []string  `json:"stop,omitempty"`
}

// GenerateRequests generates missing answers from independent recipe requests.
// It preserves authored roles, max_tokens and stop; it does not choose defaults.
// Requests are serial and use the same failure and credential policy as Generate.
func GenerateRequests(ctx context.Context, client *http.Client, endpoint, authorization string, a Artifact, requests map[string]GenerationRequest) (Artifact, error) {
	b, err := json.Marshal(requests)
	if err != nil {
		return a, err
	}
	return generateRequests(ctx, client, endpoint, authorization, a, requests, b, true)
}

func generateRequests(ctx context.Context, client *http.Client, endpoint, authorization string, a Artifact, requests map[string]GenerationRequest, promptBytes []byte, strict bool) (Artifact, error) {
	b, err := json.Marshal(a)
	if err != nil {
		return a, err
	}
	if _, err := evaluate(bytes.NewReader(b), func(string, string, string) (bool, bool, error) { return false, false, nil }); err != nil {
		return a, err
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || client == nil || strings.ContainsAny(authorization, "\r\n") {
		return a, fmt.Errorf("invalid generation endpoint, client or authorization")
	}
	if len(requests) > 100000 {
		return a, fmt.Errorf("too many prompts")
	}
	for _, c := range a.Cases {
		if c.Prediction == nil && c.Outcome == "success" {
			p, ok := requests[c.ID]
			if !ok || !validGenerationRequest(p, strict) {
				return a, fmt.Errorf("missing or oversized prompt for case %s", c.ID)
			}
		}
	}
	copyClient := *client
	copyClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	promptHash := sha256.Sum256(promptBytes)
	a.PromptSHA256 = hex.EncodeToString(promptHash[:])
	a.Cases = append([]Case(nil), a.Cases...)
	for i, c := range a.Cases {
		if c.Prediction != nil || c.Outcome != "success" {
			continue
		}
		if err := ctx.Err(); err != nil {
			return a, fmt.Errorf("generation context: %w", err)
		}
		request := requests[c.ID]
		payload := map[string]any{"model": a.Model, "stream": false, "messages": request.Messages}
		if request.MaxTokens > 0 {
			payload["max_tokens"] = request.MaxTokens
		}
		if len(request.Stop) > 0 {
			payload["stop"] = request.Stop
		}
		body, _ := json.Marshal(payload)
		req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
		if err != nil {
			return a, fmt.Errorf("create generation request")
		}
		req.Header.Set("Content-Type", "application/json")
		if authorization != "" {
			req.Header.Set("Authorization", authorization)
		}
		resp, err := copyClient.Do(req)
		if err != nil {
			a.Cases[i].Outcome = "failure"
			continue
		}
		data, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20+1))
		closeErr := resp.Body.Close()
		if readErr != nil || closeErr != nil || len(data) > 1<<20 || !utf8.Valid(data) || resp.StatusCode != 200 {
			a.Cases[i].Outcome = "failure"
			continue
		}
		var result struct {
			Error   json.RawMessage `json:"error"`
			Model   *string         `json:"model"`
			Choices []struct {
				Index   *int `json:"index"`
				Message struct {
					Content *string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if uniqueJSON(data) != nil || json.Unmarshal(data, &result) != nil || len(result.Error) > 0 && !bytes.Equal(bytes.TrimSpace(result.Error), []byte("null")) || result.Model != nil && *result.Model != a.Model {
			a.Cases[i].Outcome = "failure"
			continue
		}
		found := false
		for _, choice := range result.Choices {
			if choice.Index == nil {
				a.Cases[i].Prediction = nil
				a.Cases[i].Outcome = "failure"
				break
			}
			if *choice.Index == 0 {
				if found || choice.Message.Content == nil || len(*choice.Message.Content) > 65536 {
					a.Cases[i].Prediction = nil
					a.Cases[i].Outcome = "failure"
					break
				}
				a.Cases[i].Prediction = choice.Message.Content
				found = true
			}
		}
		if !found {
			a.Cases[i].Outcome = "failure"
		}
	}
	return a, nil
}

func validGenerationRequest(r GenerationRequest, strict bool) bool {
	if len(r.Messages) == 0 || len(r.Messages) > 128 || strict && (r.MaxTokens < 1 || r.MaxTokens > 1048576) || len(r.Stop) > 16 {
		return false
	}
	size := 0
	for _, m := range r.Messages {
		if m.Role != "system" && m.Role != "user" && m.Role != "assistant" || !utf8.ValidString(m.Content) {
			return false
		}
		size += len(m.Content)
	}
	if size > 65536 || r.Messages[len(r.Messages)-1].Role != "user" {
		return false
	}
	for _, stop := range r.Stop {
		if stop == "" || len(stop) > 4096 || !utf8.ValidString(stop) {
			return false
		}
	}
	return true
}
