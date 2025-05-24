package main

import (
	"bufio"
	_ "embed"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

//go:embed doc.go
var documentation string

var (
	flagAll                        = flag.Bool("all", false, "Process all commits on the current branch, not just unpushed ones")
	flagDryRun                     = flag.Bool("dry-run", true, "Show what would be changed without making any modifications (default: true)")
	flagRun                        = flag.Bool("run", false, "Actually execute the changes (overrides dry-run)")
	flagVerbose                    = flag.Bool("verbose", false, "Enable verbose output showing detailed processing information")
	flagHelp                       = flag.Bool("help", false, "Show usage information")
	flagLimit                      = flag.Int("limit", 0, "Limit the number of commits to process (0 = no limit)")
	flagMsgCommand                 = flag.String("msg-command", "", "Command to generate new commit message (receives cleaned message on stdin)")
	flagMsgCommandLimit            = flag.Int("msg-command-limit", 0, "Limit the number of times msg-command is invoked (0 = no limit, falls back to cleaned messages)")
	flagMsgUseGitAutoCommitMessage = flag.Bool("msg-use-git-auto-commit-message", false, "Use git-auto-commit-message --message-only (shortcut for -msg-command)")
	flagGenerateScript             = flag.Bool("generate-script", false, "Generate a bash script to review and run manually")
)

var (
	generatedWithPattern = regexp.MustCompile(`(?i)^\s*🤖\s*generated with.*$`)
	coAuthoredByPattern  = regexp.MustCompile(`(?i)^\s*co-authored-by:.*$`)
)

func main() {
	flag.Parse()

	if *flagHelp {
		printUsage()
		os.Exit(0)
	}

	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		switch err.(type) {
		case *NotInGitRepoError:
			os.Exit(2)
		case *NoCommitsError:
			os.Exit(3)
		default:
			os.Exit(1)
		}
	}
}

type NotInGitRepoError struct {
	msg string
}

func (e *NotInGitRepoError) Error() string {
	return e.msg
}

type NoCommitsError struct {
	msg string
}

func (e *NoCommitsError) Error() string {
	return e.msg
}

func run() error {
	// Check if we're in a Git repository
	if err := checkGitRepo(); err != nil {
		return &NotInGitRepoError{err.Error()}
	}

	// If -run is specified, it overrides dry-run
	if *flagRun {
		*flagDryRun = false
	}

	// Handle convenience flag for git-auto-commit-message
	if *flagMsgUseGitAutoCommitMessage {
		if *flagMsgCommand != "" {
			return fmt.Errorf("cannot use both -msg-command and -msg-use-git-auto-commit-message")
		}
		*flagMsgCommand = "git-auto-commit-message --message-only"
	}

	// Check for clean working tree (unless dry-run)
	if !*flagDryRun {
		if err := checkCleanWorkingTree(); err != nil {
			return err
		}
	}

	// Get commits to process
	commits, err := getCommitsToProcess()
	if err != nil {
		return err
	}

	if len(commits) == 0 {
		return &NoCommitsError{"No commits found to process"}
	}

	// Apply limit if specified
	if *flagLimit > 0 && len(commits) > *flagLimit {
		commits = commits[:*flagLimit]
	}

	if *flagVerbose {
		fmt.Printf("Found %d commits to process\n", len(commits))
	}

	if *flagGenerateScript {
		return generateRebaseScript(commits)
	}

	if *flagDryRun {
		fmt.Printf("Found %d commits that need cleaning:\n", len(commits))
		for i, commit := range commits {
			title := strings.Split(commit.Message, "\n")[0]
			fmt.Printf("  %d. %s - %s\n", i+1, commit.SHA[:8], title)
		}

		if *flagVerbose {
			fmt.Println("\nDetailed view:")
			for _, commit := range commits {
				cleaned, _ := cleanCommitMessage(commit.Message)
				fmt.Printf("\nCommit %s:\n", commit.SHA[:8])
				fmt.Printf("  Original message:\n%s\n", indentText(commit.Message))
				fmt.Printf("  Cleaned message:\n%s\n", indentText(cleaned))

				// Test message generation if specified
				if *flagMsgCommand != "" {
					fmt.Printf("  Testing message generation...\n")
					if generated, err := testMessageGeneration(cleaned); err != nil {
						fmt.Printf("  Generated message (ERROR): %v\n", err)
					} else {
						fmt.Printf("  Generated message:\n%s\n", indentText(generated))
					}
				}
			}
		}

		fmt.Println("\nTo actually run the cleaning:")
		fmt.Printf("  %s -run", os.Args[0])
		for _, arg := range os.Args[1:] {
			if arg != "-dry-run" { // Skip the default dry-run flag
				fmt.Printf(" %s", arg)
			}
		}
		fmt.Println()
		fmt.Println("\nTo generate a reviewable script:")
		fmt.Printf("  %s -generate-script", os.Args[0])
		for _, arg := range os.Args[1:] {
			if arg != "-dry-run" { // Skip the default dry-run flag
				fmt.Printf(" %s", arg)
			}
		}
		fmt.Println(" > clean.sh")

		return nil
	}

	// Use interactive rebase to process commits
	processedCount, err := processCommitsWithRebase(commits)
	if err != nil {
		return err
	}

	if processedCount == 0 {
		fmt.Println("No commits required cleaning")
	} else if *flagDryRun {
		fmt.Printf("Would process %d commits\n", processedCount)
	} else {
		fmt.Printf("Successfully processed %d commits\n", processedCount)
	}

	return nil
}

