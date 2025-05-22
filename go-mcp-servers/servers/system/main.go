package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/tmc/misc/go-mcp-servers/lib/mcpframework"
)

func main() {
	server := mcpframework.NewServer("system-mcp-server", "1.0.0")
	server.SetInstructions("A Model Context Protocol server that provides system utilities including process management, system information, and shell command execution.")

	// Register system tools
	registerSystemTools(server)
	setupResourceHandlers(server)

	// Run the server
	ctx := context.Background()
	if err := server.Run(ctx, os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}

func registerSystemTools(server *mcpframework.Server) {
	// Execute shell command tool
	server.RegisterTool("exec_command", "Execute a shell command", &mcpframework.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"command": map[string]interface{}{
				"type":        "string",
				"description": "The command to execute",
			},
			"args": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
				"description": "Command arguments",
			},
			"timeout": map[string]interface{}{
				"type":        "integer",
				"description": "Command timeout in seconds",
				"default":     30,
			},
			"working_dir": map[string]interface{}{
				"type":        "string",
				"description": "Working directory for the command",
			},
		},
		Required: []string{"command"},
	}, handleExecCommand)

	// Get system information tool
	server.RegisterTool("system_info", "Get system information", &mcpframework.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"detailed": map[string]interface{}{
				"type":        "boolean",
				"description": "Include detailed system information",
				"default":     false,
			},
		},
	}, handleSystemInfo)

	// List processes tool
	server.RegisterTool("list_processes", "List running processes", &mcpframework.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"filter": map[string]interface{}{
				"type":        "string",
				"description": "Filter processes by name or command",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of processes to return",
				"default":     50,
			},
		},
	}, handleListProcesses)

	// Kill process tool
	server.RegisterTool("kill_process", "Kill a process by PID", &mcpframework.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"pid": map[string]interface{}{
				"type":        "integer",
				"description": "Process ID to kill",
			},
			"signal": map[string]interface{}{
				"type":        "string",
				"description": "Signal to send (TERM, KILL, etc.)",
				"default":     "TERM",
			},
		},
		Required: []string{"pid"},
	}, handleKillProcess)

	// Get environment variables tool
	server.RegisterTool("get_env", "Get environment variables", &mcpframework.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"var_name": map[string]interface{}{
				"type":        "string",
				"description": "Specific environment variable name (optional)",
			},
			"filter": map[string]interface{}{
				"type":        "string",
				"description": "Filter variables by prefix",
			},
		},
	}, handleGetEnv)

	// Check disk usage tool
	server.RegisterTool("disk_usage", "Check disk usage", &mcpframework.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Path to check disk usage for",
				"default":     "/",
			},
		},
	}, handleDiskUsage)

	// Network information tool
	server.RegisterTool("network_info", "Get network interface information", &mcpframework.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"interface": map[string]interface{}{
				"type":        "string",
				"description": "Specific network interface (optional)",
			},
		},
	}, handleNetworkInfo)

	// Check service status tool
	server.RegisterTool("service_status", "Check service status (systemd/launchd)", &mcpframework.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"service_name": map[string]interface{}{
				"type":        "string",
				"description": "Name of the service to check",
			},
		},
		Required: []string{"service_name"},
	}, handleServiceStatus)
}

