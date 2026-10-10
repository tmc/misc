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
)

// Generate fills missing successful-case predictions through an explicit OpenAI
// chat endpoint. An independent prompts map prevents reference answers from
// accidentally becoming model inputs.
// Each failed request remains a failed case. Calls are serial, bounded by the
// caller context, and credentials are only sent in the Authorization header.
func Generate(ctx context.Context, client *http.Client, endpoint, authorization string, a Artifact, prompts map[string]string) (Artifact, error) {
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
	if len(prompts) > 100000 {
		return a, fmt.Errorf("too many prompts")
	}
	for _, c := range a.Cases {
		if c.Prediction == nil && c.Outcome == "success" {
			p, ok := prompts[c.ID]
			if !ok || len(p) > 65536 {
				return a, fmt.Errorf("missing or oversized prompt for case %s", c.ID)
			}
		}
	}
	copyClient := *client
	copyClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	promptBytes, _ := json.Marshal(prompts)
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
		body, _ := json.Marshal(map[string]any{"model": a.Model, "stream": false, "messages": []map[string]string{{"role": "user", "content": prompts[c.ID]}}})
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
		if readErr != nil || closeErr != nil || len(data) > 1<<20 || resp.StatusCode != 200 {
			a.Cases[i].Outcome = "failure"
			continue
		}
		var result struct {
			Choices []struct {
				Index   int `json:"index"`
				Message struct {
					Content *string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if json.Unmarshal(data, &result) != nil {
			a.Cases[i].Outcome = "failure"
			continue
		}
		found := false
		for _, choice := range result.Choices {
			if choice.Index == 0 && choice.Message.Content != nil && len(*choice.Message.Content) <= 65536 {
				if found {
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