type Commit struct {
	SHA     string
	Message string
	Author  string
	Date    string
}

func checkGitRepo() error {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("not in a Git repository")
	}
	return nil
}

func checkCleanWorkingTree() error {
	// Check for staged changes
	stagedCmd := exec.Command("git", "diff", "--cached", "--quiet")
	if err := stagedCmd.Run(); err != nil {
		return fmt.Errorf("you have staged changes. Please commit or unstage them before running this tool")
	}

	// Check for unstaged changes (excluding untracked files)
	unstagedCmd := exec.Command("git", "diff", "--quiet")
	if err := unstagedCmd.Run(); err != nil {
		return fmt.Errorf("you have unstaged changes. Please commit or stash them before running this tool")
	}

	return nil
}

func getCommitsToProcess() ([]Commit, error) {
	var args []string
	if *flagAll {
		args = []string{"log", "--format=%H|%s|%an|%ad", "--date=iso"}
	} else {
		// Get unpushed commits
		args = []string{"log", "--format=%H|%s|%an|%ad", "--date=iso", "@{upstream}..HEAD"}
	}

	cmd := exec.Command("git", args...)
	output, err := cmd.Output()
	if err != nil {
		if !*flagAll {
			// Try without upstream (no remote configured)
			args = []string{"log", "--format=%H|%s|%an|%ad", "--date=iso"}
			cmd = exec.Command("git", args...)
			output, err = cmd.Output()
			if err != nil {
				return nil, fmt.Errorf("failed to get commits: %v", err)
			}
		} else {
			return nil, fmt.Errorf("failed to get commits: %v", err)
		}
	}

	var commits []Commit
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) < 4 {
			continue
		}

		sha := parts[0]

		// Get full commit message
		msgCmd := exec.Command("git", "show", "-s", "--format=%B", sha)
		msgOutput, err := msgCmd.Output()
		if err != nil {
			return nil, fmt.Errorf("failed to get commit message for %s: %v", sha, err)
		}

		commit := Commit{
			SHA:     sha,
			Message: string(msgOutput),
			Author:  parts[2],
			Date:    parts[3],
		}

		// Only include commits that need cleaning
		if _, needsCleaning := cleanCommitMessage(commit.Message); needsCleaning {
			commits = append(commits, commit)
		}
	}

	return commits, nil
}

func cleanCommitMessage(message string) (string, bool) {
	lines := strings.Split(message, "\n")
	var cleanedLines []string
	changed := false

	for _, line := range lines {
		if generatedWithPattern.MatchString(line) || coAuthoredByPattern.MatchString(line) {
			changed = true
			continue
		}
		cleanedLines = append(cleanedLines, line)
	}

	// Remove trailing empty lines
	for len(cleanedLines) > 0 && strings.TrimSpace(cleanedLines[len(cleanedLines)-1]) == "" {
		cleanedLines = cleanedLines[:len(cleanedLines)-1]
	}

	return strings.Join(cleanedLines, "\n"), changed
}

