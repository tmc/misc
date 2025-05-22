package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	ctx "github.com/tmc/misc/ctx-plugins"
)

// Version information
const (
	toolName    = "ctx-example"
	toolVersion = "0.1.0"
)

// Tool-specific options
type exampleOptions struct {
	ctx.Options
	IncludeHidden bool   // Include hidden files
	MaxDepth      int    // Maximum directory depth
	Pattern       string // File pattern to match
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--capabilities" {
		printCapabilities()
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "--plan-relevance" {
		printPlanRelevance()
		return
	}

	if len(os.Args) > 1 && (os.Args[1] == "--help" || os.Args[1] == "-h") {
		printHelp()
		return
	}

	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Printf("%s version %s\n", toolName, toolVersion)
		return
	}

	options := parseFlags()

	// If no arguments, use current directory
	args := flag.Args()
	if len(args) == 0 {
		args = []string{"."}
	}

	// Gather directory information
	path := args[0]
	cmd := fmt.Sprintf("Listing files in %s", path)
	stdout, stderr, err := gatherDirectoryInfo(path, options)

	// Format and print output
	ctx.PrintOutput(options.Options, toolName, cmd, stdout, stderr, err)

	// Exit with error code if command failed and exit-code is set
	if err != nil {
		os.Exit(1)
	}
}

// printCapabilities outputs the tool's capabilities in JSON format
func printCapabilities() {
	capabilities := ctx.Capabilities{
		Name:        toolName,
		Description: "Example context tool that lists directory information",
		Version:     toolVersion,
		Author:      "Claude",
		Flags: []ctx.Flag{
			{Name: "escape", Type: "bool", Description: "Enable escaping of special characters"},
			{Name: "json", Type: "bool", Description: "Output in JSON format"},
			{Name: "tag", Type: "string", Description: "Override the output tag name"},
			{Name: "include-hidden", Type: "bool", Description: "Include hidden files in output"},
			{Name: "max-depth", Type: "int", Description: "Maximum directory depth", Default: "1"},
			{Name: "pattern", Type: "string", Description: "File pattern to match", Default: "*"},
		},
		EnvVars: []ctx.EnvironmentVar{
			{Name: "CTX_TOOL_ESCAPE", Type: "bool", Description: "Enable escaping of special characters"},
			{Name: "CTX_TOOL_JSON", Type: "bool", Description: "Output in JSON format"},
			{Name: "CTX_TOOL_TAG", Type: "string", Description: "Override the output tag name"},
			{Name: "CTX_EXAMPLE_INCLUDE_HIDDEN", Type: "bool", Description: "Include hidden files in output"},
			{Name: "CTX_EXAMPLE_MAX_DEPTH", Type: "int", Description: "Maximum directory depth", Default: "1"},
			{Name: "CTX_EXAMPLE_PATTERN", Type: "string", Description: "File pattern to match", Default: "*"},
		},
		Relevance: ctx.RelevanceScores{
			Repo:       0.3,
			Filesystem: 0.9,
			Language: map[string]float64{
				"go":         0.4,
				"python":     0.4,
				"javascript": 0.4,
				"java":       0.4,
				"c":          0.4,
				"cpp":        0.4,
			},
		},
	}

	fmt.Printf("%s\n", ctx.FormatJSON(capabilities))
}

// printPlanRelevance outputs the tool's relevance to the current context
func printPlanRelevance() {
	// Check if we're in a directory with lots of files
	files, err := filepath.Glob("*")
	relevance := ctx.PlanRelevance{
		Score:  0.2,
		Reason: "Basic directory information is generally useful",
	}

	if err == nil && len(files) > 10 {
		relevance.Score = 0.6
		relevance.Reason = "Directory contains many files, listing would be helpful"
	}

	// Check if we're in a git repo
	_, err = exec.Command("git", "rev-parse", "--is-inside-work-tree").Output()
	if err == nil {
		relevance.Score = 0.7
		relevance.Reason = "Directory is inside a git repository, listing would be helpful"
	}

	fmt.Printf("%s\n", ctx.FormatJSON(relevance))
}

