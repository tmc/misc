package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/tmc/misc/go-mcp-servers/lib/mcpframework"
)

func main() {
	server := mcpframework.NewServer("git-mcp-server", "1.0.0")
	server.SetInstructions("A Model Context Protocol server that provides Git repository operations including status, commits, branches, and diffs.")

	// Register Git tools
	registerGitTools(server)
	setupResourceHandlers(server)

	// Run the server
	ctx := context.Background()
	if err := server.Run(ctx, os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}

func registerGitTools(server *mcpframework.Server) {
	// Git status tool
	server.RegisterTool("git_status", "Get the status of a Git repository", &mcpframework.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"repo_path": map[string]interface{}{
				"type":        "string",
				"description": "Path to the Git repository",
				"default":     ".",
			},
		},
	}, handleGitStatus)

	// Git log tool
	server.RegisterTool("git_log", "Get the commit history", &mcpframework.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"repo_path": map[string]interface{}{
				"type":        "string",
				"description": "Path to the Git repository",
				"default":     ".",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Number of commits to show",
				"default":     10,
			},
			"oneline": map[string]interface{}{
				"type":        "boolean",
				"description": "Show one line per commit",
				"default":     false,
			},
		},
	}, handleGitLog)

	// Git diff tool
	server.RegisterTool("git_diff", "Show differences between commits, commit and working tree, etc", &mcpframework.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"repo_path": map[string]interface{}{
				"type":        "string",
				"description": "Path to the Git repository",
				"default":     ".",
			},
			"staged": map[string]interface{}{
				"type":        "boolean",
				"description": "Show staged changes",
				"default":     false,
			},
			"file": map[string]interface{}{
				"type":        "string",
				"description": "Specific file to diff",
			},
		},
	}, handleGitDiff)

	// Git branches tool
	server.RegisterTool("git_branches", "List Git branches", &mcpframework.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"repo_path": map[string]interface{}{
				"type":        "string",
				"description": "Path to the Git repository",
				"default":     ".",
			},
			"remote": map[string]interface{}{
				"type":        "boolean",
				"description": "Include remote branches",
				"default":     false,
			},
		},
	}, handleGitBranches)

	// Git commit tool
	server.RegisterTool("git_commit", "Create a Git commit", &mcpframework.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"repo_path": map[string]interface{}{
				"type":        "string",
				"description": "Path to the Git repository",
				"default":     ".",
			},
			"message": map[string]interface{}{
				"type":        "string",
				"description": "Commit message",
			},
			"add_all": map[string]interface{}{
				"type":        "boolean",
				"description": "Add all tracked files before committing",
				"default":     false,
			},
		},
		Required: []string{"message"},
	}, handleGitCommit)

	// Git add tool
	server.RegisterTool("git_add", "Add files to the staging area", &mcpframework.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"repo_path": map[string]interface{}{
				"type":        "string",
				"description": "Path to the Git repository",
				"default":     ".",
			},
			"files": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
				"description": "Files to add (use ['.'] for all files)",
			},
		},
		Required: []string{"files"},
	}, handleGitAdd)

	// Git checkout tool
	server.RegisterTool("git_checkout", "Checkout a branch or commit", &mcpframework.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"repo_path": map[string]interface{}{
				"type":        "string",
				"description": "Path to the Git repository",
				"default":     ".",
			},
			"target": map[string]interface{}{
				"type":        "string",
				"description": "Branch name or commit hash to checkout",
			},
			"create_branch": map[string]interface{}{
				"type":        "boolean",
				"description": "Create a new branch",
				"default":     false,
			},
		},
		Required: []string{"target"},
	}, handleGitCheckout)

	// Git show tool
	server.RegisterTool("git_show", "Show details of a specific commit", &mcpframework.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"repo_path": map[string]interface{}{
				"type":        "string",
				"description": "Path to the Git repository",
				"default":     ".",
			},
			"commit": map[string]interface{}{
				"type":        "string",
				"description": "Commit hash or reference",
				"default":     "HEAD",
			},
		},
	}, handleGitShow)
}

func setupResourceHandlers(server *mcpframework.Server) {
	// Set up resource listing for Git-related files
	server.SetResourceLister(func(ctx context.Context) (*mcpframework.ListResourcesResult, error) {
		cwd, _ := os.Getwd()
		gitDir := filepath.Join(cwd, ".git")
		
		var resources []mcpframework.Resource
		
		// Check if this is a Git repository
		if _, err := os.Stat(gitDir); err == nil {
			resources = append(resources,
				mcpframework.Resource{
					URI:         "git://status",
					Name:        "Git Status",
					Description: "Current Git repository status",
					MimeType:    "text/plain",
				},
				mcpframework.Resource{
					URI:         "git://log",
					Name:        "Git Log",
					Description: "Git commit history",
					MimeType:    "text/plain",
				},
				mcpframework.Resource{
					URI:         "git://branches",
					Name:        "Git Branches",
					Description: "Git branches",
					MimeType:    "text/plain",
				},
			)
		}
		
		return &mcpframework.ListResourcesResult{Resources: resources}, nil
	})

	// Set up resource reading for Git data
	server.RegisterResourceHandler("git://*", func(ctx context.Context, uri string) (*mcpframework.ReadResourceResult, error) {
		switch {
		case strings.HasSuffix(uri, "/status"):
			return getGitResourceContent("status", ".")
		case strings.HasSuffix(uri, "/log"):
			return getGitResourceContent("log", ".", "--oneline", "-10")
		case strings.HasSuffix(uri, "/branches"):
			return getGitResourceContent("branch", ".", "-a")
		default:
			return nil, fmt.Errorf("unsupported Git resource")
		}
	})
}

