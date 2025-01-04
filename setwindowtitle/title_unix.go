//go:build darwin || linux

package main

import (
	"fmt"
	"strings"
)

func setWindowTitle(title string) error {
	term := detectTerminal()

	switch {
	case strings.Contains(term, "kitty"):
		// Kitty terminal protocol
		fmt.Printf("\x1b]2;%s\x1b\\", title)
	case strings.Contains(term, "iterm"):
		// iTerm2 proprietary escape sequence
		fmt.Printf("\033]1337;SetBadgeFormat=%s\007", title)
		// Also set regular title
		fmt.Printf("\033]0;%s\007", title)
	default:
		// Standard xterm/vt100 escape sequence
		// Works in most terminals including Terminal.app, xterm, gnome-terminal
		fmt.Printf("\033]0;%s\007", title)
	}
	return nil
}

