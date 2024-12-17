package llm

import (
    "context"
    "errors"
    "time"

    "go.k6.io/k6/js/modules"
    "go.k6.io/k6/metrics"
)

type LLMMetrics struct {
    // Streaming-specific metrics
    TTFT_Stream         *metrics.Metric // Time to first token (streaming only)
    TokenLatency_Stream *metrics.Metric // Inter-token latency (streaming only)
    TokensPerSec_Stream *metrics.Metric // Token generation rate (streaming)

    // Non-streaming metrics
    TotalLatency_Sync   *metrics.Metric // Total completion time
    TokensPerSec_Sync   *metrics.Metric // Overall token rate

    // Common metrics
    TotalTokens        *metrics.Metric // Total tokens generated
    RequestDuration    *metrics.Metric // Overall request duration
    RequestErrors      *metrics.Metric // Error counter

    registry          *metrics.Registry
}

func registerMetrics(vu modules.VU) (*LLMMetrics, error) {
    registry := vu.InitEnv().Registry
    m := &LLMMetrics{registry: registry}

    var err error

    // Streaming-specific metrics
    if m.TTFT_Stream, err = registry.NewMetric(
        "llm_ttft", metrics.Trend, metrics.Time); err != nil {
        return nil, errors.Unwrap(err)
    }

    if m.TokenLatency_Stream, err = registry.NewMetric(
        "llm_token_latency", metrics.Trend, metrics.Time); err != nil {
        return nil, errors.Unwrap(err)
    }

    if m.TokensPerSec_Stream, err = registry.NewMetric(
        "llm_tokens_per_second_stream", metrics.Gauge); err != nil {
        return nil, errors.Unwrap(err)
    }

    // Non-streaming metrics
    if m.TotalLatency_Sync, err = registry.NewMetric(
        "llm_completion_time", metrics.Trend, metrics.Time); err != nil {
        return nil, errors.Unwrap(err)
    }

    if m.TokensPerSec_Sync, err = registry.NewMetric(
        "llm_tokens_per_second_sync", metrics.Gauge); err != nil {
        return nil, errors.Unwrap(err)
    }

    // Common metrics
    if m.TotalTokens, err = registry.NewMetric(
        "llm_total_tokens", metrics.Counter); err != nil {
        return nil, errors.Unwrap(err)
    }

    if m.RequestDuration, err = registry.NewMetric(
        "llm_request_duration", metrics.Trend, metrics.Time); err != nil {
        return nil, errors.Unwrap(err)
    }

    if m.RequestErrors, err = registry.NewMetric(
        "llm_errors", metrics.Counter); err != nil {
        return nil, errors.Unwrap(err)
    }

    return m, nil
}

// Only record TTFT for streaming responses
func (m *LLMMetrics) RecordTimeToFirstToken(d time.Duration) {
    sample := metrics.Sample{
        TimeSeries: metrics.TimeSeries{
            Metric: m.TTFT_Stream,
        },
        Time: time.Now(),
        Value: float64(d.Milliseconds()),
    }
    metrics.PushIfNotDone(context.Background(), m.registry.Samples, metrics.Samples{sample})
}

// Record latency differently for streaming vs non-streaming
func (m *LLMMetrics) RecordLatency(d time.Duration, tokens int, isStreaming bool) {
    var samples []metrics.Sample

    if isStreaming {
        // For streaming, record per-token latency
        tokenLatency := float64(d.Milliseconds()) / float64(tokens)
        samples = append(samples, metrics.Sample{
            TimeSeries: metrics.TimeSeries{
                Metric: m.TokenLatency_Stream,
            },
            Time: time.Now(),
            Value: tokenLatency,
        })
    } else {
        // For non-streaming, record total completion time
        samples = append(samples, metrics.Sample{
            TimeSeries: metrics.TimeSeries{
                Metric: m.TotalLatency_Sync,
            },
            Time: time.Now(),
            Value: float64(d.Milliseconds()),
        })
    }

    // Record total tokens for both cases
    samples = append(samples, metrics.Sample{
        TimeSeries: metrics.TimeSeries{
            Metric: m.TotalTokens,
        },
        Time: time.Now(),
        Value: float64(tokens),
    })

    metrics.PushIfNotDone(context.Background(), m.registry.Samples, metrics.Samples(samples))
}

func (m *LLMMetrics) RecordTokenRate(tokensPerSecond float64, isStreaming bool) {
    metric := m.TokensPerSec_Stream
    if !isStreaming {
        metric = m.TokensPerSec_Sync
    }

    sample := metrics.Sample{
        TimeSeries: metrics.TimeSeries{
            Metric: metric,
        },
        Time: time.Now(),
        Value: tokensPerSecond,
    }
    metrics.PushIfNotDone(context.Background(), m.registry.Samples, metrics.Samples{sample})
}

func (m *LLMMetrics) RecordError() {
    sample := metrics.Sample{
        TimeSeries: metrics.TimeSeries{
            Metric: m.RequestErrors,
        },
        Time: time.Now(),
        Value: 1,
    }
    metrics.PushIfNotDone(context.Background(), m.registry.Samples, metrics.Samples{sample})
}

