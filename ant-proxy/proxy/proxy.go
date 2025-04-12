// Package proxy implements the core proxy logic for ant-proxy.
//
// It handles routing incoming requests, transforming them as needed,
// dispatching them to different backend providers (like Anthropic or Gemini),
// and then handling and transforming the responses back to the client.
//
// The proxy package is designed to be extensible, allowing for the addition
// of new backend providers and transformation logic.
//
// Key components include:
//
//   - Request routing: Determines which backend provider to use based on configuration or request analysis.
//   - Request transformation: Adapts incoming requests to the format expected by the backend provider.
//   - Provider dispatch: Sends the transformed request to the selected backend provider.
//   - Response handling: Receives responses from the backend provider.
//   - Response transformation: Adapts backend responses back to the client.
//
// Example usage (conceptual - requires further implementation):
//
//	import (
//		"github.com/tmc/misc/ant-proxy/anthropic"
//		"github.com/tmc/misc/ant-proxy/gemini"
//		"github.com/tmc/misc/ant-proxy/proxy"
//	)
//
//	func main() {
//		anthropicClient := anthropic.NewClient(anthropic.Config{APIKey: "..."})
//		geminiClient := gemini.NewClient(gemini.Config{APIKey: "..."})
//
//		p := proxy.NewProxy(
//			proxy.WithProvider("anthropic", anthropicClient),
//			proxy.WithProvider("gemini", geminiClient),
//			// ... other options ...
//		)
//
//		// ... handle incoming request and use proxy to process it ...
//		// response, err := p.HandleRequest(ctx, incomingRequest)
//		// ...
//	}
package proxy

import (
	"context"
	"fmt"
	"log"

	"github.com/tmc/misc/ant-proxy/anthropic"
	"github.com/tmc/misc/ant-proxy/gemini"
)

// ProviderClient is an interface that defines the methods a backend provider client must implement.
// For now, it's a placeholder.  It should be expanded to be more generic
// to handle different provider APIs.
type ProviderClient interface {
	SendMessage(context.Context, interface{}) (interface{}, error) // Generic interface for now
}

// ExtractorPattern represents a pattern for extracting data from a stream
type ExtractorPattern struct {
	Conditions  []Condition
	ExtractPath string
}

// Condition represents a condition for matching in an extractor pattern
type Condition struct {
	Path  string
	Value string
}

// Proxy is the core proxy type.
type Proxy struct {
	providers       map[string]ProviderClient
	recordSessions  bool
	sessionsDir     string
	defaultProvider string
}

// NewProxy creates a new Proxy instance.
// It accepts functional options for configuration.
func NewProxy(opts ...Option) *Proxy {
	p := &Proxy{
		providers: make(map[string]ProviderClient),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Option is a functional option for configuring the Proxy.
type Option func(*Proxy)

// WithProvider adds a backend provider to the proxy.
func WithProvider(name string, client ProviderClient) Option {
	return func(p *Proxy) {
		p.providers[name] = client
	}
}

// transformAnthropicToGemini transforms an Anthropic MessageRequest to a Gemini ChatRequest.
func transformAnthropicToGemini(anthropicReq anthropic.MessageRequest) gemini.ChatRequest {
	chatMessages := make([]gemini.ChatMessage, len(anthropicReq.Messages))
	for i, msg := range anthropicReq.Messages {
		chatMessages[i] = gemini.ChatMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	return gemini.ChatRequest{
		Messages:        chatMessages,
		MaxOutputTokens: anthropicReq.MaxTokens,
	}
}

// transformGeminiToAnthropic transforms a Gemini ChatResponse to an Anthropic MessageResponse.
func transformGeminiToAnthropic(geminiResp *gemini.ChatResponse) *anthropic.MessageResponse {
	if geminiResp == nil {
		return &anthropic.MessageResponse{} // Return empty response if geminiResp is nil
	}
	return &anthropic.MessageResponse{
		Content: geminiResp.Content,
	}
}

// NewExtractorPattern creates a new ExtractorPattern
func NewExtractorPattern(extractPath string, conditions ...Condition) ExtractorPattern {
	return ExtractorPattern{
		ExtractPath: extractPath,
		Conditions:  conditions,
	}
}

// NewCondition creates a new Condition
func NewCondition(path string, value string) Condition {
	return Condition{
		Path:  path,
		Value: value,
	}
}

// HandleRequest processes an incoming request.
func (p *Proxy) HandleRequest(ctx context.Context, request interface{}) (interface{}, error) {
	// TODO: Implement request routing logic to select a provider.
	// TODO: Implement request transformation if needed.
	// TODO: Dispatch request to the selected provider using p.providers.
	// TODO: Handle and transform the response.

	log.Printf("Proxy received request: %+v\n", request) // For demonstration

	// For now, just use a default provider (e.g., "gemini" if available)
	var providerName string
	if _, ok := p.providers["gemini"]; ok {
		providerName = "gemini"
	} else if len(p.providers) > 0 {
		for name := range p.providers {
			providerName = name
			break // Just pick the first one if gemini is not available
		}
	} else {
		return nil, fmt.Errorf("no providers configured")
	}

	provider, ok := p.providers[providerName]
	if !ok {
		return nil, fmt.Errorf("provider '%s' not found", providerName)
	}

	log.Printf("Proxy routing request to provider: %s\n", providerName)

	// Type assertion and provider-specific handling
	switch providerName {
	case "anthropic":
		// Assuming the request is an Anthropic MessageRequest
		anthropicReq, ok := request.(anthropic.MessageRequest)
		if !ok {
			return nil, fmt.Errorf("invalid request type: expected anthropic.MessageRequest, got %T", request)
		}

		anthropicResp, err := provider.SendMessage(ctx, anthropicReq)
		if err != nil {
			return nil, fmt.Errorf("anthropic provider error: %w", err)
		}

		// Type assert the response from interface{} to anthropic.MessageResponse
		resp, ok := anthropicResp.(interface{})
		if !ok {
			return nil, fmt.Errorf("unexpected response type from anthropic: %T", anthropicResp)
		}
		return resp, nil

	case "gemini":
		// Assuming the request is an Anthropic MessageRequest, transform it to Gemini ChatRequest
		anthropicReq, ok := request.(anthropic.MessageRequest)
		if !ok {
			return nil, fmt.Errorf("invalid request type: expected anthropic.MessageRequest, got %T", request)
		}

		geminiReq := transformAnthropicToGemini(anthropicReq)
		geminiResp, err := provider.SendMessage(ctx, geminiReq)
		if err != nil {
			return nil, fmt.Errorf("gemini provider error: %w", err)
		}

		// Type assert the response from interface{} to gemini.ChatResponse
		resp, ok := geminiResp.(interface{})
		if !ok {
			return nil, fmt.Errorf("unexpected response type from gemini: %T", geminiResp)
		}

		// Transform Gemini response back to Anthropic response
		anthropicResponse := transformGeminiToAnthropic(resp.(*gemini.ChatResponse)) // Type assert here
		log.Printf("Transformed Gemini response: %+v", anthropicResponse)            // Added logging
		return anthropicResponse, nil

	default:
		return nil, fmt.Errorf("unsupported provider: %s", providerName)
	}
}
