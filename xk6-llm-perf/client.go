package llmperf

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/grafana/sobek"
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/lib/netext/httpext"
	"go.k6.io/k6/metrics"
)

type Client struct {
	config          Config
	metrics         *LLMPerfMetrics
	vu              modules.VU
	http            *http.Client
	transport       http.RoundTripper
	limiters        *prefillLimiters
	warmupRemaining int32
	endpointIndex   uint64
}

func (c *Client) Complete(call sobek.FunctionCall) sobek.Value {
	rt := c.vu.Runtime()
	ctx := c.vu.Context()
	if c.metrics.samples == nil {
		if state := c.vu.State(); state != nil {
			c.metrics.samples = state.Samples
		}
	}
	startTime := time.Now()

	// Parse request from argument
	var req CompletionRequest
	if err := exportValue(call.Argument(0), &req); err != nil {
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
			Timeout:   timeout,
			Transport: c.transport,
		}
		if c.http.Transport == nil {
			c.http.Transport = newTransport()
		}
	}

	// Add model from config if not set in request
	if req.Model == "" {
		req.Model = c.config.Model
	}
	if err := validateConfig(c.config); err != nil {
		c.metrics.RecordMetric("llm_errors", 1)
		return rt.ToValue(err)
	}
	if err := validateCompletionRequest(req); err != nil {
		c.metrics.RecordMetric("llm_errors", 1)
		return rt.ToValue(err)
	}
	config := c.config
	config.Model = req.Model
	if req.Stream && req.StreamOptions == nil {
		req.StreamOptions = &StreamOptions{IncludeUsage: true}
	}
	inputTokens := c.estimateInputTokens(req.Messages)

	// Prepare request body
	body, err := json.Marshal(req)
	if err != nil {
		c.metrics.RecordMetric("llm_errors", 1)
		return rt.ToValue(fmt.Errorf("marshal request: %w", err))
	}

	// Create request
	endpoint := c.nextEndpoint()
	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint+"/chat/completions", bytes.NewReader(body))
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
	httpReq, trace := c.traceHTTPRequest(httpReq)

	recordMetrics := c.takeWarmupSample()
	releasePrefill, err := c.acquirePrefill(ctx, config)
	if err != nil {
		c.recordMetric(recordMetrics, "llm_errors", 1)
		return rt.ToValue(err)
	}

	// Make request
	resp, err := c.http.Do(httpReq)
	if err != nil {
		releasePrefill()
		c.recordHTTPMetrics(recordMetrics, httpReq, nil, trace, err)
		c.recordMetric(recordMetrics, "llm_errors", 1)
		return rt.ToValue(fmt.Errorf("do request: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		releasePrefill()
		c.recordMetric(recordMetrics, "llm_errors", 1)
		body, _ := io.ReadAll(resp.Body)
		c.recordHTTPMetrics(recordMetrics, httpReq, resp, trace, nil)
		return rt.ToValue(fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body)))
	}

	var completion CompletionResponse
	if req.Stream {
		completion, err = c.handleStreamingResponse(resp, startTime, inputTokens, releasePrefill, recordMetrics)
	} else {
		defer releasePrefill()
		completion, err = c.handleSyncResponse(resp, startTime, inputTokens, recordMetrics)
	}
	if err != nil {
		c.recordHTTPMetrics(recordMetrics, httpReq, resp, trace, err)
		c.recordMetric(recordMetrics, "llm_errors", 1)
		return rt.ToValue(err)
	}
	c.recordHTTPMetrics(recordMetrics, httpReq, resp, trace, nil)

	return rt.ToValue(completion)
}

func (c *Client) traceHTTPRequest(req *http.Request) (*http.Request, *httpext.Tracer) {
	if !c.canRecordHTTPMetrics() {
		return req, nil
	}
	trace := &httpext.Tracer{}
	return req.WithContext(httptrace.WithClientTrace(req.Context(), trace.Trace())), trace
}

func (c *Client) canRecordHTTPMetrics() bool {
	state := c.vu.State()
	return state != nil && state.BuiltinMetrics != nil && state.Samples != nil
}

