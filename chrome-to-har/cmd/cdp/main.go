// The CDP command-line tool for Chrome DevTools Protocol interaction.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/pkg/errors"
	"github.com/tmc/misc/chrome-to-har/internal/browser"
	"github.com/tmc/misc/chrome-to-har/internal/chromeprofiles"
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
	
	// Enhanced aliases for Playwright-like commands
	"wait":      `@wait $1`,  // Custom command prefix @
	"waitfor":   `@waitfor $1`,
	"text":      `@text $1`,
	"hover":     `@hover $1`,
	"select":    `@select $1 $2`,
	"check":     `@check $1`,
	"uncheck":   `@uncheck $1`,
	"press":     `@press $1`,
	"fill":      `@fill $1 $2`,
	"clear":     `@clear $1`,
	"visible":   `@visible $1`,
	"hidden":    `@hidden $1`,
	"enabled":   `@enabled $1`,
	"disabled":  `@disabled $1`,
	"count":     `@count $1`,
	"attr":      `@attr $1 $2`,
	"css":       `@css $1 $2`,
	"route":     `@route $1 $2`,
	"waitrequest": `@waitrequest $1`,
	"waitresponse": `@waitresponse $1`,
}

// BrowserCandidate represents a potential browser installation
type BrowserCandidate struct {
	Name        string
	Path        string
	Version     string
	IsRunning   bool
	ProcessID   int
	DebugPort   int
}

// ChromeTab represents a Chrome tab
type ChromeTab struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	URL   string `json:"url"`
	Type  string `json:"type"`
}

// HAREntry represents a single HAR entry
type HAREntry struct {
	StartedDateTime string                 `json:"startedDateTime"`
	Request         map[string]interface{} `json:"request"`
	Response        map[string]interface{} `json:"response"`
	Time            float64                `json:"time"`
}

// HARLog represents the HAR log structure
type HARLog struct {
	Version string      `json:"version"`
	Creator interface{} `json:"creator"`
	Pages   []interface{} `json:"pages"`
	Entries []HAREntry   `json:"entries"`
}

// HAR represents the top-level HAR structure
type HAR struct {
	Log HARLog `json:"log"`
}

// NetworkRecorder records network events for HAR generation
type NetworkRecorder struct {
	entries []HAREntry
	mu      sync.RWMutex
}

// AddEntry adds a new HAR entry to the recorder
func (nr *NetworkRecorder) AddEntry(entry HAREntry) {
	nr.mu.Lock()
	defer nr.mu.Unlock()
	nr.entries = append(nr.entries, entry)
}

// GetEntries returns all recorded HAR entries
func (nr *NetworkRecorder) GetEntries() []HAREntry {
	nr.mu.RLock()
	defer nr.mu.RUnlock()
	return append([]HAREntry(nil), nr.entries...)
}

// SaveHAR saves the recorded entries to a HAR file
func (nr *NetworkRecorder) SaveHAR(filename string) error {
	entries := nr.GetEntries()
	har := HAR{
		Log: HARLog{
			Version: "1.2",
			Creator: map[string]interface{}{
				"name":    "CDP-Enhanced",
				"version": "1.0",
			},
			Pages:   []interface{}{},
			Entries: entries,
		},
	}

	data, err := json.MarshalIndent(har, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0644)
}

