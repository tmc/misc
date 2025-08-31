package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/anthropic"
	"github.com/tmc/langchaingo/llms/ollama"
	"github.com/tmc/langchaingo/llms/openai"
	"golang.org/x/term"
)

var (
	systemPrompt = `You are a shell programming assistant.

You will be provided input that represents the task the user is trying to get done and will output the most likely shell command to achieve their goal.

Output ONLY the exact command that would be run, not any additional information or context.`
	fewShot1 = `I want to list the top 5 largest files`
	fewShot2 = `ls -S | head -n 5`
)

func appendToShellHistory(command string) error {
	// Check if shell history is disabled
	if os.Getenv("AI_NO_HISTORY") != "" {
		return nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("error getting user home directory: %v", err)
	}

	// Default to writing to a tool-specific history file
	historyDir := filepath.Join(homeDir, ".ai_cli")
	if err := os.MkdirAll(historyDir, 0755); err != nil {
		return fmt.Errorf("error creating history directory: %v", err)
	}

	historyFile := filepath.Join(historyDir, "history.txt")

	// By default, try to write to shell history unless disabled
	if os.Getenv("AI_NO_SHELL_HISTORY") == "" {
		shell := os.Getenv("SHELL")
		var shellHistoryFile string
		var formattedCommand string

		switch {
		case strings.Contains(shell, "zsh"):
			shellHistoryFile = filepath.Join(homeDir, ".zsh_history")
			// zsh history format includes timestamps
			epochStr := os.Getenv("EPOCHSECONDS")
			timePrefix := ": 0;"
			if epochStr != "" {
				timePrefix = ": " + epochStr + ";"
			}
			formattedCommand = timePrefix + command
		case strings.Contains(shell, "fish"):
			// Fish has a complex history format - skip for now
			// shellHistoryFile = filepath.Join(homeDir, ".local/share/fish/fish_history")
			return nil
		case strings.Contains(shell, "bash"):
			shellHistoryFile = filepath.Join(homeDir, ".bash_history")
			formattedCommand = command
		default:
			// Unknown shell - don't try to modify its history
			return nil
		}

		// Try to append to shell history
		f, err := os.OpenFile(shellHistoryFile, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			// Fall back to our own history file
			fmt.Printf("Note: Unable to write to shell history, using tool history instead\n")
		} else {
			defer f.Close()
			if _, err = f.WriteString(formattedCommand + "\n"); err != nil {
				return fmt.Errorf("error writing to shell history file: %v", err)
			}
			return nil
		}
	}

	// Write to our own history file
	f, err := os.OpenFile(historyFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("error opening history file %s: %v", historyFile, err)
	}
	defer f.Close()

	// Include timestamp for our own history format
	timestamp := time.Now().Format(time.RFC3339)
	if _, err = f.WriteString(fmt.Sprintf("[%s] %s\n", timestamp, command)); err != nil {
		return fmt.Errorf("error writing to history file: %v", err)
	}

	return nil
}

func generateAndHandleCommand(llm llms.Model, ctx context.Context, input string) {
	sp := systemPrompt
	unameOutput, _ := exec.Command("uname", "-a").CombinedOutput()
	sp += " the current uname output is: " + string(unameOutput)
	content := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, sp),
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

	if os.Getenv("AI_AUTORUN") != "" {
		executeCommand(command)
		return
	}

	fmt.Print("\nRun this command? [Y/n/e/m]: ")

	// Reset terminal to normal mode for input
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		_, err := term.GetState(fd)
		if err != nil {
			fmt.Printf("Error getting terminal state: %v\n", err)
			return
		}

		// Read a single character or control sequence
		var b [3]byte // Buffer large enough for a few bytes
		n, err := os.Stdin.Read(b[:])
		if err != nil {
			fmt.Printf("\nError reading response: %v\n", err)
			return
		}

		// Check for Enter key (CR or LF)
		if n == 1 && (b[0] == '\r' || b[0] == '\n') {
			fmt.Println()
			executeCommand(command)
			return
		}

		// Echo the character
		fmt.Printf("%s\n", string(b[0]))

		// Convert to string and lowercase
		response := strings.ToLower(string(b[0]))

		switch response {
		case "y":
			executeCommand(command)
		case "e":
			editedCommand := editCommand(command)
			if editedCommand != "" {
				fmt.Printf("Executing edited command: %s\n", editedCommand)
				executeCommand(editedCommand)
			}
		case "m":
			fmt.Print("Modify command: ")
			reader := bufio.NewReader(os.Stdin)
			modifiedCommand, _ := reader.ReadString('\n')
			modifiedCommand = strings.TrimSpace(modifiedCommand)

			if modifiedCommand != "" {
				fmt.Printf("Executing modified command: %s\n", modifiedCommand)
				executeCommand(modifiedCommand)
			}
		default:
			fmt.Println("Command not executed.")
		}
	} else {
		// Fallback for non-terminal stdin
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))

		if response == "" || response == "y" {
			executeCommand(command)
		} else if response == "e" {
			editedCommand := editCommand(command)
			if editedCommand != "" {
				fmt.Printf("Executing edited command: %s\n", editedCommand)
				executeCommand(editedCommand)
			}
		} else if response == "m" {
			fmt.Print("Modify command: ")
			modifiedCommand, _ := reader.ReadString('\n')
			modifiedCommand = strings.TrimSpace(modifiedCommand)

			if modifiedCommand != "" {
				fmt.Printf("Executing modified command: %s\n", modifiedCommand)
				executeCommand(modifiedCommand)
			}
		}
	}
}

