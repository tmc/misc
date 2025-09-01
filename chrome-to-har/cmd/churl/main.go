// Command churl is like curl but runs through Chrome and can handle JavaScript/SPAs.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/pkg/errors"
	"github.com/tmc/misc/chrome-to-har/internal/browser"
	"github.com/tmc/misc/chrome-to-har/internal/chromeprofiles"
	chromeErrors "github.com/tmc/misc/chrome-to-har/internal/errors"
	"github.com/tmc/misc/chrome-to-har/internal/recorder"
)

type options struct {
	// Output options
	outputFile   string
	outputFormat string // html, har, text, json

	// Chrome options
	profileDir string
	headless   bool
	debugPort  int
	timeout    int
	chromePath string
	verbose    bool

	// Remote Chrome options
	remoteHost string
	remotePort int
	remoteTab  string
	listTabs   bool

	// Wait options
	waitNetworkIdle bool
	waitSelector    string
	stableTimeout   int

	// Request options
	headers        headerSlice
	method         string
	data           string
	followRedirect bool

	// Authentication
	username string
	password string

	// Proxy options
	proxy         string // HTTP/HTTPS proxy server
	socks5Proxy   string // SOCKS5 proxy server
	proxyUser     string // Proxy authentication (user:password)
	proxyBypass   string // Comma-separated list of hosts to bypass proxy
	proxyUsername string // Parsed proxy username
	proxyPassword string // Parsed proxy password

	// Script injection
	scriptBefore     stringSlice
	scriptAfter      stringSlice
	scriptFileBefore stringSlice
	scriptFileAfter  stringSlice

	// Blocking options
	blockingEnabled     bool        // Enable URL/domain blocking
	blockingVerbose     bool        // Enable verbose blocking logging
	blockURLPatterns    stringSlice // URL patterns to block
	blockDomains        stringSlice // Domains to block
	blockRegexPatterns  stringSlice // Regex patterns to block
	allowURLs           stringSlice // URLs to allow (whitelist)
	allowDomains        stringSlice // Domains to allow (whitelist)
	blockingRuleFile    string      // File containing blocking rules
	blockCommonAds      bool        // Block common ad domains
	blockCommonTracking bool        // Block common tracking domains

	// WebSocket options
	webSocketEnabled     bool        // Enable WebSocket monitoring
	webSocketMessages    stringSlice // Messages to send to WebSocket
	webSocketWaitFor     string      // Wait for specific WebSocket condition
	webSocketTimeout     int         // WebSocket wait timeout in seconds
	webSocketURLPattern  string      // WebSocket URL pattern to monitor
	webSocketDataPattern string      // WebSocket data pattern to match
	webSocketDirection   string      // WebSocket direction filter (sent/received)
	webSocketOutputFile  string      // Output file for WebSocket data
	webSocketStats       bool        // Show WebSocket statistics
}

// headerSlice allows multiple -H flags
type headerSlice []string

func (h *headerSlice) String() string {
	return strings.Join(*h, ", ")
}

func (h *headerSlice) Set(value string) error {
	*h = append(*h, value)
	return nil
}

// stringSlice allows multiple string flags
type stringSlice []string

func (s *stringSlice) String() string {
	return strings.Join(*s, ", ")
}