func getGitResourceContent(subcommand, repoPath string, args ...string) (*mcpframework.ReadResourceResult, error) {
	cmdArgs := append([]string{subcommand}, args...)
	cmd := exec.Command("git", cmdArgs...)
	cmd.Dir = repoPath
	
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git command failed: %w", err)
	}

	return &mcpframework.ReadResourceResult{
		Contents: []interface{}{
			mcpframework.TextContent{
				Type: "text",
				Text: string(output),
			},
		},
	}, nil
}

func handleGitStatus(ctx context.Context, params mcpframework.CallToolParams) (*mcpframework.CallToolResult, error) {
	var args struct {
		RepoPath string `json:"repo_path"`
	}
	if err := json.Unmarshal(params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if args.RepoPath == "" {
		args.RepoPath = "."
	}

	return runGitCommand(args.RepoPath, "status", "--porcelain", "-b"), nil
}

func handleGitLog(ctx context.Context, params mcpframework.CallToolParams) (*mcpframework.CallToolResult, error) {
	var args struct {
		RepoPath string `json:"repo_path"`
		Limit    int    `json:"limit"`
		Oneline  bool   `json:"oneline"`
	}
	if err := json.Unmarshal(params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if args.RepoPath == "" {
		args.RepoPath = "."
	}
	if args.Limit == 0 {
		args.Limit = 10
	}

	gitArgs := []string{"log", fmt.Sprintf("-%d", args.Limit)}
	if args.Oneline {
		gitArgs = append(gitArgs, "--oneline")
	} else {
		gitArgs = append(gitArgs, "--pretty=format:%h - %an, %ar : %s")
	}

	return runGitCommand(args.RepoPath, gitArgs...), nil
}

func handleGitDiff(ctx context.Context, params mcpframework.CallToolParams) (*mcpframework.CallToolResult, error) {
	var args struct {
		RepoPath string `json:"repo_path"`
		Staged   bool   `json:"staged"`
		File     string `json:"file"`
	}
	if err := json.Unmarshal(params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if args.RepoPath == "" {
		args.RepoPath = "."
	}

	gitArgs := []string{"diff"}
	if args.Staged {
		gitArgs = append(gitArgs, "--staged")
	}
	if args.File != "" {
		gitArgs = append(gitArgs, args.File)
	}

	return runGitCommand(args.RepoPath, gitArgs...), nil
}

func handleGitBranches(ctx context.Context, params mcpframework.CallToolParams) (*mcpframework.CallToolResult, error) {
	var args struct {
		RepoPath string `json:"repo_path"`
		Remote   bool   `json:"remote"`
	}
	if err := json.Unmarshal(params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if args.RepoPath == "" {
		args.RepoPath = "."
	}

	gitArgs := []string{"branch"}
	if args.Remote {
		gitArgs = append(gitArgs, "-a")
	}

	return runGitCommand(args.RepoPath, gitArgs...), nil
}

func handleGitCommit(ctx context.Context, params mcpframework.CallToolParams) (*mcpframework.CallToolResult, error) {
	var args struct {
		RepoPath string `json:"repo_path"`
		Message  string `json:"message"`
		AddAll   bool   `json:"add_all"`
	}
	if err := json.Unmarshal(params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if args.RepoPath == "" {
		args.RepoPath = "."
	}

	// Add all files if requested
	if args.AddAll {
		if result := runGitCommand(args.RepoPath, "add", "-A"); result.IsError {
			return result, nil
		}
	}

	return runGitCommand(args.RepoPath, "commit", "-m", args.Message), nil
}

func handleGitAdd(ctx context.Context, params mcpframework.CallToolParams) (*mcpframework.CallToolResult, error) {
	var args struct {
		RepoPath string   `json:"repo_path"`
		Files    []string `json:"files"`
	}
	if err := json.Unmarshal(params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if args.RepoPath == "" {
		args.RepoPath = "."
	}

	gitArgs := append([]string{"add"}, args.Files...)
	return runGitCommand(args.RepoPath, gitArgs...), nil
}

func handleGitCheckout(ctx context.Context, params mcpframework.CallToolParams) (*mcpframework.CallToolResult, error) {
	var args struct {
		RepoPath     string `json:"repo_path"`
		Target       string `json:"target"`
		CreateBranch bool   `json:"create_branch"`
	}
	if err := json.Unmarshal(params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if args.RepoPath == "" {
		args.RepoPath = "."
	}

	gitArgs := []string{"checkout"}
	if args.CreateBranch {
		gitArgs = append(gitArgs, "-b")
	}
	gitArgs = append(gitArgs, args.Target)

	return runGitCommand(args.RepoPath, gitArgs...), nil
}

func handleGitShow(ctx context.Context, params mcpframework.CallToolParams) (*mcpframework.CallToolResult, error) {
	var args struct {
		RepoPath string `json:"repo_path"`
		Commit   string `json:"commit"`
	}
	if err := json.Unmarshal(params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if args.RepoPath == "" {
		args.RepoPath = "."
	}
	if args.Commit == "" {
		args.Commit = "HEAD"
	}

	return runGitCommand(args.RepoPath, "show", args.Commit), nil
}

func runGitCommand(repoPath string, args ...string) *mcpframework.CallToolResult {
	cmd := exec.Command("git", args...)
	cmd.Dir = repoPath
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return &mcpframework.CallToolResult{
			Content: []interface{}{
				mcpframework.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Git command failed: %s\nOutput: %s", err.Error(), string(output)),
				},
			},
			IsError: true,
		}
	}

	return &mcpframework.CallToolResult{
		Content: []interface{}{
			mcpframework.TextContent{
				Type: "text",
				Text: string(output),
			},
		},
	}
}