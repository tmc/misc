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
	"context"
	"encoding/json"
	"fmt"
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
	Content string `json:"content"` // Placeholder, adjust based on actual response structure
	// Add other relevant fields from the Anthropic API response as needed
}

// SendMessage sends a message to the Anthropic API /v1/messages endpoint.
func (c *Client) SendMessage(ctx context.Context, req interface{}) (interface{}, error) {
	// Type assertion to convert the interface{} to MessageRequest
	messageReq, ok := req.(MessageRequest)
	if !ok {
		return nil, fmt.Errorf("invalid request type: expected MessageRequest, got %T", req)
	}

	// TODO: Implement the actual HTTP request to the Anthropic API
	// using c.config and c.client.
	// This is a placeholder implementation.

	log.Printf("Anthropic API Request: %+v\n", messageReq) // For demonstration

	// Simulate a response
	resp := &MessageResponse{
		Content: "This is a simulated Anthropic response.",
	}

	// Convert the response to interface{}
	return interface{}(resp), nil
}

func (c *Client) String() string {
	b, err := json.Marshal(c)
	if err != nil {
		return fmt.Sprintf("Anthropic Client marshal err: %v", err)
	}
	return string(b)
}