func (s *stringSlice) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func main() {
	opts := options{}

	// Output options
	flag.StringVar(&opts.outputFile, "o", "", "Output file (default: stdout)")
	flag.StringVar(&opts.outputFormat, "output-format", "html", "Output format: html, har, text, json")

	// Chrome options
	flag.StringVar(&opts.profileDir, "profile", "", "Chrome profile directory to use")
	flag.BoolVar(&opts.headless, "headless", true, "Run Chrome in headless mode")
	flag.IntVar(&opts.debugPort, "debug-port", 0, "Use specific port for Chrome DevTools (0 for auto)")
	flag.IntVar(&opts.timeout, "timeout", 180, "Global timeout in seconds")
	flag.StringVar(&opts.chromePath, "chrome-path", "", "Path to Chrome executable")
	flag.BoolVar(&opts.verbose, "verbose", false, "Enable verbose logging")

	// Remote Chrome options
	flag.StringVar(&opts.remoteHost, "remote-host", "", "Connect to remote Chrome at this host")
	flag.IntVar(&opts.remotePort, "remote-port", 9222, "Remote Chrome debugging port")
	flag.StringVar(&opts.remoteTab, "remote-tab", "", "Connect to specific tab ID or URL")
	flag.BoolVar(&opts.listTabs, "list-tabs", false, "List available tabs on remote Chrome")

	// Wait options
	flag.BoolVar(&opts.waitNetworkIdle, "wait-network-idle", true, "Wait until network activity becomes idle")
	flag.StringVar(&opts.waitSelector, "wait-for", "", "Wait for specific CSS selector to appear")
	flag.IntVar(&opts.stableTimeout, "stable-timeout", 30, "Max time in seconds to wait for stability")

	// Request options
	flag.Var(&opts.headers, "H", "Add request header (can be used multiple times)")
	flag.StringVar(&opts.method, "X", "GET", "HTTP method to use")
	flag.StringVar(&opts.data, "d", "", "Data to send (for POST/PUT)")
	flag.BoolVar(&opts.followRedirect, "L", true, "Follow redirects")

	// Authentication
	flag.StringVar(&opts.username, "u", "", "Username for basic auth (user:password)")

	// Proxy options
	flag.StringVar(&opts.proxy, "proxy", "", "HTTP/HTTPS proxy server (e.g., http://proxy.example.com:8080)")
	flag.StringVar(&opts.socks5Proxy, "socks5-proxy", "", "SOCKS5 proxy server (e.g., socks5://proxy.example.com:1080)")
	flag.StringVar(&opts.proxyUser, "proxy-user", "", "Proxy authentication (user:password)")
	flag.StringVar(&opts.proxyBypass, "proxy-bypass", "", "Comma-separated list of hosts to bypass proxy")

	// Script injection
	flag.Var(&opts.scriptBefore, "script-before", "JavaScript to execute before page load (can be used multiple times)")
	flag.Var(&opts.scriptAfter, "script-after", "JavaScript to execute after page load (can be used multiple times)")
	flag.Var(&opts.scriptFileBefore, "script-file-before", "JavaScript file to execute before page load (can be used multiple times)")
	flag.Var(&opts.scriptFileAfter, "script-file-after", "JavaScript file to execute after page load (can be used multiple times)")

	// Blocking options
	flag.BoolVar(&opts.blockingEnabled, "block-enabled", false, "Enable URL/domain blocking")
	flag.BoolVar(&opts.blockingVerbose, "block-verbose", false, "Enable verbose blocking logging")
	flag.Var(&opts.blockURLPatterns, "block-url", "URL pattern to block (can be used multiple times)")
	flag.Var(&opts.blockDomains, "block-domain", "Domain to block (can be used multiple times)")
	flag.Var(&opts.blockRegexPatterns, "block-regex", "Regex pattern to block (can be used multiple times)")
	flag.Var(&opts.allowURLs, "allow-url", "URL to allow/whitelist (can be used multiple times)")
	flag.Var(&opts.allowDomains, "allow-domain", "Domain to allow/whitelist (can be used multiple times)")
	flag.StringVar(&opts.blockingRuleFile, "block-file", "", "File containing blocking rules")
	flag.BoolVar(&opts.blockCommonAds, "block-ads", false, "Block common ad domains")
	flag.BoolVar(&opts.blockCommonTracking, "block-tracking", false, "Block common tracking domains")

	// WebSocket options
	flag.BoolVar(&opts.webSocketEnabled, "ws-enabled", false, "Enable WebSocket monitoring")
	flag.Var(&opts.webSocketMessages, "ws-send", "Message to send to WebSocket (can be used multiple times)")
	flag.StringVar(&opts.webSocketWaitFor, "ws-wait-for", "", "Wait for WebSocket condition (open, closed, message, first_message, etc.)")
	flag.IntVar(&opts.webSocketTimeout, "ws-timeout", 30, "WebSocket wait timeout in seconds")
	flag.StringVar(&opts.webSocketURLPattern, "ws-url-pattern", "*", "WebSocket URL pattern to monitor")
	flag.StringVar(&opts.webSocketDataPattern, "ws-data-pattern", "", "WebSocket data pattern to match")
	flag.StringVar(&opts.webSocketDirection, "ws-direction", "", "WebSocket direction filter (sent/received)")
	flag.StringVar(&opts.webSocketOutputFile, "ws-output", "", "Output file for WebSocket data")
	flag.BoolVar(&opts.webSocketStats, "ws-stats", false, "Show WebSocket statistics")

	// Custom usage message
	flag.Usage = func() {
		w := tabwriter.NewWriter(os.Stderr, 0, 0, 2, ' ', 0)
		defer w.Flush()

		fmt.Fprintf(w, "churl - Chrome-powered curl for JavaScript/SPA support\n\n")
		fmt.Fprintf(w, "Usage:\n")
		fmt.Fprintf(w, "  churl [options] URL\n\n")
		fmt.Fprintf(w, "Options:\n")

		flag.VisitAll(func(f *flag.Flag) {
			def := f.DefValue
			if def != "" {
				def = fmt.Sprintf(" (default: %s)", def)
			}

			typ := ""
			switch f.Value.String() {
			case "false", "true":
				typ = "bool"
			case "0":
				typ = "int"
			case "[]":
				typ = "list"
			default:
				typ = "string"
			}

			fmt.Fprintf(w, "  -%s\t%s\t%s%s\n", f.Name, typ, f.Usage, def)
		})
	}

	flag.Parse()

	// Handle --list-tabs separately
	if opts.listTabs && opts.remoteHost != "" {
		tabs, err := browser.ListTabs(opts.remoteHost, opts.remotePort)
		if err != nil {
			log.Fatalf("Failed to list tabs: %v", err)
		}

		fmt.Printf("Available tabs on %s:%d:\n\n", opts.remoteHost, opts.remotePort)
		for i, tab := range tabs {
			fmt.Printf("[%d] %s\n", i, tab.Title)
			fmt.Printf("    URL: %s\n", tab.URL)
			fmt.Printf("    Type: %s\n", tab.Type)
			fmt.Printf("    ID: %s\n\n", tab.ID)
		}
		return
	}

	// Check for URL argument
	if flag.NArg() != 1 {
		fmt.Println("Error: URL is required")
		flag.Usage()
		os.Exit(1)
	}

	url := flag.Arg(0)

	// Parse basic auth from username flag (user:password format)
	if opts.username != "" && strings.Contains(opts.username, ":") {
		parts := strings.SplitN(opts.username, ":", 2)
		opts.username = parts[0]
		if len(parts) > 1 {
			opts.password = parts[1]
		}
	}

	// Validate proxy options
	if opts.proxy != "" && opts.socks5Proxy != "" {
		fmt.Println("Error: Cannot specify both --proxy and --socks5-proxy")
		flag.Usage()
		os.Exit(1)
	}

	// Parse proxy authentication if provided
	var proxyUsername, proxyPassword string
	if opts.proxyUser != "" {
		parts := strings.SplitN(opts.proxyUser, ":", 2)
		proxyUsername = parts[0]
		if len(parts) > 1 {
			proxyPassword = parts[1]
		} else {
			fmt.Println("Error: --proxy-user must be in format user:password")
			flag.Usage()
			os.Exit(1)
		}
	}

	// Create a context with the user-specified timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(opts.timeout)*time.Second)
	defer cancel()

	// Handle interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		if opts.verbose {
			log.Println("Interrupt received, shutting down...")
		}
		cancel()
	}()

	// Auto-detect Chrome path if not specified
	if opts.chromePath == "" {
		if chromePath, detected := detectChromePath(); detected {
			if opts.verbose {
				log.Printf("Auto-detected Chrome path: %s", chromePath)
			}
			opts.chromePath = chromePath
		}
	}

	// Create profile manager
	pm, err := chromeprofiles.NewProfileManager(
		chromeprofiles.WithVerbose(opts.verbose),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Store parsed proxy credentials in opts
	opts.proxyUsername = proxyUsername
	opts.proxyPassword = proxyPassword

	// Run the churl command
	if err := run(ctx, pm, url, opts); err != nil {
		if err == context.DeadlineExceeded {
			log.Fatal("Operation timed out. Try increasing the timeout value with --timeout flag.")
		} else if err == context.Canceled {
			log.Fatal("Operation was canceled. This might indicate an internal timeout issue.")
		} else if strings.Contains(err.Error(), "context canceled") {
			log.Fatal("Operation was canceled due to context timeout. Try increasing the timeout value with --timeout flag.")
		} else if chromeErr, ok := err.(*chromeErrors.ChromeError); ok {
			// Print user-friendly error message
			fmt.Fprintf(os.Stderr, "Error: %s\n", chromeErr.UserMessage())
			if suggestions := chromeErr.Suggestions(); len(suggestions) > 0 {
				fmt.Fprintf(os.Stderr, "\nSuggestions:\n")
				for _, suggestion := range suggestions {
					fmt.Fprintf(os.Stderr, "  - %s\n", suggestion)
				}
			}
			if opts.verbose {
				fmt.Fprintf(os.Stderr, "\nDetailed error: %s\n", chromeErrors.FormatError(err))
			}
			os.Exit(1)
		} else {
			log.Fatal(err)
		}
	}
}

