//go:build linux

package main

import (
	"fmt"
)

func setWindowTitle(title string) error {
	// ANSI escape sequence to set window title
	fmt.Printf("\033]0;%s\007", title)
	return nil
}
