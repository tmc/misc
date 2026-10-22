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
	"go.k6.io/k6/v2/js/modules"
	"go.k6.io/k6/v2/lib/netext/httpext"
	"go.k6.io/k6/v2/metrics"
)

// Client executes inference requests without following redirects.
// Redirect responses fail without forwarding credentials or request bodies.
type Client struct {
	controls        *controlGroups
	config          Config
	metrics         *LLMPerfMetrics
	vu              modules.VU
	http            *http.Client
	transport       http.RoundTripper
	limiters        *prefillLimiters
	requestLimiters *prefillLimiters
	warmupRemaining int64
	endpointIndex   uint64
	measurement     *Measurement
	run             *runState
	protocol        string
}

func (c *Client) Complete(call sobek.FunctionCall) sobek.Value {
	rt := c.vu.Runtime()
	ctx := c.vu.Context()
	c.measurement = nil
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
		if c.config.UseK6Transport {
			if state := c.vu.State(); state == nil || state.Transport == nil {
				return rt.ToValue(fmt.Errorf("k6 transport is unavailable"))
			} else {
				c.transport = state.Transport
			}
		}
		c.http = &http.Client{
			Timeout:       timeout,
			Transport:     c.transport,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		}
		if c.http.Transport == nil {
			c.http.Transport = newTransport()
		}
	}

	c.protocol = "openai-chat"
	if req.Raw != nil {
		if err := validateRaw(req.Raw); err != nil {
			return rt.ToValue(err)
		}
		c.protocol = req.Raw.Protocol
		if model, ok := req.Raw.Body["model"].(string); ok {
			req.Model = model
		}
		req.Stream, _ = req.Raw.Body["stream"].(bool)
		if req.Raw.Stream != nil {
			req.Stream = *req.Raw.Stream
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
	expectation := req.Work
	req.Work = nil
	counts := req.Measurement
	req.Measurement = nil
	scheduledAt, cancelAfter := req.ScheduledAt, req.CancelAfter
	req.ScheduledAt, req.CancelAfter = nil, ""

	serializationStart := time.Now()
	// Prepare request body
	var body []byte
	var err error
	path := "/chat/completions"
	if req.Raw != nil {
		path = req.Raw.Path
		if _, ok := req.Raw.Body["model"]; req.Raw.Body != nil && !ok && req.Model != "" && c.protocol != "tgi" && c.protocol != "hf-tei-rankings" && c.protocol != "nim-image-retrieval" && c.protocol != "nv-embedqa" && c.protocol != "solido-rag" && c.protocol != "providerwire-v4" {
			req.Raw.Body["model"] = req.Model
		}
		body, err = json.Marshal(req.Raw.Body)
	} else {
		body, err = marshalTyped(req)
	}
	contentType := "application/json"
	if req.Raw != nil && req.Raw.Multipart != nil {
		body, contentType, err = encodeMultipart(req.Raw.Multipart)
	}
	serializationLatency := ms(time.Since(serializationStart))
	if err != nil {
		c.metrics.RecordMetric("llm_errors", 1)
		return rt.ToValue(fmt.Errorf("marshal request: %w", err))
	}

	if len(body) > 16*1024*1024 {
		return rt.ToValue(fmt.Errorf("request exceeds 16 MiB limit"))
	}
	// Create request
	endpoint := c.nextEndpoint()
	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint+path, bytes.NewReader(body))
	if err != nil {
		c.metrics.RecordMetric("llm_errors", 1)
		return rt.ToValue(fmt.Errorf("create request: %w", err))
	}

	// Set headers
	httpReq.Header.Set("Content-Type", contentType)
	if c.protocol == "anthropic" {
		httpReq.Header.Set("x-api-key", c.config.APIKey)
		httpReq.Header.Set("anthropic-version", "2023-06-01")
	} else {
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.config.APIKey))
	}
	if c.protocol == "providerwire-v4" {
		httpReq.Header.Set("Ai-Language-Model-Id", req.Model)
		httpReq.Header.Set("Ai-Language-Model-Specification-Version", "4")
		httpReq.Header.Set("Ai-Language-Model-Streaming", fmt.Sprint(req.Stream))
	}
	if req.Stream {
		httpReq.Header.Set("Accept", "text/event-stream")
	} else {
		httpReq.Header.Set("Accept", "application/json")
	}

	recordMetrics := c.takeWarmupSample()
	phase := c.config.Phase
	if phase == "" {
		phase = "measurement"
	}
	if !recordMetrics {
		phase = "warmup"
	}
	recordMetrics = recordMetrics && phase == "measurement"
	m := &Measurement{SchemaVersion: 1, Phase: phase, Endpoint: sanitizedEndpoint(endpoint), Model: req.Model, Outcome: "failure", ErrorStage: "admission", Recorded: recordMetrics, StartedAt: millis(startTime), RTT: float64(c.config.NetworkRTT), RTTSource: c.config.RTTSource, start: startTime, counts: counts, estimatedInput: inputTokens}
	if m.RTTSource == "" {
		m.RTTSource = "user_supplied"
	}
	m.SerializationLatency = &serializationLatency
	m.RequestBodyBytes = int64(len(body))
	if transport, ok := c.http.Transport.(*http.Transport); ok && !c.config.UseK6Transport {
		m.MaxConnsPerHost = transport.MaxConnsPerHost
	}
	m.Stream = req.Stream
	m.TransportProfile = "shared-http2-enabled-v1"
	if c.config.UseK6Transport {
		m.TransportProfile = "k6-transport"
	}
	if expectation != nil {
		m.Work = &WorkReceipt{Expectation: *expectation}
	}
	m.Protocol = c.protocol
	m.noInputEstimate = len(req.Tools) > 0 || req.Raw != nil
	m.noOutputEstimate = c.protocol == "json" || c.protocol == "audio-transcription" || c.protocol == "image" || c.protocol == "video" || c.protocol == "retrieval" || nonGenerationProtocol(c.protocol)
	for _, msg := range req.Messages {
		if hasMedia(msg.Content) || len(msg.ToolCalls) > 0 {
			m.noInputEstimate = true
		}
	}
	m.ScheduledAt = scheduledAt
	c.measurement = m
	defer c.finishMeasurement(m)
	timeout := c.http.Timeout
	policyCancel := false
	if cancelAfter != "" {
		d, _ := time.ParseDuration(cancelAfter)
		if d < timeout {
			timeout = d
			policyCancel = true
		}
	}
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	httpReq = httpReq.WithContext(requestCtx)
	httpReq, trace := c.traceHTTPRequest(httpReq)
	httpReq, m.trace = attachTrace(httpReq)
	queueStart := time.Now()
	if config.ControlGroup != "" {
		if c.controls == nil {
			return rt.ToValue(fmt.Errorf("control groups are unavailable"))
		}
		release, err := c.controls.acquire(requestCtx, config.ControlGroup, false)
		if err != nil {
			m.AdmissionWait = ms(time.Since(queueStart))
			return rt.ToValue(err)
		}
		defer release()
	} else if c.requestLimiters != nil && config.RequestConcurrency > 0 {
		capConfig := config
		capConfig.PrefillConcurrency = config.RequestConcurrency
		release, err := c.requestLimiters.acquire(requestCtx, capConfig)
		if err != nil {
			m.AdmissionWait = ms(time.Since(queueStart))
			return rt.ToValue(fmt.Errorf("request admission: %w", err))
		}
		defer release()
	}
	releasePrefill, err := c.acquirePrefill(requestCtx, config)
	m.AdmissionWait = ms(time.Since(queueStart))
	if err != nil {
		c.recordMetric(recordMetrics, "llm_errors", 1)
		return rt.ToValue(err)
	}

	httpReq, m.TraceContext = attachTraceContext(httpReq, config)

	// Make request
	m.Attempted = true
	m.ErrorStage = "transport"
	m.requestStart = time.Now()
	m.DispatchedAt = millis(m.requestStart)
	if c.run != nil {
		m.runGeneration = c.run.dispatch(m)
	}
	resp, err := c.http.Do(httpReq)
	if err != nil {
		releasePrefill()
		if requestCtx.Err() == context.Canceled {
			m.Outcome = "cancelled"
		} else if requestCtx.Err() == context.DeadlineExceeded {
			m.Outcome = "timeout"
			if policyCancel {
				m.Outcome = "cancelled"
			}
		}
		c.recordHTTPMetrics(recordMetrics, httpReq, nil, trace, err)
		c.recordMetric(recordMetrics, "llm_errors", 1)
		return rt.ToValue(fmt.Errorf("do request: %w", err))
	}
	resp.Body = &measurementBody{ReadCloser: resp.Body, measurement: m}
	defer resp.Body.Close()
	m.Status = resp.StatusCode
	m.ErrorStage = "http"

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		releasePrefill()
		c.recordMetric(recordMetrics, "llm_errors", 1)
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64*1024))
		c.recordHTTPMetrics(recordMetrics, httpReq, resp, trace, nil)
		return rt.ToValue(fmt.Errorf("unexpected status code %d", resp.StatusCode))
	}

	var completion CompletionResponse
	submissionRecorded := false
	m.ErrorStage = "protocol"
	if req.Stream && c.protocol == "image" {
		defer releasePrefill()
		completion, err = readImageStream(resp.Body, func(now time.Time, terminal bool) {
			if m.firstOutput.IsZero() {
				m.firstOutput = now
			}
			m.last = now
			releasePrefill()
		})
		completion.Status = resp.StatusCode
		m.reported = completion.Usage
		m.responseEnd = time.Now()
	} else if req.Stream {
		completion, err = c.handleStreamingResponse(resp, startTime, inputTokens, releasePrefill, recordMetrics)
	} else {
		defer releasePrefill()
		completion, err = c.handleSyncResponse(resp, startTime, inputTokens, recordMetrics, httpReq, req.Raw, requestCtx, func() {
			value := m.trace.finish(time.Now())
			m.HTTPTrace = &value
			m.trace = nil
			c.recordHTTPMetrics(recordMetrics, httpReq, resp, trace, nil)
			submissionRecorded = true
		})
	}
	if err != nil {
		if requestCtx.Err() == context.Canceled {
			m.Outcome = "cancelled"
		} else if requestCtx.Err() == context.DeadlineExceeded {
			m.Outcome = "timeout"
			if policyCancel {
				m.Outcome = "cancelled"
			}
		}
		if !submissionRecorded {
			c.recordHTTPMetrics(recordMetrics, httpReq, resp, trace, err)
		}
		c.recordMetric(recordMetrics, "llm_errors", 1)
		return rt.ToValue(err)
	}
	if !submissionRecorded {
		c.recordHTTPMetrics(recordMetrics, httpReq, resp, trace, nil)
	}

	m.Outcome, m.ErrorStage = "success", ""
	observeWork(m, completion)
	c.finishMeasurement(m)
	completion.Usage = Usage{ReasoningTokens: m.ReasoningTokens, CachedTokens: m.CachedTokens, CacheWriteTokens: m.CacheWriteTokens}
	if m.InputTokens.Value != nil {
		completion.Usage.PromptTokens = *m.InputTokens.Value
	}
	if m.OutputTokens.Value != nil {
		completion.Usage.CompletionTokens = *m.OutputTokens.Value
	}
	completion.Usage.TotalTokens = completion.Usage.PromptTokens + completion.Usage.CompletionTokens
	if m.ProviderTotalTokens != nil {
		completion.Usage.TotalTokens = *m.ProviderTotalTokens
	}
	completion.Measurement = m
	value, err := jsonValue(rt, completion)
	if err != nil {
		return rt.ToValue(err)
	}
	return value
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
	ctx, cancel := context.WithCancel(req.Context())
	stop := context.AfterFunc(c.vu.Context(), cancel)
	defer stop()
	defer cancel()
	if c.vu.Context().Err() != nil {
		cancel()
	}
	c.metrics.pipeline.push(ctx, state.Samples, trail)
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
	if resp == nil {
		tagsAndMeta.SetSystemTagOrMetaIfEnabled(enabled, metrics.TagStatus, "0")
		tagsAndMeta.SetSystemTagOrMetaIfEnabled(enabled, metrics.TagExpectedResponse, "false")
		return
	}
	tagsAndMeta.SetSystemTagOrMetaIfEnabled(enabled, metrics.TagStatus, strconv.Itoa(resp.StatusCode))
	tagsAndMeta.SetSystemTagOrMetaIfEnabled(enabled, metrics.TagProto, resp.Proto)
	tagsAndMeta.SetSystemTagOrMetaIfEnabled(enabled, metrics.TagExpectedResponse, strconv.FormatBool(reqErr == nil && resp.StatusCode < 400))
}