func (c *Client) recordHTTPMetrics(record bool, req *http.Request, resp *http.Response, trace *httpext.Tracer, reqErr error) {
	if !record || trace == nil || !c.canRecordHTTPMetrics() {
		return
	}
	state := c.vu.State()
	tagsAndMeta := c.httpTagsAndMeta(req, resp, reqErr)
	trail := trace.Done()
	trail.SaveSamples(state.BuiltinMetrics, &tagsAndMeta)
	if state.BuiltinMetrics.HTTPReqFailed != nil {
		failed := 0.0
		if reqErr != nil || resp == nil || resp.StatusCode >= 400 {
			failed = 1
		}
		trail.Failed.Valid = true
		trail.Failed.Bool = failed == 1
		trail.Samples = append(trail.Samples, metrics.Sample{
			TimeSeries: metrics.TimeSeries{
				Metric: state.BuiltinMetrics.HTTPReqFailed,
				Tags:   tagsAndMeta.Tags,
			},
			Time:     trail.EndTime,
			Metadata: tagsAndMeta.Metadata,
			Value:    failed,
		})
	}
	metrics.PushIfNotDone(req.Context(), state.Samples, trail)
}

func (c *Client) httpTagsAndMeta(req *http.Request, resp *http.Response, reqErr error) metrics.TagsAndMeta {
	state := c.vu.State()
	if state.Tags != nil {
		tagsAndMeta := state.Tags.GetCurrentValues()
		c.addHTTPSystemTags(&tagsAndMeta, req, resp, reqErr)
		return tagsAndMeta
	}
	tagsAndMeta := metrics.TagsAndMeta{Tags: c.metrics.registry.RootTagSet()}
	c.addHTTPSystemTags(&tagsAndMeta, req, resp, reqErr)
	return tagsAndMeta
}

func (c *Client) addHTTPSystemTags(tagsAndMeta *metrics.TagsAndMeta, req *http.Request, resp *http.Response, reqErr error) {
	state := c.vu.State()
	enabled := state.Options.SystemTags
	if enabled == nil {
		enabled = &metrics.DefaultSystemTagSet
	}
	tagsAndMeta.SetSystemTagOrMetaIfEnabled(enabled, metrics.TagName, req.URL.String())
	tagsAndMeta.SetSystemTagOrMetaIfEnabled(enabled, metrics.TagURL, req.URL.String())
	tagsAndMeta.SetSystemTagOrMetaIfEnabled(enabled, metrics.TagMethod, req.Method)
	if reqErr != nil || resp == nil {
		tagsAndMeta.SetSystemTagOrMetaIfEnabled(enabled, metrics.TagStatus, "0")
		tagsAndMeta.SetSystemTagOrMetaIfEnabled(enabled, metrics.TagExpectedResponse, "false")
		return
	}
	tagsAndMeta.SetSystemTagOrMetaIfEnabled(enabled, metrics.TagStatus, strconv.Itoa(resp.StatusCode))
	tagsAndMeta.SetSystemTagOrMetaIfEnabled(enabled, metrics.TagProto, resp.Proto)
	tagsAndMeta.SetSystemTagOrMetaIfEnabled(enabled, metrics.TagExpectedResponse, strconv.FormatBool(resp.StatusCode < 400))
}

func (c *Client) acquirePrefill(ctx context.Context, config Config) (func(), error) {
	if c.limiters == nil {
		return func() {}, nil
	}
	return c.limiters.acquire(ctx, config)
}

func (c *Client) nextEndpoint() string {
	endpoints := normalizedBaseURLs(c.config)
	if len(endpoints) == 1 {
		return endpoints[0]
	}
	n := atomic.AddUint64(&c.endpointIndex, 1)
	return endpoints[(n-1)%uint64(len(endpoints))]
}

