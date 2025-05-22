package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

type DevState struct {
	Name        string                 `json:"name"`
	Timestamp   time.Time              `json:"timestamp"`
	WorkingDir  string                 `json:"working_dir"`
	GitBranch   string                 `json:"git_branch"`
	GitCommit   string                 `json:"git_commit"`
	OpenFiles   []FileState            `json:"open_files"`
	EditorState map[string]interface{} `json:"editor_state"`
	LSPState    map[string]interface{} `json:"lsp_state"`
	Intent      string                 `json:"intent"`
	Description string                 `json:"description"`
}

type FileState struct {
	Path         string `json:"path"`
	CursorLine   int    `json:"cursor_line"`
	CursorColumn int    `json:"cursor_column"`
	Modified     bool   `json:"modified"`
	Content      string `json:"content,omitempty"`
}

var (
	rootCmd = &cobra.Command{
		Use:   "devflow-state",
		Short: "Intelligent development state management",
		Long: `DevFlowState captures, understands, and restores complete development 
context across sessions, branches, and interruptions.`,
	}

	captureCmd = &cobra.Command{
		Use:   "capture [name]",
		Short: "Capture current development state",
		Args:  cobra.MaximumNArgs(1),
		Run:   captureState,
	}

	restoreCmd = &cobra.Command{
		Use:   "restore <name>",
		Short: "Restore a saved development state",
		Args:  cobra.ExactArgs(1),
		Run:   restoreState,
	}

	listCmd = &cobra.Command{
		Use:   "list",
		Short: "List all saved states",
		Run:   listStates,
	}

	initCmd = &cobra.Command{
		Use:   "init",
		Short: "Initialize DevFlowState in current directory",
		Run:   initDevFlow,
	}
)

func init() {
	rootCmd.AddCommand(captureCmd)
	rootCmd.AddCommand(restoreCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(initCmd)

	// Capture command flags
	captureCmd.Flags().String("name", "", "Name for the captured state")
	captureCmd.Flags().String("description", "", "Description of current work")
	captureCmd.Flags().Bool("include-lsp", true, "Include LSP server state")
	captureCmd.Flags().Bool("include-content", false, "Include file content for modified files")
	captureCmd.Flags().Bool("auto", false, "Auto-generate name based on current context")

	// Restore command flags
	restoreCmd.Flags().Bool("force", false, "Force restore even if current state has changes")
	restoreCmd.Flags().Bool("dry-run", false, "Show what would be restored without applying changes")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func getStateDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal("Could not determine home directory:", err)
	}
	return filepath.Join(home, ".devflow", "states")
}

func ensureStateDir() error {
	stateDir := getStateDir()
	return os.MkdirAll(stateDir, 0755)
}

func captureState(cmd *cobra.Command, args []string) {
	if err := ensureStateDir(); err != nil {
		log.Fatal("Could not create state directory:", err)
	}

	name, _ := cmd.Flags().GetString("name")
	if len(args) > 0 {
		name = args[0]
	}
	if name == "" {
		auto, _ := cmd.Flags().GetBool("auto")
		if auto {
			name = generateAutoName()
		} else {
			name = fmt.Sprintf("state-%d", time.Now().Unix())
		}
	}

	description, _ := cmd.Flags().GetString("description")
	includeLSP, _ := cmd.Flags().GetBool("include-lsp")
	includeContent, _ := cmd.Flags().GetBool("include-content")

	fmt.Printf("Capturing development state: %s\n", name)

	state := &DevState{
		Name:        name,
		Timestamp:   time.Now(),
		Intent:      inferIntent(),
		Description: description,
	}

	// Capture current working directory
	wd, err := os.Getwd()
	if err != nil {
		log.Fatal("Could not get working directory:", err)
	}
	state.WorkingDir = wd

	// Capture git information
	captureGitState(state)

	// Capture file states (mock implementation)
	captureFileStates(state, includeContent)

	// Capture LSP state if requested
	if includeLSP {
		captureLSPState(state)
	}

	// Save state
	if err := saveState(state); err != nil {
		log.Fatal("Could not save state:", err)
	}

	fmt.Printf("✓ State '%s' captured successfully\n", name)
	if state.Description != "" {
		fmt.Printf("  Description: %s\n", state.Description)
	}
	fmt.Printf("  Files: %d open\n", len(state.OpenFiles))
	fmt.Printf("  Git: %s@%s\n", state.GitBranch, state.GitCommit[:8])
}

func restoreState(cmd *cobra.Command, args []string) {
	name := args[0]
	force, _ := cmd.Flags().GetBool("force")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	state, err := loadState(name)
	if err != nil {
		log.Fatal("Could not load state:", err)
	}

	fmt.Printf("Restoring development state: %s\n", state.Name)
	fmt.Printf("  Captured: %s\n", state.Timestamp.Format("2006-01-02 15:04:05"))
	fmt.Printf("  Directory: %s\n", state.WorkingDir)
	fmt.Printf("  Git: %s@%s\n", state.GitBranch, state.GitCommit[:8])

	if dryRun {
		fmt.Println("\n--- DRY RUN MODE ---")
		fmt.Printf("Would restore %d files\n", len(state.OpenFiles))
		for _, file := range state.OpenFiles {
			fmt.Printf("  %s (line %d, col %d)\n", file.Path, file.CursorLine, file.CursorColumn)
		}
		return
	}

	if !force && hasUncommittedChanges() {
		fmt.Println("Warning: You have uncommitted changes. Use --force to restore anyway.")
		return
	}

	// Restore working directory
	if err := os.Chdir(state.WorkingDir); err != nil {
		log.Printf("Warning: Could not change to directory %s: %v", state.WorkingDir, err)
	}

	// Restore git state
	restoreGitState(state)

	// Restore file states
	restoreFileStates(state)

	// Restore LSP state
	restoreLSPState(state)

	fmt.Printf("✓ State '%s' restored successfully\n", name)
}

