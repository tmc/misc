package main

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
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
	if len(os.Args) < 3 {
		return fmt.Errorf("usage: %s <plugin_name> <description>", os.Args[0])
	}

	pluginInfo := PluginInfo{
		Name:        os.Args[1],
		Description: os.Args[2],
		Language:    "go",
		OutputDir:   filepath.Join(".", os.Args[1]),
		Version:     "0.1.0",
	}

	if err := os.MkdirAll(pluginInfo.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	pluginSpec, err := getCtxPluginSpec()
	if err != nil {
		log.Println("issue getting live ctx spec, using built-in")
	}

	var sysPrompt bytes.Buffer
	err = template.Must(template.New("systemPrompt").Parse(systemPrompt)).Execute(&sysPrompt, pluginSpec)
	if err != nil {
		log.Println("issue executing template, using built-in")
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

	pluginFiles, err := generatePluginFiles(ctx, client, pluginInfo)
	if err != nil {
		return fmt.Errorf("failed to generate plugin files: %w", err)
	}

	for filename, content := range pluginFiles {
		filePath := filepath.Join(pluginInfo.OutputDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write file %s: %w", filename, err)
		}
		fmt.Printf("Created %s\n", filePath)
	}

	fmt.Printf("Plugin '%s' generated successfully in %s\n", pluginInfo.Name, pluginInfo.OutputDir)
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

func generatePluginFiles(ctx context.Context, client llms.Model, pluginInfo PluginInfo) (map[string]string, error) {
	prompt := fmt.Sprintf("Generate a ctx plugin with the following details:\nName: %s\nLanguage: %s\nDescription: %s\nCapabilities: %s\nVersion: %s\n\nProvide the content for the main source file, README.md, and LICENSE (MIT). For Go plugins, also include go.mod content. Implement the required flags (--capabilities, --plan-relevance) and include placeholder logic for the main plugin functionality.", pluginInfo.Name, pluginInfo.Language, pluginInfo.Description, pluginInfo.Capabilities, pluginInfo.Version)

	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, systemPrompt),
		llms.TextParts(llms.ChatMessageTypeHuman, prompt),
	}

	resp, err := client.GenerateContent(ctx, messages, llms.WithTemperature(0.1), llms.WithMaxTokens(4000))
	if err != nil {
		return nil, fmt.Errorf("failed to generate content: %w", err)
	}

	content := resp.Choices[0].Content

	files := make(map[string]string)
	currentFile := ""
	var fileContent strings.Builder

	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "===") && strings.HasSuffix(line, "===") {
			if currentFile != "" {
				files[currentFile] = fileContent.String()
				fileContent.Reset()
			}
			currentFile = strings.TrimSpace(strings.TrimPrefix(strings.TrimSuffix(line, "==="), "==="))
		} else {
			fileContent.WriteString(line + "\n")
		}
	}

	if currentFile != "" {
		files[currentFile] = fileContent.String()
	}

	return files, nil
}