func run(ctx context.Context, pm chromeprofiles.ProfileManager, url string, opts options) error {
	if opts.verbose {
		log.Printf("Starting run function with URL: %s", url)
	}

	// Load script files and combine with inline scripts
	scriptsBefore, err := loadScripts(opts.scriptBefore, opts.scriptFileBefore, opts.verbose)
	if err != nil {
		return chromeErrors.Wrap(err, chromeErrors.InvalidScriptError, "failed to load before scripts")
	}

	scriptsAfter, err := loadScripts(opts.scriptAfter, opts.scriptFileAfter, opts.verbose)
	if err != nil {
		return chromeErrors.Wrap(err, chromeErrors.InvalidScriptError, "failed to load after scripts")
	}

	if opts.verbose && (len(scriptsBefore) > 0 || len(scriptsAfter) > 0) {
		log.Printf("Loaded %d before scripts and %d after scripts", len(scriptsBefore), len(scriptsAfter))
	}

	// Parse request headers
	headers := make(map[string]string)
	for _, h := range opts.headers {
		parts := strings.SplitN(h, ":", 2)
		if len(parts) != 2 {
			return chromeErrors.WithContext(
				chromeErrors.New(chromeErrors.InvalidHeaderError, "invalid header format (expected 'name: value')"),
				"header", h,
			)
		}
		name := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		headers[name] = value
	}

	// Verify profile if provided
	if opts.profileDir != "" {
		profiles, err := pm.ListProfiles()
		if err != nil {
			return chromeErrors.Wrap(err, chromeErrors.ProfileSetupError, "failed to list Chrome profiles")
		}

		profileExists := false
		for _, p := range profiles {
			if p == opts.profileDir {
				profileExists = true
				break
			}
		}

		if !profileExists {
			return chromeErrors.WithContext(
				chromeErrors.New(chromeErrors.ProfileNotFoundError, "specified Chrome profile does not exist"),
				"profile", opts.profileDir,
			)
		}
	}

	// Set up browser options
	browserOpts := []browser.Option{
		browser.WithHeadless(opts.headless),
		browser.WithTimeout(opts.timeout),
		browser.WithVerbose(opts.verbose),
		browser.WithWaitNetworkIdle(opts.waitNetworkIdle),
		browser.WithStableTimeout(opts.stableTimeout),
	}

	if opts.chromePath != "" {
		browserOpts = append(browserOpts, browser.WithChromePath(opts.chromePath))
	}

	if opts.debugPort > 0 {
		browserOpts = append(browserOpts, browser.WithDebugPort(opts.debugPort))
	}

	if opts.waitSelector != "" {
		browserOpts = append(browserOpts, browser.WithWaitSelector(opts.waitSelector))
	}

	if opts.profileDir != "" {
		browserOpts = append(browserOpts, browser.WithProfile(opts.profileDir))
	}

	// Add script injection options
	if len(scriptsBefore) > 0 {
		browserOpts = append(browserOpts, browser.WithScriptsBefore(scriptsBefore))
	}
	if len(scriptsAfter) > 0 {
		browserOpts = append(browserOpts, browser.WithScriptsAfter(scriptsAfter))
	}

	// Add remote Chrome options if specified
	if opts.remoteHost != "" {
		browserOpts = append(browserOpts, browser.WithRemoteChrome(opts.remoteHost, opts.remotePort))
		if opts.remoteTab != "" {
			browserOpts = append(browserOpts, browser.WithRemoteTab(opts.remoteTab))
		}

		if opts.verbose {
			log.Printf("Connecting to remote Chrome at %s:%d", opts.remoteHost, opts.remotePort)
			if opts.remoteTab != "" {
				log.Printf("Connecting to tab: %s", opts.remoteTab)
			}
		}
	}

	// Add proxy options if specified
	if opts.proxy != "" || opts.socks5Proxy != "" {
		proxyServer := opts.proxy
		if opts.socks5Proxy != "" {
			proxyServer = opts.socks5Proxy
		}

		browserOpts = append(browserOpts, browser.WithProxy(proxyServer))

		if opts.proxyBypass != "" {
			browserOpts = append(browserOpts, browser.WithProxyBypassList(opts.proxyBypass))
		}

		if opts.proxyUsername != "" && opts.proxyPassword != "" {
			browserOpts = append(browserOpts, browser.WithProxyAuth(opts.proxyUsername, opts.proxyPassword))
		}

		if opts.verbose {
			log.Printf("Using proxy server: %s", proxyServer)
			if opts.proxyBypass != "" {
				log.Printf("Proxy bypass list: %s", opts.proxyBypass)
			}
			if opts.proxyUsername != "" {
				log.Printf("Proxy authentication enabled for user: %s", opts.proxyUsername)
			}
		}
	}

	// Add blocking options if enabled
	if opts.blockingEnabled {
		browserOpts = append(browserOpts, browser.WithBlocking(true))
		
		if opts.blockingVerbose {
			browserOpts = append(browserOpts, browser.WithBlockingVerbose(true))
		}

		// Add URL patterns to block
		for _, pattern := range opts.blockURLPatterns {
			browserOpts = append(browserOpts, browser.WithBlockedURLPattern(pattern))
		}

		// Add domains to block
		for _, domain := range opts.blockDomains {
			browserOpts = append(browserOpts, browser.WithBlockedDomain(domain))
		}

		// Add regex patterns to block
		for _, pattern := range opts.blockRegexPatterns {
			browserOpts = append(browserOpts, browser.WithBlockedRegexPattern(pattern))
		}

		// Add URLs to allow
		for _, url := range opts.allowURLs {
			browserOpts = append(browserOpts, browser.WithAllowedURL(url))
		}

		// Add domains to allow
		for _, domain := range opts.allowDomains {
			browserOpts = append(browserOpts, browser.WithAllowedDomain(domain))
		}

		// Add blocking rule file
		if opts.blockingRuleFile != "" {
			browserOpts = append(browserOpts, browser.WithBlockingRuleFile(opts.blockingRuleFile))
		}

		// Add common blocking rules
		if opts.blockCommonAds {
			browserOpts = append(browserOpts, browser.WithBlockCommonAds(true))
		}

		if opts.blockCommonTracking {
			browserOpts = append(browserOpts, browser.WithBlockCommonTracking(true))
		}

		if opts.verbose {
			log.Printf("URL/domain blocking enabled")
		}
	}

	// Create and launch browser with a separate context to avoid timeout during launch
	// The browser itself should not be subject to the operation timeout
	launchCtx := context.Background()
	b, err := browser.New(launchCtx, pm, browserOpts...)
	if err != nil {
		return chromeErrors.Wrap(err, chromeErrors.ChromeLaunchError, "failed to create browser instance")
	}

	if err := b.Launch(launchCtx); err != nil {
		return chromeErrors.Wrap(err, chromeErrors.ChromeLaunchError, "failed to launch browser")
	}
	defer b.Close()

	// Set up request headers
	if len(headers) > 0 {
		if err := b.SetRequestHeaders(headers); err != nil {
			return chromeErrors.Wrap(err, chromeErrors.NetworkError, "failed to set request headers")
		}
	}

	// Set up basic auth if provided
	if opts.username != "" && opts.password != "" {
		if err := b.SetBasicAuth(opts.username, opts.password); err != nil {
			return chromeErrors.Wrap(err, chromeErrors.AuthenticationError, "failed to set basic authentication")
		}
	}

	// Create recorder for HAR output if needed
	var rec *recorder.Recorder
	if opts.outputFormat == "har" {
		var recOpts []recorder.Option
		if opts.verbose {
			recOpts = append(recOpts, recorder.WithVerbose(true))
		}

		rec, err = recorder.New(recOpts...)
		if err != nil {
			return chromeErrors.Wrap(err, chromeErrors.NetworkRecordError, "failed to create network recorder")
		}

		// Enable network monitoring with proper timeout handling
		if opts.verbose {
			log.Printf("Enabling network monitoring for HAR output...")
		}

		// Check if browser context is working
		select {
		case <-b.Context().Done():
			return errors.Wrap(b.Context().Err(), "browser context is done before enabling network monitoring")
		default:
			// Context is active
		}

		enableCtx, enableCancel := context.WithTimeout(b.Context(), 10*time.Second)
		defer enableCancel()

		if err := chromedp.Run(enableCtx, network.Enable()); err != nil {
			return chromeErrors.Wrap(err, chromeErrors.NetworkError, "failed to enable network monitoring")
		}

		// Set up event listener for network events
		chromedp.ListenTarget(b.Context(), rec.HandleNetworkEvent(b.Context()))
	}

	// Navigate to the URL or make custom HTTP request
	if opts.method != "GET" || opts.data != "" {
		// Use HTTPRequest for custom methods or when data is provided
		if err := b.HTTPRequest(opts.method, url, opts.data, headers); err != nil {
			return chromeErrors.WithContext(
				chromeErrors.Wrap(err, chromeErrors.NetworkError, "failed to make HTTP request"),
				"method", opts.method,
			)
		}
	} else {
		// Use regular navigation for GET requests without data
		if err := b.Navigate(url); err != nil {
			return chromeErrors.WithContext(
				chromeErrors.Wrap(err, chromeErrors.ChromeNavigationError, "failed to navigate to URL"),
				"url", url,
			)
		}
	}

	// Handle WebSocket functionality if enabled
	if opts.webSocketEnabled {
		if err := handleWebSocketOperations(ctx, b, opts); err != nil {
			return chromeErrors.Wrap(err, chromeErrors.NetworkError, "WebSocket operations failed")
		}
	}

	// Get the output based on the requested format
	var output []byte
	var outputErr error

	switch opts.outputFormat {
	case "html":
		var html string
		html, outputErr = b.GetHTML()
		output = []byte(html)

	case "har":
		if rec == nil {
			return chromeErrors.New(chromeErrors.InternalError, "recorder not initialized for HAR output")
		}

		// Write HAR to a temporary file and read it back
		tmpFile, err := os.CreateTemp("", "churl-*.har")
		if err != nil {
			return chromeErrors.FileError("create", "temp file", err)
		}
		defer os.Remove(tmpFile.Name())

		if err := rec.WriteHAR(tmpFile.Name()); err != nil {
			return chromeErrors.WithContext(
				chromeErrors.FileError("write", tmpFile.Name(), err),
				"format", "har",
			)
		}

		output, outputErr = os.ReadFile(tmpFile.Name())

	case "text":
		var html string
		html, outputErr = b.GetHTML()
		if outputErr == nil {
			// This is a very simple text extraction. A real implementation would
			// use a proper HTML to text converter.
			text := strings.ReplaceAll(html, "\n", " ")
			text = strings.ReplaceAll(text, "<script", "\n<script")
			text = strings.ReplaceAll(text, "</script>", "</script>\n")
			text = strings.ReplaceAll(text, "<style", "\n<style")
			text = strings.ReplaceAll(text, "</style>", "</style>\n")
			text = strings.ReplaceAll(text, "<", "\n<")

			// Extract text nodes
			var sb strings.Builder
			for _, line := range strings.Split(text, "\n") {
				if !strings.HasPrefix(line, "<") {
					content := strings.TrimSpace(line)
					if content != "" {
						sb.WriteString(content)
						sb.WriteString("\n")
					}
				}
			}

			output = []byte(sb.String())
		}

	case "json":
		type PageInfo struct {
			URL     string `json:"url"`
			Title   string `json:"title"`
			Content string `json:"content"`
		}

		info := PageInfo{}

		info.URL, outputErr = b.GetURL()
		if outputErr != nil {
			return chromeErrors.Wrap(outputErr, chromeErrors.ChromeScriptError, "failed to get URL")
		}

		info.Title, outputErr = b.GetTitle()
		if outputErr != nil {
			return chromeErrors.Wrap(outputErr, chromeErrors.ChromeScriptError, "failed to get title")
		}

		info.Content, outputErr = b.GetHTML()
		if outputErr != nil {
			return chromeErrors.Wrap(outputErr, chromeErrors.ChromeScriptError, "failed to get HTML")
		}

		output, outputErr = json.MarshalIndent(info, "", "  ")

	default:
		return chromeErrors.WithContext(
			chromeErrors.New(chromeErrors.ValidationError, "unsupported output format"),
			"format", opts.outputFormat,
		)
	}

	if outputErr != nil {
		return chromeErrors.Wrap(outputErr, chromeErrors.InternalError, "failed to get output")
	}

	// Write the output
	var outWriter io.Writer = os.Stdout
	if opts.outputFile != "" {
		file, err := os.Create(opts.outputFile)
		if err != nil {
			return chromeErrors.FileError("create", opts.outputFile, err)
		}
		defer file.Close()
		outWriter = file
	}

	_, err = outWriter.Write(output)
	return err
}

