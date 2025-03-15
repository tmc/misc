// Package gemini provides functionality for interacting with the Google Gemini API.
//
// It includes types and functions for constructing and handling requests
// and responses specific to the Gemini API endpoints, focusing on
// chat and language model interactions.
//
// This package aims to provide a clean and Go-idiomatic interface
// for interacting with the Gemini API within the ant-proxy.
//
// Example usage:
//
//	import "github.com/tmc/misc/ant-proxy/gemini"
//
//	func main() {
//		cfg := gemini.Config{
//			APIKey: "AIza...",
//			Model:  "gemini-pro",
//		}
//		client := gemini.NewClient(cfg)
//
//		req := gemini.ChatRequest{
//			Messages: []gemini.ChatMessage{
//				{
//					Role:    "user",
//					Content: "Hello, Gemini!",
//				},
//			},
//			MaxOutputTokens: 1024,
//		}
//
//		resp, err := client.SendMessage(context.Background(), req)
//		if err != nil {
//			log.Fatal(err)
//		}
//		fmt.Println(resp.Content)
//	}
package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

// Config holds the configuration parameters for the Gemini API client.
type Config struct {
	APIKey string
	Model  string
	// Add other Gemini specific config options if needed
}

// Client is a client for interacting with the Gemini API.
type Client struct {
	config Config
	client *http.Client // Consider using a more robust HTTP client if needed
}

// NewClient creates a new Gemini API client.
func NewClient(cfg Config) *Client {
	return &Client{
		config: cfg,
		client: &http.Client{},
	}
}

// ChatMessage represents a message in the Gemini API chat message structure.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest represents the request body for the Gemini chat endpoint.
type ChatRequest struct {
	Messages      []ChatMessage `json:"messages"`
	MaxOutputTokens int           `json:"maxOutputTokens"`
	// Add other relevant fields from the Gemini API request as needed
}

// ChatResponse represents the response body from the Gemini chat endpoint.
type ChatResponse struct {
	Content string `json:"content"` // Placeholder, adjust based on actual response structure
	// Add other relevant fields from the Gemini API response as needed
}

// Gemini API request structure
type GeminiRequest struct {
	Contents      []Content       `json:"contents"`
	GenerationConfig GenerationConfig `json:"generationConfig"`
}

type Content struct {
	Parts []Part `json:"parts"`
	Role string `json:"role"`
}

type Part struct {
	Text string `json:"text"`
}

type GenerationConfig struct {
	Temperature   float64 `json:"temperature"`
	TopK          int     `json:"topK"`
	TopP          float64 `json:"topP"`
	MaxOutputTokens int     `json:"maxOutputTokens"`
	ResponseMimeType string `json:"responseMimeType"`
}

// SendMessage sends a message to the Gemini API chat endpoint.
func (c *Client) SendMessage(ctx context.Context, req interface{}) (interface{}, error) {
	// Type assertion to convert the interface{} to ChatRequest
	chatReq, ok := req.(ChatRequest)
	if !ok {
		return nil, fmt.Errorf("invalid request type: expected ChatRequest, got %T", req)
	}

	// Construct the Gemini API URL
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", c.config.Model, c.config.APIKey)

	// Construct the Gemini request body
	geminiRequest := GeminiRequest{
		Contents: []Content{},
		GenerationConfig: GenerationConfig{
			Temperature:   1.0,
			TopK:          64,
			TopP:          0.95,
			MaxOutputTokens: chatReq.MaxOutputTokens,
			ResponseMimeType: "text/plain",
		},
	}

	for _, msg := range chatReq.Messages {
		geminiRequest.Contents = append(geminiRequest.Contents, Content{
			Parts: []Part{{Text: msg.Content}},
			Role:    msg.Role,
		})
	}

	// Marshal the request body to JSON
	requestBody, err := json.Marshal(geminiRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Create a new HTTP request
	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

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
		return nil, fmt.Errorf("API returned unexpected status code: %d, body: %s", httpResp.StatusCode, string(responseBody))
	}

	// Unmarshal the response body
	var geminiResponse map[string]interface{}
	if err := json.Unmarshal(responseBody, &geminiResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response body: %w", err)
	}

	log.Printf("Gemini API Response: %+v\n", geminiResponse)

	// Extract the content from the response
	content, ok := geminiResponse["candidates"].([]interface{})[0].(map[string]interface{})["content"].(map[string]interface{})["parts"].([]interface{})[0].(map[string]interface{})["text"].(string)
	if !ok {
		return nil, fmt.Errorf("failed to extract content from response")
	}

	// Simulate a response
	resp := &ChatResponse{
		Content: content,
	}

	// Convert the response to interface{}
	return interface{}(resp), nil
}

func (c *Client) String() string {
	b, err := json.Marshal(c)
	if err != nil {
		return fmt.Sprintf("Gemini Client marshal err: %v", err)
	}
	return string(b)
}
