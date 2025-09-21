package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

)

// InteractiveMode represents an interactive CDP session
type InteractiveMode struct {
	ctx      context.Context
	registry *CommandRegistry
	help     *HelpSystem
	history  []string
	verbose  bool
}

// NewInteractiveMode creates a new interactive session
func NewInteractiveMode(ctx context.Context, verbose bool) *InteractiveMode {
	registry := NewCommandRegistry()
	return &InteractiveMode{
		ctx:      ctx,
		registry: registry,
		help:     NewHelpSystem(registry),
		history:  make([]string, 0),
		verbose:  verbose,
	}
}

// Run starts the interactive session
func (im *InteractiveMode) Run() error {
	im.showWelcome()

	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print("cdp> ")

		if !scanner.Scan() {
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Add to history
		im.history = append(im.history, line)

		// Handle special commands
		if im.handleSpecialCommand(line) {
			continue
		}

		// Check for exit
		if line == "exit" || line == "quit" || line == "q" {
			fmt.Println("Goodbye!")
			break
		}

		// Execute command
		if err := im.executeCommand(line); err != nil {
			fmt.Printf("Error: %v\n", err)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading input: %w", err)
	}

	return nil
}

// showWelcome displays the welcome message
func (im *InteractiveMode) showWelcome() {
	fmt.Println("\n╭─────────────────────────────────────────────────────────╮")
	fmt.Println("│      Welcome to CDP Interactive Mode                    │")
	fmt.Println("│      Chrome DevTools Protocol Command Line Interface    │")
	fmt.Println("╰─────────────────────────────────────────────────────────╯")
	fmt.Println()
	fmt.Println("Type 'help' for available commands or 'quick' for quick reference")
	fmt.Println("Type 'exit' or 'quit' to leave")
	fmt.Println()
}

// handleSpecialCommand handles special non-CDP commands
func (im *InteractiveMode) handleSpecialCommand(line string) bool {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return false
	}

	cmd := parts[0]
	args := parts[1:]

	switch cmd {
	case "help", "h", "?":
		im.help.ShowHelp(args)
		return true

	case "list", "ls":
		im.help.ListCommands()
		return true

	case "search", "find":
		if len(args) > 0 {
			im.help.SearchCommands(strings.Join(args, " "))
		} else {
			fmt.Println("Usage: search <term>")
		}
		return true

	case "quick", "qr", "ref":
		im.help.ShowQuickReference()
		return true

	case "history", "hist":
		im.showHistory()
		return true

	case "clear", "cls":
		im.clearScreen()
		return true

	case "verbose":
		im.verbose = !im.verbose
		fmt.Printf("Verbose mode: %v\n", im.verbose)
		return true

	case "version", "ver":
		fmt.Println("CDP Tool v1.0.0")
		return true

	default:
		return false
	}
}

// executeCommand executes a CDP command
func (im *InteractiveMode) executeCommand(line string) error {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return nil
	}

	cmdName := parts[0]
	args := parts[1:]

	// Check if it's a registered command
	if cmd, found := im.registry.GetCommand(cmdName); found {
		if im.verbose {
			fmt.Printf("Executing: %s\n", cmd.Name)
		}
		return cmd.Handler(im.ctx, args)
	}

	// Try to execute as raw CDP command
	if strings.Contains(cmdName, ".") {
		return im.executeRawCDP(line)
	}

	// Try to get completions
	completions := im.help.GetCompletions(cmdName)
	if len(completions) > 0 {
		fmt.Printf("Unknown command '%s'. Did you mean:\n", cmdName)
		for _, c := range completions {
			fmt.Printf("  • %s\n", c)
		}
		return nil
	}

	return fmt.Errorf("unknown command: %s", cmdName)
}

// executeRawCDP executes a raw CDP command
func (im *InteractiveMode) executeRawCDP(command string) error {
	// Parse Domain.method {params}
	parts := strings.SplitN(command, " ", 2)
	if len(parts) == 0 {
		return fmt.Errorf("empty command")
	}

	method := parts[0]
	if !strings.Contains(method, ".") {
		return fmt.Errorf("invalid CDP format: expected 'Domain.method'")
	}

	// Parse parameters
	params := "{}"
	if len(parts) > 1 {
		params = strings.TrimSpace(parts[1])
	}

	if im.verbose {
		fmt.Printf("Raw CDP: %s %s\n", method, params)
	}

	// Execute using chromedp (simplified - would need full CDP implementation)
	fmt.Printf("Executing CDP: %s with params: %s\n", method, params)
	fmt.Println("(Note: Raw CDP execution requires full implementation)")

	return nil
}

// showHistory displays command history
func (im *InteractiveMode) showHistory() {
	if len(im.history) == 0 {
		fmt.Println("No command history")
		return
	}

	fmt.Println("\nCommand History:")
	fmt.Println("────────────────")
	for i, cmd := range im.history {
		fmt.Printf("%3d: %s\n", i+1, cmd)
	}
	fmt.Println()
}

// clearScreen clears the terminal screen
func (im *InteractiveMode) clearScreen() {
	// ANSI escape code to clear screen
	fmt.Print("\033[2J\033[H")
	im.showWelcome()
}

// TabComplete provides tab completion for commands
func (im *InteractiveMode) TabComplete(partial string) []string {
	return im.help.GetCompletions(partial)
}

// ExecuteScript executes a script file
func (im *InteractiveMode) ExecuteScript(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("opening script file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0

	fmt.Printf("Executing script: %s\n", filename)
	fmt.Println("────────────────────────")

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}

		fmt.Printf("[%d] %s\n", lineNum, line)

		// Execute command
		if err := im.executeCommand(line); err != nil {
			return fmt.Errorf("line %d: %w", lineNum, err)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading script: %w", err)
	}

	fmt.Println("\nScript execution completed")
	return nil
}

// BatchExecute executes multiple commands in batch
func (im *InteractiveMode) BatchExecute(commands []string) error {
	fmt.Println("Executing batch commands:")
	fmt.Println("─────────────────────────")

	for i, cmd := range commands {
		fmt.Printf("[%d/%d] %s\n", i+1, len(commands), cmd)

		if err := im.executeCommand(cmd); err != nil {
			return fmt.Errorf("command %d: %w", i+1, err)
		}
	}

	fmt.Println("\nBatch execution completed")
	return nil
}

// SaveSession saves the current session to a file
func (im *InteractiveMode) SaveSession(filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("creating session file: %w", err)
	}
	defer file.Close()

	fmt.Fprintf(file, "# CDP Session - %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(file, "# Commands: %d\n\n", len(im.history))

	for _, cmd := range im.history {
		fmt.Fprintf(file, "%s\n", cmd)
	}

	fmt.Printf("Session saved to: %s (%d commands)\n", filename, len(im.history))
	return nil
}

// LoadSession loads and executes a saved session
func (im *InteractiveMode) LoadSession(filename string) error {
	return im.ExecuteScript(filename)
}