// detectChromePath attempts to find Chrome or any Chromium-based browser in common installation locations
func detectChromePath() (string, bool) {
	// Try to find browsers in PATH first (ordered by preference)
	for _, browser := range []string{
		"google-chrome", "chrome", "chromium", "chromium-browser",
		"brave", "brave-browser", "msedge", "edge", "vivaldi", "opera"} {
		if path, err := exec.LookPath(browser); err == nil {
			return path, true
		}
	}

	// Check OS-specific locations
	switch runtime.GOOS {
	case "darwin":
		paths := []string{
			// Chrome variants
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary",

			// Chromium variants
			"/Applications/Chromium.app/Contents/MacOS/Chromium",

			// Brave
			"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",

			// Microsoft Edge
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",

			// Opera
			"/Applications/Opera.app/Contents/MacOS/Opera",

			// Vivaldi
			"/Applications/Vivaldi.app/Contents/MacOS/Vivaldi",

			// User-level installations
			"~/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"~/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
			"~/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
			"~/Applications/Chromium.app/Contents/MacOS/Chromium",
		}
		for _, path := range paths {
			expandedPath := path
			if strings.HasPrefix(path, "~/") {
				home, err := os.UserHomeDir()
				if err == nil {
					expandedPath = filepath.Join(home, path[2:])
				}
			}
			if _, err := os.Stat(expandedPath); err == nil {
				return expandedPath, true
			}
		}
	case "windows":
		paths := []string{
			// Chrome
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,

			// Edge
			`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
			`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,

			// Brave
			`C:\Program Files\BraveSoftware\Brave-Browser\Application\brave.exe`,
			`C:\Program Files (x86)\BraveSoftware\Brave-Browser\Application\brave.exe`,

			// Vivaldi
			`C:\Program Files\Vivaldi\Application\vivaldi.exe`,
			`C:\Program Files (x86)\Vivaldi\Application\vivaldi.exe`,

			// Opera
			`C:\Program Files\Opera\launcher.exe`,
			`C:\Program Files (x86)\Opera\launcher.exe`,
		}
		for _, path := range paths {
			if _, err := os.Stat(path); err == nil {
				return path, true
			}
		}
	case "linux":
		paths := []string{
			// Chrome
			"/usr/bin/google-chrome",
			"/usr/bin/google-chrome-stable",
			"/opt/google/chrome/chrome",

			// Chromium
			"/usr/bin/chromium",
			"/usr/bin/chromium-browser",
			"/snap/bin/chromium",

			// Brave
			"/usr/bin/brave-browser",
			"/usr/bin/brave",
			"/opt/brave.com/brave/brave",
			"/snap/bin/brave",

			// Edge
			"/usr/bin/microsoft-edge",
			"/usr/bin/microsoft-edge-stable",
			"/opt/microsoft/msedge/msedge",

			// Vivaldi
			"/usr/bin/vivaldi",
			"/usr/bin/vivaldi-stable",
			"/opt/vivaldi/vivaldi",

			// Opera
			"/usr/bin/opera",
			"/usr/bin/opera-stable",
		}
		for _, path := range paths {
			if _, err := os.Stat(path); err == nil {
				return path, true
			}
		}
	}

	return "", false
}