// checkRunningChrome checks if Chrome is running on a specific port
func checkRunningChrome(port int) bool {
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/json/version", port))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

// getChromeTabs gets list of available tabs from Chrome
func getChromeTabs(port int) ([]ChromeTab, error) {
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/json/list", port))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	var tabs []ChromeTab
	if err := json.NewDecoder(resp.Body).Decode(&tabs); err != nil {
		return nil, err
	}
	
	return tabs, nil
}

// discoverBrowsers finds all available browser installations and running processes
func discoverBrowsers(verbose bool) ([]BrowserCandidate, error) {
	var candidates []BrowserCandidate
	
	// Check for running browsers first
	runningBrowsers, err := findRunningBrowsers(verbose)
	if err != nil && verbose {
		log.Printf("Warning: failed to find running browsers: %v", err)
	}
	candidates = append(candidates, runningBrowsers...)
	
	// Check for installed browsers
	installedBrowsers, err := findInstalledBrowsers(verbose)
	if err != nil && verbose {
		log.Printf("Warning: failed to find installed browsers: %v", err)
	}
	candidates = append(candidates, installedBrowsers...)
	
	return candidates, nil
}

// findRunningBrowsers detects currently running browser processes
func findRunningBrowsers(verbose bool) ([]BrowserCandidate, error) {
	var candidates []BrowserCandidate
	
	// Use ps to find running browser processes
	cmd := exec.Command("ps", "aux")
	output, err := cmd.Output()
	if err != nil {
		return candidates, err
	}
	
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "Chrome") || strings.Contains(line, "Chromium") || 
		   strings.Contains(line, "Brave") || strings.Contains(line, "Edge") {
			
			// Parse the process line to extract useful information
			fields := strings.Fields(line)
			if len(fields) < 11 {
				continue
			}
			
			processName := filepath.Base(fields[10])
			var browserName, browserPath string
			var debugPort int
			
			// Extract browser info
			if strings.Contains(line, "Google Chrome Canary") {
				browserName = "Chrome Canary"
				browserPath = extractExecutablePath(line, "Google Chrome Canary")
			} else if strings.Contains(line, "Google Chrome") {
				browserName = "Chrome"
				browserPath = extractExecutablePath(line, "Google Chrome")
			} else if strings.Contains(line, "Chromium") {
				browserName = "Chromium"
				browserPath = extractExecutablePath(line, "Chromium")
			} else if strings.Contains(line, "Brave") {
				browserName = "Brave"
				browserPath = extractExecutablePath(line, "Brave")
			}
			
			// Extract debug port if present
			if strings.Contains(line, "--remote-debugging-port=") {
				portStr := extractFlag(line, "--remote-debugging-port=")
				if portStr != "" {
					fmt.Sscanf(portStr, "%d", &debugPort)
				}
			}
			
			if browserName != "" && browserPath != "" {
				candidate := BrowserCandidate{
					Name:      browserName,
					Path:      browserPath,
					IsRunning: true,
					DebugPort: debugPort,
				}
				
				// Avoid duplicates
				found := false
				for _, existing := range candidates {
					if existing.Path == candidate.Path && existing.DebugPort == candidate.DebugPort {
						found = true
						break
					}
				}
				
				if !found {
					candidates = append(candidates, candidate)
					if verbose {
						log.Printf("Found running browser: %s at %s (debug port: %d)", 
							browserName, browserPath, debugPort)
					}
				}
			}
		}
	}
	
	return candidates, nil
}