func (c *Client) handleStreamingResponse(resp *http.Response, startTime time.Time, inputTokens int, releasePrefill func(), recordMetrics bool) (CompletionResponse, error) {
	var (
		result      CompletionResponse
		reader      = NewSSEReader(resp.Body)
		firstToken  = true
		firstOutput = true
		complete    bool
		lastEvent   time.Time
		lastChunk   time.Time
		content     strings.Builder
		chunks      int
		usage       Usage
		stats       requestStats
	)
	prefillReleased := false
	release := func() {
		if prefillReleased {
			return
		}
		prefillReleased = true
		releasePrefill()
	}
	defer release()

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
			complete = true
			break
		}

		var chunk ChatCompletionChunk
		if err := json.Unmarshal([]byte(event.Data), &chunk); err != nil {
			return result, fmt.Errorf("unmarshal chunk: %w", err)
		}

		now := time.Now()
		if !lastEvent.IsZero() && now.After(lastEvent) {
			c.recordMetric(recordMetrics, "llm_inter_chunk_latency", float64(now.Sub(lastEvent).Milliseconds()))
		}
		lastEvent = now
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			text := chunk.Choices[0].Delta.Content
			if firstToken {
				stats.TTFT = now.Sub(startTime)
				c.recordMetric(recordMetrics, "llm_ttft", float64(stats.TTFT.Milliseconds()))
				c.recordAdjustedMetric(recordMetrics, "llm_ttft_adjusted", stats.TTFT)
				firstToken = false
				lastChunk = now
				release()
			}
			if firstOutput {
				stats.TTFO = now.Sub(startTime)
				c.recordMetric(recordMetrics, "llm_ttfo", float64(stats.TTFO.Milliseconds()))
				c.recordAdjustedMetric(recordMetrics, "llm_ttfo_adjusted", stats.TTFO)
				firstOutput = false
			}
			content.WriteString(text)
			chunks++
			if !lastChunk.IsZero() && now.After(lastChunk) {
				stats.MaxTokenLatency = maxDuration(stats.MaxTokenLatency, now.Sub(lastChunk))
				c.recordMetric(recordMetrics, "llm_token_latency", float64(now.Sub(lastChunk).Milliseconds()))
			}
			lastChunk = now
		}
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.ReasoningContent != "" && firstToken {
			stats.TTFT = now.Sub(startTime)
			c.recordMetric(recordMetrics, "llm_ttft", float64(stats.TTFT.Milliseconds()))
			c.recordAdjustedMetric(recordMetrics, "llm_ttft_adjusted", stats.TTFT)
			firstToken = false
			lastChunk = now
			release()
		}
		for _, choice := range chunk.Choices {
			if choice.FinishReason != "" {
				complete = true
			}
		}
		if chunk.Usage.TotalTokens > 0 || chunk.Usage.PromptTokens > 0 || chunk.Usage.CompletionTokens > 0 {
			usage = chunk.Usage
		}
	}
	if !complete {
		return result, fmt.Errorf("stream ended before completion marker")
	}

	result.Choices = []Choice{{
		Message: Message{
			Role:    "assistant",
			Content: content.String(),
		},
	}}

	totalTime := time.Since(startTime)
	stats.RequestLatency = totalTime
	c.recordMetric(recordMetrics, "llm_completion_time", float64(totalTime.Milliseconds()))
	c.recordMetric(recordMetrics, "llm_request_latency", float64(totalTime.Milliseconds()))
	c.recordAdjustedMetric(recordMetrics, "llm_request_latency_adjusted", totalTime)
	c.recordMetric(recordMetrics, "llm_requests", 1)

	promptTokens := usage.PromptTokens
	if promptTokens == 0 {
		promptTokens = inputTokens
	}
	outputTokens := usage.CompletionTokens
	if outputTokens == 0 && usage.TotalTokens > promptTokens {
		outputTokens = usage.TotalTokens - promptTokens
	}
	estimatedOutputTokens := c.estimateTokenCount(content.String())
	if outputTokens == 0 {
		outputTokens = estimatedOutputTokens
	}
	if outputTokens == 0 && chunks > 0 {
		outputTokens = chunks
	}
	c.recordTokenMetrics(recordMetrics, promptTokens, outputTokens, inputTokens, estimatedOutputTokens, totalTime)
	c.recordGoodRequest(recordMetrics, stats)

	return result, nil
}