// loadScripts combines inline scripts and file-based scripts into a single slice
func loadScripts(inlineScripts []string, scriptFiles []string, verbose bool) ([]string, error) {
	var allScripts []string

	// Add inline scripts first
	allScripts = append(allScripts, inlineScripts...)

	// Load and add file-based scripts
	for _, scriptFile := range scriptFiles {
		if verbose {
			log.Printf("Loading script file: %s", scriptFile)
		}

		content, err := os.ReadFile(scriptFile)
		if err != nil {
			return nil, chromeErrors.WithContext(
				chromeErrors.FileError("read", scriptFile, err),
				"operation", "load_script",
			)
		}

		script := string(content)
		if script == "" {
			if verbose {
				log.Printf("Warning: script file %s is empty", scriptFile)
			}
			continue
		}

		// Basic validation for JavaScript syntax
		if err := validateJavaScript(script); err != nil {
			return nil, chromeErrors.WithContext(
				chromeErrors.Wrap(err, chromeErrors.InvalidScriptError, "script validation failed"),
				"file", scriptFile,
			)
		}

		allScripts = append(allScripts, script)

		if verbose {
			log.Printf("Successfully loaded script file %s (%d characters)", scriptFile, len(script))
		}
	}

	return allScripts, nil
}

// validateJavaScript performs basic validation of JavaScript content
func validateJavaScript(script string) error {
	// Trim whitespace and check for empty content
	script = strings.TrimSpace(script)
	if script == "" {
		return chromeErrors.New(chromeErrors.ValidationError, "script is empty")
	}

	// Basic checks for potentially dangerous patterns
	// This is a simple validation - more sophisticated validation could be added

	// Check for balanced braces (basic syntax check)
	braceCount := 0
	parenCount := 0
	bracketCount := 0

	for _, char := range script {
		switch char {
		case '{':
			braceCount++
		case '}':
			braceCount--
		case '(':
			parenCount++
		case ')':
			parenCount--
		case '[':
			bracketCount++
		case ']':
			bracketCount--
		}
	}

	if braceCount != 0 {
		return chromeErrors.New(chromeErrors.InvalidScriptError, "unbalanced braces in script")
	}
	if parenCount != 0 {
		return chromeErrors.New(chromeErrors.InvalidScriptError, "unbalanced parentheses in script")
	}
	if bracketCount != 0 {
		return chromeErrors.New(chromeErrors.InvalidScriptError, "unbalanced brackets in script")
	}

	return nil
}