// findInstalledBrowsers looks for browser installations on the system
func findInstalledBrowsers(verbose bool) ([]BrowserCandidate, error) {
	var candidates []BrowserCandidate
	
	switch runtime.GOOS {
	case "darwin":
		return findMacOSBrowsers(verbose)
	case "linux":
		return findLinuxBrowsers(verbose)
	case "windows":
		return findWindowsBrowsers(verbose)
	default:
		return candidates, fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

// findMacOSBrowsers finds browser installations on macOS
func findMacOSBrowsers(verbose bool) ([]BrowserCandidate, error) {
	var candidates []BrowserCandidate
	
	// macOS browser paths in order of preference
	browserPaths := []struct {
		name string
		path string
	}{
		{"Chrome Canary", "/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary"},
		{"Chrome", "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"},
		{"Chrome Beta", "/Applications/Google Chrome Beta.app/Contents/MacOS/Google Chrome Beta"},
		{"Chrome Dev", "/Applications/Google Chrome Dev.app/Contents/MacOS/Google Chrome Dev"},
		{"Chromium", "/Applications/Chromium.app/Contents/MacOS/Chromium"},
		{"Brave", "/Applications/Brave Browser.app/Contents/MacOS/Brave Browser"},
		{"Edge", "/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge"},
		{"Edge Beta", "/Applications/Microsoft Edge Beta.app/Contents/MacOS/Microsoft Edge Beta"},
		{"Edge Dev", "/Applications/Microsoft Edge Dev.app/Contents/MacOS/Microsoft Edge Dev"},
		{"Vivaldi", "/Applications/Vivaldi.app/Contents/MacOS/Vivaldi"},
		{"Opera", "/Applications/Opera.app/Contents/MacOS/Opera"},
		{"Chrome for Testing", "/Users/" + os.Getenv("USER") + "/.cache/puppeteer/chrome/*/chrome-mac*/Google Chrome for Testing.app/Contents/MacOS/Google Chrome for Testing"},
	}
	
	for _, browser := range browserPaths {
		// Handle glob patterns for Chrome for Testing
		if strings.Contains(browser.path, "*") {
			matches, err := filepath.Glob(browser.path)
			if err == nil {
				for _, match := range matches {
					if _, err := os.Stat(match); err == nil {
						version := extractVersionFromPath(match)
						candidates = append(candidates, BrowserCandidate{
							Name:    browser.name,
							Path:    match,
							Version: version,
						})
						if verbose {
							log.Printf("Found browser: %s at %s (version: %s)", browser.name, match, version)
						}
					}
				}
			}
		} else {
			if _, err := os.Stat(browser.path); err == nil {
				version := getBrowserVersion(browser.path)
				candidates = append(candidates, BrowserCandidate{
					Name:    browser.name,
					Path:    browser.path,
					Version: version,
				})
				if verbose {
					log.Printf("Found browser: %s at %s (version: %s)", browser.name, browser.path, version)
				}
			}
		}
	}
	
	return candidates, nil
}

// findLinuxBrowsers finds browser installations on Linux
func findLinuxBrowsers(verbose bool) ([]BrowserCandidate, error) {
	var candidates []BrowserCandidate
	
	// Common Linux browser commands
	browserCommands := []struct {
		name    string
		command string
	}{
		{"Chrome", "google-chrome"},
		{"Chrome Beta", "google-chrome-beta"},
		{"Chrome Dev", "google-chrome-unstable"},
		{"Chromium", "chromium"},
		{"Chromium Browser", "chromium-browser"},
		{"Brave", "brave-browser"},
		{"Edge", "microsoft-edge"},
		{"Edge Beta", "microsoft-edge-beta"},
		{"Edge Dev", "microsoft-edge-dev"},
		{"Vivaldi", "vivaldi"},
		{"Opera", "opera"},
	}
	
	for _, browser := range browserCommands {
		if path, err := exec.LookPath(browser.command); err == nil {
			version := getBrowserVersion(path)
			candidates = append(candidates, BrowserCandidate{
				Name:    browser.name,
				Path:    path,
				Version: version,
			})
			if verbose {
				log.Printf("Found browser: %s at %s (version: %s)", browser.name, path, version)
			}
		}
	}
	
	return candidates, nil
}

// findWindowsBrowsers finds browser installations on Windows
func findWindowsBrowsers(verbose bool) ([]BrowserCandidate, error) {
	var candidates []BrowserCandidate
	
	// Common Windows browser paths
	programFiles := os.Getenv("PROGRAMFILES")
	programFilesX86 := os.Getenv("PROGRAMFILES(X86)")
	localAppData := os.Getenv("LOCALAPPDATA")
	
	browserPaths := []struct {
		name string
		path string
	}{
		{"Chrome", filepath.Join(programFiles, "Google", "Chrome", "Application", "chrome.exe")},
		{"Chrome", filepath.Join(programFilesX86, "Google", "Chrome", "Application", "chrome.exe")},
		{"Chrome", filepath.Join(localAppData, "Google", "Chrome", "Application", "chrome.exe")},
		{"Edge", filepath.Join(programFiles, "Microsoft", "Edge", "Application", "msedge.exe")},
		{"Edge", filepath.Join(programFilesX86, "Microsoft", "Edge", "Application", "msedge.exe")},
		{"Brave", filepath.Join(localAppData, "BraveSoftware", "Brave-Browser", "Application", "brave.exe")},
		{"Vivaldi", filepath.Join(localAppData, "Vivaldi", "Application", "vivaldi.exe")},
		{"Opera", filepath.Join(localAppData, "Programs", "Opera", "opera.exe")},
	}
	
	for _, browser := range browserPaths {
		if _, err := os.Stat(browser.path); err == nil {
			version := getBrowserVersion(browser.path)
			candidates = append(candidates, BrowserCandidate{
				Name:    browser.name,
				Path:    browser.path,
				Version: version,
			})
			if verbose {
				log.Printf("Found browser: %s at %s (version: %s)", browser.name, browser.path, version)
			}
		}
	}
	
	return candidates, nil
}

// extractExecutablePath extracts the full executable path from a process line
func extractExecutablePath(processLine, browserName string) string {
	// This is a simplified implementation
	// In practice, you might need more sophisticated parsing
	if strings.Contains(processLine, "/Applications/") {
		start := strings.Index(processLine, "/Applications/")
		if start != -1 {
			end := strings.Index(processLine[start:], " ")
			if end == -1 {
				return processLine[start:]
			}
			return processLine[start : start+end]
		}
	}
	return ""
}

// extractFlag extracts a flag value from a command line
func extractFlag(commandLine, flag string) string {
	index := strings.Index(commandLine, flag)
	if index == -1 {
		return ""
	}
	
	start := index + len(flag)
	end := strings.Index(commandLine[start:], " ")
	if end == -1 {
		return commandLine[start:]
	}
	
	return commandLine[start : start+end]
}

// extractVersionFromPath extracts version information from a path
func extractVersionFromPath(path string) string {
	// Extract version from paths like "chrome/mac_arm-131.0.6778.204"
	parts := strings.Split(path, "/")
	for _, part := range parts {
		if strings.Contains(part, ".") && len(part) > 5 {
			// Looks like a version number
			return part
		}
	}
	return "unknown"
}

// getBrowserVersion attempts to get the version of a browser executable
func getBrowserVersion(browserPath string) string {
	cmd := exec.Command(browserPath, "--version")
	output, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	
	version := strings.TrimSpace(string(output))
	// Extract just the version number
	parts := strings.Fields(version)
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	
	return "unknown"
}

// selectBestBrowser chooses the best browser from available candidates
func selectBestBrowser(candidates []BrowserCandidate, verbose bool) *BrowserCandidate {
	if len(candidates) == 0 {
		return nil
	}
	
	// Preference order:
	// 1. Running browsers with debug ports
	// 2. Chrome Canary (latest features)
	// 3. Chrome stable
	// 4. Other Chromium-based browsers
	
	// First, prefer running browsers with debug ports
	for _, candidate := range candidates {
		if candidate.IsRunning && candidate.DebugPort > 0 {
			if verbose {
				log.Printf("Selected running browser: %s (debug port: %d)", candidate.Name, candidate.DebugPort)
			}
			return &candidate
		}
	}
	
	// Then prefer running browsers
	for _, candidate := range candidates {
		if candidate.IsRunning {
			if verbose {
				log.Printf("Selected running browser: %s", candidate.Name)
			}
			return &candidate
		}
	}
	
	// Browser preference order (non-running)
	preferenceOrder := []string{
		"Chrome Canary",
		"Chrome Beta", 
		"Chrome Dev",
		"Chrome",
		"Chromium",
		"Brave",
		"Edge",
		"Vivaldi",
		"Opera",
	}
	
	for _, preferred := range preferenceOrder {
		for _, candidate := range candidates {
			if candidate.Name == preferred {
				if verbose {
					log.Printf("Selected browser: %s at %s", candidate.Name, candidate.Path)
				}
				return &candidate
			}
		}
	}
	
	// Fallback to first available
	if verbose {
		log.Printf("Selected fallback browser: %s at %s", candidates[0].Name, candidates[0].Path)
	}
	return &candidates[0]
}

// High-level commands that use the Page API
var enhancedCommands = map[string]func(*browser.Page, []string) error{
	"wait": func(p *browser.Page, args []string) error {
		if len(args) < 1 {
			return errors.New("selector required")
		}
		return p.WaitForSelector(args[0])
	},
	"waitfor": func(p *browser.Page, args []string) error {
		if len(args) < 1 {
			return errors.New("milliseconds required")
		}
		var ms int
		fmt.Sscanf(args[0], "%d", &ms)
		time.Sleep(time.Duration(ms) * time.Millisecond)
		return nil
	},
	"text": func(p *browser.Page, args []string) error {
		if len(args) < 1 {
			return errors.New("selector required")
		}
		text, err := p.GetText(args[0])
		if err != nil {
			return err
		}
		fmt.Println("Text:", text)
		return nil
	},
	"hover": func(p *browser.Page, args []string) error {
		if len(args) < 1 {
			return errors.New("selector required")
		}
		return p.Hover(args[0])
	},
	"fill": func(p *browser.Page, args []string) error {
		if len(args) < 2 {
			return errors.New("selector and text required")
		}
		return p.Type(args[0], args[1])
	},
	"clear": func(p *browser.Page, args []string) error {
		if len(args) < 1 {
			return errors.New("selector required")
		}
		el, err := p.QuerySelector(args[0])
		if err != nil {
			return err
		}
		if el == nil {
			return errors.New("element not found")
		}
		return el.Clear()
	},
	"press": func(p *browser.Page, args []string) error {
		if len(args) < 1 {
			return errors.New("key required")
		}
		return p.Press(args[0])
	},
	"select": func(p *browser.Page, args []string) error {
		if len(args) < 2 {
			return errors.New("selector and value required")
		}
		return p.SelectOption(args[0], args[1])
	},
	"visible": func(p *browser.Page, args []string) error {
		if len(args) < 1 {
			return errors.New("selector required")
		}
		visible, err := p.ElementVisible(args[0])
		if err != nil {
			return err
		}
		fmt.Printf("Visible: %v\n", visible)
		return nil
	},
	"count": func(p *browser.Page, args []string) error {
		if len(args) < 1 {
			return errors.New("selector required")
		}
		elements, err := p.QuerySelectorAll(args[0])
		if err != nil {
			return err
		}
		fmt.Printf("Count: %d\n", len(elements))
		return nil
	},
	"attr": func(p *browser.Page, args []string) error {
		if len(args) < 2 {
			return errors.New("selector and attribute name required")
		}
		value, err := p.GetAttribute(args[0], args[1])
		if err != nil {
			return err
		}
		fmt.Printf("Attribute %s: %s\n", args[1], value)
		return nil
	},
}

func main() {
	var (
		url            string
		headless       bool
		debugPort      int
		timeout        int
		verbose        bool
		remoteHost     string
		remotePort     int
		remoteTab      string
		listTabs       bool
		listBrowsers   bool
		chromePath     string
		autoDiscover   bool
		
		// New features
		jsCode         string
		tabID          string
		harFile        string
		interactive    bool
	)
	
	flag.StringVar(&url, "url", "about:blank", "URL to navigate to on start")
	flag.BoolVar(&headless, "headless", false, "Run Chrome in headless mode")
	flag.IntVar(&debugPort, "debug-port", 0, "Connect to Chrome on specific port (0 for auto)")
	flag.IntVar(&timeout, "timeout", 60, "Timeout in seconds")
	flag.BoolVar(&verbose, "verbose", false, "Enable verbose logging")
	flag.StringVar(&remoteHost, "remote-host", "", "Connect to remote Chrome at this host")
	flag.IntVar(&remotePort, "remote-port", 9222, "Remote Chrome debugging port")
	flag.StringVar(&remoteTab, "remote-tab", "", "Connect to specific tab ID or URL")
	flag.BoolVar(&listTabs, "list-tabs", false, "List available tabs on remote Chrome")
	flag.BoolVar(&listBrowsers, "list-browsers", false, "List all discovered browsers and exit")
	flag.StringVar(&chromePath, "chrome-path", "", "Path to specific Chrome executable")
	flag.BoolVar(&autoDiscover, "auto-discover", true, "Automatically discover and prefer running browsers")
	
	// New flags
	flag.StringVar(&jsCode, "js", "", "JavaScript code to execute in console")
	flag.StringVar(&tabID, "tab", "", "Target specific tab ID")
	flag.StringVar(&harFile, "har", "", "Save HAR file to this path")
	flag.BoolVar(&interactive, "interactive", false, "Keep browser open for interaction")
	
	flag.Parse()
	
	// Handle browser discovery and listing
	if listBrowsers {
		candidates, err := discoverBrowsers(verbose)
		if err != nil {
			log.Fatalf("Failed to discover browsers: %v", err)
		}
		
		fmt.Println("Discovered browsers:")
		fmt.Println("==================")
		
		for i, candidate := range candidates {
			status := "Installed"
			if candidate.IsRunning {
				status = "Running"
				if candidate.DebugPort > 0 {
					status += fmt.Sprintf(" (debug port: %d)", candidate.DebugPort)
				}
			}
			
			fmt.Printf("[%d] %s\n", i+1, candidate.Name)
			fmt.Printf("    Path: %s\n", candidate.Path)
			fmt.Printf("    Version: %s\n", candidate.Version)
			fmt.Printf("    Status: %s\n", status)
			fmt.Println()
		}
		
		if len(candidates) > 0 {
			best := selectBestBrowser(candidates, verbose)
			fmt.Printf("Best choice: %s at %s\n", best.Name, best.Path)
			if best.IsRunning && best.DebugPort > 0 {
				fmt.Printf("Recommended: Use -remote-host=localhost -remote-port=%d\n", best.DebugPort)
			}
		} else {
			fmt.Println("No compatible browsers found.")
		}
		return
	}
	
	// Check for existing Chrome instances with debug ports first
	if remoteHost == "" {
		debugPorts := []int{9222, 9223, 9224, 9225}
		for _, port := range debugPorts {
			if checkRunningChrome(port) {
				remoteHost = "localhost"
				remotePort = port
				if verbose {
					log.Printf("Found running Chrome on port %d, connecting...", port)
				}
				break
			}
		}
	}
	
	// Auto-discover browser if not explicitly specified and no running Chrome found
	var selectedBrowser *BrowserCandidate
	if autoDiscover && chromePath == "" && remoteHost == "" {
		candidates, err := discoverBrowsers(verbose)
		if err != nil && verbose {
			log.Printf("Warning: browser discovery failed: %v", err)
		}
		
		if len(candidates) > 0 {
			selectedBrowser = selectBestBrowser(candidates, verbose)
			
			// If we found a running browser with debug port, connect to it instead
			if selectedBrowser.IsRunning && selectedBrowser.DebugPort > 0 {
				remoteHost = "localhost"
				remotePort = selectedBrowser.DebugPort
				if verbose {
					log.Printf("Auto-connecting to running browser: %s (port %d)", 
						selectedBrowser.Name, selectedBrowser.DebugPort)
				}
			} else if selectedBrowser.Path != "" {
				chromePath = selectedBrowser.Path
				if verbose {
					log.Printf("Auto-selected browser: %s at %s", 
						selectedBrowser.Name, selectedBrowser.Path)
				}
			}
		}
	}
	
	// Handle --list-tabs separately
	if listTabs {
		// Check for running Chrome instances first
		debugPorts := []int{9222, 9223, 9224, 9225}
		for _, port := range debugPorts {
			if checkRunningChrome(port) {
				tabs, err := getChromeTabs(port)
				if err != nil {
					log.Printf("Failed to get tabs on port %d: %v", port, err)
					continue
				}
				
				fmt.Printf("Available tabs on port %d:\n", port)
				for i, tab := range tabs {
					fmt.Printf("[%d] %s - %s\n", i, tab.Title, tab.URL)
					fmt.Printf("    ID: %s\n", tab.ID)
				}
				return
			}
		}
		
		// Fallback to remote host if specified
		if remoteHost != "" {
			tabs, err := browser.ListTabs(remoteHost, remotePort)
			if err != nil {
				log.Fatalf("Failed to list tabs: %v", err)
			}
			
			fmt.Printf("Available tabs on %s:%d:\n\n", remoteHost, remotePort)
			for i, tab := range tabs {
				fmt.Printf("[%d] %s\n", i, tab.Title)
				fmt.Printf("    URL: %s\n", tab.URL)
				fmt.Printf("    Type: %s\n", tab.Type)
				fmt.Printf("    ID: %s\n\n", tab.ID)
			}
			return
		}
		
		log.Fatal("No running Chrome found with debug port enabled")
	}
	
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
	
	var browserCtx context.Context
	var browserCancel context.CancelFunc
	var enhancedBrowser *browser.Browser
	var enhancedPage *browser.Page
	
	// Use enhanced browser API when connecting to remote Chrome
	if remoteHost != "" {
		// Handle direct tab connection for specific operations
		if jsCode != "" || tabID != "" || harFile != "" {
			// Get available tabs
			tabs, err := getChromeTabs(remotePort)
			if err != nil {
				log.Fatalf("Failed to get tabs: %v", err)
			}
			
			// Find target tab
			var targetTabID string
			if tabID != "" {
				targetTabID = tabID
			}
			
			// Connect to specific tab
			var remoteURL string
			if targetTabID != "" {
				remoteURL = fmt.Sprintf("ws://localhost:%d/devtools/page/%s", remotePort, targetTabID)
			} else {
				remoteURL = fmt.Sprintf("ws://localhost:%d", remotePort)
			}
			
			allocCtx, allocCancel := chromedp.NewRemoteAllocator(ctx, remoteURL)
			defer allocCancel()
			
			if verbose {
				browserCtx, browserCancel = chromedp.NewContext(allocCtx, chromedp.WithLogf(log.Printf))
			} else {
				browserCtx, browserCancel = chromedp.NewContext(allocCtx)
			}
			
			// Set up HAR recording if requested
			var recorder *NetworkRecorder
			if harFile != "" {
				recorder = &NetworkRecorder{}
				
				// Enable network monitoring
				if err := chromedp.Run(browserCtx, network.Enable()); err != nil {
					log.Fatalf("Failed to enable network monitoring: %v", err)
				}
				
				// Set up network event listeners
				chromedp.ListenTarget(browserCtx, func(ev interface{}) {
					switch ev := ev.(type) {
					case *network.EventResponseReceived:
						if verbose {
							log.Printf("Response received: %s", ev.Response.URL)
						}
						
						// Create basic HAR entry
						entry := HAREntry{
							StartedDateTime: time.Now().Format(time.RFC3339),
							Request: map[string]interface{}{
								"method": "GET", // Simplified
								"url":    ev.Response.URL,
								"headers": []interface{}{},
							},
							Response: map[string]interface{}{
								"status": ev.Response.Status,
								"statusText": ev.Response.StatusText,
								"headers": []interface{}{},
								"content": map[string]interface{}{
									"size": 0,
									"mimeType": ev.Response.MimeType,
								},
							},
							Time: 0, // Simplified
						}
						
						recorder.AddEntry(entry)
					}
				})
				
				fmt.Printf("Recording network traffic to: %s\n", harFile)
			}
			
			// Execute JavaScript if provided
			if jsCode != "" {
				var result *runtime.RemoteObject
				if err := chromedp.Run(browserCtx,
					chromedp.Evaluate(jsCode, &result),
				); err != nil {
					log.Fatalf("Failed to execute JavaScript: %v", err)
				}
				
				fmt.Printf("✓ Executed JavaScript in Chrome on port %d\n", remotePort)
				if targetTabID != "" {
					fmt.Printf("Target tab ID: %s\n", targetTabID)
				}
				fmt.Printf("Code: %s\n", jsCode)
				
				if result != nil && result.Value != nil {
					fmt.Printf("Result: %s\n", string(result.Value))
				}
				
				// Save HAR file if recording and exit
				if recorder != nil {
					if err := recorder.SaveHAR(harFile); err != nil {
						log.Printf("Failed to save HAR file: %v", err)
					} else {
						fmt.Printf("HAR file saved to: %s\n", harFile)
						fmt.Printf("Recorded %d network requests\n", len(recorder.GetEntries()))
					}
				}
				
				return
			}
			
			// Navigate to URL if specified
			if url != "about:blank" {
				if err := chromedp.Run(browserCtx, chromedp.Navigate(url)); err != nil {
					if verbose {
						log.Printf("Failed to navigate to %s: %v", url, err)
					}
				}
			}
			
			// If HAR capture without JS, wait for user interaction
			if harFile != "" {
				fmt.Printf("Connected to Chrome on port %d\n", remotePort)
				if targetTabID != "" {
					fmt.Printf("Target tab ID: %s\n", targetTabID)
				}
				fmt.Println("Press Ctrl+C to stop recording and save HAR file...")
				
				// Wait for signal or timeout
				select {
				case <-ctx.Done():
					if verbose {
						log.Println("Context cancelled...")
					}
				case <-sigChan:
					if verbose {
						log.Println("Signal received...")
					}
				}
				
				// Save HAR file
				if err := recorder.SaveHAR(harFile); err != nil {
					log.Printf("Failed to save HAR file: %v", err)
				} else {
					fmt.Printf("HAR file saved to: %s\n", harFile)
					fmt.Printf("Recorded %d network requests\n", len(recorder.GetEntries()))
				}
				
				return
			}
		} else {
			// Use enhanced browser API for interactive mode
			// Create profile manager
			pm, err := chromeprofiles.NewProfileManager(
				chromeprofiles.WithVerbose(verbose),
			)
			if err != nil {
				log.Fatal(err)
			}
			
			// Set up browser options
			browserOpts := []browser.Option{
				browser.WithHeadless(headless),
				browser.WithVerbose(verbose),
				browser.WithTimeout(timeout),
			}
			
			if remoteHost != "" {
				browserOpts = append(browserOpts, browser.WithRemoteChrome(remoteHost, remotePort))
				if remoteTab != "" {
					browserOpts = append(browserOpts, browser.WithRemoteTab(remoteTab))
				}
			}
			
			if debugPort > 0 {
				browserOpts = append(browserOpts, browser.WithDebugPort(debugPort))
			}
			
			// Create browser
			enhancedBrowser, err = browser.New(ctx, pm, browserOpts...)
			if err != nil {
				log.Fatalf("Failed to create browser: %v", err)
			}
			defer enhancedBrowser.Close()
			
			// Launch browser
			if err := enhancedBrowser.Launch(ctx); err != nil {
				log.Fatalf("Failed to launch browser: %v", err)
			}
			
			// Get or create page
			if remoteTab != "" {
				// When we connect to a specific remote tab, the browser context
				// is already connected to that tab. Get a page wrapper for it.
				enhancedPage = enhancedBrowser.GetCurrentPage()
			} else {
				pages, err := enhancedBrowser.Pages()
				if err != nil || len(pages) == 0 {
					enhancedPage, err = enhancedBrowser.NewPage()
					if err != nil {
						log.Fatalf("Failed to create page: %v", err)
					}
				} else {
					enhancedPage = pages[0]
				}
			}
			
			// Navigate to initial URL
			if url != "about:blank" && enhancedPage != nil {
				if err := enhancedPage.Navigate(url); err != nil {
					log.Printf("Warning: Failed to navigate to %s: %v", url, err)
				}
			}
			
			browserCtx = enhancedPage.Context()
			browserCancel = func() {} // browser.Close() will handle cleanup
			
			if remoteHost != "" {
				fmt.Printf("Connected to remote Chrome at %s:%d\n", remoteHost, remotePort)
				if remoteTab != "" {
					fmt.Printf("Connected to tab: %s\n", remoteTab)
				}
			}
			
			if verbose {
				fmt.Println("Using enhanced browser API for remote Chrome connection")
			}
		}
	} else {
		// Local Chrome instance
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
		
		if debugPort > 0 {
			opts = append(opts, chromedp.Flag("remote-debugging-port", fmt.Sprintf("%d", debugPort)))
		}
		
		// Add Chrome path if specified or discovered
		if chromePath != "" {
			opts = append(opts, chromedp.ExecPath(chromePath))
			if verbose {
				log.Printf("Using Chrome at: %s", chromePath)
			}
		}
		
		// Create Chrome allocator
		allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, opts...)
		defer allocCancel()
		
		// Create Chrome browser context
		if verbose {
			browserCtx, browserCancel = chromedp.NewContext(allocCtx, chromedp.WithLogf(log.Printf))
		} else {
			browserCtx, browserCancel = chromedp.NewContext(allocCtx)
		}
		
		// Execute JavaScript if provided (for new Chrome instances)
		if jsCode != "" {
			var result *runtime.RemoteObject
			if err := chromedp.Run(browserCtx,
				chromedp.Navigate(url),
				chromedp.Evaluate(jsCode, &result),
			); err != nil {
				log.Fatalf("Failed to execute JavaScript: %v", err)
			}
			
			fmt.Printf("✓ Executed JavaScript in new Chrome instance\n")
			fmt.Printf("Code: %s\n", jsCode)
			
			if result != nil && result.Value != nil {
				fmt.Printf("Result: %s\n", string(result.Value))
			}
			
			return
		}
		
		// Start and connect to browser
		if err := chromedp.Run(browserCtx, chromedp.Navigate(url)); err != nil {
			log.Fatalf("Error launching Chrome: %v", err)
		}
	}
	defer browserCancel()
	
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
		
		// Execute command with enhanced page if available
		if enhancedPage != nil && strings.HasPrefix(cmdToRun, "@") {
			// Execute enhanced command
			if err := executeEnhancedCommand(enhancedPage, strings.TrimPrefix(cmdToRun, "@")); err != nil {
				fmt.Printf("Error: %v\n", err)
			}
		} else {
			// Execute standard CDP command
			if err := executeCommand(browserCtx, cmdToRun); err != nil {
				fmt.Printf("Error: %v\n", err)
			}
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
	fmt.Println("  help enhanced     - List enhanced commands (remote Chrome only)")
	fmt.Println("  exit / quit       - Exit the program")
}

func executeEnhancedCommand(page *browser.Page, command string) error {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return errors.New("empty command")
	}

	cmdName := parts[0]
	args := parts[1:]

	if handler, ok := enhancedCommands[cmdName]; ok {
		return handler(page, args)
	}

	return fmt.Errorf("unknown enhanced command: %s", cmdName)
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

func printEnhancedCommands() {
	fmt.Println("\nEnhanced Commands (prefixed with @):")
	fmt.Println("\nWaiting:")
	fmt.Println("  @wait <selector>      - Wait for element to appear")
	fmt.Println("  @waitfor <ms>         - Wait for milliseconds")
	
	fmt.Println("\nElement Interaction:")
	fmt.Println("  @text <selector>      - Get element text")
	fmt.Println("  @hover <selector>     - Hover over element")
	fmt.Println("  @fill <sel> <text>    - Fill input field")
	fmt.Println("  @clear <selector>     - Clear input field")
	fmt.Println("  @press <key>          - Press keyboard key")
	fmt.Println("  @select <sel> <val>   - Select dropdown option")
	
	fmt.Println("\nElement State:")
	fmt.Println("  @visible <selector>   - Check if element is visible")
	fmt.Println("  @count <selector>     - Count matching elements")
	fmt.Println("  @attr <sel> <name>    - Get attribute value")
	
	fmt.Println("\nNetwork:")
	fmt.Println("  @route <pattern> <action>  - Intercept requests (abort/log)")
	
	fmt.Println("\nNote: These commands are only available when connected to remote Chrome")
}