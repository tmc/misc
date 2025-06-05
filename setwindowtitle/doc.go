// Command setwindowtitle sets the terminal window title.
//
// This cross-platform tool changes the title of your terminal window
// using ANSI escape sequences. It works with most modern terminal
// emulators including iTerm2, Terminal.app, GNOME Terminal, and others.
//
// Usage:
//
//	setwindowtitle <title>
//
// Examples:
//
//	# Set a simple title
//	setwindowtitle "Development"
//
//	# Set title with current directory
//	setwindowtitle "Working in $(pwd)"
//
//	# Use in scripts to track progress
//	setwindowtitle "Building project..."
//	make build
//	setwindowtitle "Build complete"
//
// The tool automatically detects the terminal type and uses the
// appropriate escape sequences. On unsupported terminals, it will
// fail gracefully with an error message.
package main

//go:generate go run github.com/tmc/misc/gocmddoc@latest -o README.md