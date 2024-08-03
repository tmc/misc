package main

import (
	"bufio"
	"context"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/anthropic"
)

//go:embed system-prompt.txt
var systemPrompt string

type GitCommand struct {
	Command     string `json:"command"`
	Explanation string `json:"explanation"`
	IsMutating  bool   `json:"is_mutating"`
}

type UserPreference struct {
	Prompt   string `json:"prompt"`
	Command  string `json:"command"`
	Feedback int    `json:"feedback"` // 1-5 rating
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	disablePreferences := flag.Bool("no-pref", false, "Disable preference tracking")
	flag.Parse()

	prompt := strings.Join(flag.Args(), " ")
	if prompt == "" {
		return fmt.Errorf("please provide a natural language prompt describing the git operation")
	}

	ctx := context.Background()
	client, err := anthropic.New()
	if err != nil {
		return fmt.Errorf("failed to create Anthropic client: %w", err)
	}

	gitContext, err := getGitContext()
	if err != nil {
		return fmt.Errorf("failed to get git context: %w", err)
	}

	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, systemPrompt),
		llms.TextParts(llms.ChatMessageTypeHuman, fmt.Sprintf("Git context:\n%s\n\nUser prompt: %s", gitContext, prompt)),
	}

	resp, err := client.GenerateContent(ctx, messages, llms.WithTemperature(0.1), llms.WithMaxTokens(4000))
	if err != nil {
		return fmt.Errorf("failed to generate content: %w", err)
	}

	var commands []GitCommand
	if err := json.Unmarshal([]byte(resp.Choices[0].Content), &commands); err != nil {
		return fmt.Errorf("failed to parse generated commands: %w", err)
	}

	selectedCommand, err := presentCommands(commands)
	if err != nil {
		return err
	}

	if selectedCommand.IsMutating {
		confirmed, err := confirmMutatingOperation(selectedCommand)
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Println("Operation cancelled.")
			return nil
		}
	}

	if err := executeGitCommand(selectedCommand.Command); err != nil {
		return fmt.Errorf("failed to execute git command: %w", err)
	}

	if !*disablePreferences {
		if err := handleUserPreference(prompt, selectedCommand.Command); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to handle user preference: %v\n", err)
		}
	}

	return nil
}

func getGitContext() (string, error) {
	commands := []string{
		"git status --short",
		"git branch --show-current",
		"git log -1 --oneline",
	}

	var wg sync.WaitGroup
	results := make([]string, len(commands))
	errors := make([]error, len(commands))

	for i, cmd := range commands {
		wg.Add(1)
		go func(i int, cmd string) {
			defer wg.Done()
			output, err := exec.Command("sh", "-c", cmd).Output()
			if err != nil {
				errors[i] = err
				return
			}
			results[i] = strings.TrimSpace(string(output))
		}(i, cmd)
	}

	wg.Wait()

	for _, err := range errors {
		if err != nil {
			return "", fmt.Errorf("error executing git command: %w", err)
		}
	}

	return strings.Join(results, "\n"), nil
}

func presentCommands(commands []GitCommand) (GitCommand, error) {
	fmt.Println("Candidate git commands:")
	for i, cmd := range commands {
		fmt.Printf("%d. %s\n", i+1, cmd.Command)
		fmt.Printf("   Explanation: %s\n\n", cmd.Explanation)
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Select a command (number) or 'q' to quit: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			return GitCommand{}, fmt.Errorf("failed to read user input: %w", err)
		}

		input = strings.TrimSpace(input)
		if input == "q" {
			return GitCommand{}, fmt.Errorf("user chose to quit")
		}

		var selection int
		if _, err := fmt.Sscanf(input, "%d", &selection); err != nil {
			fmt.Println("Invalid input. Please enter a number or 'q'.")
			continue
		}

		if selection < 1 || selection > len(commands) {
			fmt.Println("Invalid selection. Please choose a number within the range.")
			continue
		}

		return commands[selection-1], nil
	}
}

func confirmMutatingOperation(cmd GitCommand) (bool, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("The selected command '%s' will modify files. Are you sure you want to proceed? (Y/n): ", cmd.Command)
	input, err := reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("failed to read user input: %w", err)
	}

	input = strings.TrimSpace(strings.ToLower(input))
	return input == "y" || input == "yes" || input == "", nil
}

func executeGitCommand(command string) error {
	cmd := exec.Command("sh", "-c", command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func handleUserPreference(prompt, command string) error {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("How well did this command meet your needs? (1-5, or press Enter to skip): ")
	input, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read user input: %w", err)
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return nil
	}

	var rating int
	if _, err := fmt.Sscanf(input, "%d", &rating); err != nil || rating < 1 || rating > 5 {
		return fmt.Errorf("invalid rating: please enter a number between 1 and 5")
	}

	preference := UserPreference{
		Prompt:   prompt,
		Command:  command,
		Feedback: rating,
	}

	return saveUserPreference(preference)
}

func saveUserPreference(pref UserPreference) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}

	prefFile := filepath.Join(homeDir, ".gitplan_preferences.txt")
	f, err := os.OpenFile(prefFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open preferences file: %w", err)
	}
	defer f.Close()

	prefJSON, err := json.Marshal(pref)
	if err != nil {
		return fmt.Errorf("failed to marshal preference data: %w", err)
	}

	if _, err := f.WriteString(string(prefJSON) + "\n"); err != nil {
		return fmt.Errorf("failed to write preference data: %w", err)
	}

	return nil
}
