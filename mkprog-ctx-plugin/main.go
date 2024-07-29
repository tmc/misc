package main

import (
	"bytes"
	"context"
	_ "embed"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/anthropic"
)

//go:embed system-prompt.txt
var systemPrompt string

type PluginInfo struct {
	Name         string
	Language     string
	OutputDir    string
	Description  string
	Capabilities string
	Version      string
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	temperature := flag.Float64("temp", 0.1, "Set the temperature for AI generation (0.0 to 1.0)")
	flag.Parse()

	args := flag.Args()
	if len(args) < 2 {
		return fmt.Errorf("usage: %s <plugin_name> <description>", os.Args[0])
	}

	pluginInfo := PluginInfo{
		Name:        args[0],
		Description: strings.Join(args[1:], " "),
		Language:    "go",
		OutputDir:   filepath.Join(".", args[0]),
		Version:     "0.1.0",
	}

	if err := os.MkdirAll(pluginInfo.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	pluginSpec, err := getCtxPluginSpec()
	if err != nil {
		fmt.Println("Warning: issue getting live ctx spec, using built-in")
	}

	var sysPrompt bytes.Buffer
	err = template.Must(template.New("systemPrompt").Parse(systemPrompt)).Execute(&sysPrompt, pluginSpec)
	if err != nil {
		fmt.Println("Warning: issue executing template, using built-in")
		sysPrompt.WriteString(systemPrompt)
	}

	if os.Getenv("DEBUG") == "1" {
		fmt.Println(sysPrompt.String())
	}

	client, err := anthropic.New(
		anthropic.WithAnthropicBetaHeader(anthropic.MaxTokensAnthropicSonnet35),
	)
	if err != nil {
		return fmt.Errorf("failed to create Anthropic client: %w", err)
	}

	ctx := context.Background()

	fw := &fileWriter{outputDir: pluginInfo.OutputDir}

	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, sysPrompt.String()),
		llms.TextParts(llms.ChatMessageTypeHuman, strings.Join(args, " ")),
	}

	_, err = client.GenerateContent(ctx,
		messages,
		llms.WithTemperature(*temperature),
		llms.WithMaxTokens(8000),
		llms.WithStreamingFunc(fw.streamContent),
	)

	if err != nil {
		return fmt.Errorf("content generation failed: %w", err)
	}

	if err := fw.close(); err != nil {
		return fmt.Errorf("failed to close last file: %w", err)
	}

	if err := runGoImports(pluginInfo.OutputDir); err != nil {
		return fmt.Errorf("failed to run goimports: %w", err)
	}

	fmt.Printf("Plugin '%s' generated successfully in %s\n", pluginInfo.Name, pluginInfo.OutputDir)
	fmt.Printf("\nUsage:\n")
	fmt.Printf("cd %s\n", pluginInfo.OutputDir)
	fmt.Printf("go mod tidy; go run .\n\n")
	fmt.Printf("Optional: go install\n")
	fmt.Printf("Then run: %s\n", pluginInfo.Name)
	return nil
}

func getCtxPluginSpec() (string, error) {
	cmd := exec.Command("ctx", "--print-plugin-spec")
	output, err := cmd.Output()
	if err != nil {
		// Try alternative flag if the first one fails
		cmd = exec.Command("ctx", "-print-plugin-spec")
		output, err = cmd.Output()
		if err != nil {
			return "", fmt.Errorf("failed to get ctx plugin spec: %w", err)
		}
	}
	return string(output), nil
}

var fileNameRe = regexp.MustCompile(`(?m)^=== (.*) ===$`)

type fileWriter struct {
	currentFile *os.File
	buffer      bytes.Buffer
	outputDir   string
}

func (fw *fileWriter) streamContent(ctx context.Context, chunk []byte) error {
	fw.buffer.Write(chunk)

	for {
		line, err := fw.buffer.ReadBytes('\n')
		if err != nil {
			// If we don't have a full line, put it back in the buffer and wait for more data
			fw.buffer.Write(line)
			break
		}

		if match := fileNameRe.FindSubmatch(line); match != nil {
			// We found a new file header
			if fw.currentFile != nil {
				if err := fw.currentFile.Close(); err != nil {
					return fmt.Errorf("failed to close file: %w", err)
				}
			}

			fileName := string(match[1])
			fullPath := filepath.Join(fw.outputDir, fileName)
			fw.currentFile, err = os.Create(fullPath)
			if err != nil {
				return fmt.Errorf("failed to create file %s: %w", fullPath, err)
			}
			fmt.Printf("Creating file: %s\n", fullPath)
		} else if fw.currentFile != nil {
			// Write the line to the current file
			if _, err := fw.currentFile.Write(line); err != nil {
				return fmt.Errorf("failed to write to file: %w", err)
			}
		}
	}

	return nil
}

func (fw *fileWriter) close() error {
	if fw.currentFile != nil {
		// Write any remaining content in the buffer
		if _, err := fw.currentFile.Write(fw.buffer.Bytes()); err != nil {
			return fmt.Errorf("failed to write final content: %w", err)
		}
		if err := fw.currentFile.Close(); err != nil {
			return fmt.Errorf("failed to close final file: %w", err)
		}
		fw.currentFile = nil
		fw.buffer.Reset()
	}
	return nil
}

func runGoImports(dir string) error {
	_, err := exec.LookPath("goimports")
	if err != nil {
		fmt.Println("goimports not found, skipping...")
		return nil
	}
	fmt.Println("Running goimports...")
	cmd := exec.Command("goimports", "-w", dir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
