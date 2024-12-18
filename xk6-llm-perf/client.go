package llmperf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/grafana/sobek"
	"go.k6.io/k6/js/modules"
)

type Client struct {
	config  Config
	metrics *LLMPerfMetrics
	vu      modules.VU
	http    *http.Client
}

func (c *Client) Complete(call sobek.FunctionCall) sobek.Value {
	rt := c.vu.Runtime()
	ctx := c.vu.Context()
	startTime := time.Now()
	log.Printf("Starting completion request to %s", c.config.BaseURL)

	// Parse request from argument
	var req CompletionRequest
	if err := rt.ExportTo(call.Argument(0), &req); err != nil {
		return rt.ToValue(fmt.Errorf("invalid completion request: %w", err))
	}

	if c.http == nil {
		timeout := 30 * time.Second
		if c.config.Timeout != "" {
			var err error
			timeout, err = time.ParseDuration(c.config.Timeout)
			if err != nil {
				return rt.ToValue(fmt.Errorf("invalid timeout: %w", err))
			}
		}
		c.http = &http.Client{
			Timeout: timeout,
		}
		log.Printf("Created HTTP client with timeout %v", timeout)
	}

	// Add model from config if not set in request
	if req.Model == "" {
		req.Model = c.config.Model
		log.Printf("Using model from config: %s", req.Model)
	}

	// Prepare request body
	body, err := json.Marshal(req)
	if err != nil {
		c.metrics.RecordMetric("llm_errors", 1)
		return rt.ToValue(fmt.Errorf("marshal request: %w", err))
	}
	log.Printf("Request body: %s", string(body))

	// Create request
	url := fmt.Sprintf("%s/chat/completions", strings.TrimRight(c.config.BaseURL, "/"))
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		c.metrics.RecordMetric("llm_errors", 1)
		return rt.ToValue(fmt.Errorf("create request: %w", err))
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.config.APIKey))
	if req.Stream {
		httpReq.Header.Set("Accept", "text/event-stream")
	} else {
		httpReq.Header.Set("Accept", "application/json")
	}
	log.Printf("Making request to %s with headers: %v", url, httpReq.Header)

	// Make request
	resp, err := c.http.Do(httpReq)
	if err != nil {
		c.metrics.RecordMetric("llm_errors", 1)
		return rt.ToValue(fmt.Errorf("do request: %w", err))
	}
	defer resp.Body.Close()

	log.Printf("Got response with status: %d", resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		c.metrics.RecordMetric("llm_errors", 1)
		body, _ := io.ReadAll(resp.Body)
		return rt.ToValue(fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body)))
	}

	var completion CompletionResponse
	if req.Stream {
		completion, err = c.handleStreamingResponse(resp, startTime)
	} else {
		completion, err = c.handleSyncResponse(resp, startTime)
	}
	if err != nil {
		return rt.ToValue(err)
	}

	return rt.ToValue(completion)
}

func (c *Client) handleStreamingResponse(resp *http.Response, startTime time.Time) (CompletionResponse, error) {
	var (
		result     CompletionResponse
		reader     = NewSSEReader(resp.Body)
		firstChunk = true
		content    strings.Builder
		tokens     int
	)

	result.Status = resp.StatusCode
	for {
		event, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return result, fmt.Errorf("read stream: %w", err)
		}

		if event.Data == "[DONE]" {
			break
		}

		var chunk ChatCompletionChunk
		if err := json.Unmarshal([]byte(event.Data), &chunk); err != nil {
			return result, fmt.Errorf("unmarshal chunk: %w", err)
		}

		now := time.Now()
		if firstChunk {
			c.metrics.RecordMetric("llm_ttft", float64(now.Sub(startTime).Milliseconds()))
			firstChunk = false
		}

		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			content.WriteString(chunk.Choices[0].Delta.Content)
			tokens += len(chunk.Choices[0].Delta.Content) // Simple approximation
			c.metrics.RecordMetric("llm_token_latency", float64(now.Sub(startTime).Milliseconds()))
		}
	}

	result.Choices = []Choice{{
		Message: Message{
			Role:    "assistant",
			Content: content.String(),
		},
	}}

	totalTime := time.Since(startTime)
	c.metrics.RecordMetric("llm_completion_time", float64(totalTime.Milliseconds()))
	if tokens > 0 {
		tokensPerSecond := float64(tokens) / totalTime.Seconds()
		c.metrics.RecordMetric("llm_tokens_per_second", tokensPerSecond)
		c.metrics.RecordMetric("llm_total_tokens", float64(tokens))
	}

	return result, nil
}

func (c *Client) handleSyncResponse(resp *http.Response, startTime time.Time) (CompletionResponse, error) {
	var completion CompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&completion); err != nil {
		return completion, fmt.Errorf("decode response: %w", err)
	}

	completion.Status = resp.StatusCode

	totalTime := time.Since(startTime)
	c.metrics.RecordMetric("llm_completion_time", float64(totalTime.Milliseconds()))

	if completion.Usage.TotalTokens > 0 {
		tokensPerSecond := float64(completion.Usage.TotalTokens) / totalTime.Seconds()
		c.metrics.RecordMetric("llm_tokens_per_second", tokensPerSecond)
		c.metrics.RecordMetric("llm_total_tokens", float64(completion.Usage.TotalTokens))
	}

	return completion, nil
}