// handleWebSocketOperations handles WebSocket-specific operations
func handleWebSocketOperations(ctx context.Context, b *browser.Browser, opts options) error {
	// Get the first page from the browser
	page, err := b.NewPage()
	if err != nil {
		return errors.Wrap(err, "failed to create new page")
	}

	// Enable WebSocket monitoring
	if err := page.EnableWebSocketMonitoring(); err != nil {
		return errors.Wrap(err, "failed to enable WebSocket monitoring")
	}

	// Set up WebSocket event handlers for verbose logging
	if opts.verbose {
		page.SetWebSocketConnectionHandler(
			func(conn *browser.WebSocketConnection) {
				log.Printf("WebSocket connected: %s", conn.URL)
			},
			func(conn *browser.WebSocketConnection) {
				log.Printf("WebSocket disconnected: %s", conn.URL)
			},
			func(conn *browser.WebSocketConnection, err error) {
				log.Printf("WebSocket error on %s: %v", conn.URL, err)
			},
		)

		page.SetWebSocketFrameHandler(
			func(conn *browser.WebSocketConnection, frame *browser.WebSocketFrame) {
				log.Printf("WebSocket frame received from %s: %s (%d bytes)", 
					conn.URL, frame.Type, frame.Size)
			},
			func(conn *browser.WebSocketConnection, frame *browser.WebSocketFrame) {
				log.Printf("WebSocket frame sent to %s: %s (%d bytes)", 
					conn.URL, frame.Type, frame.Size)
			},
		)
	}

	// Wait for WebSocket condition if specified
	if opts.webSocketWaitFor != "" {
		condition := browser.WebSocketWaitCondition(opts.webSocketWaitFor)
		timeout := time.Duration(opts.webSocketTimeout) * time.Second
		
		waitOpts := []browser.WebSocketWaitOption{
			browser.WithWebSocketWaitTimeout(timeout),
			browser.WithURLPattern(opts.webSocketURLPattern),
		}
		
		if opts.webSocketDataPattern != "" {
			waitOpts = append(waitOpts, browser.WithDataPattern(opts.webSocketDataPattern))
		}
		
		if opts.webSocketDirection != "" {
			waitOpts = append(waitOpts, browser.WithDirection(opts.webSocketDirection))
		}

		if opts.verbose {
			log.Printf("Waiting for WebSocket condition: %s", opts.webSocketWaitFor)
		}

		conn, err := page.WaitForWebSocket(condition, waitOpts...)
		if err != nil {
			return errors.Wrap(err, "failed to wait for WebSocket condition")
		}

		if opts.verbose {
			log.Printf("WebSocket condition met: %s on %s", opts.webSocketWaitFor, conn.URL)
		}
	}

	// Send WebSocket messages if specified
	if len(opts.webSocketMessages) > 0 {
		// Wait for a WebSocket connection first
		timeout := time.Duration(opts.webSocketTimeout) * time.Second
		conn, err := page.WaitForWebSocketConnection(opts.webSocketURLPattern, timeout)
		if err != nil {
			return errors.Wrap(err, "failed to wait for WebSocket connection")
		}

		// Send each message
		for _, message := range opts.webSocketMessages {
			if err := page.SendWebSocketMessage(conn.ID, message); err != nil {
				return errors.Wrapf(err, "failed to send WebSocket message: %s", message)
			}
			
			if opts.verbose {
				log.Printf("Sent WebSocket message to %s: %s", conn.URL, message)
			}
		}
	}

	// Generate WebSocket output if requested
	if opts.webSocketOutputFile != "" {
		if err := generateWebSocketOutput(page, opts); err != nil {
			return errors.Wrap(err, "failed to generate WebSocket output")
		}
	}

	// Show WebSocket statistics if requested
	if opts.webSocketStats {
		showWebSocketStats(page, opts.verbose)
	}

	return nil
}