func processCommitsWithRebase(commits []Commit) (int, error) {
	if len(commits) == 0 {
		return 0, nil
	}

	fmt.Fprintf(os.Stderr, "[DEBUG] Starting rebase process for %d commits\n", len(commits))

	// Find the oldest commit to rebase from
	oldestCommit := commits[len(commits)-1]
	fmt.Fprintf(os.Stderr, "[DEBUG] Oldest commit to process: %s\n", oldestCommit.SHA[:8])

	// Get the parent of the oldest commit for rebase starting point
	parentCmd := exec.Command("git", "show", "-s", "--format=%P", oldestCommit.SHA)
	parentOutput, err := parentCmd.Output()
	if err != nil {
		return 0, fmt.Errorf("failed to get parent of oldest commit: %v", err)
	}
	parents := strings.Fields(strings.TrimSpace(string(parentOutput)))

	var rebaseFrom string
	if len(parents) > 0 {
		rebaseFrom = parents[0]
		fmt.Fprintf(os.Stderr, "[DEBUG] Rebase from parent: %s\n", rebaseFrom[:8])
	} else {
		// If no parent, use --root
		rebaseFrom = "--root"
		fmt.Fprintf(os.Stderr, "[DEBUG] Rebase from root (no parent)\n")
	}

	// Check current branch and git state
	branchCmd := exec.Command("git", "branch", "--show-current")
	branchOutput, err := branchCmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[DEBUG] Warning: could not get current branch: %v\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "[DEBUG] Current branch: %s\n", strings.TrimSpace(string(branchOutput)))
	}

	// Check working tree status
	statusCmd := exec.Command("git", "status", "--porcelain")
	statusOutput, err := statusCmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[DEBUG] Warning: could not get git status: %v\n", err)
	} else {
		if len(strings.TrimSpace(string(statusOutput))) == 0 {
			fmt.Fprintf(os.Stderr, "[DEBUG] Working tree is clean\n")
		} else {
			fmt.Fprintf(os.Stderr, "[DEBUG] Working tree status:\n%s\n", string(statusOutput))
		}
	}

	// Create temporary scripts for the rebase
	sequenceScript, err := createSequenceEditor(commits)
	if err != nil {
		return 0, err
	}
	defer os.Remove(sequenceScript)
	fmt.Fprintf(os.Stderr, "[DEBUG] Created sequence editor: %s\n", sequenceScript)

	editorScript, err := createCommitEditor(commits)
	if err != nil {
		return 0, err
	}
	defer os.Remove(editorScript)
	fmt.Fprintf(os.Stderr, "[DEBUG] Created commit editor: %s\n", editorScript)

	// Show what commits will be affected
	logCmd := exec.Command("git", "log", "--oneline", rebaseFrom+"..HEAD")
	logOutput, err := logCmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[DEBUG] Warning: could not get commit log: %v\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "[DEBUG] Commits to be rebased:\n%s\n", string(logOutput))
	}

	// Run interactive rebase
	fmt.Fprintf(os.Stderr, "[DEBUG] Starting interactive rebase with command: git rebase -i %s\n", rebaseFrom)
	cmd := exec.Command("git", "rebase", "-i", rebaseFrom)
	cmd.Env = append(os.Environ(),
		"GIT_SEQUENCE_EDITOR="+sequenceScript,
		"GIT_EDITOR="+editorScript,
	)

	// Always show git output for debugging
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "[DEBUG] Rebase command failed with error: %v\n", err)
		return 0, fmt.Errorf("interactive rebase failed: %v", err)
	}

	fmt.Fprintf(os.Stderr, "[DEBUG] Rebase completed successfully\n")

	// Check the state after rebase
	finalStatusCmd := exec.Command("git", "status", "--porcelain")
	finalStatusOutput, err := finalStatusCmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[DEBUG] Warning: could not get final git status: %v\n", err)
	} else {
		if len(strings.TrimSpace(string(finalStatusOutput))) == 0 {
			fmt.Fprintf(os.Stderr, "[DEBUG] Final working tree is clean\n")
		} else {
			fmt.Fprintf(os.Stderr, "[DEBUG] Final working tree status:\n%s\n", string(finalStatusOutput))
		}
	}

	return len(commits), nil
}

