// The CDP command-line tool for Chrome DevTools Protocol interaction.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/pkg/errors"
)

var aliases = map[string]string{
	// Shortcuts for common operations
	"goto":      `Page.navigate {"url":"$1"}`,
	"reload":    `Page.reload {}`,
	"title":     `Runtime.evaluate {"expression":"document.title"}`,
	"url":       `Runtime.evaluate {"expression":"window.location.href"}`,
	"html":      `Runtime.evaluate {"expression":"document.documentElement.outerHTML"}`,
	"cookies":   `Network.getAllCookies {}`,
	"screenshot": `Page.captureScreenshot {}`,
	"pdf":       `Page.printToPDF {}`,
	
	// Debugging
	"pause":     `Debugger.pause {}`,
	"resume":    `Debugger.resume {}`,
	"step":      `Debugger.stepInto {}`,
	"next":      `Debugger.stepOver {}`,
	"out":       `Debugger.stepOut {}`,
	
	// DOM interaction
	"click":     `Runtime.evaluate {"expression":"document.querySelector('$1').click()"}`,
	"focus":     `Runtime.evaluate {"expression":"document.querySelector('$1').focus()"}`,
	"type":      `Input.insertText {"text":"$1"}`,
	
	// Device emulation
	"mobile":    `Emulation.setDeviceMetricsOverride {"width":375,"height":812,"deviceScaleFactor":3,"mobile":true}`,
	"desktop":   `Emulation.clearDeviceMetricsOverride {}`,
	
	// Performance & coverage
	"metrics":   `Performance.getMetrics {}`,
	"coverage_start": `Profiler.startPreciseCoverage {"callCount":true,"detailed":true}`,
	"coverage_take":  `Profiler.takePreciseCoverage {}`,
	"coverage_stop":  `Profiler.stopPreciseCoverage {}`,
}

func main() {
	var (
		url       string
		headless  bool
		debugPort int
		timeout   int
		verbose   bool
	)
	
	flag.StringVar(&url, "url", "about:blank", "URL to navigate to on start")
	flag.BoolVar(&headless, "headless", false, "Run Chrome in headless mode")
	flag.IntVar(&debugPort, "debug-port", 0, "Connect to Chrome on specific port (0 for auto)")
	flag.IntVar(&timeout, "timeout", 60, "Timeout in seconds")
	flag.BoolVar(&verbose, "verbose", false, "Enable verbose logging")
	
	flag.Parse()
	
	// Set up context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()
	
	// Handle Ctrl+C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Signal received, shutting down...")
		cancel()
	}()
	
	// Chrome options
	opts := []chromedp.ExecAllocatorOption{
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		
		// Add stability flags
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("disable-background-timer-throttling", true),
		chromedp.Flag("disable-popup-blocking", true),
		chromedp.Flag("disable-sync", true),
	}
	
	if headless {
		opts = append(opts, chromedp.Headless)
		if verbose {
			log.Println("Running Chrome in headless mode")
		}
	}
	
	// Create Chrome allocator
	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, opts...)
	defer allocCancel()
	
	// Create Chrome browser context
	var browserCtx context.Context
	var browserCancel context.CancelFunc
	
	if verbose {
		browserCtx, browserCancel = chromedp.NewContext(allocCtx, chromedp.WithLogf(log.Printf))
	} else {
		browserCtx, browserCancel = chromedp.NewContext(allocCtx)
	}
	defer browserCancel()
	
	// Start and connect to browser
	if err := chromedp.Run(browserCtx, chromedp.Navigate(url)); err != nil {
		log.Fatalf("Error launching Chrome: %v", err)
	}
	
	// Interactive loop
	fmt.Println("Connected to Chrome. Type commands or 'help' for assistance.")
	fmt.Println("Examples: 'goto https://example.com', 'title', 'screenshot'")
	
	scanner := bufio.NewScanner(os.Stdin)
	
	for {
		fmt.Print("cdp> ")
		if !scanner.Scan() {
			break
		}
		
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		
		if line == "exit" || line == "quit" {
			break
		}
		
		if line == "help" {
			printHelp()
			continue
		}
		
		if line == "help aliases" {
			printAliases()
			continue
		}
		
		// Process command or alias
		var cmdToRun string
		parts := strings.SplitN(line, " ", 2)
		cmd := parts[0]
		
		if alias, ok := aliases[cmd]; ok {
			// It's an alias
			cmdToRun = alias
			
			// Check if it has parameters
			if strings.Contains(alias, "$1") && len(parts) > 1 {
				cmdToRun = strings.ReplaceAll(cmdToRun, "$1", parts[1])
			}
			
			fmt.Printf("Alias: %s\n", cmdToRun)
		} else {
			// Raw CDP command
			cmdToRun = line
		}
		
		// Parse CDP command and execute
		if err := executeCommand(browserCtx, cmdToRun); err != nil {
			fmt.Printf("Error: %v\n", err)
		}
	}
	
	if err := scanner.Err(); err != nil {
		log.Fatalf("Error reading input: %v", err)
	}
	
	fmt.Println("Exiting...")
}

