// Package cli provides functionality for working with the protoc-gen-anything plugin.
package cli

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GeneratorOptions holds configuration for the code generator.
type GeneratorOptions struct {
	// TemplatesDir is the path to the templates directory.
	TemplatesDir string

	// OutputDir is the path to output generated files.
	OutputDir string

	// ProtocBinary is the path to the protoc compiler.
	ProtocBinary string

	// PluginPath is the path to the protoc-gen-anything binary.
	PluginPath string

	// Verbose enables verbose output.
	Verbose bool

	// ContinueOnError makes the generator continue when errors occur.
	ContinueOnError bool

	// ProtocFlags contains additional flags to pass to protoc.
	ProtocFlags string

	// SkipProtoc skips running protoc (for testing or environments without protoc).
	SkipProtoc bool
}

// Generator manages the code generation process.
type Generator struct {
	options *GeneratorOptions
	protoFiles []string
}

// NewGenerator creates a new Generator with the given options and proto files.
func NewGenerator(options *GeneratorOptions, protoFiles []string) *Generator {
	return &Generator{
		options: options,
		protoFiles: protoFiles,
	}
}

// Run executes the code generation process.
func (g *Generator) Run() error {
	// Validate proto files
	if err := g.validateProtoFiles(); err != nil {
		return fmt.Errorf("invalid proto files: %w", err)
	}

	// Verify the templates directory exists
	if _, err := os.Stat(g.options.TemplatesDir); os.IsNotExist(err) {
		return fmt.Errorf("templates directory '%s' does not exist", g.options.TemplatesDir)
	}

	// Create the output directory if it doesn't exist
	if err := os.MkdirAll(g.options.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Get absolute path to output directory
	absOutputDir, err := filepath.Abs(g.options.OutputDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path to output directory: %w", err)
	}

	// Skip protoc execution if requested
	if g.options.SkipProtoc {
		if g.options.Verbose {
			fmt.Fprintf(os.Stderr, "Skipping protoc execution (SkipProtoc=true)\n")
			fmt.Fprintf(os.Stderr, "Templates directory: %s\n", g.options.TemplatesDir)
			fmt.Fprintf(os.Stderr, "Output directory: %s\n", absOutputDir)
			fmt.Fprintf(os.Stderr, "Proto files: %s\n", strings.Join(g.protoFiles, ", "))
		}
		return nil
	}

	// Build the protoc command
	protocArgs, err := g.buildProtocArgs(absOutputDir)
	if err != nil {
		return fmt.Errorf("failed to build protoc arguments: %w", err)
	}

	// Execute protoc
	if g.options.Verbose {
		fmt.Fprintf(os.Stderr, "Executing: %s %s\n", g.options.ProtocBinary, strings.Join(protocArgs, " "))
	}

	// Check if protoc is available
	_, err = exec.LookPath(g.options.ProtocBinary)
	if err != nil {
		return fmt.Errorf("protoc binary not found at '%s': %w", g.options.ProtocBinary, err)
	}

	cmd := exec.Command(g.options.ProtocBinary, protocArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("protoc execution failed: %w", err)
	}

	if g.options.Verbose {
		fmt.Fprintf(os.Stderr, "Successfully generated code in %s\n", absOutputDir)
	}

	return nil
}

// validateProtoFiles checks if all proto files exist
func (g *Generator) validateProtoFiles() error {
	if len(g.protoFiles) == 0 {
		return fmt.Errorf("no proto files specified")
	}

	for _, file := range g.protoFiles {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			return fmt.Errorf("proto file %s does not exist", file)
		}
	}
	return nil
}

// buildProtocArgs constructs the arguments for the protoc command
func (g *Generator) buildProtocArgs(absOutputDir string) ([]string, error) {
	protocArgs := []string{}

	// Add proto files
	for _, protoFile := range g.protoFiles {
		protocArgs = append(protocArgs, protoFile)
	}

	// Add plugin options
	pluginOpts := []string{
		"templates=" + g.options.TemplatesDir,
	}

	if g.options.Verbose {
		pluginOpts = append(pluginOpts, "verbose=true")
		pluginOpts = append(pluginOpts, "log_level=debug")
	}

	if g.options.ContinueOnError {
		pluginOpts = append(pluginOpts, "continue_on_error=true")
	}

	// Add plugin path if specified
	if g.options.PluginPath != "" {
		protocArgs = append(protocArgs, "--plugin=protoc-gen-anything="+g.options.PluginPath)
	}

	// Add output directory
	protocArgs = append(protocArgs, "--anything_out="+absOutputDir)

	// Add plugin options
	protocArgs = append(protocArgs, "--anything_opt="+strings.Join(pluginOpts, ","))

	// Add any additional protoc flags
	if g.options.ProtocFlags != "" {
		for _, flag := range strings.Split(g.options.ProtocFlags, " ") {
			protocArgs = append(protocArgs, flag)
		}
	}

	return protocArgs, nil
}

// FindTemplateFiles discovers all template files in the given directory
func FindTemplateFiles(dir string) ([]string, error) {
	var files []string

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Only include files with .tmpl extension
		if !d.IsDir() && strings.HasSuffix(path, ".tmpl") {
			relPath, err := filepath.Rel(dir, path)
			if err != nil {
				return err
			}
			files = append(files, relPath)
		}

		return nil
	})

	return files, err
}

// ResolveTemplate analyzes a template filename to determine what type of entity it applies to
func ResolveTemplate(filename string) string {
	// Extract entity type from file name
	if strings.Contains(filename, "{{.Message.") {
		return "message"
	} else if strings.Contains(filename, "{{.Service.") {
		return "service"
	} else if strings.Contains(filename, "{{.Method.") {
		return "method"
	} else if strings.Contains(filename, "{{.Enum.") {
		return "enum"
	} else if strings.Contains(filename, "{{.Oneof.") {
		return "oneof"
	} else if strings.Contains(filename, "{{.Field.") {
		return "field"
	} else if strings.Contains(filename, "{{.File.") {
		return "file"
	}
	return "file" // Default to file-level template
}