func createSequenceEditor(commits []Commit) (string, error) {
	script := `#!/bin/bash
# Mark commits that need cleaning for reword
echo "[SEQUENCE] Processing rebase todo list: $1" >&2
cat "$1" >&2

commitSHAs=("`

	for _, commit := range commits {
		script += commit.SHA + "` `"
	}
	script = strings.TrimSuffix(script, "` `") + `")

echo "[SEQUENCE] Commits to mark for reword: ${commitSHAs[*]}" >&2

while read -r line; do
	echo "[SEQUENCE] Processing line: $line" >&2
	for sha in "${commitSHAs[@]}"; do
		# Match against both short and long SHA formats
		short_sha="${sha:0:7}"
		if [[ "$line" == pick\ $short_sha* ]] || [[ "$line" == pick\ $sha* ]]; then
			echo "[SEQUENCE] Marking for reword: $line" >&2
			echo "reword ${line#pick }"
			continue 2
		fi
	done
	echo "[SEQUENCE] Keeping as-is: $line" >&2
	echo "$line"
done < "$1" > "$1.tmp" && mv "$1.tmp" "$1"

echo "[SEQUENCE] Final todo list:" >&2
cat "$1" >&2
`

	tmpFile, err := os.CreateTemp("", "clean-cc-sequence-*.sh")
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()

	if _, err := tmpFile.WriteString(script); err != nil {
		return "", err
	}

	if err := os.Chmod(tmpFile.Name(), 0755); err != nil {
		return "", err
	}

	return tmpFile.Name(), nil
}