func (c *Client) handleSyncResponse(resp *http.Response, startTime time.Time, inputTokens int, recordMetrics bool) (CompletionResponse, error) {
	var completion CompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&completion); err != nil {
		return completion, fmt.Errorf("decode response: %w", err)
	}

	completion.Status = resp.StatusCode

	totalTime := time.Since(startTime)
	stats := requestStats{RequestLatency: totalTime}
	c.recordMetric(recordMetrics, "llm_completion_time", float64(totalTime.Milliseconds()))
	c.recordMetric(recordMetrics, "llm_request_latency", float64(totalTime.Milliseconds()))
	c.recordAdjustedMetric(recordMetrics, "llm_request_latency_adjusted", totalTime)
	c.recordMetric(recordMetrics, "llm_requests", 1)

	promptTokens := completion.Usage.PromptTokens
	if promptTokens == 0 {
		promptTokens = inputTokens
	}
	c.recordTokenMetrics(recordMetrics, promptTokens, c.completionTokens(completion), inputTokens, c.estimateCompletionTokens(completion), totalTime)
	c.recordGoodRequest(recordMetrics, stats)

	return completion, nil
}

func newTransport() *http.Transport {
	return &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		MaxIdleConns:          1000,
		MaxIdleConnsPerHost:   100,
		MaxConnsPerHost:       0,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

func (c *Client) recordTokenMetrics(recordMetrics bool, promptTokens, outputTokens, estimatedPromptTokens, estimatedOutputTokens int, elapsed time.Duration) {
	if promptTokens > 0 {
		c.recordMetric(recordMetrics, "llm_prompt_tokens", float64(promptTokens))
		c.recordMetric(recordMetrics, "llm_input_sequence_length", float64(promptTokens))
	}
	if outputTokens > 0 {
		c.recordMetric(recordMetrics, "llm_completion_tokens", float64(outputTokens))
		c.recordMetric(recordMetrics, "llm_output_sequence_length", float64(outputTokens))
		c.recordMetric(recordMetrics, "llm_total_tokens", float64(promptTokens+outputTokens))
		if seconds := elapsed.Seconds(); seconds > 0 {
			c.recordMetric(recordMetrics, "llm_tokens_per_second", float64(outputTokens)/seconds)
		}
	}
	c.recordDiscrepancy(recordMetrics, "llm_prompt_token_discrepancy", promptTokens, estimatedPromptTokens)
	c.recordDiscrepancy(recordMetrics, "llm_completion_token_discrepancy", outputTokens, estimatedOutputTokens)
}

func (c *Client) completionTokens(completion CompletionResponse) int {
	if completion.Usage.CompletionTokens > 0 {
		return completion.Usage.CompletionTokens
	}
	if completion.Usage.TotalTokens > completion.Usage.PromptTokens {
		return completion.Usage.TotalTokens - completion.Usage.PromptTokens
	}
	return c.estimateCompletionTokens(completion)
}

func (c *Client) recordDiscrepancy(recordMetrics bool, metric string, reported, estimated int) {
	if reported <= 0 || estimated <= 0 {
		return
	}
	diff := reported - estimated
	if diff < 0 {
		diff = -diff
	}
	c.recordMetric(recordMetrics, metric, float64(diff)/float64(reported)*100)
	if diff > 0 {
		c.recordMetric(recordMetrics, "llm_usage_discrepancies", 1)
	}
}

func (c *Client) estimateCompletionTokens(completion CompletionResponse) int {
	var text strings.Builder
	for _, choice := range completion.Choices {
		text.WriteString(choice.Message.Content)
		text.WriteByte(' ')
	}
	return c.estimateTokenCount(text.String())
}

func (c *Client) estimateInputTokens(messages []Message) int {
	var text strings.Builder
	for _, msg := range messages {
		text.WriteString(msg.Content)
		text.WriteByte(' ')
	}
	return c.estimateTokenCount(text.String())
}

func (c *Client) estimateTokenCount(text string) int {
	return estimateTokenCount(text, c.config.TokenMultiplier)
}

func estimateTokenCount(text string, multiplier float64) int {
	words := len(strings.Fields(text))
	if words == 0 {
		return 0
	}
	if multiplier <= 0 {
		multiplier = 1.33
	}
	byWords := int(float64(words)*multiplier + 0.5)
	byChars := (len([]rune(text)) + 3) / 4
	if byChars > byWords {
		return byChars
	}
	if byWords < 1 {
		return 1
	}
	return byWords
}

type requestStats struct {
	TTFT            time.Duration
	TTFO            time.Duration
	MaxTokenLatency time.Duration
	RequestLatency  time.Duration
}

func (c *Client) recordGoodRequest(recordMetrics bool, stats requestStats) {
	if c.config.MaxTTFT > 0 && (stats.TTFT == 0 || stats.TTFT > time.Duration(c.config.MaxTTFT)*time.Millisecond) {
		c.recordMetric(recordMetrics, "llm_good_request", 0)
		return
	}
	if c.config.MaxTTFO > 0 && (stats.TTFO == 0 || stats.TTFO > time.Duration(c.config.MaxTTFO)*time.Millisecond) {
		c.recordMetric(recordMetrics, "llm_good_request", 0)
		return
	}
	if c.config.MaxTokenLatency > 0 && (stats.MaxTokenLatency == 0 || stats.MaxTokenLatency > time.Duration(c.config.MaxTokenLatency)*time.Millisecond) {
		c.recordMetric(recordMetrics, "llm_good_request", 0)
		return
	}
	c.recordMetric(recordMetrics, "llm_good_request", 1)
}

func (c *Client) takeWarmupSample() bool {
	for {
		remaining := atomic.LoadInt32(&c.warmupRemaining)
		if remaining <= 0 {
			return true
		}
		if atomic.CompareAndSwapInt32(&c.warmupRemaining, remaining, remaining-1) {
			return false
		}
	}
}

func (c *Client) recordMetric(record bool, name string, value float64) {
	if !record {
		return
	}
	c.metrics.RecordMetric(name, value)
}

func (c *Client) recordAdjustedMetric(record bool, name string, value time.Duration) {
	if c.config.NetworkRTT <= 0 {
		return
	}
	adjusted := value - time.Duration(c.config.NetworkRTT)*time.Millisecond
	if adjusted < 0 {
		adjusted = 0
	}
	c.recordMetric(record, name, float64(adjusted.Milliseconds()))
}

func maxDuration(a, b time.Duration) time.Duration {
	if b > a {
		return b
	}
	return a
}

func validateConfig(config Config) error {
	endpoints := normalizedBaseURLs(config)
	if len(endpoints) == 0 {
		return fmt.Errorf("baseURL is required")
	}
	for i, endpoint := range endpoints {
		u, err := url.Parse(endpoint)
		if err != nil {
			return fmt.Errorf("parse baseURL[%d]: %w", i, err)
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return fmt.Errorf("baseURL[%d] must use http or https", i)
		}
		if u.Host == "" {
			return fmt.Errorf("baseURL[%d] must include host", i)
		}
	}
	if config.NetworkRTT < 0 {
		return fmt.Errorf("networkRTT must be non-negative")
	}
	if config.TokenMultiplier < 0 {
		return fmt.Errorf("tokenMultiplier must be non-negative")
	}
	if config.PrefillConcurrency < 0 {
		return fmt.Errorf("prefillConcurrency must be non-negative")
	}
	if config.WarmupCount < 0 {
		return fmt.Errorf("warmupCount must be non-negative")
	}
	return nil
}

func normalizedBaseURLs(config Config) []string {
	var endpoints []string
	for _, endpoint := range config.BaseURLs {
		endpoint = strings.TrimRight(strings.TrimSpace(endpoint), "/")
		if endpoint != "" {
			endpoints = append(endpoints, endpoint)
		}
	}
	if len(endpoints) == 0 {
		endpoint := strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
		if endpoint != "" {
			endpoints = append(endpoints, endpoint)
		}
	}
	return endpoints
}

func validateCompletionRequest(req CompletionRequest) error {
	if strings.TrimSpace(req.Model) == "" {
		return fmt.Errorf("model is required")
	}
	if len(req.Messages) == 0 {
		return fmt.Errorf("messages must not be empty")
	}
	for i, msg := range req.Messages {
		if strings.TrimSpace(msg.Role) == "" {
			return fmt.Errorf("messages[%d].role is required", i)
		}
		if strings.TrimSpace(msg.Content) == "" {
			return fmt.Errorf("messages[%d].content is required", i)
		}
	}
	if req.MaxTokens < 0 {
		return fmt.Errorf("max_tokens must be non-negative")
	}
	if req.Temperature < 0 {
		return fmt.Errorf("temperature must be non-negative")
	}
	return nil
}
