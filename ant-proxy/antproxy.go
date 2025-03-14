// Package main is the entry point for the ant-proxy application.
//
// It sets up and runs the proxy server, configuring it to listen for
// incoming Anthropic API requests and forward them to other providers
// like Google Gemini, OpenAI, or Ollama, based on the proxy's configuration.
//
// The main package is responsible for:
//
//   - Loading configuration from environment variables or configuration files.
//   - Initializing backend provider clients (Anthropic, Gemini, etc.).
//   - Creating and configuring the proxy instance.
//   - Setting up HTTP handlers to receive Anthropic API requests.
//   - Starting the HTTP server to listen for incoming connections.
//
// Example usage (running the proxy):
//
//	go run . -anthropic-api-key sk-ant-... -gemini-api-key AIza... -listen-address :8080
//
// This will start the ant-proxy server, listening on port 8080, configured
// with Anthropic and Gemini API keys. Incoming requests to the proxy will be
// routed and processed according to the proxy's logic.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/tmc/misc/ant-proxy/anthropic"
	"github.com/tmc/misc/ant-proxy/gemini"
	"github.com/tmc/misc/ant-proxy/proxy"
)

func main() {
	var (
		listenAddress = flag.String("listen-address", ":9091", "Listen address for the proxy server")
		verbose       = flag.Bool("v", false, "Enable verbose logging")
		harFile       = flag.String("har", "", "Write HTTP Archive (HAR) file to specified path")
	)
	flag.Parse()

	anthropicAPIKey := os.Getenv("ANTHROPIC_API_KEY")
	geminiAPIKey := os.Getenv("GOOGLE_API_KEY")

	if anthropicAPIKey == "" || geminiAPIKey == "" {
		log.Fatal("ANTHROPIC_API_KEY and GOOGLE_API_KEY environment variables are required.")
	}

	anthropicConfig := anthropic.Config{
		APIKey:     anthropicAPIKey,
		APIVersion: "2023-06-01", // Example version, make configurable if needed
		Model:      "claude-3-sonnet-20250219", // Example model, make configurable
	}
	anthropicClient := anthropic.NewClient(anthropicConfig)

	geminiConfig := gemini.Config{
		APIKey: geminiAPIKey,
		Model:  "gemini-pro", // Example model, make configurable
	}
	geminiClient := gemini.NewClient(geminiConfig)

	p := proxy.NewProxy(
		proxy.WithProvider("anthropic", anthropicClient),
		proxy.WithProvider("gemini", geminiClient),
		// Add more providers and options as needed
	)

	http.HandleFunc("/v1/messages", func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Capture request details for HAR
		var harEntry struct {
			Request  map[string]interface{} `json:"request"`
			Response map[string]interface{} `json:"response"`
			Timings  map[string]int64       `json:"timings"`
			StartedDateTime string `json:"startedDateTime"`
			Time int64 `json:"time"`
		}

		harEntry.StartedDateTime = start.Format(time.RFC3339)
		harEntry.Request = make(map[string]interface{})
		harEntry.Response = make(map[string]interface{})
		harEntry.Timings = make(map[string]int64)

		// 1. Parse incoming Anthropic request from r.Body.
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to read request body: %v", err), http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		if *verbose {
			log.Printf("Received request:\n%s\n", string(body))
		}

		harEntry.Request["body"] = string(body)
		harEntry.Request["url"] = r.URL.String()
		harEntry.Request["method"] = r.Method
		harEntry.Request["headers"] = r.Header

		var anthropicRequest anthropic.MessageRequest
		if err := json.Unmarshal(body, &anthropicRequest); err != nil {
			http.Error(w, fmt.Sprintf("Failed to unmarshal Anthropic request: %v", err), http.StatusBadRequest)
			return
		}

		// 2. Call p.HandleRequest(ctx, anthropicRequest).
		ctx := context.Background() // Use request context if available
		resp, err := p.HandleRequest(ctx, anthropicRequest)
		if err != nil {
			http.Error(w, fmt.Sprintf("Proxy error: %v", err), http.StatusInternalServerError)
			return
		}

		// 3. Write the response back to w.
		w.Header().Set("Content-Type", "application/json")
		responseBody, err := json.Marshal(resp)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to marshal proxy response: %v", err), http.StatusInternalServerError)
			return
		}

		w.Write(responseBody)

		harEntry.Response["body"] = string(responseBody)
		harEntry.Response["headers"] = w.Header()
		harEntry.Response["status"] = http.StatusOK

		if *verbose {
			log.Printf("Sent response:\n%s\n", string(responseBody))
		}

		elapsed := time.Since(start)
		harEntry.Time = elapsed.Milliseconds()
		harEntry.Timings["total"] = elapsed.Milliseconds()

		if *harFile != "" {
			harJSON, err := json.MarshalIndent(harEntry, "", "  ")
			if err != nil {
				log.Printf("Error marshaling HAR entry: %v", err)
			} else {
				f, err := os.OpenFile(*harFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
				if err != nil {
					log.Printf("Error opening HAR file: %v", err)
				}
				defer f.Close()
				if _, err := f.WriteString(string(harJSON) + ",\n"); err != nil {
					log.Printf("Error writing to HAR file: %v", err)
				}
			}
		}
	})

	log.Printf("Starting ant-proxy server on %s", *listenAddress)
	if err := http.ListenAndServe(*listenAddress, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