func createCommitEditor(commits []Commit) (string, error) {
	// Create a comprehensive commit editor that handles each commit specifically
	script := `#!/bin/bash
# Clean commit messages during rebase
msgFile="$1"

echo "[EDITOR] Called with message file: $msgFile" >&2
echo "[EDITOR] Current working directory: $(pwd)" >&2
echo "[EDITOR] Git status in editor:" >&2
git status --porcelain >&2 2>/dev/null || echo "[EDITOR] Could not get git status" >&2

echo "[EDITOR] Original message content:" >&2
cat "$msgFile" >&2
echo "[EDITOR] ---" >&2

# Get current commit SHA being processed
CURRENT_SHA=$(git rev-parse HEAD 2>/dev/null || echo "unknown")
echo "[EDITOR] Current commit SHA: $CURRENT_SHA" >&2

# Look for the title line to identify which commit we're processing`

	// Add specific handling for each commit
	// Process commits in reverse order (oldest first) for git-auto-commit-message
	commandInvocations := 0
	processOrder := commits
	if *flagMsgCommand != "" {
		// Reverse the slice to process oldest commits first
		processOrder = make([]Commit, len(commits))
		for i, commit := range commits {
			processOrder[len(commits)-1-i] = commit
		}
	}

	for i, commit := range processOrder {
		cleaned, _ := cleanCommitMessage(commit.Message)
		titleLine := strings.Split(commit.Message, "\n")[0]

		// Determine if we should use git-auto-commit-message for this commit
		shouldUseAutoCommit := false
		if *flagMsgCommand != "" {
			shouldGenerate := *flagMsgCommandLimit == 0 || commandInvocations < *flagMsgCommandLimit
			if shouldGenerate {
				shouldUseAutoCommit = true
				commandInvocations++
			}
		}

		script += fmt.Sprintf(`

# Handle commit %d: %s
if grep -Fq %q "$msgFile"; then
    echo "[EDITOR] Identified commit %d (%s)" >&2`, i+1, commit.SHA[:8], titleLine, i+1, commit.SHA[:8])

		// Generate the message handling logic
		if shouldUseAutoCommit {
			script += fmt.Sprintf(`
    # Set environment variable for git-auto-commit-message
    export GIT_AUTO_COMMIT_MESSAGE_CONTEXT="rebase-cleaning-claude-attribution"
    
    # Try git-auto-commit-message, fallback to cleaned message
    if GENERATED_MSG=$(%s 2>&2) && [ -n "$GENERATED_MSG" ]; then
        echo "[EDITOR] Using generated message for commit %d" >&2
        echo "$GENERATED_MSG" > "$msgFile"
    else`, *flagMsgCommand, i+1)
		}

		// Always include the cleaned message fallback
		cleanedWithNewline := cleaned
		if !strings.HasSuffix(cleanedWithNewline, "\n") {
			cleanedWithNewline += "\n"
		}

		script += fmt.Sprintf(`
        echo "[EDITOR] Using cleaned message for commit %d" >&2
        cat > "$msgFile" << 'MSG_EOF'
%sMSG_EOF`, i+1, cleanedWithNewline)

		if shouldUseAutoCommit {
			script += `
    fi`
		}

		script += `
    echo "[EDITOR] Final message content:" >&2
    cat "$msgFile" >&2
    exit 0
fi`
	}

	script += `

# If no specific match found, try generic cleaning
echo "[EDITOR] No specific commit match found, attempting generic cleaning" >&2

if grep -iE "🤖.*generated with.*" "$msgFile" >/dev/null 2>&1 || grep -iE "co-authored-by:.*claude" "$msgFile" >/dev/null 2>&1; then
    echo "[EDITOR] Found attribution lines, cleaning generically" >&2
    
    # Create a cleaned version by processing line by line
    awk '
    BEGIN { skip_empty = 0 }
    /🤖.*[Gg]enerated with.*[Cc]laude/ { next }
    /[Cc]o-[Aa]uthored-[Bb]y:.*[Cc]laude/ { next }
    /^[[:space:]]*$/ { 
        if (skip_empty) next
        empty_lines[++empty_count] = $0
        next 
    }
    {
        # Print any stored empty lines
        for (i = 1; i <= empty_count; i++) {
            print empty_lines[i]
        }
        empty_count = 0
        print $0
        skip_empty = 1
    }
    ' "$msgFile" > "$msgFile.tmp"
    
    # Replace the original file
    mv "$msgFile.tmp" "$msgFile"
    
    echo "[EDITOR] Cleaned message content:" >&2
    cat "$msgFile" >&2
else
    echo "[EDITOR] No attribution found, leaving message unchanged" >&2
fi
`

	tmpFile, err := os.CreateTemp("", "clean-cc-editor-*.sh")
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()

	if _, err := tmpFile.WriteString(script); err != nil {
		return "", err
	}

	if err := os.Chmod(tmpFile.Name(), 0755); err != nil {
		return "", err
	}

	return tmpFile.Name(), nil
}

func findCommitBySHA(commits []Commit, sha string) *Commit {
	for _, commit := range commits {
		if commit.SHA == sha {
			return &commit
		}
	}
	return nil
}

func testMessageGeneration(cleanedMessage string) (string, error) {
	if *flagVerbose {
		fmt.Fprintf(os.Stderr, "[DEBUG] Testing message generation with command: %s\n", *flagMsgCommand)
		fmt.Fprintf(os.Stderr, "[DEBUG] Input message:\n%s\n", cleanedMessage)
	}

	// Simple test that just runs the command with the cleaned message as input
	cmd := exec.Command("bash", "-c", *flagMsgCommand)
	cmd.Stdin = strings.NewReader(cleanedMessage)
	cmd.Env = append(os.Environ(), "GIT_NON_INTERACTIVE=1")

	output, err := cmd.Output()
	if err != nil {
		if *flagVerbose {
			fmt.Fprintf(os.Stderr, "[DEBUG] Message command failed: %v\n", err)
		}
		return "", fmt.Errorf("message command failed: %v", err)
	}

	generatedMessage := strings.TrimSpace(string(output))
	if *flagVerbose {
		fmt.Fprintf(os.Stderr, "[DEBUG] Generated message:\n%s\n", generatedMessage)
	}

	if generatedMessage == "" {
		return cleanedMessage, nil
	}

	return generatedMessage, nil
}

