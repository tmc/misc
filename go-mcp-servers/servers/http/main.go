package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/tmc/misc/go-mcp-servers/lib/mcpframework"
)

func main() {
	server := mcpframework.NewServer("http-mcp-server", "1.0.0")
	server.SetInstructions("A Model Context Protocol server that provides HTTP client operations including GET, POST, PUT, DELETE requests and URL parsing.")

	// Register HTTP tools
	registerHTTPTools(server)
	setupResourceHandlers(server)

	// Run the server
	ctx := context.Background()
	if err := server.Run(ctx, os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}

func registerHTTPTools(server *mcpframework.Server) {
	// HTTP GET tool
	server.RegisterTool("http_get", "Perform an HTTP GET request", &mcpframework.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"url": map[string]interface{}{
				"type":        "string",
				"description": "The URL to request",
			},
			"headers": map[string]interface{}{
				"type":        "object",
				"description": "HTTP headers to include",
			},
			"timeout": map[string]interface{}{
				"type":        "integer",
				"description": "Request timeout in seconds",
				"default":     30,
			},
		},
		Required: []string{"url"},
	}, handleHTTPGet)

	// HTTP POST tool
	server.RegisterTool("http_post", "Perform an HTTP POST request", &mcpframework.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"url": map[string]interface{}{
				"type":        "string",
				"description": "The URL to request",
			},
			"body": map[string]interface{}{
				"type":        "string",
				"description": "Request body",
			},
			"headers": map[string]interface{}{
				"type":        "object",
				"description": "HTTP headers to include",
			},
			"content_type": map[string]interface{}{
				"type":        "string",
				"description": "Content-Type header",
				"default":     "application/json",
			},
			"timeout": map[string]interface{}{
				"type":        "integer",
				"description": "Request timeout in seconds",
				"default":     30,
			},
		},
		Required: []string{"url"},
	}, handleHTTPPost)

	// HTTP PUT tool
	server.RegisterTool("http_put", "Perform an HTTP PUT request", &mcpframework.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"url": map[string]interface{}{
				"type":        "string",
				"description": "The URL to request",
			},
			"body": map[string]interface{}{
				"type":        "string",
				"description": "Request body",
			},
			"headers": map[string]interface{}{
				"type":        "object",
				"description": "HTTP headers to include",
			},
			"content_type": map[string]interface{}{
				"type":        "string",
				"description": "Content-Type header",
				"default":     "application/json",
			},
			"timeout": map[string]interface{}{
				"type":        "integer",
				"description": "Request timeout in seconds",
				"default":     30,
			},
		},
		Required: []string{"url"},
	}, handleHTTPPut)

	// HTTP DELETE tool
	server.RegisterTool("http_delete", "Perform an HTTP DELETE request", &mcpframework.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"url": map[string]interface{}{
				"type":        "string",
				"description": "The URL to request",
			},
			"headers": map[string]interface{}{
				"type":        "object",
				"description": "HTTP headers to include",
			},
			"timeout": map[string]interface{}{
				"type":        "integer",
				"description": "Request timeout in seconds",
				"default":     30,
			},
		},
		Required: []string{"url"},
	}, handleHTTPDelete)

	// URL parse tool
	server.RegisterTool("parse_url", "Parse and analyze a URL", &mcpframework.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"url": map[string]interface{}{
				"type":        "string",
				"description": "The URL to parse",
			},
		},
		Required: []string{"url"},
	}, handleParseURL)

	// HTTP HEAD tool
	server.RegisterTool("http_head", "Perform an HTTP HEAD request to get headers only", &mcpframework.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"url": map[string]interface{}{
				"type":        "string",
				"description": "The URL to request",
			},
			"headers": map[string]interface{}{
				"type":        "object",
				"description": "HTTP headers to include",
			},
			"timeout": map[string]interface{}{
				"type":        "integer",
				"description": "Request timeout in seconds",
				"default":     30,
			},
		},
		Required: []string{"url"},
	}, handleHTTPHead)

	// HTTP OPTIONS tool
	server.RegisterTool("http_options", "Perform an HTTP OPTIONS request", &mcpframework.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"url": map[string]interface{}{
				"type":        "string",
				"description": "The URL to request",
			},
			"headers": map[string]interface{}{
				"type":        "object",
				"description": "HTTP headers to include",
			},
			"timeout": map[string]interface{}{
				"type":        "integer",
				"description": "Request timeout in seconds",
				"default":     30,
			},
		},
		Required: []string{"url"},
	}, handleHTTPOptions)
}

