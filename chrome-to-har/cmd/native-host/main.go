// Native messaging host for Chrome AI extension
package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand"
	"os"
	"sync"
	"time"
)

// Message represents a native messaging message
type Message struct {
	Type  string      `json:"type"`
	Data  interface{} `json:"data,omitempty"`
	ID    string      `json:"id,omitempty"`
	Error string      `json:"error,omitempty"`
}

// AIRequest represents an AI operation request
type AIRequest struct {
	Action  string            `json:"action"`
	Prompt  string            `json:"prompt,omitempty"`
	Options map[string]string `json:"options,omitempty"`
}

// AIResponse represents an AI operation response
type AIResponse struct {
	Success  bool   `json:"success"`
	Response string `json:"response,omitempty"`
	API      string `json:"api,omitempty"`
	Error    string `json:"error,omitempty"`
	Retries  int    `json:"retries,omitempty"`
}

// RetryConfig holds retry configuration
type RetryConfig struct {
	MaxRetries    int
	BaseDelay     time.Duration
	MaxDelay      time.Duration
	Multiplier    float64
	JitterEnabled bool
}

// DefaultRetryConfig returns default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:    3,
		BaseDelay:     100 * time.Millisecond,
		MaxDelay:      5 * time.Second,
		Multiplier:    2.0,
		JitterEnabled: true,
	}
}

// MessageProcessor handles message processing with retry logic
type MessageProcessor struct {
	retryConfig RetryConfig
	mu          sync.RWMutex
	stats       map[string]int // Track retry counts per message type
}

// NewMessageProcessor creates a new message processor
func NewMessageProcessor() *MessageProcessor {
	return &MessageProcessor{
		retryConfig: DefaultRetryConfig(),
		stats:       make(map[string]int),
	}
}

// CalculateDelay calculates delay with exponential backoff and jitter
func (mp *MessageProcessor) CalculateDelay(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}

	delay := float64(mp.retryConfig.BaseDelay) * math.Pow(mp.retryConfig.Multiplier, float64(attempt-1))

	if delay > float64(mp.retryConfig.MaxDelay) {
		delay = float64(mp.retryConfig.MaxDelay)
	}

	// Add jitter to prevent thundering herd
	if mp.retryConfig.JitterEnabled {
		jitter := delay * 0.1 * (0.5 - rand.Float64()) // ±10% jitter
		delay += jitter
	}

	return time.Duration(delay)
}

// ProcessWithRetry processes a message with retry logic
func (mp *MessageProcessor) ProcessWithRetry(message Message, processFn func(Message) (Message, error)) Message {
	var lastErr error
	var response Message

	for attempt := 0; attempt <= mp.retryConfig.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := mp.CalculateDelay(attempt)
			log.Printf("Retrying message %s (attempt %d/%d) after %v", message.ID, attempt+1, mp.retryConfig.MaxRetries+1, delay)
			time.Sleep(delay)
		}

		response, lastErr = processFn(message)
		if lastErr == nil {
			// Success
			if attempt > 0 {
				log.Printf("Message %s succeeded after %d retries", message.ID, attempt)
				mp.updateStats(message.Type, attempt)
			}
			return response
		}

		log.Printf("Message %s failed (attempt %d/%d): %v", message.ID, attempt+1, mp.retryConfig.MaxRetries+1, lastErr)
	}

	// All retries failed
	log.Printf("Message %s failed after all retries: %v", message.ID, lastErr)
	mp.updateStats(message.Type, mp.retryConfig.MaxRetries+1)

	return Message{
		Type:  "error",
		ID:    message.ID,
		Error: fmt.Sprintf("Failed after %d retries: %v", mp.retryConfig.MaxRetries, lastErr),
	}
}

// UpdateStats updates retry statistics
func (mp *MessageProcessor) updateStats(messageType string, retries int) {
	mp.mu.Lock()
	defer mp.mu.Unlock()
	mp.stats[messageType] = retries
}

// GetStats returns current retry statistics
func (mp *MessageProcessor) GetStats() map[string]int {
	mp.mu.RLock()
	defer mp.mu.RUnlock()
	stats := make(map[string]int)
	for k, v := range mp.stats {
		stats[k] = v
	}
	return stats
}

func main() {
	// Set up logging to a file (since stdout is used for messaging)
	logFile, err := os.OpenFile("/tmp/chrome-ai-native-host.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err == nil {
		log.SetOutput(logFile)
		defer logFile.Close()
	}

	log.Println("Native messaging host started")

	// Initialize message processor with retry logic
	processor := NewMessageProcessor()

	// Main message loop with enhanced error handling
	for {
		message, err := readMessageWithRetry()
		if err != nil {
			if err == io.EOF {
				log.Println("Extension disconnected")
				break
			}
			log.Printf("Error reading message: %v", err)
			continue
		}

		log.Printf("Received message: %+v", message)

		// Process message with retry logic
		response := processor.ProcessWithRetry(message, func(msg Message) (Message, error) {
			result := handleMessage(msg)
			if result.Error != "" {
				return result, fmt.Errorf("%s", result.Error)
			}
			return result, nil
		})

		if err := writeMessageWithRetry(response); err != nil {
			log.Printf("Error writing response after retries: %v", err)
			break
		}

		log.Printf("Sent response: %+v", response)
	}

	// Log final statistics
	stats := processor.GetStats()
	log.Printf("Final retry statistics: %+v", stats)
	log.Println("Native messaging host ended")
}