func executeCommand(ctx context.Context, command string) error {
	// Parse Domain.method {params}
	parts := strings.SplitN(command, " ", 2)
	if len(parts) == 0 {
		return errors.New("empty command")
	}
	
	method := parts[0]
	if !strings.Contains(method, ".") {
		return errors.New("invalid command format: expected 'Domain.method'")
	}
	
	// Parse parameters
	var params json.RawMessage
	if len(parts) > 1 {
		paramStr := strings.TrimSpace(parts[1])
		if paramStr == "" || paramStr == "{}" {
			params = json.RawMessage("{}")
		} else {
			// Validate JSON
			var temp map[string]interface{}
			if err := json.Unmarshal([]byte(paramStr), &temp); err != nil {
				return errors.Wrap(err, "invalid JSON parameters")
			}
			params = json.RawMessage(paramStr)
		}
	} else {
		params = json.RawMessage("{}")
	}
	
	// Special case for Runtime.evaluate since it's very common
	if method == "Runtime.evaluate" {
		var evalParams runtime.EvaluateParams
		if err := json.Unmarshal(params, &evalParams); err != nil {
			return errors.Wrap(err, "parsing Runtime.evaluate parameters")
		}
		
		var result interface{}
		if err := chromedp.Run(ctx, chromedp.Evaluate(evalParams.Expression, &result)); err != nil {
			return err
		}
		
		fmt.Println("Result:", result)
		return nil
	}
	
	// Special case for navigation which is very common
	if method == "Page.navigate" {
		var navParams struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal(params, &navParams); err != nil {
			return errors.Wrap(err, "parsing Page.navigate parameters")
		}
		
		if err := chromedp.Run(ctx, chromedp.Navigate(navParams.URL)); err != nil {
			return err
		}
		
		fmt.Println("Navigated to:", navParams.URL)
		return nil
	}
	
	// Special case for screenshots which are very common
	if method == "Page.captureScreenshot" {
		var buf []byte
		if err := chromedp.Run(ctx, chromedp.CaptureScreenshot(&buf)); err != nil {
			return err
		}
		
		// Save screenshot to file
		filename := fmt.Sprintf("screenshot-%d.png", time.Now().Unix())
		if err := os.WriteFile(filename, buf, 0644); err != nil {
			return errors.Wrap(err, "saving screenshot")
		}
		
		fmt.Println("Screenshot saved to:", filename)
		return nil
	}
	
	// For other commands, we provide a simplified implementation
	// which doesn't support all CDP methods but covers the basics
	fmt.Printf("Executing: %s with params %s\n", method, string(params))
	fmt.Println("(This is a simplified implementation that doesn't support all CDP methods)")
	
	// Execute appropriate CDP action if we know how to handle it
	if strings.HasPrefix(method, "Runtime.") {
		return executeCDPRuntime(ctx, method, params)
	} else if strings.HasPrefix(method, "Page.") {
		return executeCDPPage(ctx, method, params)
	} else if strings.HasPrefix(method, "Network.") {
		return executeCDPNetwork(ctx, method, params)
	} else if strings.HasPrefix(method, "DOM.") {
		return executeCDPDOM(ctx, method, params)
	}
	
	return errors.Errorf("unsupported CDP method: %s", method)
}

