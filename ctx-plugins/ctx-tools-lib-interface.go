package ctx

// This file outlines a proposed interface for a common ctx-tools library.
// It demonstrates how existing ctx plugin tools could integrate with the
// framework discovered in github.com/tmc/ctx.

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// Capabilities represents a tool's capabilities, flags, and relevance scoring.
type Capabilities struct {
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Version     string              `json:"version"`
	Author      string              `json:"author"`
	Flags       []Flag              `json:"flags"`
	EnvVars     []EnvironmentVar    `json:"environment_variables"`
	Relevance   RelevanceScores     `json:"relevance"`
}

// Flag represents a command-line flag for a tool.
type Flag struct {
	Name        string `json:"name"`
	Type        string `json:"type"` // bool, string, int, etc.
	Description string `json:"description"`
	Default     string `json:"default,omitempty"`
}

// EnvironmentVar represents an environment variable for a tool.
type EnvironmentVar struct {
	Name        string `json:"name"`
	Type        string `json:"type"` // bool, string, int, etc.
	Description string `json:"description"`
	Default     string `json:"default,omitempty"`
}

// RelevanceScores contains relevance scores for different contexts.
type RelevanceScores struct {
	Repo       float64            `json:"repo"`
	Filesystem float64            `json:"filesystem"`
	Language   map[string]float64 `json:"language"`
}

// PlanRelevance represents a tool's relevance to the current context.
type PlanRelevance struct {
	Score  float64 `json:"score"`
	Reason string  `json:"reason"`
}

// Options represents common options for all tools.
type Options struct {
	Escape bool   // Enable escaping of special characters
	JSON   bool   // Output in JSON format
	Tag    string // Override the output tag name
	Debug  bool   // Enable debug output
}

// ContextTool is the interface for all context tools.
type ContextTool interface {
	// GetCapabilities returns the tool's capabilities.
	GetCapabilities() Capabilities

	// GetPlanRelevance returns the tool's relevance for the current context.
	GetPlanRelevance() PlanRelevance

	// Run executes the tool with the given options and args.
	Run(opts Options, args []string) (string, error)
}

// Output represents structured output from a tool.
type Output struct {
	Tool   string `json:"tool"`
	Cmd    string `json:"cmd"`
	Stdout string `json:"stdout,omitempty"`
	Stderr string `json:"stderr,omitempty"`
	Error  string `json:"error,omitempty"`
}

// FormatOutput formats the tool output according to the given options.
func FormatOutput(opts Options, tool, cmd, stdout, stderr string, err error) string {
	if opts.JSON {
		return formatJSON(tool, cmd, stdout, stderr, err)
	}
	return formatXML(opts, tool, cmd, stdout, stderr, err)
}

// formatJSON formats the output as JSON.
func formatJSON(tool, cmd, stdout, stderr string, err error) string {
	output := Output{
		Tool:   tool,
		Cmd:    cmd,
		Stdout: stdout,
		Stderr: stderr,
	}
	if err != nil {
		output.Error = err.Error()
	}
	
	jsonBytes, jsonErr := json.MarshalIndent(output, "", "  ")
	if jsonErr != nil {
		// Fall back to a simpler format if JSON marshaling fails
		return fmt.Sprintf(`{"tool": %q, "error": "JSON marshaling error: %s"}`, tool, jsonErr)
	}
	return string(jsonBytes)
}

// formatXML formats the output using XML-like tags.
func formatXML(opts Options, tool, cmd, stdout, stderr string, err error) string {
	tagName := opts.Tag
	if tagName == "" {
		tagName = tool
	}
	
	var result string
	result += fmt.Sprintf("<%s cmd=%q>\n", tagName, cmd)
	
	if stdout != "" {
		result += fmt.Sprintf("  <stdout>%s</stdout>\n", escapeIfNeeded(stdout, opts.Escape))
	}
	
	if stderr != "" {
		result += fmt.Sprintf("  <stderr>%s</stderr>\n", escapeIfNeeded(stderr, opts.Escape))
	}
	
	if err != nil {
		result += fmt.Sprintf("  <e>%s</e>\n", escapeIfNeeded(err.Error(), opts.Escape))
	}
	
	result += fmt.Sprintf("</%s>\n", tagName)
	return result
}

// escapeIfNeeded escapes special characters if escape is true.
func escapeIfNeeded(s string, escape bool) string {
	if !escape {
		return s
	}
	// Simple HTML escaping for demonstration
	// In a real implementation, this would use html.EscapeString or similar
	return s // Placeholder for actual implementation
}

// WriteOutput writes the formatted output to the given writer.
func WriteOutput(w io.Writer, opts Options, tool, cmd, stdout, stderr string, err error) error {
	output := FormatOutput(opts, tool, cmd, stdout, stderr, err)
	_, writeErr := fmt.Fprint(w, output)
	return writeErr
}

// PrintOutput writes the formatted output to stdout.
func PrintOutput(opts Options, tool, cmd, stdout, stderr string, err error) {
	WriteOutput(os.Stdout, opts, tool, cmd, stdout, stderr, err)
}

// ReadOptionsFromEnv reads options from environment variables.
func ReadOptionsFromEnv(toolName string) Options {
	opts := Options{}
	
	// Check common environment variables
	if os.Getenv("CTX_TOOL_ESCAPE") == "true" {
		opts.Escape = true
	}
	if os.Getenv("CTX_TOOL_JSON") == "true" {
		opts.JSON = true
	}
	if tag := os.Getenv("CTX_TOOL_TAG"); tag != "" {
		opts.Tag = tag
	}
	if os.Getenv("CTX_TOOL_DEBUG") == "true" {
		opts.Debug = true
	}
	
	// Check tool-specific environment variables
	prefix := fmt.Sprintf("CTX_%s_", toolName)
	if os.Getenv(prefix+"ESCAPE") == "true" {
		opts.Escape = true
	}
	if os.Getenv(prefix+"JSON") == "true" {
		opts.JSON = true
	}
	if tag := os.Getenv(prefix+"TAG"); tag != "" {
		opts.Tag = tag
	}
	if os.Getenv(prefix+"DEBUG") == "true" {
		opts.Debug = true
	}
	
	return opts
}

// ExecuteCommand is a utility function to execute shell commands.
func ExecuteCommand(command string) (stdout, stderr string, err error) {
	// This is a placeholder for the actual implementation
	// In a real implementation, this would use exec.Command
	return "", "", fmt.Errorf("ExecuteCommand not implemented")
}