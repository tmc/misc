// Command simplechurl is a simplified version of churl that doesn't require Chrome.
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
	"os/signal"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/pkg/errors"
)

type options struct {
	// Output options
	outputFile   string
	outputFormat string // html, har, text, json

	// Request options
	headers        headerSlice
	method         string
	data           string
	followRedirect bool
	timeout        int
	verbose        bool

	// Authentication
	username string
	password string
}

// headerSlice allows multiple -H flags
type headerSlice []string

func (h *headerSlice) String() string {
	return strings.Join(*h, ", ")
}

func (h *headerSlice) Set(value string) error {
	*h = append(*h, value)
	return nil
}

func main() {
	opts := options{}

	// Output options
	flag.StringVar(&opts.outputFile, "o", "", "Output file (default: stdout)")
	flag.StringVar(&opts.outputFormat, "output-format", "html", "Output format: html, har, text, json")

	// Request options
	flag.Var(&opts.headers, "H", "Add request header (can be used multiple times)")
	flag.StringVar(&opts.method, "X", "GET", "HTTP method to use")
	flag.StringVar(&opts.data, "d", "", "Data to send (for POST/PUT)")
	flag.BoolVar(&opts.followRedirect, "L", true, "Follow redirects")
	flag.IntVar(&opts.timeout, "timeout", 180, "Global timeout in seconds")
	flag.BoolVar(&opts.verbose, "verbose", false, "Enable verbose logging")

	// Authentication
	flag.StringVar(&opts.username, "u", "", "Username for basic auth (user:password)")

	// Custom usage message
	flag.Usage = func() {
		w := tabwriter.NewWriter(os.Stderr, 0, 0, 2, ' ', 0)
		defer w.Flush()

		fmt.Fprintf(w, "simplechurl - Simple curl-like tool with multiple output formats\n\n")
		fmt.Fprintf(w, "Usage:\n")
		fmt.Fprintf(w, "  simplechurl [options] URL\n\n")
		fmt.Fprintf(w, "Options:\n")

		flag.VisitAll(func(f *flag.Flag) {
			def := f.DefValue
			if def != "" {
				def = fmt.Sprintf(" (default: %s)", def)
			}

			typ := ""
			switch f.Value.String() {
			case "false", "true":
				typ = "bool"
			case "0":
				typ = "int"
			case "[]":
				typ = "list"
			default:
				typ = "string"
			}

			fmt.Fprintf(w, "  -%s\t%s\t%s%s\n", f.Name, typ, f.Usage, def)
		})
	}

	flag.Parse()

	// Check for URL argument
	if flag.NArg() != 1 {
		fmt.Println("Error: URL is required")
		flag.Usage()
		os.Exit(1)
	}

	url := flag.Arg(0)

	// Parse basic auth from username flag (user:password format)
	if opts.username != "" && strings.Contains(opts.username, ":") {
		parts := strings.SplitN(opts.username, ":", 2)
		opts.username = parts[0]
		if len(parts) > 1 {
			opts.password = parts[1]
		}
	}

	// Create a context with the user-specified timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(opts.timeout)*time.Second)
	defer cancel()

	// Handle interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		if opts.verbose {
			log.Println("Interrupt received, shutting down...")
		}
		cancel()
	}()

	// Run the command
	if err := run(ctx, url, opts); err != nil {
		if err == context.DeadlineExceeded {
			log.Fatal("Operation timed out. Try increasing the timeout value.")
		} else {
			log.Fatal(err)
		}
	}
}

