package llm

import (
    "bufio"
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"
)

type ClientConfig struct {
    BaseURL     string
    Storage     StorageConfig
    HTTPTimeout time.Duration
    IsStreaming bool
    Metrics     *LLMMetrics
}

type Client struct {
    config     ClientConfig
    httpClient *http.Client
    metrics    *LLMMetrics
    storage    *Storage
}

func NewClient(config ClientConfig) (*Client, error) {
    if config.Metrics == nil {
        return nil, fmt.Errorf("metrics required")
    }

    return &Client{
        config: config,
        httpClient: &http.Client{
            Timeout: config.HTTPTimeout,
        },
        metrics: config.Metrics,
        storage: NewStorage(config.Storage),
    }, nil
}

func (c *Client) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
    startTime := time.Now()

    // Prepare request body
    jsonData, err := json.Marshal(req)
    if err != nil {
        c.metrics.RecordError()
        return nil, fmt.Errorf("marshal request: %w", err)
    }

    // Create HTTP request
    httpReq, err := http.NewRequestWithContext(ctx, "POST",
        fmt.Sprintf("%s/chat/completions", c.config.BaseURL),
        bytes.NewReader(jsonData))
    if err != nil {
        c.metrics.RecordError()
        return nil, fmt.Errorf("create request: %w", err)
    }

    httpReq.Header.Set("Content-Type", "application/json")
    httpReq.Header.Set("Accept", "application/json")
    if c.config.IsStreaming {
        httpReq.Header.Set("Accept", "text/event-stream")
    }

    // Execute request
    resp, err := c.httpClient.Do(httpReq)
    if err != nil {
        c.metrics.RecordError()
        return nil, fmt.Errorf("do request: %w", err)
    }
    defer resp.Body.Close()

    // Check response status
    if resp.StatusCode != http.StatusOK {
        c.metrics.RecordError()
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
    }

    // Record request duration
    c.metrics.RecordLatency(time.Since(startTime), 0, false)

    // Handle response based on streaming mode
    if c.config.IsStreaming {
        return c.handleStreamingResponse(resp, startTime)
    }
    return c.handleSyncResponse(resp, startTime)
}

func (c *Client) handleStreamingResponse(resp *http.Response, startTime time.Time) (*CompletionResponse, error) {
    reader := bufio.NewReader(resp.Body)
    response := &CompletionResponse{
        Status: resp.StatusCode,
    }

    var firstChunk bool = true
    var lastChunkTime time.Time
    var totalTokens int
    var buffer bytes.Buffer

    for {
        line, err := reader.ReadBytes('\n')
        if err == io.EOF {
            break
        }
        if err != nil {
            c.metrics.RecordError()
            return nil, fmt.Errorf("read stream: %w", err)
        }

        // Skip empty lines
        if len(bytes.TrimSpace(line)) == 0 {
            continue
        }

        // Parse SSE data
        if !bytes.HasPrefix(line, []byte("data: ")) {
            continue
        }
        data := bytes.TrimPrefix(line, []byte("data: "))

        // Handle "[DONE]" message
        if bytes.Equal(bytes.TrimSpace(data), []byte("[DONE]")) {
            break
        }

        // Parse chunk
        var chunk ChatCompletionChunk
        if err := json.Unmarshal(data, &chunk); err != nil {
            continue
        }

        now := time.Now()
        if firstChunk {
            c.metrics.RecordTimeToFirstToken(now.Sub(startTime))
            firstChunk = false
            lastChunkTime = now
        } else if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
            tokenCount := len(chunk.Choices[0].Delta.Content)
            if tokenCount > 0 {
                c.metrics.RecordLatency(now.Sub(lastChunkTime), tokenCount, true)
                totalTokens += tokenCount
                lastChunkTime = now
            }
        }

        // Append content
        if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
            buffer.WriteString(chunk.Choices[0].Delta.Content)
        }
    }

    response.Text = buffer.String()

    // Calculate and record final token rate
    totalTime := time.Since(startTime)
    if totalTokens > 0 {
        tokensPerSecond := float64(totalTokens) / totalTime.Seconds()
        c.metrics.RecordTokenRate(tokensPerSecond, true)
    }

    return response, nil
}

func (c *Client) handleSyncResponse(resp *http.Response, startTime time.Time) (*CompletionResponse, error) {
    var completion ChatCompletion
    if err := json.NewDecoder(resp.Body).Decode(&completion); err != nil {
        c.metrics.RecordError()
        return nil, fmt.Errorf("decode response: %w", err)
    }

    response := &CompletionResponse{
        Status: resp.StatusCode,
        Text:   completion.Choices[0].Message.Content,
    }

    totalTokens := completion.Usage.CompletionTokens
    totalTime := time.Since(startTime)

    // Record total completion time and tokens
    c.metrics.RecordLatency(totalTime, totalTokens, false)

    // Calculate and record token rate
    if totalTokens > 0 {
        tokensPerSecond := float64(totalTokens) / totalTime.Seconds()
        c.metrics.RecordTokenRate(tokensPerSecond, false)
    }

    return response, nil
}