func generateRebaseScript(commits []Commit) error {
	if len(commits) == 0 {
		fmt.Println("#!/bin/bash")
		fmt.Println("# No commits need cleaning")
		return nil
	}

	// Find the oldest commit to rebase from
	oldestCommit := commits[len(commits)-1]

	// Get the parent of the oldest commit for rebase starting point
	parentCmd := exec.Command("git", "show", "-s", "--format=%P", oldestCommit.SHA)
	parentOutput, err := parentCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get parent of oldest commit: %v", err)
	}
	parents := strings.Fields(strings.TrimSpace(string(parentOutput)))

	var rebaseFrom string
	if len(parents) > 0 {
		rebaseFrom = parents[0]
	} else {
		rebaseFrom = "--root"
	}

	fmt.Println("#!/bin/bash")
	fmt.Println("#")
	fmt.Printf("# This script will clean %d commits\n", len(commits))
	fmt.Println("#")
	fmt.Println("set -euo pipefail")
	fmt.Println("")

	// Print the commits that will be processed
	fmt.Println("echo 'Commits to be cleaned:'")
	for i, commit := range commits {
		fmt.Printf("echo '%d. %s (%s)'\n", i+1, commit.SHA[:8], strings.Split(commit.Message, "\n")[0])
	}
	fmt.Println("")

	// Create temporary directory for scripts
	fmt.Println("TEMP_DIR=$(mktemp -d)")
	fmt.Println("echo \"Using temp directory: $TEMP_DIR\"")
	fmt.Println("")

	// Generate the sequence editor script
	fmt.Println("# Create sequence editor script")
	fmt.Println("cat > \"$TEMP_DIR/sequence-editor.sh\" << 'SEQUENCE_EOF'")
	fmt.Println("#!/bin/bash")
	fmt.Println("# Mark commits for reword")

	// Add the commit SHAs to reword
	fmt.Print("COMMITS_TO_REWORD=(")
	for _, commit := range commits {
		fmt.Printf("\"%s\" ", commit.SHA)
	}
	fmt.Println(")")

	fmt.Println(`
while read -r line; do
    for sha in "${COMMITS_TO_REWORD[@]}"; do
        # Match against both short and long SHA formats
        short_sha="${sha:0:7}"
        if [[ "$line" == pick\ $short_sha* ]] || [[ "$line" == pick\ $sha* ]]; then
            echo "reword ${line#pick }"
            continue 2
        fi
    done
    echo "$line"
done < "$1" > "$1.tmp" && mv "$1.tmp" "$1"`)
	fmt.Println("SEQUENCE_EOF")
	fmt.Println("")

	// Generate the commit editor script with all the cleaned messages
	fmt.Println("# Create commit editor script with cleaned messages")
	fmt.Println("cat > \"$TEMP_DIR/commit-editor.sh\" << 'COMMIT_EOF'")
	fmt.Println("#!/bin/bash")
	fmt.Println("MSG_FILE=\"$1\"")
	fmt.Println("")
	fmt.Println("# Read the original commit message to identify which commit we're processing")
	fmt.Println("ORIGINAL_MSG=$(cat \"$MSG_FILE\")")
	fmt.Println("")

	// Add cases for each commit
	// Process commits in reverse order (oldest first) for git-auto-commit-message
	commandInvocations := 0
	processOrder := commits
	if *flagMsgCommand != "" {
		// Reverse the slice to process oldest commits first
		processOrder = make([]Commit, len(commits))
		for i, commit := range commits {
			processOrder[len(commits)-1-i] = commit
		}
	}

	for i, commit := range processOrder {
		cleaned, _ := cleanCommitMessage(commit.Message)

		// For script generation, don't test message generation during dry-run
		// Instead, embed the logic in the script to generate during rebase
		shouldUseAutoCommit := false
		if *flagMsgCommand != "" {
			shouldGenerate := *flagMsgCommandLimit == 0 || commandInvocations < *flagMsgCommandLimit
			if shouldGenerate {
				shouldUseAutoCommit = true
				commandInvocations++
			} else {
				// Add a comment in the script explaining why we're not generating
				fmt.Printf("# Commit %d: Using cleaned message (msg-command limit %d reached)\n", i+1, *flagMsgCommandLimit)
			}
		}

		// Use the first line as identifier
		titleLine := strings.Split(commit.Message, "\n")[0]

		fmt.Printf("# Handle commit: %s\n", commit.SHA[:8])
		fmt.Printf("if grep -Fq %q \"$MSG_FILE\"; then\n", titleLine)

		// Generate the message handling logic
		if shouldUseAutoCommit {
			fmt.Printf("    # Set environment variable for git-auto-commit-message\n")
			fmt.Printf("    export GIT_AUTO_COMMIT_MESSAGE_CONTEXT=\"rebase-cleaning-claude-attribution\"\n")
			fmt.Printf("    \n")
			fmt.Printf("    # Try git-auto-commit-message, fallback to cleaned message\n")
			fmt.Printf("    if GENERATED_MSG=$(%s 2>&2) && [ -n \"$GENERATED_MSG\" ]; then\n", *flagMsgCommand)
			fmt.Println("        echo \"$GENERATED_MSG\" > \"$MSG_FILE\"")
			fmt.Println("    else")
		}

		// Always include the cleaned message fallback
		fmt.Println("        cat > \"$MSG_FILE\" << 'MSG_EOF'")
		fmt.Print(cleaned)
		if !strings.HasSuffix(cleaned, "\n") {
			fmt.Println()
		}
		fmt.Println("MSG_EOF")

		if shouldUseAutoCommit {
			fmt.Println("    fi")
		}

		fmt.Println("    exit 0")
		fmt.Println("fi")
		fmt.Println("")
	}

	fmt.Println("# If no match found, leave message as-is")
	fmt.Println("COMMIT_EOF")
	fmt.Println("")

	// Make scripts executable
	fmt.Println("chmod +x \"$TEMP_DIR/sequence-editor.sh\"")
	fmt.Println("chmod +x \"$TEMP_DIR/commit-editor.sh\"")
	fmt.Println("")

	// Show what will happen
	fmt.Println("echo 'About to run interactive rebase...'")
	fmt.Println("echo 'This will rewrite git history for the following commits:'")
	fmt.Println("git log --oneline " + rebaseFrom + "..HEAD")
	fmt.Println("")

	// Confirmation prompt
	fmt.Println("read -p 'Continue? (y/N): ' -n 1 -r")
	fmt.Println("echo")
	fmt.Println("if [[ ! $REPLY =~ ^[Yy]$ ]]; then")
	fmt.Println("    echo 'Aborted'")
	fmt.Println("    rm -rf \"$TEMP_DIR\"")
	fmt.Println("    exit 1")
	fmt.Println("fi")
	fmt.Println("")

	// Run the rebase
	fmt.Println("echo 'Running interactive rebase...'")
	fmt.Printf("GIT_SEQUENCE_EDITOR=\"$TEMP_DIR/sequence-editor.sh\" \\\n")
	fmt.Printf("GIT_EDITOR=\"$TEMP_DIR/commit-editor.sh\" \\\n")
	fmt.Printf("git rebase -i %s\n", rebaseFrom)
	fmt.Println("")

	// Cleanup
	fmt.Println("echo 'Cleaning up...'")
	fmt.Println("rm -rf \"$TEMP_DIR\"")
	fmt.Println("")
	fmt.Println("if [ $? -eq 0 ]; then")
	fmt.Printf("    echo 'Successfully cleaned %d commits!'\n", len(commits))
	fmt.Println("else")
	fmt.Println("    echo 'Rebase failed. You may need to resolve conflicts manually.'")
	fmt.Println("    exit 1")
	fmt.Println("fi")

	return nil
}

func addGitNote(noteRef, commitSHA, note string) error {
	cmd := exec.Command("git", "notes", "--ref="+noteRef, "add", "-f", "-m", note, commitSHA)
	return cmd.Run()
}

func indentText(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = "    " + line
	}
	return strings.Join(lines, "\n")
}

func printUsage() {
	doc := documentation
	// Strip Go comment delimiters
	if strings.HasPrefix(doc, "/*") {
		doc = doc[2:]
	}
	if idx := strings.Index(doc, "*/"); idx != -1 {
		doc = doc[:idx]
	}
	
	// Just print everything up to "# NOTES" section
	if idx := strings.Index(doc, "\n# NOTES"); idx != -1 {
		doc = doc[:idx]
	}
	
	fmt.Print(strings.TrimSpace(doc))
}
