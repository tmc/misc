// gitwatch displays real-time Git repository activity in a terminal with rich visuals.
package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"
)

// Format patterns that can be cycled through
var formatPatterns = []string{
	// Standard format (default)
	"%C(bold yellow)%h%C(auto)%d %C(bold white)%s %C(dim cyan)(%cr) %C(dim green)[%an]%C(reset)",
	// Compact format with hash and message only
	"%C(bold yellow)%h%C(auto)%d %C(bold white)%s%C(reset)",
	// Detailed format with commit date
	"%C(bold yellow)%h%C(auto)%d %C(bold white)%s %C(dim cyan)[%ad] %C(dim green)[%an]%C(reset) %C(blue)<%ae>%C(reset)",
	// Stats focused format (better for branch visualization)
	"%C(bold yellow)%h%C(auto)%d %C(bold white)%s %C(dim cyan)(%ar)%C(reset)",
}

// initAlternateScreen enters alternate screen buffer and hides cursor
func initAlternateScreen() {
	fmt.Print("\033[?1049h\033[H\033[?25l")
}

// exitAlternateScreen restores normal screen buffer and shows cursor
func exitAlternateScreen() {
	fmt.Print("\033[?25h\033[?1049l")
}

// clearScreen moves cursor to home and clears screen without flicker
func clearScreen() {
	fmt.Print("\033[H\033[2J")
}

// countLines counts the number of lines in the given text
func countLines(text string) int {
	if len(text) == 0 {
		return 0
	}
	return strings.Count(text, "\n") + 1
}

// executeCommand runs a command and captures its output as a string
func executeCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	err := cmd.Run()
	return buf.String(), err
}

// printWithLimit prints text but ensures it doesn't exceed the given number of lines
func printWithLimit(text string, maxLines int) int {
	if maxLines <= 0 {
		return 0
	}

	scanner := bufio.NewScanner(strings.NewReader(text))
	linesShown := 0

	for scanner.Scan() && linesShown < maxLines {
		fmt.Println(scanner.Text())
		linesShown++
	}

	return linesShown
}

// getTerminalHeight returns the height of the terminal in lines.
// Returns a default value if unable to detect.
func getTerminalHeight() int {
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		return 24 // Default for non-terminal output
	}

	_, height, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return 24 // Default if we can't get terminal size
	}
	return height
}

// calculateCommitLines determines how many commits can fit in the terminal.
// Reserves space for header, worktrees, and footer.
func calculateCommitLines(termHeight int, compactMode bool) int {
	// Reserve lines:
	// - 1 line for clearing/positioning
	// - 3-8 lines for worktrees (depending on mode)
	// - 2 lines for footer (format info + refresh info)
	// - 1 line for buffer

	worktreeLines := 8
	if compactMode {
		worktreeLines = 3
	}

	reservedLines := 1 + worktreeLines + 2 + 1
	availableLines := termHeight - reservedLines

	// Ensure we show at least 5 commits
	if availableLines < 5 {
		return 5
	}

	return availableLines
}

