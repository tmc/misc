// gitwatch displays real-time Git repository activity in a terminal with rich visuals.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"time"
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

func main() {
	// Parse command-line flags
	numCommits := flag.Int("n", 20, "Number of commits to display")
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

	// Main display loop
	for {
		// Clear the screen
		fmt.Print("\033[2J\033[H")
		
		// If rotation is enabled, check if it's time to rotate
		if *rotate && time.Since(lastRotation) >= rotationInterval {
			currentFormat = (currentFormat + 1) % len(formatPatterns)
			lastRotation = time.Now()
		}
		
		// Get the current format pattern
		format := formatPatterns[currentFormat]
		
		// Run git log command with the selected format
		logCmd := exec.Command("git", "-c", "color.ui=always", "log", "--all", "--graph", 
			"--pretty=format:" + format, 
			"--date-order", fmt.Sprintf("-%d", *numCommits))
		logCmd.Stdout = os.Stdout
		logCmd.Run()
		
		// Print worktrees section
		if *compactMode {
			fmt.Print("\n\033[1;34m=== Worktrees ===\033[0m ")
			
			// Run compact worktree command that outputs everything on one line
			wtCmd := exec.Command("bash", "-c", "git worktree list | awk '{print \"\\033[1;34m[\"$1\"]\033[0m\"}' | tr '\\n' ' '")
			wtCmd.Stdout = os.Stdout
			wtCmd.Run()
			fmt.Println()
		} else {
			fmt.Println("\n\033[1;34m=== Worktrees ===\033[0m")
			
			// Run standard worktree command
			wtCmd := exec.Command("bash", "-c", "git worktree list --porcelain | awk '{print \"\\033[1;34m\" $0 \"\\033[0m\"}'")
			wtCmd.Stdout = os.Stdout
			wtCmd.Run()
		}
		
		// Show instruction and info
		fmt.Printf("\nFormat: %d/%d | Press Ctrl+C to exit | Refreshing in %s...\n", 
			currentFormat+1, len(formatPatterns), refreshRate)
		
		time.Sleep(*refreshRate)
	}
}