func setupResourceHandlers(server *mcpframework.Server) {
	// Set up resource listing for common HTTP endpoints
	server.SetResourceLister(func(ctx context.Context) (*mcpframework.ListResourcesResult, error) {
		resources := []mcpframework.Resource{
			{
				URI:         "http://httpbin.org/get",
				Name:        "HTTPBin GET Test",
				Description: "Test HTTP GET endpoint",
				MimeType:    "application/json",
			},
			{
				URI:         "http://httpbin.org/status/200",
				Name:        "HTTPBin Status Test",
				Description: "Test HTTP status endpoint",
				MimeType:    "text/plain",
			},
		}
		return &mcpframework.ListResourcesResult{Resources: resources}, nil
	})

	// Set up resource reading for HTTP URLs
	server.RegisterResourceHandler("http://*", func(ctx context.Context, uri string) (*mcpframework.ReadResourceResult, error) {
		return fetchHTTPResource(uri, 30*time.Second)
	})

	server.RegisterResourceHandler("https://*", func(ctx context.Context, uri string) (*mcpframework.ReadResourceResult, error) {
		return fetchHTTPResource(uri, 30*time.Second)
	})
}

func fetchHTTPResource(uri string, timeout time.Duration) (*mcpframework.ReadResourceResult, error) {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(uri)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return &mcpframework.ReadResourceResult{
		Contents: []interface{}{
			mcpframework.TextContent{
				Type: "text",
				Text: string(body),
			},
		},
	}, nil
}