func main() {
	// Parse command-line flags
	numCommits := flag.Int("n", 0, "Number of commits to display (0=auto based on terminal height)")
	refreshRate := flag.Duration("r", 2*time.Second, "Refresh rate (e.g. 1s, 500ms)")
	compactMode := flag.Bool("compact", false, "Use compact vertical spacing")
	rotate := flag.Bool("rotate", false, "Rotate through different display formats")
	formatIdx := flag.Int("format", 0, "Format index (0-3)")
	flag.Parse()

	// Validate format index
	if *formatIdx < 0 || *formatIdx >= len(formatPatterns) {
		*formatIdx = 0
	}

	currentFormat := *formatIdx
	rotationInterval := 10 * time.Second // Time between format rotations
	lastRotation := time.Now()

	// Set up signal handling for clean exit
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	// Initialize alternate screen buffer
	initAlternateScreen()
	defer exitAlternateScreen()

	// Handle cleanup on signal
	go func() {
		<-sigCh
		exitAlternateScreen()
		os.Exit(0)
	}()

	// Main display loop
	for {
		// Get terminal height and calculate how many commits to show
		termHeight := getTerminalHeight()
		commitsToShow := *numCommits
		if commitsToShow == 0 {
			// Auto-calculate based on terminal height
			commitsToShow = calculateCommitLines(termHeight, *compactMode)
		}

		// Ensure commits don't overflow terminal
		maxCommits := calculateCommitLines(termHeight, *compactMode)
		if commitsToShow > maxCommits {
			commitsToShow = maxCommits
		}

		// If rotation is enabled, check if it's time to rotate
		if *rotate && time.Since(lastRotation) >= rotationInterval {
			currentFormat = (currentFormat + 1) % len(formatPatterns)
			lastRotation = time.Now()
		}

		// Get the current format pattern
		format := formatPatterns[currentFormat]

		// Build entire screen content in memory first
		var screenBuffer bytes.Buffer

		// Get git log output
		logOutput, err := executeCommand("git", "-c", "color.ui=always", "log", "--all", "--graph",
			"--pretty=format:"+format,
			"--date-order", fmt.Sprintf("-%d", commitsToShow))
		if err != nil {
			screenBuffer.WriteString(fmt.Sprintf("Error running git log: %v\n", err))
		} else {
			// Calculate how many lines we can use for the log
			worktreeLines := 8
			if *compactMode {
				worktreeLines = 3
			}
			footerLines := 2
			availableLogLines := termHeight - worktreeLines - footerLines - 2 // 2 for buffer

			// Add git log to buffer with line limit
			scanner := bufio.NewScanner(strings.NewReader(logOutput))
			linesShown := 0
			for scanner.Scan() && linesShown < availableLogLines {
				screenBuffer.WriteString(scanner.Text() + "\n")
				linesShown++
			}
		}

		// Add worktrees section to buffer
		if *compactMode {
			screenBuffer.WriteString("\n\033[1;34m=== Worktrees ===\033[0m ")
			wtOutput, err := executeCommand("bash", "-c", "git worktree list | awk '{print \"\\033[1;34m[\"$1\"]\033[0m\"}' | tr '\\n' ' '")
			if err == nil {
				screenBuffer.WriteString(strings.TrimSpace(wtOutput))
			}
			screenBuffer.WriteString("\n")
		} else {
			screenBuffer.WriteString("\n\033[1;34m=== Worktrees ===\033[0m\n")
			wtOutput, err := executeCommand("bash", "-c", "git worktree list --porcelain | awk '{print \"\\033[1;34m\" $0 \"\\033[0m\"}'")
			if err == nil {
				scanner := bufio.NewScanner(strings.NewReader(wtOutput))
				linesShown := 0
				for scanner.Scan() && linesShown < 5 {
					screenBuffer.WriteString(scanner.Text() + "\n")
					linesShown++
				}
			}
		}

		// Add footer to buffer with proper line clearing
		statusLine := ""
		if *rotate {
			statusLine = fmt.Sprintf("\n\033[2mFormat %d/%d • %d commits • %s\033[0m",
				currentFormat+1, len(formatPatterns), commitsToShow, refreshRate)
		} else {
			statusLine = fmt.Sprintf("\n\033[2m%d commits • %s\033[0m",
				commitsToShow, refreshRate)
		}
		screenBuffer.WriteString(statusLine)
		screenBuffer.WriteString("\033[K") // Clear to end of line to remove any leftover text

		// Now output everything at once with minimal screen manipulation
		fmt.Print("\033[H") // Move cursor to top-left
		fmt.Print(screenBuffer.String())
		fmt.Print("\033[J") // Clear from cursor to end of screen to remove any leftover content
		
		time.Sleep(*refreshRate)
	}
}