package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/anthropic"
)

var (
	systemPrompt = `You are a shell programming assistant.

You will be provided input that represents the task the user is trying to get done and will output the most likely shell command to achieve their goal.

Output ONLY the exact command that would be run, not any additional information or context.`
	fewShot1 = `I want to list the top 5 largest files`
	fewShot2 = `ls -S | head -n 5`
)

func appendToBashHistory(command string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("error getting user home directory: %v", err)
	}

	historyFile := filepath.Join(homeDir, ".bash_history")
	f, err := os.OpenFile(historyFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("error opening bash history file: %v", err)
	}
	defer f.Close()

	if _, err = f.WriteString(command + "\n"); err != nil {
		return fmt.Errorf("error writing to bash history file: %v", err)
	}

	return nil
}

func generateAndHandleCommand(llm *anthropic.LLM, ctx context.Context, input string) {
	content := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, systemPrompt),
		llms.TextParts(llms.ChatMessageTypeHuman, fewShot1),
		llms.TextParts(llms.ChatMessageTypeAI, fewShot2),
		llms.TextParts(llms.ChatMessageTypeHuman, input),
	}

	var generatedCommand strings.Builder
	completion, err := llm.GenerateContent(ctx, content, llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
		generatedCommand.Write(chunk)
		return nil
	}))
	if err != nil {
		log.Fatal(err)
	}
	_ = completion

	command := strings.TrimSpace(generatedCommand.String())
	fmt.Printf("Generated command: %s\n", command)

	if os.Getenv("INSHELLA_AUTORUN") != "" {
		executeCommand(command)
	} else {
		fmt.Print("\nDo you want to run this command? (y/n): ")
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))
		if response == "y" || response == "yes" {
			executeCommand(command)
		}
	}
}

func executeCommand(command string) {
	cmd := exec.Command("sh", "-c", command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		fmt.Printf("Error running command: %v\n", err)
	} else {
		if err := appendToBashHistory(command); err != nil {
			fmt.Printf("Error appending to bash history: %v\n", err)
		}
	}
}

func main() {
	llm, err := anthropic.New(anthropic.WithModel("claude-3-5-sonnet-20240620"))
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()

	if len(os.Args) > 1 {
		// If command-line arguments are provided, use them directly
		input := strings.Join(os.Args[1:], " ")
		generateAndHandleCommand(llm, ctx, input)
	} else {
		// If no arguments, enter interactive mode
		reader := bufio.NewReader(os.Stdin)
		for {
			fmt.Print("Enter your command (or 'exit' to quit): ")
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(input)

			if input == "exit" {
				break
			}

			generateAndHandleCommand(llm, ctx, input)
		}
	}
}