func handleHTTPGet(ctx context.Context, params mcpframework.CallToolParams) (*mcpframework.CallToolResult, error) {
	var args struct {
		URL     string            `json:"url"`
		Headers map[string]string `json:"headers"`
		Timeout int               `json:"timeout"`
	}
	if err := json.Unmarshal(params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if args.Timeout == 0 {
		args.Timeout = 30
	}

	return performHTTPRequest("GET", args.URL, "", args.Headers, args.Timeout), nil
}

func handleHTTPPost(ctx context.Context, params mcpframework.CallToolParams) (*mcpframework.CallToolResult, error) {
	var args struct {
		URL         string            `json:"url"`
		Body        string            `json:"body"`
		Headers     map[string]string `json:"headers"`
		ContentType string            `json:"content_type"`
		Timeout     int               `json:"timeout"`
	}
	if err := json.Unmarshal(params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if args.Timeout == 0 {
		args.Timeout = 30
	}
	if args.ContentType == "" {
		args.ContentType = "application/json"
	}

	if args.Headers == nil {
		args.Headers = make(map[string]string)
	}
	args.Headers["Content-Type"] = args.ContentType

	return performHTTPRequest("POST", args.URL, args.Body, args.Headers, args.Timeout), nil
}

func handleHTTPPut(ctx context.Context, params mcpframework.CallToolParams) (*mcpframework.CallToolResult, error) {
	var args struct {
		URL         string            `json:"url"`
		Body        string            `json:"body"`
		Headers     map[string]string `json:"headers"`
		ContentType string            `json:"content_type"`
		Timeout     int               `json:"timeout"`
	}
	if err := json.Unmarshal(params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if args.Timeout == 0 {
		args.Timeout = 30
	}
	if args.ContentType == "" {
		args.ContentType = "application/json"
	}

	if args.Headers == nil {
		args.Headers = make(map[string]string)
	}
	args.Headers["Content-Type"] = args.ContentType

	return performHTTPRequest("PUT", args.URL, args.Body, args.Headers, args.Timeout), nil
}

func handleHTTPDelete(ctx context.Context, params mcpframework.CallToolParams) (*mcpframework.CallToolResult, error) {
	var args struct {
		URL     string            `json:"url"`
		Headers map[string]string `json:"headers"`
		Timeout int               `json:"timeout"`
	}
	if err := json.Unmarshal(params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if args.Timeout == 0 {
		args.Timeout = 30
	}

	return performHTTPRequest("DELETE", args.URL, "", args.Headers, args.Timeout), nil
}

func handleHTTPHead(ctx context.Context, params mcpframework.CallToolParams) (*mcpframework.CallToolResult, error) {
	var args struct {
		URL     string            `json:"url"`
		Headers map[string]string `json:"headers"`
		Timeout int               `json:"timeout"`
	}
	if err := json.Unmarshal(params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if args.Timeout == 0 {
		args.Timeout = 30
	}

	return performHTTPRequest("HEAD", args.URL, "", args.Headers, args.Timeout), nil
}

func handleHTTPOptions(ctx context.Context, params mcpframework.CallToolParams) (*mcpframework.CallToolResult, error) {
	var args struct {
		URL     string            `json:"url"`
		Headers map[string]string `json:"headers"`
		Timeout int               `json:"timeout"`
	}
	if err := json.Unmarshal(params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if args.Timeout == 0 {
		args.Timeout = 30
	}

	return performHTTPRequest("OPTIONS", args.URL, "", args.Headers, args.Timeout), nil
}

func handleParseURL(ctx context.Context, params mcpframework.CallToolParams) (*mcpframework.CallToolResult, error) {
	var args struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	parsedURL, err := url.Parse(args.URL)
	if err != nil {
		return &mcpframework.CallToolResult{
			Content: []interface{}{
				mcpframework.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error parsing URL: %s", err.Error()),
				},
			},
			IsError: true,
		}, nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Scheme: %s\n", parsedURL.Scheme))
	result.WriteString(fmt.Sprintf("Host: %s\n", parsedURL.Host))
	result.WriteString(fmt.Sprintf("Path: %s\n", parsedURL.Path))
	result.WriteString(fmt.Sprintf("RawQuery: %s\n", parsedURL.RawQuery))
	result.WriteString(fmt.Sprintf("Fragment: %s\n", parsedURL.Fragment))
	
	if parsedURL.User != nil {
		result.WriteString(fmt.Sprintf("User: %s\n", parsedURL.User.String()))
	}

	if parsedURL.Port() != "" {
		result.WriteString(fmt.Sprintf("Port: %s\n", parsedURL.Port()))
	}

	// Parse query parameters
	if parsedURL.RawQuery != "" {
		result.WriteString("\nQuery Parameters:\n")
		queryParams := parsedURL.Query()
		for key, values := range queryParams {
			for _, value := range values {
				result.WriteString(fmt.Sprintf("  %s: %s\n", key, value))
			}
		}
	}

	return &mcpframework.CallToolResult{
		Content: []interface{}{
			mcpframework.TextContent{
				Type: "text",
				Text: result.String(),
			},
		},
	}, nil
}

func performHTTPRequest(method, url, body string, headers map[string]string, timeoutSecs int) *mcpframework.CallToolResult {
	client := &http.Client{
		Timeout: time.Duration(timeoutSecs) * time.Second,
	}

	var bodyReader io.Reader
	if body != "" {
		bodyReader = bytes.NewBufferString(body)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return &mcpframework.CallToolResult{
			Content: []interface{}{
				mcpframework.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error creating request: %s", err.Error()),
				},
			},
			IsError: true,
		}
	}

	// Set headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// Perform the request
	resp, err := client.Do(req)
	if err != nil {
		return &mcpframework.CallToolResult{
			Content: []interface{}{
				mcpframework.TextContent{
					Type: "text",
					Text: fmt.Sprintf("HTTP request failed: %s", err.Error()),
				},
			},
			IsError: true,
		}
	}
	defer resp.Body.Close()

	// Read response body (except for HEAD requests)
	var responseBody string
	if method != "HEAD" {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return &mcpframework.CallToolResult{
				Content: []interface{}{
					mcpframework.TextContent{
						Type: "text",
						Text: fmt.Sprintf("Error reading response body: %s", err.Error()),
					},
				},
				IsError: true,
			}
		}
		responseBody = string(bodyBytes)
	}

	// Format response
	var result strings.Builder
	result.WriteString(fmt.Sprintf("Status: %s\n", resp.Status))
	result.WriteString(fmt.Sprintf("Status Code: %d\n", resp.StatusCode))
	
	result.WriteString("\nResponse Headers:\n")
	for key, values := range resp.Header {
		for _, value := range values {
			result.WriteString(fmt.Sprintf("  %s: %s\n", key, value))
		}
	}

	if method != "HEAD" && responseBody != "" {
		result.WriteString(fmt.Sprintf("\nContent Length: %s\n", resp.Header.Get("Content-Length")))
		if len(responseBody) > 0 {
			result.WriteString(fmt.Sprintf("\nResponse Body:\n%s", responseBody))
		}
	}

	// Determine if this is an error based on status code
	isError := resp.StatusCode >= 400

	return &mcpframework.CallToolResult{
		Content: []interface{}{
			mcpframework.TextContent{
				Type: "text",
				Text: result.String(),
			},
		},
		IsError: isError,
	}
}