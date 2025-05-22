package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/tmc/misc/go-mcp-servers/lib/mcpframework"
)

func main() {
	server := mcpframework.NewServer("filesystem-mcp-server", "1.0.0")
	server.SetInstructions("A Model Context Protocol server that provides filesystem operations including reading files, listing directories, and searching for files.")

	// Register filesystem tools
	registerFileSystemTools(server)
	setupResourceHandlers(server)

	// Run the server
	ctx := context.Background()
	if err := server.Run(ctx, os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}

func registerFileSystemTools(server *mcpframework.Server) {
	// Read file tool
	server.RegisterTool("read_file", "Read the contents of a file", &mcpframework.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "The path to the file to read",
			},
		},
		Required: []string{"path"},
	}, handleReadFile)

	// List directory tool
	server.RegisterTool("list_directory", "List the contents of a directory", &mcpframework.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "The path to the directory to list",
			},
			"recursive": map[string]interface{}{
				"type":        "boolean",
				"description": "Whether to list recursively",
				"default":     false,
			},
		},
		Required: []string{"path"},
	}, handleListDirectory)

	// Search files tool
	server.RegisterTool("search_files", "Search for files matching a pattern", &mcpframework.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"pattern": map[string]interface{}{
				"type":        "string",
				"description": "Glob pattern to search for",
			},
			"root": map[string]interface{}{
				"type":        "string",
				"description": "Root directory to search from",
				"default":     ".",
			},
		},
		Required: []string{"pattern"},
	}, handleSearchFiles)

	// Write file tool
	server.RegisterTool("write_file", "Write content to a file", &mcpframework.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "The path to the file to write",
			},
			"content": map[string]interface{}{
				"type":        "string",
				"description": "The content to write to the file",
			},
		},
		Required: []string{"path", "content"},
	}, handleWriteFile)

	// Create directory tool
	server.RegisterTool("create_directory", "Create a directory", &mcpframework.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "The path to the directory to create",
			},
			"recursive": map[string]interface{}{
				"type":        "boolean",
				"description": "Whether to create parent directories",
				"default":     false,
			},
		},
		Required: []string{"path"},
	}, handleCreateDirectory)

	// Delete file/directory tool
	server.RegisterTool("delete", "Delete a file or directory", &mcpframework.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "The path to delete",
			},
			"recursive": map[string]interface{}{
				"type":        "boolean",
				"description": "Whether to delete recursively (for directories)",
				"default":     false,
			},
		},
		Required: []string{"path"},
	}, handleDelete)
}

func setupResourceHandlers(server *mcpframework.Server) {
	// Set up resource listing
	server.SetResourceLister(func(ctx context.Context) (*mcpframework.ListResourcesResult, error) {
		// For demonstration, list some common files
		cwd, _ := os.Getwd()
		resources := []mcpframework.Resource{
			{
				URI:         "file://" + filepath.Join(cwd, "README.md"),
				Name:        "README.md",
				Description: "Project README file",
				MimeType:    "text/markdown",
			},
			{
				URI:         "file://" + filepath.Join(cwd, "go.mod"),
				Name:        "go.mod",
				Description: "Go module file",
				MimeType:    "text/plain",
			},
		}
		return &mcpframework.ListResourcesResult{Resources: resources}, nil
	})

	// Set up resource reading
	server.RegisterResourceHandler("*", func(ctx context.Context, uri string) (*mcpframework.ReadResourceResult, error) {
		if !strings.HasPrefix(uri, "file://") {
			return nil, fmt.Errorf("unsupported URI scheme")
		}
		
		path := strings.TrimPrefix(uri, "file://")
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}

		return &mcpframework.ReadResourceResult{
			Contents: []interface{}{
				mcpframework.TextContent{
					Type: "text",
					Text: string(content),
				},
			},
		}, nil
	})
}

func handleReadFile(ctx context.Context, params mcpframework.CallToolParams) (*mcpframework.CallToolResult, error) {
	var args struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	content, err := os.ReadFile(args.Path)
	if err != nil {
		return &mcpframework.CallToolResult{
			Content: []interface{}{
				mcpframework.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error reading file: %s", err.Error()),
				},
			},
			IsError: true,
		}, nil
	}

	return &mcpframework.CallToolResult{
		Content: []interface{}{
			mcpframework.TextContent{
				Type: "text",
				Text: string(content),
			},
		},
	}, nil
}