func editCommand(command string) string {
	// Create temporary file for editing
	tmpFile, err := os.CreateTemp("", "ai-*.sh")
	if err != nil {
		fmt.Printf("Error creating temp file for editing: %v\n", err)
		return ""
	}
	defer os.Remove(tmpFile.Name())

	// Write command to file
	if _, err := tmpFile.WriteString(command); err != nil {
		fmt.Printf("Error writing to temp file: %v\n", err)
		return ""
	}
	if err := tmpFile.Close(); err != nil {
		fmt.Printf("Error closing temp file: %v\n", err)
		return ""
	}

	// Determine editor to use
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim" // Default to vim if EDITOR not set
	}

	// Open editor
	editorCmd := exec.Command(editor, tmpFile.Name())
	editorCmd.Stdin = os.Stdin
	editorCmd.Stdout = os.Stdout
	editorCmd.Stderr = os.Stderr

	if err := editorCmd.Run(); err != nil {
		fmt.Printf("Error running editor: %v\n", err)
		return ""
	}

	// Read edited command
	editedBytes, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		fmt.Printf("Error reading edited command: %v\n", err)
		return ""
	}

	return strings.TrimSpace(string(editedBytes))
}

func executeCommand(command string) {
	cmd := exec.Command("sh", "-c", command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		fmt.Printf("Error running command: %v\n", err)
	} else {
		if err := appendToShellHistory(command); err != nil {
			fmt.Printf("Error appending to shell history: %v\n", err)
		}
	}
}

// handleSignals sets up signal handling to ensure proper terminal state restoration
func handleSignals(oldState *term.State) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Println("\nExiting...")
		if oldState != nil {
			// Restore terminal state before exiting
			term.Restore(int(os.Stdin.Fd()), oldState)
		}
		os.Exit(0)
	}()
}

// CreateLLM initializes the LLM based on available API keys and model preferences
func createLLM() (llms.Model, error) {
	// Try to create an Anthropic Claude model first
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		anthropicModel := os.Getenv("AI_ANTHROPIC_MODEL")
		if anthropicModel == "" {
			anthropicModel = "claude-3-7-sonnet-latest" // Default to latest Claude 3.7 Sonnet
		}

		llm, err := anthropic.New(anthropic.WithModel(anthropicModel))
		if err != nil {
			fmt.Printf("Warning: Could not initialize Anthropic model: %v\n", err)
		} else {
			fmt.Printf("Using Anthropic Claude model: %s\n", anthropicModel)
			return llm, nil
		}
	}

	// Try OpenAI if Anthropic is not available
	if os.Getenv("OPENAI_API_KEY") != "" {
		openaiModel := os.Getenv("AI_OPENAI_MODEL")
		if openaiModel == "" {
			openaiModel = "gpt-4-1106-preview" // Default to GPT-4.1
		}

		llm, err := openai.New(openai.WithModel(openaiModel))
		if err != nil {
			fmt.Printf("Warning: Could not initialize OpenAI model: %v\n", err)
		} else {
			fmt.Printf("Using OpenAI model: %s\n", openaiModel)
			return llm, nil
		}
	}

	// Finally, try Ollama as a local fallback
	ollamaModel := os.Getenv("AI_OLLAMA_MODEL")
	if ollamaModel == "" {
		ollamaModel = "gemma:3b" // Default to Gemma 3
	}

	// Check if Ollama is running with a short timeout
	client := http.Client{
		Timeout: 2 * time.Second, // Short timeout to prevent hanging
	}
	_, err := client.Get("http://localhost:11434/api/version")
	if err != nil {
		fmt.Println("Warning: Ollama server is not running or inaccessible.")
		return nil, errors.New("no available LLM providers (set ANTHROPIC_API_KEY, OPENAI_API_KEY, or run Ollama)")
	}

	llm, err := ollama.New(ollama.WithModel(ollamaModel))
	if err != nil {
		return nil, fmt.Errorf("could not initialize Ollama model: %v", err)
	}

	fmt.Printf("Using local Ollama model: %s\n", ollamaModel)
	return llm, nil
}

func main() {
	// Create LLM with fallback options
	llm, err := createLLM()
	if err != nil {
		log.Fatalf("Error initializing LLM: %v\nPlease set ANTHROPIC_API_KEY, OPENAI_API_KEY, or run Ollama locally.", err)
	}

	ctx := context.Background()

	if len(os.Args) > 1 {
		// If command-line arguments are provided, use them directly
		input := strings.Join(os.Args[1:], " ")
		generateAndHandleCommand(llm, ctx, input)
	} else {
		// If no arguments, enter interactive mode with better terminal handling
		var oldState *term.State

		// Only try to configure terminal if stdin is a terminal
		if term.IsTerminal(int(os.Stdin.Fd())) {
			var err error
			// Save the current terminal state to restore later
			oldState, err = term.GetState(int(os.Stdin.Fd()))
			if err != nil {
				fmt.Printf("Warning: Unable to get terminal state: %v\n", err)
			}
		}

		// Set up signal handling to restore terminal state on interrupt
		handleSignals(oldState)

		// Create a buffered reader for input
		reader := bufio.NewReader(os.Stdin)

		// Interactive prompt loop
		for {
			fmt.Print("ai> ")
			input, err := reader.ReadString('\n')
			if err != nil {
				fmt.Printf("Error reading input: %v\n", err)
				continue
			}

			input = strings.TrimSpace(input)
			if input == "exit" || input == "quit" {
				break
			}

			if input != "" {
				generateAndHandleCommand(llm, ctx, input)
			}
		}

		// Restore terminal state when exiting normally
		if oldState != nil {
			term.Restore(int(os.Stdin.Fd()), oldState)
		}
	}
}
