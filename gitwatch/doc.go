// Command gitwatch displays real-time Git repository activity in a terminal with rich visuals.
//
// This tool provides a live view of your Git repository, showing commit history,
// branch structure, and worktree information. It automatically refreshes to show
// the latest changes as they happen.
//
// Usage:
//
//	gitwatch [flags]
//
// Flags:
//
//	-n count       Number of commits to display (default: 20)
//	-r duration    Refresh rate, e.g. 1s, 500ms (default: 2s)
//	-compact       Use compact vertical spacing
//	-rotate        Rotate through different display formats
//	-format index  Format index 0-3 (default: 0)
//	-width pixels  Terminal width in pixels for graph calculation
//	-help, -h      Show this help message
//
// Display Formats:
//
//	0 - Standard: hash, refs, message, relative time, author
//	1 - Compact: hash, refs, message only
//	2 - Detailed: hash, refs, message, date, author, email
//	3 - Branch visualization: optimized for viewing branch structure
//
// Examples:
//
//	# Watch the current repository with default settings
//	gitwatch
//
//	# Show last 50 commits with 1-second refresh
//	gitwatch -n 50 -r 1s
//
//	# Use compact mode for more commits on screen
//	gitwatch -compact -n 40
//
//	# Cycle through display formats automatically
//	gitwatch -rotate
//
// The tool uses Git's native coloring and graph visualization for optimal display.
// Press Ctrl+C to exit.
package main

//go:generate go run github.com/tmc/misc/gocmddoc@latest -o README.md