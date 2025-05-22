package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

// GithubHandler handles GitHub API requests
type GithubHandler struct {
	Logger *zap.Logger
}

// NewGithubHandler creates a new GithubHandler
func NewGithubHandler() *GithubHandler {
	logger, _ := zap.NewProduction()
	return &GithubHandler{
		Logger: logger,
	}
}

// HandleGetGist fetches a GitHub Gist
func (h *GithubHandler) HandleGetGist(w http.ResponseWriter, r *http.Request) {
	// Get Gist ID from URL
	vars := mux.Vars(r)
	gistID := vars["id"]

	// Call GitHub API
	resp, err := http.Get(fmt.Sprintf("https://api.github.com/gists/%s", gistID))
	if err != nil {
		h.Logger.Error("Failed to fetch Gist", zap.Error(err))
		http.Error(w, "Failed to fetch Gist", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		h.Logger.Error("GitHub API error", 
			zap.Int("status", resp.StatusCode), 
			zap.String("body", string(body)))
		http.Error(w, fmt.Sprintf("GitHub API error: %s", resp.Status), resp.StatusCode)
		return
	}

	// Parse response
	var gist GistResponse
	if err := json.NewDecoder(resp.Body).Decode(&gist); err != nil {
		h.Logger.Error("Failed to parse Gist response", zap.Error(err))
		http.Error(w, "Failed to parse Gist", http.StatusInternalServerError)
		return
	}

	// Extract playground configuration
	config, err := h.extractPlaygroundConfig(gist)
	if err != nil {
		h.Logger.Error("Failed to extract playground configuration", zap.Error(err))
		http.Error(w, "Invalid Gist format", http.StatusBadRequest)
		return
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(config); err != nil {
		h.Logger.Error("Failed to encode response", zap.Error(err))
	}
}

// HandleCreateGist creates a new GitHub Gist
func (h *GithubHandler) HandleCreateGist(w http.ResponseWriter, r *http.Request) {
	// Check if GitHub token is provided
	token := r.Header.Get("Authorization")
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	if token == "" {
		http.Error(w, "GitHub token required", http.StatusUnauthorized)
		return
	}
	if !strings.HasPrefix(token, "Bearer ") && !strings.HasPrefix(token, "token ") {
		token = "token " + token
	}

	// Parse request
	var req CreateGistRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Prepare GitHub API request
	gistFiles, err := h.createGistFiles(req.Config)
	if err != nil {
		h.Logger.Error("Failed to create Gist files", zap.Error(err))
		http.Error(w, "Failed to prepare Gist", http.StatusInternalServerError)
		return
	}

	// Create Gist request payload
	gistReq := map[string]interface{}{
		"description": req.Description,
		"public":      req.Public,
		"files":       gistFiles,
	}

	// Call GitHub API
	reqBody, _ := json.Marshal(gistReq)
	httpReq, err := http.NewRequest("POST", "https://api.github.com/gists", bytes.NewBuffer(reqBody))
	if err != nil {
		h.Logger.Error("Failed to create HTTP request", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	httpReq.Header.Set("Authorization", token)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/vnd.github.v3+json")

	// Send request
	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		h.Logger.Error("Failed to create Gist", zap.Error(err))
		http.Error(w, "Failed to create Gist", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		h.Logger.Error("GitHub API error", 
			zap.Int("status", resp.StatusCode), 
			zap.String("body", string(body)))
		http.Error(w, fmt.Sprintf("GitHub API error: %s", resp.Status), resp.StatusCode)
		return
	}

	// Parse response
	var gistResp GistResponse
	if err := json.NewDecoder(resp.Body).Decode(&gistResp); err != nil {
		h.Logger.Error("Failed to parse GitHub response", zap.Error(err))
		http.Error(w, "Failed to parse GitHub response", http.StatusInternalServerError)
		return
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{
		"id":  gistResp.ID,
		"url": gistResp.HTMLURL,
	}); err != nil {
		h.Logger.Error("Failed to encode response", zap.Error(err))
	}
}

// HandleUpdateGist updates an existing GitHub Gist
func (h *GithubHandler) HandleUpdateGist(w http.ResponseWriter, r *http.Request) {
	// Get Gist ID from URL
	vars := mux.Vars(r)
	gistID := vars["id"]

	// Check if GitHub token is provided
	token := r.Header.Get("Authorization")
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	if token == "" {
		http.Error(w, "GitHub token required", http.StatusUnauthorized)
		return
	}
	if !strings.HasPrefix(token, "Bearer ") && !strings.HasPrefix(token, "token ") {
		token = "token " + token
	}

	// Parse request
	var req CreateGistRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Prepare GitHub API request
	gistFiles, err := h.createGistFiles(req.Config)
	if err != nil {
		h.Logger.Error("Failed to create Gist files", zap.Error(err))
		http.Error(w, "Failed to prepare Gist", http.StatusInternalServerError)
		return
	}

	// Create Gist request payload
	gistReq := map[string]interface{}{
		"description": req.Description,
		"files":       gistFiles,
	}

	// Call GitHub API
	reqBody, _ := json.Marshal(gistReq)
	httpReq, err := http.NewRequest("PATCH", fmt.Sprintf("https://api.github.com/gists/%s", gistID), bytes.NewBuffer(reqBody))
	if err != nil {
		h.Logger.Error("Failed to create HTTP request", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	httpReq.Header.Set("Authorization", token)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/vnd.github.v3+json")

	// Send request
	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		h.Logger.Error("Failed to update Gist", zap.Error(err))
		http.Error(w, "Failed to update Gist", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		h.Logger.Error("GitHub API error", 
			zap.Int("status", resp.StatusCode), 
			zap.String("body", string(body)))
		http.Error(w, fmt.Sprintf("GitHub API error: %s", resp.Status), resp.StatusCode)
		return
	}

	// Parse response
	var gistResp GistResponse
	if err := json.NewDecoder(resp.Body).Decode(&gistResp); err != nil {
		h.Logger.Error("Failed to parse GitHub response", zap.Error(err))
		http.Error(w, "Failed to parse GitHub response", http.StatusInternalServerError)
		return
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{
		"id":  gistResp.ID,
		"url": gistResp.HTMLURL,
	}); err != nil {
		h.Logger.Error("Failed to encode response", zap.Error(err))
	}
}

// extractPlaygroundConfig extracts playground configuration from a Gist
func (h *GithubHandler) extractPlaygroundConfig(gist GistResponse) (*PlaygroundConfig, error) {
	// Look for playground.json file
	configFile, ok := gist.Files["playground.json"]
	if !ok {
		return nil, fmt.Errorf("playground.json not found in Gist")
	}

	// Parse configuration
	var config PlaygroundConfig
	if err := json.Unmarshal([]byte(configFile.Content), &config); err != nil {
		return nil, fmt.Errorf("failed to parse playground.json: %w", err)
	}

	// If proto files are not defined in the configuration, extract them from Gist files
	if len(config.Proto.Files) == 0 {
		for filename, file := range gist.Files {
			if strings.HasSuffix(filename, ".proto") {
				config.Proto.Files = append(config.Proto.Files, ProtoFile{
					Name:    filename,
					Content: file.Content,
				})
			}
		}
	}

	// If templates are not defined in the configuration, extract them from Gist files
	if len(config.Templates) == 0 {
		for filename, file := range gist.Files {
			if strings.HasSuffix(filename, ".tmpl") {
				config.Templates = append(config.Templates, Template{
					Name:    filename,
					Content: file.Content,
				})
			}
		}
	}

	return &config, nil
}

// createGistFiles creates Gist files from playground configuration
func (h *GithubHandler) createGistFiles(config *PlaygroundConfig) (map[string]map[string]string, error) {
	files := make(map[string]map[string]string)

	// Add playground.json
	configJSON, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal configuration: %w", err)
	}
	files["playground.json"] = map[string]string{
		"content": string(configJSON),
	}

	// Add proto files
	for _, file := range config.Proto.Files {
		files[file.Name] = map[string]string{
			"content": file.Content,
		}
	}

	// Add template files
	for _, tmpl := range config.Templates {
		files[tmpl.Name] = map[string]string{
			"content": tmpl.Content,
		}
	}

	// Add README.md
	readmeContent := fmt.Sprintf(`# Protoc-Gen-Anything Playground Configuration

This Gist contains configuration for the Protoc-Gen-Anything Playground.

## Proto Files
%s

## Templates
%s

## Usage

To use this configuration, visit the playground and load it using the Gist ID:
https://playground.example.com/?gist=%s
`, h.listProtoFiles(config.Proto.Files), h.listTemplates(config.Templates), "GIST_ID_PLACEHOLDER")

	files["README.md"] = map[string]string{
		"content": readmeContent,
	}

	return files, nil
}

// listProtoFiles formats a list of proto files for the README
func (h *GithubHandler) listProtoFiles(files []ProtoFile) string {
	var sb strings.Builder
	for _, file := range files {
		sb.WriteString(fmt.Sprintf("- `%s`\n", file.Name))
	}
	return sb.String()
}

// listTemplates formats a list of templates for the README
func (h *GithubHandler) listTemplates(files []Template) string {
	var sb strings.Builder
	for _, file := range files {
		sb.WriteString(fmt.Sprintf("- `%s`\n", file.Name))
	}
	return sb.String()
}