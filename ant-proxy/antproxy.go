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
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/tmc/misc/ant-proxy/anthropic"
	"github.com/tmc/misc/ant-proxy/gemini"
	"github.com/tmc/misc/ant-proxy/proxy"
	"golang.org/x/term"
)

var (
	listenAddress = flag.String("listen-address", ":9091", "Listen address for the proxy server")
	verbose       = flag.Bool("v", false, "Enable verbose logging")
	harFile       = flag.String("har", "", "Write HTTP Archive (HAR) file to specified path")
	delayDuration       = flag.Duration("delay", 0, "delay between chunks") // Corrected variable name
	pathsString = flag.String("path", "type=content_block_delta,delta.type=text_delta,delta.text input_json_delta=delta.partial_json",
	"paths to extract (space-separated patterns)")
	webServer   = flag.Bool("w", false, "run HTTP server that streams output")
	port        = flag.Int("port", 8072, "HTTP server port when using -w")
	showMeta    = flag.Bool("m", false, "include metadata (tokens, stop reason) in output, defaults to txtar format")
	veryVerbose = flag.Bool("vv", false, "very verbose output (print raw lines and formatted JSON)")
	failOnError = flag.Bool("fail-on-error", true, "exit with error on JSON parsing failures")
	catNonSSE   = flag.Bool("cat-non-sse", true, "output raw content for non-SSE input instead of exiting with error")
	inFile      = flag.String("f", "-", "input file") // Re-declare inFile flag
	// For test environment detection
	inTest = len(os.Getenv("WORK")) > 0
)

func main() {
	// Set log output to stderr
	log.SetOutput(greyTextWriter{w: os.Stderr})
	flag.Parse()

	// If veryVerbose is set, also set verbose
	if *veryVerbose {
		v := true
		verbose = &v
	}

	anthropicAPIKey := os.Getenv("ANTHROPIC_API_KEY")
	geminiAPIKey := os.Getenv("GOOGLE_API_KEY")

	if anthropicAPIKey == "" || geminiAPIKey == "" {
		log.Fatal("ANTHROPIC_API_KEY and GOOGLE_API_KEY environment variables are required.")
	}

	anthropicConfig := anthropic.Config{
		APIKey:     anthropicAPIKey,
		APIVersion: "2023-06-01", // Example version, make configurable if needed
		Model:      "claude-3-7-sonnet-20250219", // Example model, make configurable
	}
	anthropicClient := anthropic.NewClient(anthropicConfig)

	geminiConfig := gemini.Config{
		APIKey: geminiAPIKey,
		ModelName:    "gemini-2.0-flash", // Example model, make configurable
		ModelVersion: "v1beta",
	}
	geminiClient := gemini.NewClient(geminiConfig)

	p := proxy.NewProxy(
		proxy.WithProvider("anthropic", anthropicClient),
		proxy.WithProvider("gemini", geminiClient),
		// Add more providers and options as needed
	)

	// If watch mode is enabled, start the HTTP server
	if *webServer {
		runServer(p) // Pass proxy instance to runServer
		return
	}

	// Open the input file
	var in *os.File
	var err error

	if *inFile == "-" {
		in = os.Stdin
	} else {
		in, err = os.Open(*inFile)
		if err != nil {
			log.Fatalf("Error opening file: %v", err)
		}
		defer in.Close()
	}

	// Process the input (will automatically handle SSE or regular content)
	if err := processInput(in, os.Stdout, nil); err != nil { // patterns not used in CLI mode yet
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// runServer starts an HTTP server that processes POSTed data (SSE or regular content)
func runServer(p *proxy.Proxy) { // Accept Proxy instance
	addr := ":" + strconv.Itoa(*port)

	// Define handler for POST requests
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Received request: %s %s", r.Method, r.URL.Path)
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Set headers for streaming response
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		// 1. Parse incoming Anthropic request from r.Body.
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to read request body: %v", err), http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

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

		// 3. Write the response back to w as SSE stream

		// For now, simulate SSE by sending the entire content as a single data event
		if anthropicResp, ok := resp.(*anthropic.MessageResponse); ok { // Use anthropic response type
			flusher, ok := w.(http.Flusher)
			if !ok {
				http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
				return
			}

			// Simulate streaming by sending chunk by chunk with delay
			chunkSize := 10 // Example chunk size
			content := anthropicResp.Content
			for i := 0; i < len(content); i += chunkSize {
				end := i + chunkSize
				if end > len(content) {
					end = len(content)
				}
				chunk := content[i:end]
				sseData := fmt.Sprintf("data: %s\n\n", chunk)
				if _, err := fmt.Fprint(w, sseData); err != nil {
					log.Printf("Failed to write SSE data: %v", err)
					return // Stop streaming if write fails
				}
				flusher.Flush() // Flush the chunk to the client
				time.Sleep(*delayDuration) // Apply delay between chunks
			}

		} else {
			// Fallback for non-streaming response (or errors) - send as regular JSON
			w.Header().Set("Content-Type", "application/json") // Reset content type to json for non-sse
			responseBody, _ := json.Marshal(resp)              // Ignoring marshal error for simplicity in example
			w.Write(responseBody)
		}
	})

	// Start the server
	log.Printf("Starting server on %s...", addr)
	if err := http.ListenAndServe(addr, nil); err != nil { // Check error here
		log.Fatalf("Failed to start server: %v", err)
	}
}

type greyTextWriter struct {
w io.Writer
}

// Write writes the given byte slice to the underlying writer with grey text color
func (g greyTextWriter) Write(p []byte) (n int, err error) {
g.w.Write([]byte("\033[90m"))
n, err = g.w.Write(p)
g.w.Write([]byte("\033[0m"))
return n, err
}