// readMessage reads a message from stdin using Chrome's native messaging protocol
func readMessage() (Message, error) {
	var message Message

	// Read the 4-byte length header
	var length uint32
	if err := binary.Read(os.Stdin, binary.LittleEndian, &length); err != nil {
		return message, err
	}

	// Validate message length
	if length > 1024*1024 { // 1MB limit
		return message, fmt.Errorf("message too large: %d bytes", length)
	}

	// Read the JSON message
	messageBytes := make([]byte, length)
	if _, err := io.ReadFull(os.Stdin, messageBytes); err != nil {
		return message, err
	}

	// Parse JSON
	if err := json.Unmarshal(messageBytes, &message); err != nil {
		return message, err
	}

	return message, nil
}

// readMessageWithRetry reads a message with retry logic
func readMessageWithRetry() (Message, error) {
	var message Message
	var err error

	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			delay := time.Duration(100*attempt) * time.Millisecond
			log.Printf("Retrying message read (attempt %d) after %v", attempt+1, delay)
			time.Sleep(delay)
		}

		message, err = readMessage()
		if err == nil {
			return message, nil
		}

		if err == io.EOF {
			// Don't retry on EOF
			return message, err
		}

		log.Printf("Message read failed (attempt %d): %v", attempt+1, err)
	}

	return message, fmt.Errorf("failed to read message after retries: %v", err)
}

// writeMessage writes a message to stdout using Chrome's native messaging protocol
func writeMessage(message Message) error {
	// Marshal to JSON
	messageBytes, err := json.Marshal(message)
	if err != nil {
		return err
	}

	// Write length header
	length := uint32(len(messageBytes))
	if err := binary.Write(os.Stdout, binary.LittleEndian, length); err != nil {
		return err
	}

	// Write message
	if _, err := os.Stdout.Write(messageBytes); err != nil {
		return err
	}

	return nil
}

// writeMessageWithRetry writes a message with retry logic
func writeMessageWithRetry(message Message) error {
	var err error

	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			delay := time.Duration(100*attempt) * time.Millisecond
			log.Printf("Retrying message write (attempt %d) after %v", attempt+1, delay)
			time.Sleep(delay)
		}

		err = writeMessage(message)
		if err == nil {
			return nil
		}

		log.Printf("Message write failed (attempt %d): %v", attempt+1, err)
	}

	return fmt.Errorf("failed to write message after retries: %v", err)
}

// handleMessage processes incoming messages and returns appropriate responses
func handleMessage(message Message) Message {
	switch message.Type {
	case "ping":
		return Message{
			Type: "pong",
			ID:   message.ID,
			Data: map[string]interface{}{
				"timestamp": time.Now().Unix(),
				"status":    "ready",
			},
		}

	case "ai_request":
		return handleAIRequest(message)

	case "status":
		return Message{
			Type: "status_response",
			ID:   message.ID,
			Data: map[string]interface{}{
				"status":     "running",
				"version":    "1.0.0",
				"timestamp":  time.Now().Unix(),
				"capability": "ai_proxy",
			},
		}

	default:
		return Message{
			Type:  "error",
			ID:    message.ID,
			Error: fmt.Sprintf("Unknown message type: %s", message.Type),
		}
	}
}

// handleAIRequest processes AI-related requests with enhanced error handling
func handleAIRequest(message Message) Message {
	// Parse the AI request
	var request AIRequest
	if requestData, ok := message.Data.(map[string]interface{}); ok {
		// Convert map to JSON and back to struct
		requestBytes, err := json.Marshal(requestData)
		if err != nil {
			return Message{
				Type:  "ai_response",
				ID:    message.ID,
				Error: fmt.Sprintf("Failed to marshal request: %v", err),
			}
		}

		if err := json.Unmarshal(requestBytes, &request); err != nil {
			return Message{
				Type:  "ai_response",
				ID:    message.ID,
				Error: fmt.Sprintf("Failed to unmarshal request: %v", err),
			}
		}
	} else {
		return Message{
			Type:  "ai_response",
			ID:    message.ID,
			Error: "Invalid AI request format",
		}
	}

	log.Printf("Processing AI request: %+v", request)

	// Validate request
	if request.Action == "" {
		return Message{
			Type:  "ai_response",
			ID:    message.ID,
			Error: "Missing action in AI request",
		}
	}

	// For now, simulate AI processing
	// In a real implementation, this would:
	// 1. Validate the request
	// 2. Call the actual AI service
	// 3. Return the response

	response := AIResponse{
		Success:  true,
		Response: fmt.Sprintf("Simulated AI response to: %s", request.Prompt),
		API:      "native_simulation",
	}

	// Simulate processing delay with some variability
	processingTime := 500 * time.Millisecond
	if len(request.Prompt) > 100 {
		processingTime = 1000 * time.Millisecond // Longer prompts take more time
	}
	time.Sleep(processingTime)

	return Message{
		Type: "ai_response",
		ID:   message.ID,
		Data: response,
	}
}

// TODO: Implement actual AI proxy functionality
// This would involve:
// 1. Managing Chrome instances
// 2. Executing JavaScript to call AI APIs
// 3. Bridging responses back to the extension
// 4. Error handling and retry logic
