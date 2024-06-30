package main

import (
	"bufio"
	"context"
	_ "embed"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/anthropic"
)

//go:embed system-prompt.txt
var systemPrompt string

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) != 2 {
		return fmt.Errorf("usage: %s <directory>", os.Args[0])
	}

	dir := os.Args[1]
	ctx := context.Background()

	client, err := anthropic.New()
	if err != nil {
		return fmt.Errorf("failed to create Anthropic client: %w", err)
	}

	for {
		testsPassed, testOutput := runTests(dir)
		if testsPassed {
			fmt.Println("All tests passed. Exiting.")
			return nil
		}

		fmt.Println("Tests failed. Attempting to fix...")

		sourceCode, err := readSourceFiles(dir)
		if err != nil {
			return fmt.Errorf("failed to read source files: %w", err)
		}

		fixedCode, err := getFixedCode(ctx, client, sourceCode, testOutput)
		if err != nil {
			return fmt.Errorf("failed to get fixed code: %w", err)
		}

		if err := applyFixes(dir, fixedCode); err != nil {
			return fmt.Errorf("failed to apply fixes: %w", err)
		}

		fmt.Println("Applied fixes. Retrying tests...")
	}
}

func runTests(dir string) (bool, string) {
	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	return err == nil, string(output)
}

func readSourceFiles(dir string) (string, error) {
	var sourceCode strings.Builder

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".go") {
			content, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}
			sourceCode.WriteString(fmt.Sprintf("=== %s ===\n", path))
			sourceCode.Write(content)
			sourceCode.WriteString("\n\n")
		}
		return nil
	})

	return sourceCode.String(), err
}

func getFixedCode(ctx context.Context, client llms.Model, sourceCode, testOutput string) (string, error) {
	prompt := fmt.Sprintf("Source code:\n\n%s\n\nTest output:\n\n%s\n\nPlease provide the fixed source code for all files that need changes. Use the same file headers as in the original source code.", sourceCode, testOutput)

	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, systemPrompt),
		llms.TextParts(llms.ChatMessageTypeHuman, prompt),
	}

	resp, err := client.GenerateContent(ctx, messages, llms.WithTemperature(0.1), llms.WithMaxTokens(4000))
	if err != nil {
		return "", err
	}

	return resp.Choices[0].Content, nil
}

func applyFixes(dir string, fixedCode string) error {
	scanner := bufio.NewScanner(strings.NewReader(fixedCode))
	var currentFile string
	var fileContent strings.Builder

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "=== ") && strings.HasSuffix(line, " ===") {
			if currentFile != "" {
				if err := os.WriteFile(currentFile, []byte(fileContent.String()), 0644); err != nil {
					return err
				}
				fileContent.Reset()
			}
			currentFile = filepath.Join(dir, strings.TrimPrefix(strings.TrimSuffix(line, " ==="), "=== "))
		} else {
			fileContent.WriteString(line)
			fileContent.WriteString("\n")
		}
	}

	if currentFile != "" {
		if err := os.WriteFile(currentFile, []byte(fileContent.String()), 0644); err != nil {
			return err
		}
	}

	return scanner.Err()
}