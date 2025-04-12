// Package anthropic provides functionality for interacting with the Anthropic API.
//
// It includes types and functions for constructing and handling requests
// and responses specific to the Anthropic API endpoints, particularly
// the /v1/messages endpoint used for chat-like interactions.
//
// This package aims to abstract away the details of the Anthropic API,
// providing a Go-idiomatic interface for use within the ant-proxy.
//
// Example usage:
//
//	import "github.com/tmc/misc/ant-proxy/anthropic"
//
//	func main() {
//		cfg := anthropic.Config{
//			APIKey: "sk-ant-...",
//			APIVersion: "2023-06-01",
//			Model: "claude-3-sonnet-20250219",
//		}
//		client := anthropic.NewClient(cfg)
//
//		req := anthropic.MessageRequest{
//			Messages: []anthropic.Message{
//				{
//					Role:    "user",
//					Content: "Hello, Claude!",
//				},
//			},
//			MaxTokens: 1024,
//		}
//
//		resp, err := client.SendMessage(context.Background(), req)
//		if err != nil {
//			log.Fatal(err)
//		}
//		fmt.Println(resp.Content)
//	}
package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

// Config holds the configuration parameters for the Anthropic API client.
type Config struct {
	APIKey     string
	APIVersion string
	Model      string
}

// Client is a client for interacting with the Anthropic API.
type Client struct {
	config Config
	client *http.Client // Consider using a more robust HTTP client if needed
}

// NewClient creates a new Anthropic API client.
func NewClient(cfg Config) *Client {
	return &Client{
		config: cfg,
		client: &http.Client{},
	}
}

// Message represents a message in the Anthropic API message structure.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// MessageRequest represents the request body for the /v1/messages endpoint.
type MessageRequest struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	Messages  []Message `json:"messages"`
	// Add other relevant fields from the Anthropic API request as needed
}

// MessageResponse represents the response body from the /v1/messages endpoint.
type MessageResponse struct {
	ID           string `json:"id"`
	Type         string `json:"type"`
	Role         string `json:"role"`
	Content      string `json:"content"`
	Model        string `json:"model"`
	StopReason   string `json:"stop_reason,omitempty"`
	StopSequence string `json:"stop_sequence,omitempty"`
	Usage        struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage,omitempty"`
}

// SendMessage sends a message to the Anthropic API /v1/messages endpoint.
func (c *Client) SendMessage(ctx context.Context, req interface{}) (interface{}, error) {
	// Type assertion to convert the interface{} to MessageRequest
	messageReq, ok := req.(MessageRequest)
	if !ok {
		return nil, fmt.Errorf("invalid request type: expected MessageRequest, got %T", req)
	}

	// Set the model if not already set
	if messageReq.Model == "" {
		messageReq.Model = c.config.Model
	}

	// Construct API URL
	url := "https://api.anthropic.com/v1/messages"

	// Marshal the request body
	requestBody, err := json.Marshal(messageReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Create a new HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.config.APIKey)
	httpReq.Header.Set("anthropic-version", c.config.APIVersion)

	log.Printf("Anthropic API Request: %+v\n", messageReq)

	// Send the request
	httpResp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send HTTP request: %w", err)
	}
	defer httpResp.Body.Close()

	// Read the response body
	responseBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check the response status code
	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned unexpected status code: %d, body: %s", 
			httpResp.StatusCode, string(responseBody))
	}

	// Unmarshal the response body
	var anthropicResponse MessageResponse
	if err := json.Unmarshal(responseBody, &anthropicResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response body: %w", err)
	}

	log.Printf("Anthropic API Response: %+v\n", anthropicResponse)

	// Convert the response to interface{}
	return interface{}(&anthropicResponse), nil
}

func (c *Client) String() string {
	b, err := json.Marshal(c)
	if err != nil {
		return fmt.Sprintf("Anthropic Client marshal err: %v", err)
	}
	return string(b)
}
