package openai

import (
    "context"
    "fmt"

    "github.com/openai/openai-go"
    "github.com/openai/openai-go/option"
    "github.com/tmc/misc/xk6-llm-perf"
)

func init() {
    llmperf.RegisterProvider("openai", NewProvider)
}

type Provider struct {
    client *openai.Client
    config Config
}

type Config struct {
    BaseURL     string  `json:"baseURL"`
    Model       string  `json:"model"`
    APIKey      string  `json:"apiKey"`
    Temperature float64 `json:"temperature"`
    MaxTokens   int     `json:"maxTokens"`
}

func NewProvider(rawConfig map[string]interface{}) (llmperf.LLMProvider, error) {
    var config Config
    // ... implement config parsing ...

    opts := []option.RequestOption{
        option.WithBaseURL(config.BaseURL),
    }
    if config.APIKey != "" {
        opts = append(opts, option.WithHeader("Authorization", "Bearer "+config.APIKey))
    }

    return &Provider{
        client: openai.NewClient(opts...),
        config: config,
    }, nil
}

