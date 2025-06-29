// Native messaging host for Chrome AI extension
package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"time"
)

// Message represents a native messaging message
type Message struct {
	Type   string      `json:"type"`
	Data   interface{} `json:"data,omitempty"`
	ID     string      `json:"id,omitempty"`
	Error  string      `json:"error,omitempty"`
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
}

func main() {
	// Set up logging to a file (since stdout is used for messaging)
	logFile, err := os.OpenFile("/tmp/chrome-ai-native-host.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err == nil {
		log.SetOutput(logFile)
		defer logFile.Close()
	}

	log.Println("Native messaging host started")

	// Main message loop
	for {
		message, err := readMessage()
		if err != nil {
			if err == io.EOF {
				log.Println("Extension disconnected")
				break
			}
			log.Printf("Error reading message: %v", err)
			continue
		}

		log.Printf("Received message: %+v", message)

		response := handleMessage(message)
		
		if err := writeMessage(response); err != nil {
			log.Printf("Error writing response: %v", err)
			break
		}

		log.Printf("Sent response: %+v", response)
	}

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

// handleAIRequest processes AI-related requests
func handleAIRequest(message Message) Message {
	// Parse the AI request
	var request AIRequest
	if requestData, ok := message.Data.(map[string]interface{}); ok {
		// Convert map to JSON and back to struct
		requestBytes, _ := json.Marshal(requestData)
		json.Unmarshal(requestBytes, &request)
	} else {
		return Message{
			Type:  "ai_response",
			ID:    message.ID,
			Error: "Invalid AI request format",
		}
	}

	log.Printf("Processing AI request: %+v", request)

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

	// Simulate processing delay
	time.Sleep(500 * time.Millisecond)

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