func handleListDirectory(ctx context.Context, params mcpframework.CallToolParams) (*mcpframework.CallToolResult, error) {
	var args struct {
		Path      string `json:"path"`
		Recursive bool   `json:"recursive"`
	}
	if err := json.Unmarshal(params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	var result strings.Builder

	if args.Recursive {
		err := filepath.WalkDir(args.Path, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			
			prefix := ""
			if d.IsDir() {
				prefix = "DIR  "
			} else {
				prefix = "FILE "
			}
			
			result.WriteString(fmt.Sprintf("%s %s (%d bytes)\n", prefix, path, info.Size()))
			return nil
		})
		if err != nil {
			return &mcpframework.CallToolResult{
				Content: []interface{}{
					mcpframework.TextContent{
						Type: "text",
						Text: fmt.Sprintf("Error listing directory: %s", err.Error()),
					},
				},
				IsError: true,
			}, nil
		}
	} else {
		entries, err := os.ReadDir(args.Path)
		if err != nil {
			return &mcpframework.CallToolResult{
				Content: []interface{}{
					mcpframework.TextContent{
						Type: "text",
						Text: fmt.Sprintf("Error listing directory: %s", err.Error()),
					},
				},
				IsError: true,
			}, nil
		}

		for _, entry := range entries {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			
			prefix := ""
			if entry.IsDir() {
				prefix = "DIR  "
			} else {
				prefix = "FILE "
			}
			
			result.WriteString(fmt.Sprintf("%s %s (%d bytes)\n", prefix, entry.Name(), info.Size()))
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

func handleSearchFiles(ctx context.Context, params mcpframework.CallToolParams) (*mcpframework.CallToolResult, error) {
	var args struct {
		Pattern string `json:"pattern"`
		Root    string `json:"root"`
	}
	if err := json.Unmarshal(params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if args.Root == "" {
		args.Root = "."
	}

	matches, err := filepath.Glob(filepath.Join(args.Root, args.Pattern))
	if err != nil {
		return &mcpframework.CallToolResult{
			Content: []interface{}{
				mcpframework.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error searching files: %s", err.Error()),
				},
			},
			IsError: true,
		}, nil
	}

	var result strings.Builder
	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil {
			continue
		}
		
		prefix := ""
		if info.IsDir() {
			prefix = "DIR  "
		} else {
			prefix = "FILE "
		}
		
		result.WriteString(fmt.Sprintf("%s %s (%d bytes)\n", prefix, match, info.Size()))
	}

	if result.Len() == 0 {
		result.WriteString("No files found matching pattern")
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

func handleWriteFile(ctx context.Context, params mcpframework.CallToolParams) (*mcpframework.CallToolResult, error) {
	var args struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	err := os.WriteFile(args.Path, []byte(args.Content), 0644)
	if err != nil {
		return &mcpframework.CallToolResult{
			Content: []interface{}{
				mcpframework.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error writing file: %s", err.Error()),
				},
			},
			IsError: true,
		}, nil
	}

	return &mcpframework.CallToolResult{
		Content: []interface{}{
			mcpframework.TextContent{
				Type: "text",
				Text: fmt.Sprintf("Successfully wrote %d bytes to %s", len(args.Content), args.Path),
			},
		},
	}, nil
}

func handleCreateDirectory(ctx context.Context, params mcpframework.CallToolParams) (*mcpframework.CallToolResult, error) {
	var args struct {
		Path      string `json:"path"`
		Recursive bool   `json:"recursive"`
	}
	if err := json.Unmarshal(params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	var err error
	if args.Recursive {
		err = os.MkdirAll(args.Path, 0755)
	} else {
		err = os.Mkdir(args.Path, 0755)
	}

	if err != nil {
		return &mcpframework.CallToolResult{
			Content: []interface{}{
				mcpframework.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error creating directory: %s", err.Error()),
				},
			},
			IsError: true,
		}, nil
	}

	return &mcpframework.CallToolResult{
		Content: []interface{}{
			mcpframework.TextContent{
				Type: "text",
				Text: fmt.Sprintf("Successfully created directory: %s", args.Path),
			},
		},
	}, nil
}

func handleDelete(ctx context.Context, params mcpframework.CallToolParams) (*mcpframework.CallToolResult, error) {
	var args struct {
		Path      string `json:"path"`
		Recursive bool   `json:"recursive"`
	}
	if err := json.Unmarshal(params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	var err error
	if args.Recursive {
		err = os.RemoveAll(args.Path)
	} else {
		err = os.Remove(args.Path)
	}

	if err != nil {
		return &mcpframework.CallToolResult{
			Content: []interface{}{
				mcpframework.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error deleting: %s", err.Error()),
				},
			},
			IsError: true,
		}, nil
	}

	return &mcpframework.CallToolResult{
		Content: []interface{}{
			mcpframework.TextContent{
				Type: "text",
				Text: fmt.Sprintf("Successfully deleted: %s", args.Path),
			},
		},
	}, nil
}