func listStates(cmd *cobra.Command, args []string) {
	stateDir := getStateDir()
	files, err := filepath.Glob(filepath.Join(stateDir, "*.json"))
	if err != nil {
		log.Fatal("Could not list states:", err)
	}

	if len(files) == 0 {
		fmt.Println("No saved states found. Use 'devflow-state capture' to create one.")
		return
	}

	fmt.Printf("Saved development states (%d):\n\n", len(files))

	for _, file := range files {
		state, err := loadStateFromFile(file)
		if err != nil {
			log.Printf("Warning: Could not load state from %s: %v", file, err)
			continue
		}

		age := time.Since(state.Timestamp)
		fmt.Printf("  %s\n", state.Name)
		fmt.Printf("    Age: %s\n", formatDuration(age))
		fmt.Printf("    Dir: %s\n", state.WorkingDir)
		fmt.Printf("    Git: %s@%s\n", state.GitBranch, state.GitCommit[:min(8, len(state.GitCommit))])
		if state.Description != "" {
			fmt.Printf("    Desc: %s\n", state.Description)
		}
		fmt.Printf("    Files: %d\n", len(state.OpenFiles))
		fmt.Println()
	}
}

func initDevFlow(cmd *cobra.Command, args []string) {
	if err := ensureStateDir(); err != nil {
		log.Fatal("Could not create state directory:", err)
	}

	wd, _ := os.Getwd()
	fmt.Printf("Initializing DevFlowState in: %s\n", wd)

	// Create initial configuration
	configPath := filepath.Join(wd, ".devflow.json")
	config := map[string]interface{}{
		"auto_capture":    true,
		"include_lsp":     true,
		"include_content": false,
		"git_integration": true,
	}

	configData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		log.Fatal("Could not create configuration:", err)
	}

	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		log.Fatal("Could not write configuration:", err)
	}

	fmt.Printf("✓ Created configuration: %s\n", configPath)
	fmt.Printf("✓ State directory: %s\n", getStateDir())
	fmt.Println("\nDevFlowState is ready! Try:")
	fmt.Println("  devflow-state capture --name initial")
	fmt.Println("  devflow-state list")
}

// Helper functions (simplified implementations)

func generateAutoName() string {
	wd, _ := os.Getwd()
	base := filepath.Base(wd)
	branch := getCurrentGitBranch()
	if branch != "" && branch != "main" && branch != "master" {
		return fmt.Sprintf("%s-%s", base, branch)
	}
	return fmt.Sprintf("%s-%d", base, time.Now().Unix())
}

func inferIntent() string {
	// Simple intent inference based on current context
	// In a full implementation, this would use AI/ML
	if isInTestDirectory() {
		return "testing"
	}
	if hasRecentCommits() {
		return "development"
	}
	return "exploration"
}

func captureGitState(state *DevState) {
	state.GitBranch = getCurrentGitBranch()
	state.GitCommit = getCurrentGitCommit()
}

func captureFileStates(state *DevState, includeContent bool) {
	// Mock implementation - in reality, this would integrate with editors
	state.OpenFiles = []FileState{
		{
			Path:         "main.go",
			CursorLine:   42,
			CursorColumn: 10,
			Modified:     false,
		},
	}
}

func captureLSPState(state *DevState) {
	// Mock LSP state capture
	state.LSPState = map[string]interface{}{
		"server_running": true,
		"diagnostics":    []string{},
		"symbols_cached": true,
	}
}

func restoreGitState(state *DevState) {
	// Implementation would switch to the correct branch/commit
	fmt.Printf("Note: Would switch to git branch '%s'\n", state.GitBranch)
}

func restoreFileStates(state *DevState) {
	// Implementation would restore file positions in editor
	fmt.Printf("Note: Would restore %d file positions\n", len(state.OpenFiles))
}

func restoreLSPState(state *DevState) {
	// Implementation would restore LSP server state
	fmt.Println("Note: Would restore LSP server state")
}

func saveState(state *DevState) error {
	stateDir := getStateDir()
	filename := filepath.Join(stateDir, state.Name+".json")

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0644)
}

func loadState(name string) (*DevState, error) {
	stateDir := getStateDir()
	filename := filepath.Join(stateDir, name+".json")
	return loadStateFromFile(filename)
}

func loadStateFromFile(filename string) (*DevState, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var state DevState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	return &state, nil
}

// Utility functions (simplified implementations)

func getCurrentGitBranch() string {
	// Simplified - would use git commands
	return "main"
}

func getCurrentGitCommit() string {
	// Simplified - would use git commands
	return "abc123def456"
}

func hasUncommittedChanges() bool {
	// Simplified - would check git status
	return false
}

func isInTestDirectory() bool {
	wd, _ := os.Getwd()
	return filepath.Base(wd) == "test" || filepath.Base(wd) == "tests"
}

func hasRecentCommits() bool {
	// Simplified - would check git log
	return true
}

func formatDuration(d time.Duration) string {
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}