func (c *Client) acquirePrefill(ctx context.Context, config Config) (func(), error) {
	if config.ControlGroup != "" {
		return c.controls.acquire(ctx, config.ControlGroup, true)
	}
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
		result       CompletionResponse
		reader       = NewSSEReader(resp.Body)
		firstToken   = true
		firstOutput  = true
		complete     bool
		wideTerminal bool
		lastEvent    time.Time
		lastChunk    time.Time
		content      strings.Builder
		reasoning    strings.Builder
		chunks       int
		usage        Usage
		stats        requestStats
	)
	toolCalls := map[int]*ToolCall{}
	choices := streamChoices{}
	streamBytes := 0
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
	var providerwire providerwireStream
	var responses responsesStream
	if c.protocol == "openai-responses" && c.measurement != nil {
		c.measurement.AIPerfContentPolicy = responsesContentPolicy
		c.measurement.AIPerfContentEvents = new(int)
	}
	for {
		event, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return result, fmt.Errorf("read stream: %w", err)
		}

		if event.Data == "[DONE]" {
			if c.protocol == "providerwire-v4" {
				return result, fmt.Errorf("providerwire requires finish event")
			}
			wideTerminal = true
			complete = true
			break
		}

		parsingStart := time.Now()
		var envelope struct {
			Error json.RawMessage `json:"error"`
		}
		if err := json.Unmarshal([]byte(event.Data), &envelope); err == nil && len(envelope.Error) > 0 && string(envelope.Error) != "null" {
			c.addParsingTime(parsingStart)
			return result, fmt.Errorf("provider stream error")
		}
		streamBytes += len(event.Data)
		if streamBytes > 16*1024*1024 {
			return result, fmt.Errorf("stream exceeds 16 MiB response limit")
		}
		var chunk ChatCompletionChunk
		var terminal bool
		if c.protocol == "providerwire-v4" {
			chunk, terminal, err = providerwire.decode([]byte(event.Data))
		} else if c.protocol == "openai-responses" {
			chunk, terminal, err = responses.decode([]byte(event.Data))
		} else {
			chunk, terminal, err = decodeChunk([]byte(event.Data), c.protocol)
		}
		c.addParsingTime(parsingStart)
		if err != nil {
			return result, fmt.Errorf("unmarshal chunk: %w", err)
		}
		if err := choices.add(chunk); err != nil {
			return result, err
		}

		if chunk.ProviderStatus != "" {
			result.ProviderStatus = chunk.ProviderStatus
		}
		if chunk.ID != "" {
			result.ID = chunk.ID
		}
		if len(chunk.Sources) != 0 {
			result.Sources = chunk.Sources
		}
		if chunk.Model != "" {
			if result.Model != "" && chunk.Model != result.Model {
				return result, fmt.Errorf("stream model changed")
			}
			result.Model = chunk.Model
		}
		if chunk.Object != "" {
			result.Object = chunk.Object
		}
		if chunk.Created != 0 {
			result.Created = chunk.Created
		}
		placements := make([]Choice, len(chunk.Choices))
		for i, choice := range chunk.Choices {
			placements[i] = Choice{Index: choice.Index, SpeculativeDecoding: choice.SpeculativeDecoding}
		}
		primary := -1
		for i, choice := range chunk.Choices {
			if choice.Index != 0 && c.measurement != nil {
				c.measurement.MultiChoice = true
				c.measurement.noOutputEstimate = true
			}
			if choice.Index == 0 {
				primary = i
			}
		}
		if c.measurement != nil && chunk.specDecodeUnavailable != "" && c.measurement.SpecDecodeUnavailable == "" {
			c.measurement.SpecDecode = nil
			c.measurement.SpecDecodeUnavailable = chunk.specDecodeUnavailable
		}
		c.readSpecDecodePlacements(chunk.Metrics.SpeculativeDecoding, placements)
		now := time.Now()
		if chunk.aiperfContent && c.measurement != nil {
			recordResponsesTiming(c.measurement, now, startTime)
		}
		if !lastEvent.IsZero() && now.After(lastEvent) {
			c.recordMetric(recordMetrics, "llm_inter_chunk_latency", (float64(now.Sub(lastEvent)) / float64(time.Millisecond)))
		}
		lastEvent = now
		if primary >= 0 {
			delta := chunk.Choices[primary].Delta
			hasContent := delta.Content != "" || delta.ReasoningContent != "" || len(delta.ToolCalls) > 0
			if hasContent {
				if c.measurement != nil {
					c.measurement.content(now, delta.Content != "" || len(delta.ToolCalls) > 0, chunk.Usage)
				}
				if firstToken {
					stats.TTFT = now.Sub(startTime)
					c.recordMetric(recordMetrics, "llm_ttft", ms(stats.TTFT))
					c.recordAdjustedMetric(recordMetrics, "llm_ttft_adjusted", stats.TTFT)
					firstToken = false
					release()
				}
				if (delta.Content != "" || len(delta.ToolCalls) > 0) && firstOutput {
					stats.TTFO = now.Sub(startTime)
					c.recordMetric(recordMetrics, "llm_ttfo", ms(stats.TTFO))
					c.recordAdjustedMetric(recordMetrics, "llm_ttfo_adjusted", stats.TTFO)
					firstOutput = false
				}
				if !lastChunk.IsZero() {
					c.recordMetric(recordMetrics, "llm_token_latency", ms(now.Sub(lastChunk)))
				}
				lastChunk = now
				chunks++
				content.WriteString(delta.Content)
				reasoning.WriteString(delta.ReasoningContent)
				if len(delta.ToolCalls) > 0 && c.measurement != nil {
					c.measurement.noOutputEstimate = true
				}
				for _, part := range delta.ToolCalls {
					if part.Index < 0 || part.Index >= 128 {
						return result, fmt.Errorf("tool call index exceeds limit")
					}
					call := toolCalls[part.Index]
					if call == nil {
						call = &ToolCall{Index: part.Index}
						toolCalls[part.Index] = call
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
						return result, fmt.Errorf("tool arguments exceed 4 MiB")
					}
				}
			}
		}
		if primary >= 0 {
			choice := chunk.Choices[primary]
			if choice.FinishReason != "" {
				complete = true
				result.Choices = []Choice{{FinishReason: choice.FinishReason}}
			}
		}
		if chunk.Usage.totalPresent || chunk.Usage.promptPresent || chunk.Usage.completionPresent || chunk.Usage.CachedTokens != nil || chunk.Usage.ReasoningTokens != nil || chunk.Usage.CacheWriteTokens != nil {
			usage, err = mergeUsage(usage, chunk.Usage)
			if err != nil {
				return result, err
			}
		}
		if c.measurement != nil {
			c.measurement.reported = usage
			c.measurement.estimatedOutput = c.estimateTokenCount(content.String() + reasoning.String())
		}
		if terminal {
			wideTerminal = true
			complete = true
			break
		}
	}
	if !complete || (!wideTerminal && !choices.complete()) {
		return result, fmt.Errorf("stream ended before completion marker")
	}

	finishReason := ""
	if len(result.Choices) > 0 {
		finishReason = result.Choices[0].FinishReason
	}
	result.Choices = []Choice{{
		FinishReason: finishReason,
		Message: Message{
			Role:             "assistant",
			Content:          content.String(),
			ReasoningContent: reasoning.String(),
		},
	}}

	for i := 0; i < 128; i++ {
		if call := toolCalls[i]; call != nil {
			result.Choices[0].Message.ToolCalls = append(result.Choices[0].Message.ToolCalls, *call)
		}
	}
	if len(choices) > 0 {
		result.Choices = choices.choices()
	}
	if err := validateCompletedTools(result.Choices); err != nil {
		return result, err
	}
	totalTime := time.Since(startTime)
	if c.measurement != nil {
		c.measurement.responseEnd = time.Now()
	}
	c.recordMetric(recordMetrics, "llm_completion_time", (float64(totalTime) / float64(time.Millisecond)))
	c.recordMetric(recordMetrics, "llm_request_latency", (float64(totalTime) / float64(time.Millisecond)))
	c.recordAdjustedMetric(recordMetrics, "llm_request_latency_adjusted", totalTime)
	c.recordMetric(recordMetrics, "llm_requests", 1)

	promptTokens := usage.PromptTokens
	if !usage.promptPresent {
		promptTokens = inputTokens
	}
	outputTokens := usage.CompletionTokens
	if !usage.completionPresent && usage.TotalTokens > promptTokens {
		outputTokens = usage.TotalTokens - promptTokens
	}
	estimatedOutputTokens := c.estimateTokenCount(content.String() + reasoning.String())
	if c.measurement != nil {
		c.measurement.reported = usage
		c.measurement.estimatedOutput = estimatedOutputTokens
	}
	if !usage.completionPresent && outputTokens == 0 {
		outputTokens = estimatedOutputTokens
	}
	if !usage.completionPresent && outputTokens == 0 && chunks > 0 {
		outputTokens = chunks
	}
	result.Usage = Usage{PromptTokens: promptTokens, CompletionTokens: outputTokens, TotalTokens: promptTokens + outputTokens}
	c.recordTokenMetrics(recordMetrics, promptTokens, outputTokens, inputTokens, estimatedOutputTokens, totalTime)

	return result, nil
}

