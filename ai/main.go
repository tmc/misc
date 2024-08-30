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

func main() {
	llm, err := anthropic.New(anthropic.WithModel("claude-3-5-sonnet-20240620"))
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()

	for {
		input := strings.Join(os.Args[1:], " ")
		if input == "" {
			fmt.Print("Enter your command (or 'exit' to quit): ")
			reader := bufio.NewReader(os.Stdin)
			input, _ = reader.ReadString('\n')
			input = strings.TrimSpace(input)
		}

		if input == "exit" {
			break
		}

		content := []llms.MessageContent{
			llms.TextParts(llms.ChatMessageTypeSystem, systemPrompt),
			llms.TextParts(llms.ChatMessageTypeHuman, fewShot1),
			llms.TextParts(llms.ChatMessageTypeAI, fewShot2),
			llms.TextParts(llms.ChatMessageTypeHuman, input),
		}

		var generatedCommand strings.Builder
		completion, err := llm.GenerateContent(ctx, content, llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			fmt.Print(string(chunk))
			generatedCommand.Write(chunk)
			return nil
		}))
		fmt.Println()
		if err != nil {
			log.Fatal(err)
		}
		_ = completion

		for {
			fmt.Print("Do you want to (r)un, (e)dit, (g)enerate again, or (q)uit? ")
			reader := bufio.NewReader(os.Stdin)
			choice, _ := reader.ReadString('\n')
			choice = strings.TrimSpace(strings.ToLower(choice))

			switch choice {
			case "r":
				cmd := exec.Command("sh", "-c", generatedCommand.String())
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				err := cmd.Run()
				if err != nil {
					fmt.Printf("Error running command: %v\n", err)
				} else {
					// Append the command to bash history
					if err := appendToBashHistory(generatedCommand.String()); err != nil {
						fmt.Printf("Error appending to bash history: %v\n", err)
					}
				}
			case "e":
				fmt.Printf("Current command: %s\n", generatedCommand.String())
				fmt.Print("Enter the edited command: ")
				editedCommand, _ := reader.ReadString('\n')
				editedCommand = strings.TrimSpace(editedCommand)
				generatedCommand.Reset()
				generatedCommand.WriteString(editedCommand)
			case "g":
				break
			case "q":
				return
			default:
				fmt.Println("Invalid choice. Please try again.")
				continue
			}

			if choice == "g" {
				break
			}
		}

		if len(os.Args) > 1 {
			break
		}
	}
}
