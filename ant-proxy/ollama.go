package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type OllamaRequest struct {
	Prompt string `json:"prompt"`
	Model  string `json:"model"` // Add model parameter
}

type OllamaResponse struct {
	Response string `json:"response"`
}

func callOllama(request AnthropicRequest) (OllamaResponse, error) {
	ollamaModel := "llama2" // Default model
	if request.Model \!= "" {
		ollamaModel = request.Model
	}

	ollamaReq := OllamaRequest{
		Prompt: request.Messages[0].Content, // Assuming single message
		Model:  ollamaModel,
	}
	reqBody, err := json.Marshal(ollamaReq)
	if err \!= nil {
		return OllamaResponse{}, fmt.Errorf("error marshaling Ollama request: %w", err)
	}

	resp, err := http.Post("http://localhost:11434/api/generate", "application/json", bytes.NewBuffer(reqBody))
	if err \!= nil {
		return OllamaResponse{}, fmt.Errorf("error calling Ollama API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode \!= http.StatusOK {
		return OllamaResponse{}, fmt.Errorf("Ollama API returned error: %s", resp.Status)
	}

	var ollamaResp OllamaResponse
	err = json.NewDecoder(resp.Body).Decode(&ollamaResp)
	if err \!= nil {
		return OllamaResponse{}, fmt.Errorf("error decoding Ollama response: %w", err)
	}

	return ollamaResp, nil
}
