package publisher

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// GitHubPublisher is responsible for creating GitHub releases and uploading assets
type GitHubPublisher struct {
	Token      string
	Owner      string
	Repository string
	BaseURL    string
}

// NewGitHubPublisher creates a new GitHub publisher
func NewGitHubPublisher(owner, repo string) (*GitHubPublisher, error) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("GITHUB_TOKEN environment variable not set")
	}

	return &GitHubPublisher{
		Token:      token,
		Owner:      owner,
		Repository: repo,
		BaseURL:    "https://api.github.com",
	}, nil
}

// CreateRelease creates a new GitHub release
func (p *GitHubPublisher) CreateRelease(version, body string) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases", p.BaseURL, p.Owner, p.Repository)

	// GitHub release request payload
	payload := map[string]interface{}{
		"tag_name":         version,
		"target_commitish": "main", // Default branch
		"name":             fmt.Sprintf("Release %s", version),
		"body":             body,
		"draft":            false,
		"prerelease":       strings.Contains(version, "-"),
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal release payload: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Authorization", fmt.Sprintf("token %s", p.Token))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	// Execute request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("GitHub API error (%d): %s", resp.StatusCode, string(body))
	}

	// Parse response to get release ID
	var release map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	// Get upload URL from response
	uploadURL, ok := release["upload_url"].(string)
	if !ok {
		return "", fmt.Errorf("upload URL not found in response")
	}

	// The upload URL contains a template, we need to remove the template part
	uploadURL = strings.Split(uploadURL, "{")[0]

	return uploadURL, nil
}

// UploadAsset uploads an asset to a GitHub release
func (p *GitHubPublisher) UploadAsset(uploadURL, assetPath string) error {
	// Open the file to upload
	file, err := os.Open(assetPath)
	if err != nil {
		return fmt.Errorf("failed to open asset file: %w", err)
	}
	defer file.Close()

	// Create multipart form writer
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filepath.Base(assetPath))
	if err != nil {
		return fmt.Errorf("failed to create form file: %w", err)
	}

	// Copy file content to form writer
	_, err = io.Copy(part, file)
	if err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}
	writer.Close()

	// Create request with query parameter for name
	url := fmt.Sprintf("%s?name=%s", uploadURL, filepath.Base(assetPath))
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Authorization", fmt.Sprintf("token %s", p.Token))
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	// Execute request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitHub API error (%d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// Verify verifies that a GitHub release exists
func (p *GitHubPublisher) Verify(version string) error {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/tags/%s", p.BaseURL, p.Owner, p.Repository, version)

	// Create HTTP request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Authorization", fmt.Sprintf("token %s", p.Token))
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	// Execute request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode >= 400 {
		return fmt.Errorf("release verification failed with status code %d", resp.StatusCode)
	}

	return nil
}