func processInput(r io.Reader, w io.Writer, patterns []proxy.ExtractorPattern) error { // Use proxy.ExtractorPattern
// Buffer some initial content to check if it contains SSE events
var buf bytes.Buffer
tee := io.TeeReader(r, &buf)

// Create scanner to peek at content
peekScanner := bufio.NewScanner(tee)
lineCount := 0
containsSSE := false

// Check first few lines to determine if this is an SSE stream
for lineCount < 10 && peekScanner.Scan() {
line := peekScanner.Text()
if strings.HasPrefix(line, "data: ") {
containsSSE = true
break
}
lineCount++
}

// If not an SSE stream and not in a test environment, just cat the content
if !containsSSE && !inTest {
// Check if stderr is a TTY for notification
isTTY := term.IsTerminal(int(os.Stderr.Fd()))

if isTTY {
fmt.Fprintln(os.Stderr, "Not an SSE stream, outputting raw content")
}

// Copy buffered content and rest of input to output
if _, err := io.Copy(w, io.MultiReader(&buf, r)); err != nil {
return fmt.Errorf("error copying raw content: %w", err)
}
return nil
}

// Process as SSE stream (or continue with standard behavior in test mode)
metadataInfo := make(map[string]any)

// Create a new scanner for the combined buffer and remaining input
scanner := bufio.NewScanner(io.MultiReader(&buf, r))

for scanner.Scan() {
line := scanner.Text()

// Handle very verbose mode - print raw line
if *veryVerbose {
log.Printf("Raw line: %s", line)
}

// Skip non-data lines
if !strings.HasPrefix(line, "data: ") {
continue
}

jsonData := strings.TrimPrefix(line, "data: ")

var m map[string]any
if err := json.Unmarshal([]byte(jsonData), &m); err != nil {
if *failOnError {
return fmt.Errorf("json unmarshal: %w", err)
}
log.Printf("Warning: Failed to parse JSON: %v", err)
continue
}

// Check for metadata in message_start or message_delta events
if *showMeta {
if typeVal, ok := m["type"].(string); ok {
if typeVal == "message_start" {
if msg, ok := m["message"].(map[string]any); ok {
for k, v := range msg {
metadataInfo[k] = v
}
}
} else if typeVal == "message_delta" {
if delta, ok := m["delta"].(map[string]any); ok {
// Merge delta into metadata
for k, v := range delta {
metadataInfo[k] = v
}
}
// Add usage information if available
if usage, ok := m["usage"].(map[string]any); ok {
metadataInfo["usage"] = usage
}
}
}
}

// Try each pattern to extract text
for _, pattern := range patterns {
// Check if all conditions match
allMatch := true
for _, cond := range pattern.conditions {
val, ok := getByPath(m, cond.path)
if !ok || fmt.Sprintf("%v", val) != cond.value {
allMatch = false
break
}
}

if allMatch {
// Extract the text using the extraction path
if text, ok := getByPath(m, pattern.extractPath); ok {
if textStr, ok := text.(string); ok {
// Log the extracted text in verbose mode
if *veryVerbose {
log.Printf("Extracted text: %s", textStr)
}

// Print the extracted text
fmt.Fprint(w, textStr)

// Apply delay if specified
if *delayDuration > 0 { // Use delayDuration here
time.Sleep(*delayDuration) // Use delayDuration here
}

break // Stop after first matching pattern
}
}
}
}
}
fmt.Fprintln(w)

if err := scanner.Err(); err != nil {
return fmt.Errorf("read error: %w", err)
}

if *showMeta && len(metadataInfo) > 0 {
fmt.Fprintln(w)
metadataOutput := formatMetadata(metadataInfo)
fmt.Fprintln(w, metadataOutput)
}
return nil
}

// formatMetadata formats the metadata into a readable string
func formatMetadata(metadataInfo map[string]any) string {
var sb strings.Builder
sb.WriteString("-- metadata --\n")

if id, ok := metadataInfo["id"].(string); ok {
sb.WriteString(fmt.Sprintf("Message ID: %s\n", id))
}

if model, ok := metadataInfo["model"].(string); ok {
sb.WriteString(fmt.Sprintf("Model: %s\n", model))
}

if usage, ok := metadataInfo["usage"].(map[string]any); ok {
inputTokens, hasInput := usage["input_tokens"]
if hasInput {
inputTokensJSON := prettyJSON(map[string]any{
"input_tokens":                inputTokens,
"cache_creation_input_tokens": usage["cache_creation_input_tokens"],
"cache_read_input_tokens":     usage["cache_read_input_tokens"],
})
sb.WriteString(fmt.Sprintf("Input tokens: %s\n", inputTokensJSON))
}

outputTokens, hasOutput := usage["output_tokens"]
if hasOutput {
outputTokensJSON := prettyJSON(map[string]any{
"output_tokens": outputTokens,
})
sb.WriteString(fmt.Sprintf("Output tokens: %s\n", outputTokensJSON))
}
}

if stopReason, ok := metadataInfo["stop_reason"].(string); ok {
sb.WriteString(fmt.Sprintf("Stop reason: %s\n", stopReason))
}

return sb.String()
}

// Helper function to get value from nested maps by path
func getByPath(m map[string]any, path string) (any, bool) {
	parts := strings.Split(path, ".")
	current := m

	for i, part := range parts {
		if i == len(parts)-1 {
			val, ok := current[part]
			return val, ok
		}

		next, ok := current[part]
		if !ok {
			return nil, false
		}

		current, ok = next.(map[string]any)
		if !ok {
			return nil, false
		}
	}

	return nil, false
}

// prettyJSON marshals an object to a pretty-printed JSON string
func prettyJSON(obj any) string {
	bytes, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", obj)
	}
	return string(bytes)
}