func (c *Client) handleSyncResponse(resp *http.Response, startTime time.Time, inputTokens int, recordMetrics bool, request *http.Request, raw *RawRequest, lifecycleContext context.Context, submissionDone func()) (CompletionResponse, error) {
	var completion CompletionResponse
	data, err := io.ReadAll(io.LimitReader(resp.Body, 16*1024*1024+1))
	if err != nil {
		return completion, fmt.Errorf("read response: %w", err)
	}
	if len(data) > 16*1024*1024 {
		return completion, fmt.Errorf("response exceeds 16 MiB limit")
	}
	if c.protocol == "video" {
		submissionDone()
		data, err = pollVideo(lifecycleContext, c.http, request, data, raw.Poll)
		if err != nil {
			return completion, err
		}
	}
	parsingStart := time.Now()
	if c.protocol == "audio-transcription" {
		format := ""
		if raw != nil {
			if raw.Multipart != nil {
				format = raw.Multipart.Fields["response_format"]
			}
			if format == "" {
				format, _ = raw.Body["response_format"].(string)
			}
		}
		completion, err = decodeAudioResponseFormat(data, format, resp.Header.Get("Content-Type"))
	} else {
		completion, err = decodeResponse(data, c.protocol)
	}
	c.addParsingTime(parsingStart)
	if err != nil {
		return completion, fmt.Errorf("decode response: %w", err)
	}
	if err := validateResponseChoices(completion.Choices); err != nil {
		return completion, err
	}
	if err := validateCompletedTools(completion.Choices); err != nil {
		return completion, err
	}

	if len(completion.Choices) == 0 && completion.Body == nil {
		return completion, fmt.Errorf("response has no choices")
	}
	completion.Status = resp.StatusCode
	c.readSpecDecodePlacements(completion.Metrics.SpeculativeDecoding, completion.Choices)
	if c.measurement != nil {
		c.measurement.reported = completion.Usage
		if len(completion.Choices) > 1 {
			c.measurement.MultiChoice = true
			c.measurement.ChoicePolicy = "all synchronous choices; provider request-level usage"
		}

		c.measurement.estimatedOutput = c.estimateCompletionTokens(completion)
		for _, choice := range completion.Choices {
			if len(choice.Message.ToolCalls) > 0 || hasMedia(choice.Message.Content) {
				c.measurement.noOutputEstimate = true
			}
		}
		for _, choice := range completion.Choices {
			if messageText(choice.Message.Content) != "" || choice.Message.ReasoningContent != "" || len(choice.Message.ToolCalls) > 0 || hasMedia(choice.Message.Content) {
				c.measurement.last = time.Now()
				break
			}
		}
	}

	totalTime := time.Since(startTime)
	if c.measurement != nil {
		c.measurement.responseEnd = time.Now()
	}
	c.recordMetric(recordMetrics, "llm_completion_time", (float64(totalTime) / float64(time.Millisecond)))
	c.recordMetric(recordMetrics, "llm_request_latency", (float64(totalTime) / float64(time.Millisecond)))
	c.recordAdjustedMetric(recordMetrics, "llm_request_latency_adjusted", totalTime)
	c.recordMetric(recordMetrics, "llm_requests", 1)

	promptTokens := completion.Usage.PromptTokens
	if !completion.Usage.promptPresent {
		promptTokens = inputTokens
	}
	c.recordTokenMetrics(recordMetrics, promptTokens, c.completionTokens(completion), inputTokens, c.estimateCompletionTokens(completion), totalTime)

	return completion, nil
}

