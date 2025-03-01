package test

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Simple version of the Request struct
type Request struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float64   `json:"temperature"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Response struct {
	Content string `json:"content"`
}

// Simple recorder handler that implements the essential functionality
func TestStandaloneRecorder(t *testing.T) {
	// Create a temp directory for recordings
	tempDir, err := ioutil.TempDir("", "ant-proxy-test")
	if err \!= nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test server with a recording handler
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read request body
		body, err := ioutil.ReadAll(r.Body)
		if err \!= nil {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		// Create a hash of the request to use as a cache key
		cacheKey := "test-key-" + strings.Replace(r.URL.Path, "/", "-", -1)
		cachePath := filepath.Join(tempDir, cacheKey+".json")

		// Check if we have a cached response
		if _, err := os.Stat(cachePath); err == nil {
			// Load cached response
			data, err := ioutil.ReadFile(cachePath)
			if err == nil {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Proxy-Cache", "HIT")
				w.Write(data)
				return
			}
		}

		// Parse request
		var req Request
		if err := json.Unmarshal(body, &req); err \!= nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		// Create a mock response
		resp := Response{
			Content: "Mocked response for model " + req.Model,
		}
		
		// Serialize response
		respData, err := json.Marshal(resp)
		if err \!= nil {
			http.Error(w, "Failed to serialize response", http.StatusInternalServerError)
			return
		}
		
		// Save response to cache
		err = ioutil.WriteFile(cachePath, respData, 0644)
		if err \!= nil {
			t.Logf("Failed to write cache file: %v", err)
		}
		
		// Return response
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Proxy-Cache", "MISS")
		w.Write(respData)
	}))
	defer server.Close()
	
	// Test with a simple request
	reqBody := `{"model":"claude-2","messages":[{"role":"user","content":"Hello test"}],"max_tokens":100,"temperature":0.7}`
	
	// First request - should be a cache miss
	resp, err := http.Post(server.URL+"/v1/messages", "application/json", strings.NewReader(reqBody))
	if err \!= nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	
	if resp.Header.Get("X-Proxy-Cache") \!= "MISS" {
		t.Errorf("Expected X-Proxy-Cache: MISS, got %s", resp.Header.Get("X-Proxy-Cache"))
	}
	
	respBody, err := ioutil.ReadAll(resp.Body)
	if err \!= nil {
		t.Fatalf("Failed to read response body: %v", err)
	}
	resp.Body.Close()
	
	var respData Response
	if err := json.Unmarshal(respBody, &respData); err \!= nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	
	expectedContent := "Mocked response for model claude-2"
	if respData.Content \!= expectedContent {
		t.Errorf("Expected content %q, got %q", expectedContent, respData.Content)
	}
	
	// Second request - should be a cache hit
	resp2, err := http.Post(server.URL+"/v1/messages", "application/json", strings.NewReader(reqBody))
	if err \!= nil {
		t.Fatalf("Failed to send second request: %v", err)
	}
	
	if resp2.Header.Get("X-Proxy-Cache") \!= "HIT" {
		t.Errorf("Expected X-Proxy-Cache: HIT, got %s", resp2.Header.Get("X-Proxy-Cache"))
	}
	
	respBody2, err := ioutil.ReadAll(resp2.Body)
	if err \!= nil {
		t.Fatalf("Failed to read second response body: %v", err)
	}
	resp2.Body.Close()
	
	var respData2 Response
	if err := json.Unmarshal(respBody2, &respData2); err \!= nil {
		t.Fatalf("Failed to parse second response: %v", err)
	}
	
	if respData2.Content \!= expectedContent {
		t.Errorf("Expected content %q in second response, got %q", expectedContent, respData2.Content)
	}
	
	// Verify we have a cache file
	files, err := ioutil.ReadDir(tempDir)
	if err \!= nil {
		t.Fatalf("Failed to read cache directory: %v", err)
	}
	
	if len(files) == 0 {
		t.Errorf("Expected at least one cache file, got none")
	}
	
	t.Logf("Created %d cache files in %s", len(files), tempDir)
}
