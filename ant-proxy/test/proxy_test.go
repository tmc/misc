package test

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/tmc/misc/ant-proxy/internal/proxy"
)

func TestProxyRecorder(t *testing.T) {
	// Create a temp directory for recordings
	tempDir, err := ioutil.TempDir("", "ant-proxy-test")
	if err \!= nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a recorder handler
	recorder := proxy.NewRecorder(tempDir)

	// Create a test server with the recorder
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder.ServeHTTP(w, r)
	}))
	defer server.Close()

	// Create an Anthropic-like request
	reqBody, _ := json.Marshal(map[string]interface{}{
		"model": "claude-2",
		"messages": []map[string]string{
			{"role": "user", "content": "Hello test"},
		},
		"max_tokens": 100,
		"temperature": 0.7,
	})

	// Send the request to the test server
	resp, err := http.Post(server.URL, "application/json", bytes.NewBuffer(reqBody))
	if err \!= nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Check the status code
	if resp.StatusCode \!= http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	// Check cache header (should be MISS on first request)
	if resp.Header.Get("X-Proxy-Cache") \!= "MISS" {
		t.Errorf("Expected X-Proxy-Cache: MISS, got %s", resp.Header.Get("X-Proxy-Cache"))
	}

	// Read the response
	respBody, err := ioutil.ReadAll(resp.Body)
	if err \!= nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	// Verify we get a response
	var respData map[string]interface{}
	if err := json.Unmarshal(respBody, &respData); err \!= nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	content, ok := respData["content"].(string)
	if \!ok {
		t.Fatalf("Expected content in response, got %v", respData)
	}
	
	t.Logf("Response: %s", content)

	// Send the exact same request again
	resp2, err := http.Post(server.URL, "application/json", bytes.NewBuffer(reqBody))
	if err \!= nil {
		t.Fatalf("Failed to send second request: %v", err)
	}
	defer resp2.Body.Close()

	// Check cache header (should be HIT on second request)
	if resp2.Header.Get("X-Proxy-Cache") \!= "HIT" {
		t.Errorf("Expected X-Proxy-Cache: HIT, got %s", resp2.Header.Get("X-Proxy-Cache"))
	}

	// Verify the recordings directory has a file
	files, err := ioutil.ReadDir(tempDir)
	if err \!= nil {
		t.Fatalf("Failed to read recordings dir: %v", err)
	}
	
	if len(files) == 0 {
		t.Errorf("Expected at least one recording file, got none")
	}

	t.Logf("Created %d recording files", len(files))
}