func newTransport() *http.Transport {
	return &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          1000,
		MaxIdleConnsPerHost:   100,
		MaxConnsPerHost:       0,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

func (c *Client) recordTokenMetrics(recordMetrics bool, promptTokens, outputTokens, estimatedPromptTokens, estimatedOutputTokens int, elapsed time.Duration) {
	if c.measurement != nil {
		in, out := c.resolveCounts(c.measurement)
		promptTokens, outputTokens = 0, 0
		if in.Value != nil {
			promptTokens = *in.Value
		}
		if out.Value != nil {
			outputTokens = *out.Value
		}
	}
	if promptTokens > 0 {
		c.recordMetric(recordMetrics, "llm_prompt_tokens", float64(promptTokens))
		c.recordMetric(recordMetrics, "llm_input_sequence_length", float64(promptTokens))
	}
	if outputTokens > 0 {
		c.recordMetric(recordMetrics, "llm_completion_tokens", float64(outputTokens))
		c.recordMetric(recordMetrics, "llm_output_sequence_length", float64(outputTokens))

		if seconds := elapsed.Seconds(); seconds > 0 {
			c.recordMetric(recordMetrics, "llm_tokens_per_second", float64(outputTokens)/seconds)
		}
	}
	if promptTokens+outputTokens > 0 {
		c.recordMetric(recordMetrics, "llm_total_tokens", float64(promptTokens+outputTokens))
	}
	if c.measurement == nil || c.measurement.reported.promptPresent {
		c.recordDiscrepancy(recordMetrics, "llm_prompt_token_discrepancy", promptTokens, estimatedPromptTokens)
	}
	if c.measurement == nil || c.measurement.reported.completionPresent {
		c.recordDiscrepancy(recordMetrics, "llm_completion_token_discrepancy", outputTokens, estimatedOutputTokens)
	}
}

func (c *Client) completionTokens(completion CompletionResponse) int {
	if completion.Usage.completionPresent {
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
		text.WriteString(messageText(choice.Message.Content))
		text.WriteString(choice.Message.ReasoningContent)
		text.WriteByte(' ')
	}
	return c.estimateTokenCount(text.String())
}

func (c *Client) estimateInputTokens(messages []Message) int {
	var text strings.Builder
	for _, msg := range messages {
		text.WriteString(messageText(msg.Content))
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
	TTFT time.Duration
	TTFO time.Duration
}

func (c *Client) takeWarmupSample() bool {
	for {
		remaining := atomic.LoadInt64(&c.warmupRemaining)
		if remaining <= 0 {
			return true
		}
		if atomic.CompareAndSwapInt64(&c.warmupRemaining, remaining, remaining-1) {
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
	c.recordMetric(record, name, float64(adjusted)/float64(time.Millisecond))
}

func maxDuration(a, b time.Duration) time.Duration {
	if b > a {
		return b
	}
	return a
}

func validateConfig(config Config) error {
	if _, err := configuredTraceContext(config); err != nil {
		return err
	}
	if config.MaxConnsPerHost < 0 || config.MaxConnsPerHost > 65536 || config.UseK6Transport && config.MaxConnsPerHost != 0 {
		return fmt.Errorf("maxConnsPerHost must be 0..65536 and excludes useK6Transport")
	}
	if len(config.ControlGroup) > 256 || config.ControlGroup != "" && (config.RequestConcurrency != 0 || config.PrefillConcurrency != 0) {
		return fmt.Errorf("controlGroup excludes static concurrency ceilings and is at most 256 bytes")
	}
	if len(config.BaseURLs) > 256 {
		return fmt.Errorf("too many endpoints")
	}
	endpoints := normalizedBaseURLs(config)
	if len(endpoints) == 0 {
		return fmt.Errorf("baseURL is required")
	}
	for i, endpoint := range endpoints {
		if len(endpoint) > 2048 {
			return fmt.Errorf("baseURL[%d] exceeds size limit", i)
		}
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
		if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return fmt.Errorf("baseURL[%d] must not contain credentials, query or fragment", i)
		}
	}
	if config.Timeout != "" {
		timeout, err := time.ParseDuration(config.Timeout)
		if err != nil {
			return fmt.Errorf("invalid timeout: %w", err)
		}
		if timeout <= 0 {
			return fmt.Errorf("timeout must be positive")
		}
	}
	if !validSLOLimit(config.MaxTTFT) || !validSLOLimit(config.MaxTTFO) || !validSLOLimit(config.MaxTokenLatency) || !validSLOLimit(config.MaxTPOT) || !validSLOLimit(config.MaxE2E) {
		return fmt.Errorf("latency limits must be finite and non-negative")
	}
	if config.NetworkRTT < 0 {
		return fmt.Errorf("networkRTT must be non-negative")
	}
	if config.Phase != "" && config.Phase != "warmup" && config.Phase != "measurement" && config.Phase != "cooldown" {
		return fmt.Errorf("phase must be warmup, measurement or cooldown")
	}
	if config.DiscrepancyTolerance != nil && *config.DiscrepancyTolerance < 0 {
		return fmt.Errorf("discrepancyTolerance must be non-negative")
	}
	if config.TokenMultiplier < 0 {
		return fmt.Errorf("tokenMultiplier must be non-negative")
	}
	if config.RequestConcurrency < 0 || config.RequestConcurrency > 100000 {
		return fmt.Errorf("requestConcurrency must be between 0 and 100000")
	}
	if config.PrefillConcurrency < 0 {
		return fmt.Errorf("prefillConcurrency must be non-negative")
	}
	if config.PrefillConcurrency > 100000 {
		return fmt.Errorf("prefillConcurrency exceeds 100000")
	}
	if len(config.RTTSource) > 256 {
		return fmt.Errorf("rttSource exceeds size limit")
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
	if err := validateWorkExpectation(req.Work); err != nil {
		return err
	}
	if len(req.Model) > 1024 {
		return fmt.Errorf("model exceeds size limit")
	}
	if req.Measurement != nil && len(req.Measurement.Tokenizer) > 256 {
		return fmt.Errorf("measurement.tokenizer exceeds size limit")
	}
	if req.ScheduledAt != nil && (!finite(*req.ScheduledAt) || *req.ScheduledAt < 0) {
		return fmt.Errorf("scheduledAt must be a non-negative finite Unix millisecond timestamp")
	}
	if req.CancelAfter != "" {
		d, err := time.ParseDuration(req.CancelAfter)
		if err != nil || d <= 0 {
			return fmt.Errorf("cancelAfter must be a positive duration")
		}
	}
	if req.Raw != nil {
		if req.Raw.Protocol == "providerwire-v4" {
			if strings.TrimSpace(req.Model) == "" {
				return fmt.Errorf("providerwire model is required")
			}
			for _, r := range req.Model {
				if r < 32 || r == 127 {
					return fmt.Errorf("invalid providerwire model header")
				}
			}
		}
		if req.Measurement != nil {
			if req.Measurement.Tokenizer == "" {
				return fmt.Errorf("measurement.tokenizer is required")
			}
			for _, n := range []*int{req.Measurement.InputTokens, req.Measurement.OutputTokens, req.Measurement.FirstChunkTokens} {
				if n != nil && *n < 0 {
					return fmt.Errorf("measurement counts must be non-negative")
				}
			}
		}
		if req.Extensions != nil {
			return fmt.Errorf("extensions require a typed request; put raw fields in body")
		}
		return validateRaw(req.Raw)
	}
	for key, value := range req.Extensions {
		if key == "" || namesTypedField(key) {
			return fmt.Errorf("extension field %q is empty or names a typed request field", key)
		}
		if value == nil {
			return fmt.Errorf("extension field %q is null or undefined; omit absent fields", key)
		}
	}
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
		if strings.TrimSpace(messageText(msg.Content)) == "" && len(msg.ToolCalls) == 0 && !hasMedia(msg.Content) {
			return fmt.Errorf("messages[%d].content is required", i)
		}
	}
	if req.Measurement != nil {
		if req.Measurement.Tokenizer == "" {
			return fmt.Errorf("measurement.tokenizer is required")
		}
		for _, n := range []*int{req.Measurement.InputTokens, req.Measurement.OutputTokens, req.Measurement.FirstChunkTokens} {
			if n != nil && *n < 0 {
				return fmt.Errorf("measurement counts must be non-negative")
			}
		}
	}
	if req.MaxCompletionTokens < 0 {
		return fmt.Errorf("max_completion_tokens must be non-negative")
	}
	if req.MaxTokens < 0 {
		return fmt.Errorf("max_tokens must be non-negative")
	}
	if req.Temperature != nil && *req.Temperature < 0 {
		return fmt.Errorf("temperature must be non-negative")
	}
	return nil
}

func (c *Client) addParsingTime(start time.Time) {
	if c.measurement == nil {
		return
	}
	if c.measurement.ParsingLatency == nil {
		c.measurement.ParsingLatency = new(float64)
	}
	*c.measurement.ParsingLatency += ms(time.Since(start))
}

// measurementBody counts consumed wire body bytes, including SSE framing.
type measurementBody struct {
	io.ReadCloser
	measurement *Measurement
}

func (b *measurementBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	b.measurement.ResponseBodyBytes += int64(n)
	return n, err
}