// printHelp outputs usage information
func printHelp() {
	fmt.Printf("Usage: %s [options] [directory]\n\n", toolName)
	fmt.Printf("Options:\n")
	fmt.Printf("  --escape            Enable escaping of special characters\n")
	fmt.Printf("  --json              Output in JSON format\n")
	fmt.Printf("  --tag <name>        Override the output tag name\n")
	fmt.Printf("  --include-hidden    Include hidden files in output\n")
	fmt.Printf("  --max-depth <n>     Maximum directory depth (default: 1)\n")
	fmt.Printf("  --pattern <pattern> File pattern to match (default: *)\n")
	fmt.Printf("  --capabilities      Display tool capabilities\n")
	fmt.Printf("  --plan-relevance    Display relevance score for current context\n")
	fmt.Printf("  --version, -v       Display version information\n")
	fmt.Printf("  --help, -h          Display this help message\n\n")
	fmt.Printf("Environment variables:\n")
	fmt.Printf("  CTX_TOOL_ESCAPE              Enable escaping of special characters\n")
	fmt.Printf("  CTX_TOOL_JSON                Output in JSON format\n")
	fmt.Printf("  CTX_TOOL_TAG                 Override the output tag name\n")
	fmt.Printf("  CTX_EXAMPLE_INCLUDE_HIDDEN   Include hidden files in output\n")
	fmt.Printf("  CTX_EXAMPLE_MAX_DEPTH        Maximum directory depth\n")
	fmt.Printf("  CTX_EXAMPLE_PATTERN          File pattern to match\n")
}

// parseFlags parses command-line flags and environment variables
func parseFlags() exampleOptions {
	// Read common options from environment
	options := exampleOptions{
		Options: ctx.ReadOptionsFromEnv("EXAMPLE"),
	}

	// Parse common flags
	flag.BoolVar(&options.Escape, "escape", options.Escape, "Enable escaping of special characters")
	flag.BoolVar(&options.JSON, "json", options.JSON, "Output in JSON format")
	flag.StringVar(&options.Tag, "tag", options.Tag, "Override the output tag name")
	flag.BoolVar(&options.Debug, "debug", options.Debug, "Enable debug output")

	// Parse tool-specific flags
	flag.BoolVar(&options.IncludeHidden, "include-hidden", false, "Include hidden files in output")
	flag.IntVar(&options.MaxDepth, "max-depth", 1, "Maximum directory depth")
	flag.StringVar(&options.Pattern, "pattern", "*", "File pattern to match")

	// Check tool-specific environment variables
	if os.Getenv("CTX_EXAMPLE_INCLUDE_HIDDEN") == "true" {
		options.IncludeHidden = true
	}
	if depthStr := os.Getenv("CTX_EXAMPLE_MAX_DEPTH"); depthStr != "" {
		if depth, err := fmt.Sscanf(depthStr, "%d", &options.MaxDepth); err != nil || depth < 1 {
			options.MaxDepth = 1
		}
	}
	if pattern := os.Getenv("CTX_EXAMPLE_PATTERN"); pattern != "" {
		options.Pattern = pattern
	}

	flag.Parse()
	return options
}

// gatherDirectoryInfo gathers information about the specified directory
func gatherDirectoryInfo(path string, options exampleOptions) (string, string, error) {
	// Ensure path exists
	info, err := os.Stat(path)
	if err != nil {
		return "", "", fmt.Errorf("error accessing path %s: %w", path, err)
	}

	if !info.IsDir() {
		return "", "", fmt.Errorf("%s is not a directory", path)
	}

	// Use find command to list directory contents with options
	args := []string{path}

	// Add depth limit
	args = append(args, "-maxdepth", fmt.Sprintf("%d", options.MaxDepth))

	// Add pattern matching
	args = append(args, "-name", options.Pattern)

	// Exclude hidden files unless requested
	if !options.IncludeHidden {
		args = append(args, "!", "-path", "*/.*")
	}

	cmd := exec.Command("find", args...)

	// Capture command output
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run the command
	err = cmd.Run()

	return stdout.String(), stderr.String(), err
}

// FormatJSON is a helper to format any value as JSON
func FormatJSON(v interface{}) string {
	return ctx.FormatJSON(toolName, "capabilities", fmt.Sprintf("%v", v), "", nil)
}