func setupResourceHandlers(server *mcpframework.Server) {
	// Set up resource listing for system information
	server.SetResourceLister(func(ctx context.Context) (*mcpframework.ListResourcesResult, error) {
		resources := []mcpframework.Resource{
			{
				URI:         "system://info",
				Name:        "System Information",
				Description: "Basic system information",
				MimeType:    "text/plain",
			},
			{
				URI:         "system://processes",
				Name:        "Running Processes",
				Description: "List of running processes",
				MimeType:    "text/plain",
			},
			{
				URI:         "system://env",
				Name:        "Environment Variables",
				Description: "System environment variables",
				MimeType:    "text/plain",
			},
		}
		return &mcpframework.ListResourcesResult{Resources: resources}, nil
	})

	// Set up resource reading for system data
	server.RegisterResourceHandler("system://*", func(ctx context.Context, uri string) (*mcpframework.ReadResourceResult, error) {
		switch {
		case strings.HasSuffix(uri, "/info"):
			return getSystemResourceContent("info")
		case strings.HasSuffix(uri, "/processes"):
			return getSystemResourceContent("processes")
		case strings.HasSuffix(uri, "/env"):
			return getSystemResourceContent("env")
		default:
			return nil, fmt.Errorf("unsupported system resource")
		}
	})
}

func getSystemResourceContent(resourceType string) (*mcpframework.ReadResourceResult, error) {
	var content string
	
	switch resourceType {
	case "info":
		content = fmt.Sprintf("OS: %s\nArchitecture: %s\nCPUs: %d\nGo Version: %s",
			runtime.GOOS, runtime.GOARCH, runtime.NumCPU(), runtime.Version())
	case "processes":
		if runtime.GOOS == "windows" {
			content = "Process listing not available on Windows"
		} else {
			cmd := exec.Command("ps", "aux")
			output, err := cmd.Output()
			if err != nil {
				return nil, err
			}
			content = string(output)
		}
	case "env":
		vars := os.Environ()
		content = strings.Join(vars, "\n")
	default:
		return nil, fmt.Errorf("unknown resource type")
	}

	return &mcpframework.ReadResourceResult{
		Contents: []interface{}{
			mcpframework.TextContent{
				Type: "text",
				Text: content,
			},
		},
	}, nil
}

func handleExecCommand(ctx context.Context, params mcpframework.CallToolParams) (*mcpframework.CallToolResult, error) {
	var args struct {
		Command    string   `json:"command"`
		Args       []string `json:"args"`
		Timeout    int      `json:"timeout"`
		WorkingDir string   `json:"working_dir"`
	}
	if err := json.Unmarshal(params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if args.Timeout == 0 {
		args.Timeout = 30
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(args.Timeout)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, args.Command, args.Args...)
	if args.WorkingDir != "" {
		cmd.Dir = args.WorkingDir
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return &mcpframework.CallToolResult{
			Content: []interface{}{
				mcpframework.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Command failed: %s\nOutput: %s", err.Error(), string(output)),
				},
			},
			IsError: true,
		}, nil
	}

	return &mcpframework.CallToolResult{
		Content: []interface{}{
			mcpframework.TextContent{
				Type: "text",
				Text: string(output),
			},
		},
	}, nil
}

