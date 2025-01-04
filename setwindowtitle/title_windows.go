//go:build windows

package main

import (
	"fmt"
	"golang.org/x/sys/windows"
)

func setWindowTitle(title string) error {
	// Try console API first
	handle := windows.GetConsoleWindow()
	if handle != 0 {
		titlePtr, err := windows.UTF16PtrFromString(title)
		if err != nil {
			return fmt.Errorf("failed to convert title: %v", err)
		}
		
		if err := windows.SetWindowText(handle, titlePtr); err != nil {
			return fmt.Errorf("failed to set window title: %v", err)
		}
		return nil
	}

	// Fallback to ANSI escape sequence for Windows Terminal and others
	fmt.Printf("\033]0;%s\007", title)
	return nil
}

