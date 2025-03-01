package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/tmc/mcp"
)

type readFileTool struct {
	allowedDirs []string
}

func (t *readFileTool) Name() string {
	return "read_file"
}

func (t *readFileTool) Description() string {
	return "Read the complete contents of a file from the file system. " +
		"Handles various text encodings and provides detailed error messages " +
		"if the file cannot be read."
}

func (t *readFileTool) Handler(ctx context.Context, args json.RawMessage) (*mcp.ToolResult, error) {
	var params struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %v", err)
	}

	// Clean and validate path
	cleanPath := filepath.Clean(params.Path)
	if strings.Contains(cleanPath, "..") {
		return nil, fmt.Errorf("path not allowed")
	}

	// Check if path is in allowed directories
	allowed := false
	for _, dir := range t.allowedDirs {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		absPath, err := filepath.Abs(cleanPath)
		if err != nil {
			continue
		}
		if strings.HasPrefix(absPath, absDir) {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, fmt.Errorf("path not allowed")
	}

	// Read file
	data, err := os.ReadFile(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", params.Path)
		}
		return nil, fmt.Errorf("error reading file: %v", err)
	}

	return &mcp.ToolResult{
		Content: []mcp.Content{{
			Type: "text",
			Text: string(data),
		}},
	}, nil
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags] <allowed-directory> [additional-directories...]\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	if len(flag.Args()) < 1 {
		flag.Usage()
		os.Exit(1)
	}

	// Create server
	srv := mcp.NewServer("filesystem-server", "1.0.0")

	// Register read_file tool
	tool := &readFileTool{allowedDirs: flag.Args()}
	if err := srv.RegisterTool(tool); err != nil {
		log.Fatal(err)
	}

	// Create transport and start server
	ctx := context.Background()
	transport := mcp.NewStdioTransport(ctx)
	defer transport.Close()

	// Process messages
	buf := make([]byte, 4096)
	for {
		n, err := transport.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}

		resp, err := srv.Handle(ctx, buf[:n])
		if err != nil {
			log.Printf("error handling message: %v", err)
			continue
		}

		if _, err := transport.Write(append(resp, '\n')); err != nil {
			log.Fatal(err)
		}
	}
}