func handleSystemInfo(ctx context.Context, params mcpframework.CallToolParams) (*mcpframework.CallToolResult, error) {
	var args struct {
		Detailed bool `json:"detailed"`
	}
	if err := json.Unmarshal(params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Operating System: %s\n", runtime.GOOS))
	result.WriteString(fmt.Sprintf("Architecture: %s\n", runtime.GOARCH))
	result.WriteString(fmt.Sprintf("CPU Count: %d\n", runtime.NumCPU()))
	result.WriteString(fmt.Sprintf("Go Version: %s\n", runtime.Version()))

	if args.Detailed {
		// Add more detailed information
		hostname, err := os.Hostname()
		if err == nil {
			result.WriteString(fmt.Sprintf("Hostname: %s\n", hostname))
		}

		cwd, err := os.Getwd()
		if err == nil {
			result.WriteString(fmt.Sprintf("Current Directory: %s\n", cwd))
		}

		// Memory statistics
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		result.WriteString(fmt.Sprintf("Memory Allocated: %d KB\n", m.Alloc/1024))
		result.WriteString(fmt.Sprintf("Total Allocations: %d KB\n", m.TotalAlloc/1024))
		result.WriteString(fmt.Sprintf("System Memory: %d KB\n", m.Sys/1024))
		result.WriteString(fmt.Sprintf("GC Runs: %d\n", m.NumGC))
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

func handleListProcesses(ctx context.Context, params mcpframework.CallToolParams) (*mcpframework.CallToolResult, error) {
	var args struct {
		Filter string `json:"filter"`
		Limit  int    `json:"limit"`
	}
	if err := json.Unmarshal(params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if args.Limit == 0 {
		args.Limit = 50
	}

	// Platform-specific process listing
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("tasklist", "/fo", "csv")
	case "darwin", "linux":
		if args.Filter != "" {
			cmd = exec.Command("ps", "aux")
		} else {
			cmd = exec.Command("ps", "aux")
		}
	default:
		return &mcpframework.CallToolResult{
			Content: []interface{}{
				mcpframework.TextContent{
					Type: "text",
					Text: "Process listing not supported on this platform",
				},
			},
			IsError: true,
		}, nil
	}

	output, err := cmd.Output()
	if err != nil {
		return &mcpframework.CallToolResult{
			Content: []interface{}{
				mcpframework.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Failed to list processes: %s", err.Error()),
				},
			},
			IsError: true,
		}, nil
	}

	result := string(output)
	if args.Filter != "" {
		lines := strings.Split(result, "\n")
		var filteredLines []string
		count := 0
		for _, line := range lines {
			if count >= args.Limit {
				break
			}
			if strings.Contains(strings.ToLower(line), strings.ToLower(args.Filter)) {
				filteredLines = append(filteredLines, line)
				count++
			}
		}
		result = strings.Join(filteredLines, "\n")
	}

	return &mcpframework.CallToolResult{
		Content: []interface{}{
			mcpframework.TextContent{
				Type: "text",
				Text: result,
			},
		},
	}, nil
}

func handleKillProcess(ctx context.Context, params mcpframework.CallToolParams) (*mcpframework.CallToolResult, error) {
	var args struct {
		PID    int    `json:"pid"`
		Signal string `json:"signal"`
	}
	if err := json.Unmarshal(params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if args.Signal == "" {
		args.Signal = "TERM"
	}

	// Find the process
	process, err := os.FindProcess(args.PID)
	if err != nil {
		return &mcpframework.CallToolResult{
			Content: []interface{}{
				mcpframework.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Process not found: %s", err.Error()),
				},
			},
			IsError: true,
		}, nil
	}

	// Determine signal
	var sig syscall.Signal
	switch strings.ToUpper(args.Signal) {
	case "TERM":
		sig = syscall.SIGTERM
	case "KILL":
		sig = syscall.SIGKILL
	case "INT":
		sig = syscall.SIGINT
	case "HUP":
		sig = syscall.SIGHUP
	default:
		return &mcpframework.CallToolResult{
			Content: []interface{}{
				mcpframework.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Unsupported signal: %s", args.Signal),
				},
			},
			IsError: true,
		}, nil
	}

	// Send signal
	err = process.Signal(sig)
	if err != nil {
		return &mcpframework.CallToolResult{
			Content: []interface{}{
				mcpframework.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Failed to send signal: %s", err.Error()),
				},
			},
			IsError: true,
		}, nil
	}

	return &mcpframework.CallToolResult{
		Content: []interface{}{
			mcpframework.TextContent{
				Type: "text",
				Text: fmt.Sprintf("Signal %s sent to process %d", args.Signal, args.PID),
			},
		},
	}, nil
}