// generateWebSocketOutput generates WebSocket output to a file
func generateWebSocketOutput(page *browser.Page, opts options) error {
	connections := page.GetWebSocketConnections()
	
	// Create WebSocket HAR exporter
	var filter *browser.WebSocketHARFilter
	if opts.webSocketURLPattern != "*" || opts.webSocketDirection != "" {
		filter = &browser.WebSocketHARFilter{
			URLPattern: opts.webSocketURLPattern,
			Direction:  opts.webSocketDirection,
		}
	}

	exporter := browser.NewWebSocketHARExporter(filter)
	
	// Generate output based on file extension
	var output []byte
	var err error
	
	if strings.HasSuffix(opts.webSocketOutputFile, ".json") {
		output, err = exporter.ExportWithWebSocketData(connections)
	} else {
		output, err = exporter.Export(connections)
	}
	
	if err != nil {
		return errors.Wrap(err, "failed to export WebSocket data")
	}

	// Write to file
	if err := os.WriteFile(opts.webSocketOutputFile, output, 0644); err != nil {
		return errors.Wrap(err, "failed to write WebSocket output file")
	}

	if opts.verbose {
		log.Printf("WebSocket data written to: %s", opts.webSocketOutputFile)
	}

	return nil
}

// showWebSocketStats displays WebSocket statistics
func showWebSocketStats(page *browser.Page, verbose bool) {
	stats := page.GetWebSocketStats()
	
	fmt.Printf("\n" + strings.Repeat("=", 50) + "\n")
	fmt.Printf("WebSocket Statistics\n")
	fmt.Printf(strings.Repeat("=", 50) + "\n")
	
	fmt.Printf("Active Connections: %v\n", stats["active_connections"])
	fmt.Printf("Total Bytes Sent: %v\n", stats["total_bytes_sent"])
	fmt.Printf("Total Bytes Received: %v\n", stats["total_bytes_received"])
	fmt.Printf("Total Messages Sent: %v\n", stats["total_messages_sent"])
	fmt.Printf("Total Messages Received: %v\n", stats["total_messages_received"])
	
	if verbose {
		fmt.Printf("\nConnection Details:\n")
		fmt.Printf(strings.Repeat("-", 30) + "\n")
		
		connections := page.GetWebSocketConnections()
		for id, conn := range connections {
			fmt.Printf("Connection %s:\n", id)
			fmt.Printf("  URL: %s\n", conn.URL)
			fmt.Printf("  State: %s\n", conn.State)
			fmt.Printf("  Protocol: %s\n", conn.Protocol)
			fmt.Printf("  Bytes Sent: %d\n", conn.BytesSent)
			fmt.Printf("  Bytes Received: %d\n", conn.BytesReceived)
			fmt.Printf("  Messages Sent: %d\n", conn.MessagesSent)
			fmt.Printf("  Messages Received: %d\n", conn.MessagesReceived)
			fmt.Printf("  Frame Count: %d\n", len(conn.Frames))
			fmt.Printf("  Connection Time: %v\n", conn.ConnectionLatency)
			fmt.Printf("\n")
		}
	}
}
