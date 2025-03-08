package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// Debug script to check Chrome installation and launch
func runChromeDebug() error {
	chromePath := "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
	fmt.Println("=== Chrome Debug Information ===")
	
	// Check if Chrome exists
	if _, err := os.Stat(chromePath); err != nil {
		fmt.Println("Chrome not found at path:", chromePath)
		return err
	}
	fmt.Println("Chrome found at:", chromePath)
	
	// Try to launch Chrome with --version
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, chromePath, "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println("Failed to run Chrome version command:", err)
		return err
	}
	fmt.Printf("Chrome version: %s\n", output)
	
	// Check Chrome profile paths
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("Failed to get home directory:", err)
	} else {
		chromeProfilePath := filepath.Join(homeDir, "Library", "Application Support", "Google", "Chrome")
		if _, err := os.Stat(chromeProfilePath); err != nil {
			fmt.Println("Chrome profile directory not found at:", chromeProfilePath)
		} else {
			profiles, err := os.ReadDir(chromeProfilePath)
			if err != nil {
				fmt.Println("Failed to read Chrome profile directory:", err)
			} else {
				fmt.Println("Found Chrome profiles:")
				for _, profile := range profiles {
					if profile.IsDir() {
						fmt.Println(" -", profile.Name())
					}
				}
			}
		}
	}

	// Check if Chrome processes are running
	fmt.Println("\nChecking for running Chrome processes:")
	psCmd := exec.Command("ps", "aux")
	grepCmd := exec.Command("grep", "Chrome")
	psOut, err := psCmd.StdoutPipe()
	if err != nil {
		fmt.Println("Error creating pipe:", err)
	} else {
		grepCmd.Stdin = psOut
		grepOut, _ := grepCmd.StdoutPipe()
		psCmd.Start()
		grepCmd.Start()
		
		var grepBytes []byte
		buffer := make([]byte, 1024)
		for {
			n, err := grepOut.Read(buffer)
			if err != nil || n == 0 {
				break
			}
			grepBytes = append(grepBytes, buffer[:n]...)
		}
		
		psCmd.Wait()
		grepCmd.Wait()
		fmt.Println(string(grepBytes))
	}
	
	// Try launching Chrome with remote debugging with a timeout
	fmt.Println("\nChecking if Chrome can be launched with remote debugging (10s timeout)...")
	tempDir, err := os.MkdirTemp("", "chrome-debug-")
	if err != nil {
		fmt.Println("Failed to create temp directory:", err)
		return err
	}
	defer os.RemoveAll(tempDir)
	
	debugCtx, debugCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer debugCancel()
	
	// Try launching Chrome with remote debugging
	debugCmd := exec.CommandContext(
		debugCtx,
		chromePath,
		"--headless",
		"--disable-gpu",
		"--no-sandbox",
		"--remote-debugging-port=9222",
		"--user-data-dir="+tempDir,
		"about:blank",
	)
	
	debugOutput, err := debugCmd.CombinedOutput()
	if err != nil {
		fmt.Println("Failed to launch Chrome with debugging:", err)
		fmt.Println("Chrome output:")
		fmt.Println(string(debugOutput))
	} else {
		fmt.Println("Chrome launched successfully with remote debugging")
		fmt.Println("Output:", string(debugOutput))
	}
	
	// Try to list available Chrome DevTools endpoints
	fmt.Println("\nTrying to connect to Chrome DevTools (if Chrome was launched)...")
	curlCtx, curlCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer curlCancel()
	curlCmd := exec.CommandContext(curlCtx, "curl", "-s", "http://localhost:9222/json/list")
	curlOutput, err := curlCmd.CombinedOutput()
	if err != nil {
		fmt.Println("Failed to connect to Chrome DevTools:", err)
	} else {
		fmt.Println("Chrome DevTools endpoints:")
		fmt.Println(string(curlOutput))
	}
	
	return nil
}