func handleGetEnv(ctx context.Context, params mcpframework.CallToolParams) (*mcpframework.CallToolResult, error) {
	var args struct {
		VarName string `json:"var_name"`
		Filter  string `json:"filter"`
	}
	if err := json.Unmarshal(params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	var result strings.Builder

	if args.VarName != "" {
		// Get specific environment variable
		value := os.Getenv(args.VarName)
		if value == "" {
			result.WriteString(fmt.Sprintf("%s: (not set)", args.VarName))
		} else {
			result.WriteString(fmt.Sprintf("%s: %s", args.VarName, value))
		}
	} else {
		// Get all environment variables
		vars := os.Environ()
		for _, envVar := range vars {
			if args.Filter == "" || strings.Contains(strings.ToLower(envVar), strings.ToLower(args.Filter)) {
				result.WriteString(envVar + "\n")
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

func handleDiskUsage(ctx context.Context, params mcpframework.CallToolParams) (*mcpframework.CallToolResult, error) {
	var args struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if args.Path == "" {
		if runtime.GOOS == "windows" {
			args.Path = "C:\\"
		} else {
			args.Path = "/"
		}
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("dir", args.Path, "/-c")
	case "darwin", "linux":
		cmd = exec.Command("df", "-h", args.Path)
	default:
		return &mcpframework.CallToolResult{
			Content: []interface{}{
				mcpframework.TextContent{
					Type: "text",
					Text: "Disk usage not supported on this platform",
				},
			},
			IsError: true,
		}, nil
	}

	output, err := cmd.Output()
	if err != nil {
		return &mcpframework.CallToolResult{
			Content: []interface{}{
				mcpframework.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Failed to get disk usage: %s", err.Error()),
				},
			},
			IsError: true,
		}, nil
	}

	return &mcpframework.CallToolResult{
		Content: []interface{}{
			mcpframework.TextContent{
				Type: "text",
				Text: string(output),
			},
		},
	}, nil
}

func handleNetworkInfo(ctx context.Context, params mcpframework.CallToolParams) (*mcpframework.CallToolResult, error) {
	var args struct {
		Interface string `json:"interface"`
	}
	if err := json.Unmarshal(params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("ipconfig", "/all")
	case "darwin":
		if args.Interface != "" {
			cmd = exec.Command("ifconfig", args.Interface)
		} else {
			cmd = exec.Command("ifconfig")
		}
	case "linux":
		if args.Interface != "" {
			cmd = exec.Command("ip", "addr", "show", args.Interface)
		} else {
			cmd = exec.Command("ip", "addr", "show")
		}
	default:
		return &mcpframework.CallToolResult{
			Content: []interface{}{
				mcpframework.TextContent{
					Type: "text",
					Text: "Network information not supported on this platform",
				},
			},
			IsError: true,
		}, nil
	}

	output, err := cmd.Output()
	if err != nil {
		return &mcpframework.CallToolResult{
			Content: []interface{}{
				mcpframework.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Failed to get network information: %s", err.Error()),
				},
			},
			IsError: true,
		}, nil
	}

	return &mcpframework.CallToolResult{
		Content: []interface{}{
			mcpframework.TextContent{
				Type: "text",
				Text: string(output),
			},
		},
	}, nil
}

func handleServiceStatus(ctx context.Context, params mcpframework.CallToolParams) (*mcpframework.CallToolResult, error) {
	var args struct {
		ServiceName string `json:"service_name"`
	}
	if err := json.Unmarshal(params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("systemctl", "status", args.ServiceName)
	case "darwin":
		cmd = exec.Command("launchctl", "list", args.ServiceName)
	case "windows":
		cmd = exec.Command("sc", "query", args.ServiceName)
	default:
		return &mcpframework.CallToolResult{
			Content: []interface{}{
				mcpframework.TextContent{
					Type: "text",
					Text: "Service status not supported on this platform",
				},
			},
			IsError: true,
		}, nil
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return &mcpframework.CallToolResult{
			Content: []interface{}{
				mcpframework.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Service status check failed: %s\nOutput: %s", err.Error(), string(output)),
				},
			},
			IsError: true,
		}, nil
	}

	return &mcpframework.CallToolResult{
		Content: []interface{}{
			mcpframework.TextContent{
				Type: "text",
				Text: string(output),
			},
		},
	}, nil
}