func executeCDPRuntime(ctx context.Context, method string, params json.RawMessage) error {
	// Only handle a few common Runtime methods as examples
	switch method {
	case "Runtime.evaluate":
		// Handled specially above
		return nil
		
	default:
		return errors.Errorf("unsupported Runtime method: %s", method)
	}
}

func executeCDPPage(ctx context.Context, method string, params json.RawMessage) error {
	// Only handle a few common Page methods as examples
	switch method {
	case "Page.navigate":
		// Handled specially above
		return nil
		
	case "Page.reload":
		return chromedp.Run(ctx, chromedp.Reload())
		
	case "Page.captureScreenshot":
		// Handled specially above
		return nil
		
	default:
		return errors.Errorf("unsupported Page method: %s", method)
	}
}

func executeCDPNetwork(ctx context.Context, method string, params json.RawMessage) error {
	// Only handle a few common Network methods as examples
	switch method {
	case "Network.getAllCookies":
		// Simple implementation that just gets cookies via JavaScript
		var cookies interface{}
		if err := chromedp.Run(ctx, chromedp.Evaluate("document.cookie", &cookies)); err != nil {
			return err
		}
		
		fmt.Println("Cookies:", cookies)
		return nil
		
	default:
		return errors.Errorf("unsupported Network method: %s", method)
	}
}

func executeCDPDOM(ctx context.Context, method string, params json.RawMessage) error {
	// Only handle a few common DOM methods as examples
	switch method {
	case "DOM.getDocument":
		// Simplified implementation
		var html string
		if err := chromedp.Run(ctx, chromedp.OuterHTML("html", &html)); err != nil {
			return err
		}
		
		fmt.Printf("HTML length: %d bytes\n", len(html))
		fmt.Println("(HTML content not shown - too large)")
		return nil
		
	default:
		return errors.Errorf("unsupported DOM method: %s", method)
	}
}

func printHelp() {
	fmt.Println("\nCDP - Chrome DevTools Protocol CLI")
	fmt.Println("\nCommand format:")
	fmt.Println("  Domain.method {\"param\":\"value\"}")
	fmt.Println("  Examples:")
	fmt.Println("    Page.navigate {\"url\":\"https://example.com\"}")
	fmt.Println("    Runtime.evaluate {\"expression\":\"document.title\"}")
	
	fmt.Println("\nCommon commands:")
	fmt.Println("  Page.navigate     - Navigate to a URL")
	fmt.Println("  Page.reload       - Reload the current page")
	fmt.Println("  Runtime.evaluate  - Evaluate JavaScript")
	fmt.Println("  DOM.getDocument   - Get the DOM document")
	fmt.Println("  Network.getAllCookies - Get all cookies")
	
	fmt.Println("\nAliases:")
	fmt.Println("  goto <url>        - Navigate to URL")
	fmt.Println("  title             - Get page title")
	fmt.Println("  html              - Get page HTML")
	fmt.Println("  screenshot        - Take screenshot")
	fmt.Println("  Type 'help aliases' for a full list")
	
	fmt.Println("\nCommands:")
	fmt.Println("  help              - Show this help")
	fmt.Println("  help aliases      - List all alias commands")
	fmt.Println("  exit / quit       - Exit the program")
}

func printAliases() {
	fmt.Println("\nAvailable Aliases:")
	
	categories := map[string][]string{
		"Navigation": {"goto", "reload"},
		"Page Info": {"title", "url", "html", "cookies"},
		"Media": {"screenshot", "pdf"},
		"Interaction": {"click", "focus", "type"},
		"Device Emulation": {"mobile", "desktop"},
		"Debugging": {"pause", "resume", "step", "next", "out"},
		"Performance": {"metrics", "coverage_start", "coverage_take", "coverage_stop"},
	}
	
	for category, cmds := range categories {
		fmt.Printf("\n%s:\n", category)
		for _, cmd := range cmds {
			if strings.Contains(aliases[cmd], "$1") {
				// Command takes parameters
				fmt.Printf("  %-15s -> %s\n", cmd+" <param>", aliases[cmd])
			} else {
				fmt.Printf("  %-15s -> %s\n", cmd, aliases[cmd])
			}
		}
	}
}