func run(ctx context.Context, url string, opts options) error {
	// Create HTTP client with appropriate settings
	client := &http.Client{
		Timeout: time.Duration(opts.timeout) * time.Second,
	}

	// Set up the client to match options
	if !opts.followRedirect {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	// Create the request
	req, err := http.NewRequestWithContext(ctx, opts.method, url, strings.NewReader(opts.data))
	if err != nil {
		return errors.Wrap(err, "creating request")
	}

	// Add headers
	for _, h := range opts.headers {
		parts := strings.SplitN(h, ":", 2)
		if len(parts) != 2 {
			return errors.Errorf("invalid header format: %s", h)
		}
		name := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		req.Header.Set(name, value)
	}

	// Set up basic auth if provided
	if opts.username != "" && opts.password != "" {
		req.SetBasicAuth(opts.username, opts.password)
	}

	// Set default headers
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "simplechurl/1.0")
	}

	if opts.verbose {
		log.Printf("Fetching URL: %s (method: %s)", url, opts.method)
		log.Printf("Request headers: %v", req.Header)
	}

	// Make the request
	resp, err := client.Do(req)
	if err != nil {
		return errors.Wrap(err, "making request")
	}
	defer resp.Body.Close()

	if opts.verbose {
		log.Printf("Response status: %s", resp.Status)
		log.Printf("Response headers: %v", resp.Header)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrap(err, "reading response body")
	}

	// Process based on requested format
	var output []byte
	switch opts.outputFormat {
	case "html":
		// Use the HTML directly
		output = body

	case "text":
		// Very simple text extraction
		text := string(body)
		text = strings.ReplaceAll(text, "<script", "\n<script")
		text = strings.ReplaceAll(text, "</script>", "</script>\n")
		text = strings.ReplaceAll(text, "<style", "\n<style")
		text = strings.ReplaceAll(text, "</style>", "</style>\n")
		text = strings.ReplaceAll(text, "<", "\n<")

		// Extract text nodes
		var sb strings.Builder
		for _, line := range strings.Split(text, "\n") {
			if !strings.HasPrefix(line, "<") {
				content := strings.TrimSpace(line)
				if content != "" {
					sb.WriteString(content)
					sb.WriteString("\n")
				}
			}
		}
		output = []byte(sb.String())

	case "json":
		// Create a JSON object with response info
		info := struct {
			URL     string            `json:"url"`
			Status  int               `json:"status"`
			Headers map[string]string `json:"headers"`
			Content string            `json:"content"`
		}{
			URL:     url,
			Status:  resp.StatusCode,
			Headers: make(map[string]string),
			Content: string(body),
		}

		// Copy headers
		for k, v := range resp.Header {
			info.Headers[k] = strings.Join(v, ", ")
		}

		output, err = json.MarshalIndent(info, "", "  ")
		if err != nil {
			return errors.Wrap(err, "marshaling JSON")
		}

	case "har":
		// Create a simplified HAR format
		harData := map[string]interface{}{
			"log": map[string]interface{}{
				"version": "1.2",
				"creator": map[string]string{
					"name":    "simplechurl",
					"version": "1.0",
				},
				"entries": []map[string]interface{}{
					{
						"request": map[string]interface{}{
							"method":  opts.method,
							"url":     url,
							"headers": req.Header,
						},
						"response": map[string]interface{}{
							"status":     resp.StatusCode,
							"statusText": resp.Status,
							"headers":    resp.Header,
							"content": map[string]interface{}{
								"size":     len(body),
								"mimeType": resp.Header.Get("Content-Type"),
								"text":     string(body),
							},
						},
						"startedDateTime": time.Now().Format(time.RFC3339),
						"time":            0, // Mock doesn't track timing
					},
				},
			},
		}

		output, err = json.MarshalIndent(harData, "", "  ")
		if err != nil {
			return errors.Wrap(err, "marshaling HAR")
		}

	default:
		return errors.Errorf("unsupported output format: %s", opts.outputFormat)
	}

	// Write the output
	var outWriter io.Writer = os.Stdout
	if opts.outputFile != "" {
		file, err := os.Create(opts.outputFile)
		if err != nil {
			return errors.Wrap(err, "creating output file")
		}
		defer file.Close()
		outWriter = file
	}

	_, err = outWriter.Write(output)
	return err
}