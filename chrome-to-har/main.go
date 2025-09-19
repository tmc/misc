package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/pkg/errors"
	"github.com/tmc/misc/chrome-to-har/internal/blocking"
	"github.com/tmc/misc/chrome-to-har/internal/chromeprofiles"
	"github.com/tmc/misc/chrome-to-har/internal/differential"
	"github.com/tmc/misc/chrome-to-har/internal/discovery"
	chromeErrors "github.com/tmc/misc/chrome-to-har/internal/errors"
	"github.com/tmc/misc/chrome-to-har/internal/recorder"
)

// Custom code implementing JavaScript interactive CLI mode

type options struct {
	profileDir      string
	outputFile      string
	differential    bool
	verbose         bool
	startURL        string
	cookiePattern   string
	urlPattern      string
	blockPattern    string
	omitPattern     string
	cookieDomains   string
	listProfiles    bool
	restoreSession  bool
	streaming       bool
	headless        bool
	filter          string
	template        string
	interactiveMode bool
	debugPort       int    // Chrome debug port
	timeout         int    // Global timeout in seconds
	chromePath      string // Path to Chrome executable
	debugMode       bool   // Run Chrome debug diagnostics
	waitStable      bool   // Wait until page is stable (network and DOM)
	stableTimeout   int    // Max time in seconds to wait for stability
	waitSelector    string // Wait for specific CSS selector to appear
	getHTML         bool   // Output HTML instead of HAR

	// Enhanced stability detection options
	waitForStability   bool // Use enhanced stability detection
	networkIdleTimeout int  // Network idle timeout in milliseconds
	domStableTimeout   int  // DOM stable timeout in milliseconds
	resourceTimeout    int  // Resource loading timeout in seconds
	stabilityRetries   int  // Number of retry attempts for stability
	waitForImages      bool // Wait for all images to load
	waitForFonts       bool // Wait for all fonts to load
	waitForStylesheets bool // Wait for all stylesheets to load
	waitForScripts     bool // Wait for all scripts to load

	// Blocking options
	blockingEnabled     bool   // Enable URL/domain blocking
	blockingVerbose     bool   // Enable verbose blocking logging
	blockURLPatterns    string // Comma-separated URL patterns to block
	blockDomains        string // Comma-separated domains to block
	blockRegexPatterns  string // Comma-separated regex patterns to block
	allowURLs           string // Comma-separated URLs to allow (whitelist)
	allowDomains        string // Comma-separated domains to allow (whitelist)
	blockingRuleFile    string // File containing blocking rules
	blockCommonAds      bool   // Block common ad domains
	blockCommonTracking bool   // Block common tracking domains

	// Differential capture options
	diffMode         bool   // Enable differential mode
	baselineCapture  string // Baseline capture name or ID
	compareWith      string // Compare with this capture
	diffOutput       string // Diff report output path
	diffFormat       string // Diff report format (json, html, text, csv)
	trackResources   bool   // Track resource changes
	trackPerformance bool   // Track performance changes
	trackStates      bool   // Track page state changes
	captureName      string // Name for the current capture
	captureLabels    string // Labels for the current capture (key=value,key2=value2)
	diffWorkDir      string // Working directory for differential captures
	listCaptures     bool   // List available captures
	deleteCapture    string // Delete a capture by ID
	minSignificance  string // Minimum significance level for reporting

	// Security options
	securityProfile       string // Security profile: strict, balanced, permissive
	allowedRemoteHosts    string // Comma-separated list of allowed remote hosts
	remoteAuthToken       string // Authentication token for remote connections
	remoteUseTLS          bool   // Use TLS for remote connections
	remoteCACert          string // Path to CA certificate for remote TLS
	maxMemoryMB           uint64 // Maximum memory usage in MB
	maxConcurrentReqs     int    // Maximum concurrent requests
	enableResourceLimits  bool   // Enable resource limiting
	scriptValidation      bool   // Enable script validation
	allowDangerousScripts bool   // Allow potentially dangerous scripts
}

type Runner struct {
	pm chromeprofiles.ProfileManager
}

func NewRunner(pm chromeprofiles.ProfileManager) *Runner {
	return &Runner{pm: pm}
}

func init() {
	flag.Usage = func() {
		w := tabwriter.NewWriter(os.Stderr, 0, 0, 2, ' ', 0)
		defer w.Flush()

		fmt.Fprintf(w, "chrome-to-har - Chrome network activity capture tool\n\n")
		fmt.Fprintf(w, "Usage:\n")
		fmt.Fprintf(w, "  chrome-to-har [options]\n\n")
		fmt.Fprintf(w, "Options:\n")

		lines := make([]string, 0)
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
			default:
				typ = "string"
			}

			lines = append(lines, fmt.Sprintf("  -%s\t%s\t%s%s\n", f.Name, typ, f.Usage, def))
		})

		for _, line := range lines {
			fmt.Fprint(w, line)
		}
	}
}

func main() {
	opts := options{}

	flag.StringVar(&opts.profileDir, "profile", "", "Chrome profile directory to use")
	flag.StringVar(&opts.outputFile, "output", "output.har", "Output HAR file")
	flag.BoolVar(&opts.differential, "diff", false, "Enable differential HAR capture")
	flag.BoolVar(&opts.verbose, "verbose", false, "Enable verbose logging")
	flag.StringVar(&opts.startURL, "url", "", "Starting URL to navigate to")
	flag.StringVar(&opts.cookiePattern, "cookies", "", "Regular expression to filter cookies in HAR output")
	flag.StringVar(&opts.urlPattern, "urls", "", "Regular expression to filter URLs")
	flag.StringVar(&opts.blockPattern, "block", "", "Regular expression of URLs to block from loading")
	flag.StringVar(&opts.omitPattern, "omit", "", "Regular expression of URLs to omit from HAR output")
	flag.StringVar(&opts.cookieDomains, "cookie-domains", "", "Comma-separated list of domains to include cookies from")
	flag.BoolVar(&opts.listProfiles, "list-profiles", false, "List available Chrome profiles")
	flag.BoolVar(&opts.restoreSession, "restore-session", false, "Restore previous session on startup")
	flag.BoolVar(&opts.streaming, "stream", false, "Stream HAR entries as they are captured (outputs NDJSON)")
	flag.BoolVar(&opts.headless, "headless", false, "Run Chrome in headless mode")
	flag.StringVar(&opts.filter, "filter", "", "JQ expression to filter HAR entries")
	flag.StringVar(&opts.template, "template", "", "Go template to transform HAR entries")
	flag.BoolVar(&opts.interactiveMode, "interactive", false, "Run in interactive CLI mode")
	flag.IntVar(&opts.debugPort, "debug-port", 0, "Use specific port for Chrome DevTools (0 for auto)")
	flag.IntVar(&opts.timeout, "timeout", 180, "Global timeout in seconds (default: 180)")
	flag.StringVar(&opts.chromePath, "chrome-path", "", "Path to Chrome executable")
	flag.BoolVar(&opts.debugMode, "debug-chrome", false, "Run Chrome debugging diagnostics")
	flag.BoolVar(&opts.waitStable, "wait-stable", false, "Wait until page is stable (network and DOM)")
	flag.IntVar(&opts.stableTimeout, "stable-timeout", 30, "Max time in seconds to wait for stability")
	flag.StringVar(&opts.waitSelector, "wait-for", "", "Wait for specific CSS selector to appear")
	flag.BoolVar(&opts.getHTML, "html", false, "Output HTML instead of HAR")

	// Enhanced stability detection flags
	flag.BoolVar(&opts.waitForStability, "wait-for-stability", false, "Use enhanced stability detection system")
	flag.IntVar(&opts.networkIdleTimeout, "network-idle-timeout", 500, "Network idle timeout in milliseconds")
	flag.IntVar(&opts.domStableTimeout, "dom-stable-timeout", 500, "DOM stable timeout in milliseconds")
	flag.IntVar(&opts.resourceTimeout, "resource-timeout", 10, "Resource loading timeout in seconds")
	flag.IntVar(&opts.stabilityRetries, "stability-retries", 3, "Number of retry attempts for stability detection")
	flag.BoolVar(&opts.waitForImages, "wait-for-images", true, "Wait for all images to load")
	flag.BoolVar(&opts.waitForFonts, "wait-for-fonts", true, "Wait for all fonts to load")
	flag.BoolVar(&opts.waitForStylesheets, "wait-for-stylesheets", true, "Wait for all stylesheets to load")
	flag.BoolVar(&opts.waitForScripts, "wait-for-scripts", true, "Wait for all scripts to load")

	// Blocking flags
	flag.BoolVar(&opts.blockingEnabled, "block-enabled", false, "Enable URL/domain blocking")
	flag.BoolVar(&opts.blockingVerbose, "block-verbose", false, "Enable verbose blocking logging")
	flag.StringVar(&opts.blockURLPatterns, "block-url", "", "Comma-separated URL patterns to block")
	flag.StringVar(&opts.blockDomains, "block-domain", "", "Comma-separated domains to block")
	flag.StringVar(&opts.blockRegexPatterns, "block-regex", "", "Comma-separated regex patterns to block")
	flag.StringVar(&opts.allowURLs, "allow-url", "", "Comma-separated URLs to allow (whitelist)")
	flag.StringVar(&opts.allowDomains, "allow-domain", "", "Comma-separated domains to allow (whitelist)")
	flag.StringVar(&opts.blockingRuleFile, "block-file", "", "File containing blocking rules")
	flag.BoolVar(&opts.blockCommonAds, "block-ads", false, "Block common ad domains")
	flag.BoolVar(&opts.blockCommonTracking, "block-tracking", false, "Block common tracking domains")

	// Differential capture flags
	flag.BoolVar(&opts.diffMode, "diff-mode", false, "Enable differential capture mode")
	flag.StringVar(&opts.baselineCapture, "baseline", "", "Baseline capture name or ID for comparison")
	flag.StringVar(&opts.compareWith, "compare-with", "", "Compare with this capture name or ID")
	flag.StringVar(&opts.diffOutput, "diff-output", "", "Diff report output path")
	flag.StringVar(&opts.diffFormat, "diff-format", "html", "Diff report format (json, html, text, csv)")
	flag.BoolVar(&opts.trackResources, "track-resources", true, "Track resource changes between captures")
	flag.BoolVar(&opts.trackPerformance, "track-performance", true, "Track performance changes between captures")
	flag.BoolVar(&opts.trackStates, "track-states", false, "Track page state changes during interactions")
	flag.StringVar(&opts.captureName, "capture-name", "", "Name for the current capture")
	flag.StringVar(&opts.captureLabels, "capture-labels", "", "Labels for the current capture (key=value,key2=value2)")
	flag.StringVar(&opts.diffWorkDir, "diff-work-dir", "", "Working directory for differential captures")
	flag.BoolVar(&opts.listCaptures, "list-captures", false, "List available captures")
	flag.StringVar(&opts.deleteCapture, "delete-capture", "", "Delete a capture by ID")
	flag.StringVar(&opts.minSignificance, "min-significance", "low", "Minimum significance level for reporting (low, medium, high)")

	flag.Parse()

	if opts.debugMode {
		if err := runChromeDebug(); err != nil {
			log.Fatalf("Chrome debugging failed: %v", err)
		}
		return
	}

	if opts.listProfiles {
		if err := listAvailableProfiles(opts.verbose); err != nil {
			log.Fatal(err)
		}
		return
	}

	// Handle differential mode operations
	if opts.diffMode || opts.listCaptures || opts.deleteCapture != "" || opts.compareWith != "" {
		if err := runDifferentialMode(opts); err != nil {
			log.Fatal(err)
		}
		return
	}

	// Set start URL for AI Studio if none provided
	if opts.startURL == "" && opts.interactiveMode {
		opts.startURL = "https://aistudio.google.com/live"
	}

	// Create a context with user-specified timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(opts.timeout)*time.Second)
	defer cancel()

	if opts.verbose {
		log.Printf("Using global timeout of %d seconds", opts.timeout)
	}

	pm, err := chromeprofiles.NewProfileManager(
		chromeprofiles.WithVerbose(opts.verbose),
	)
	if err != nil {
		log.Fatal(err)
	}

	if err := run(ctx, pm, opts); err != nil {
		if err == context.DeadlineExceeded {
			log.Fatal("Operation timed out. Try increasing the timeout value or check your Chrome browser.")
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

func listAvailableProfiles(verbose bool) error {
	pm, err := chromeprofiles.NewProfileManager(
		chromeprofiles.WithVerbose(verbose),
	)
	if err != nil {
		return err
	}

	profiles, err := pm.ListProfiles()
	if err != nil {
		return chromeErrors.Wrap(err, chromeErrors.ProfileSetupError, "failed to list available profiles")
	}

	fmt.Println("Available Chrome profiles:")
	for _, p := range profiles {
		fmt.Printf("  - %s\n", p)
	}
	return nil
}

func run(ctx context.Context, pm chromeprofiles.ProfileManager, opts options) error {
	// Validate profile
	if opts.profileDir == "" {
		profiles, err := pm.ListProfiles()
		if err != nil {
			return chromeErrors.Wrap(err, chromeErrors.ProfileSetupError, "failed to list Chrome profiles")
		}
		if len(profiles) == 0 {
			return chromeErrors.New(chromeErrors.ProfileNotFoundError, "no Chrome profiles found")
		}
		opts.profileDir = profiles[0]
		if opts.verbose {
			log.Printf("Auto-selected profile: %s", opts.profileDir)
		}
	}

	// Verify profile exists
	profiles, err := pm.ListProfiles()
	if err != nil {
		return chromeErrors.Wrap(err, chromeErrors.ProfileSetupError, "failed to list Chrome profiles for verification")
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

	runner := NewRunner(pm)
	return runner.Run(ctx, opts)
}

func (r *Runner) Run(ctx context.Context, opts options) error {
	if err := r.pm.SetupWorkdir(); err != nil {
		return chromeErrors.Wrap(err, chromeErrors.ProfileSetupError, "failed to set up profile working directory")
	}
	defer r.pm.Cleanup()

	var cookieDomains []string
	if opts.cookieDomains != "" {
		cookieDomains = splitAndTrim(opts.cookieDomains, ",")
	}

	if err := r.pm.CopyProfile(opts.profileDir, cookieDomains); err != nil {
		return chromeErrors.WithContext(
			chromeErrors.Wrap(err, chromeErrors.ProfileCopyError, "failed to copy Chrome profile"),
			"profile", opts.profileDir,
		)
	}

	// Chrome launch options
	copts := []chromedp.ExecAllocatorOption{
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		// chromedp.UserDataDir(r.pm.WorkDir()), // Temporarily comment out for testing
		// Increase timeouts to handle complex sites
		chromedp.WSURLReadTimeout(180 * time.Second), // Increase from 90 to 180 seconds
		// Disable GPU for better stability
		chromedp.DisableGPU,
		// Set Chrome path if specified
		// Add additional stability flags
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("enable-features", "NetworkService,NetworkServiceInProcess"),
		chromedp.Flag("disable-background-timer-throttling", true),
		chromedp.Flag("disable-backgrounding-occluded-windows", true),
		chromedp.Flag("disable-breakpad", true),
		chromedp.Flag("disable-client-side-phishing-detection", true),
		chromedp.Flag("disable-default-apps", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-hang-monitor", true),
		chromedp.Flag("disable-ipc-flooding-protection", true),
		chromedp.Flag("disable-popup-blocking", true),
		chromedp.Flag("disable-prompt-on-repost", true),
		chromedp.Flag("disable-renderer-backgrounding", true),
		chromedp.Flag("disable-sync", true),
		chromedp.Flag("force-color-profile", "srgb"),
		chromedp.Flag("metrics-recording-only", true),
		chromedp.Flag("no-first-run", true),
		chromedp.Flag("safebrowsing-disable-auto-update", true),
		chromedp.Flag("password-store", "basic"),
		chromedp.Flag("use-mock-keychain", true),
	}

	// Add Chrome path if specified
	if opts.chromePath != "" {
		copts = append(copts, chromedp.ExecPath(opts.chromePath))
		if opts.verbose {
			log.Printf("Using Chrome at: %s", opts.chromePath)
		}
	} else {
		// Use browser discovery system to find best available browser
		if browserPath := discovery.FindBestBrowser(); browserPath != "" {
			copts = append(copts, chromedp.ExecPath(browserPath))
			if opts.verbose {
				log.Printf("Using discovered browser at: %s", browserPath)
			}
		}
	}

	// Add remote debugging port if specified
	if opts.debugPort > 0 {
		// Convert int to string to avoid type errors
		portStr := fmt.Sprintf("%d", opts.debugPort)
		copts = append(copts, chromedp.Flag("remote-debugging-port", portStr))
		if opts.verbose {
			log.Printf("Using debug port: %s", portStr)
		}
	}

	if opts.headless {
		copts = append(copts, chromedp.Headless)
	}

	// Enable stderr/stdout capturing for Chrome for debugging
	copts = append(copts, chromedp.CombinedOutput(os.Stdout))

	// Create Chrome instance
	if opts.verbose {
		log.Printf("Launching Chrome with profile from: %s", r.pm.WorkDir())
	}

	log.Printf("Creating new Chrome process...")
	allocCtx, cancel := chromedp.NewExecAllocator(ctx, copts...)
	defer cancel()
	log.Printf("Chrome process allocator created, attempting to launch browser...")

	// Add browser debug logging if verbose
	var taskCtx context.Context
	var taskCancel context.CancelFunc

	if opts.verbose {
		taskCtx, taskCancel = chromedp.NewContext(
			allocCtx,
			chromedp.WithLogf(log.Printf),
		)
		log.Printf("Chrome DevTools context created, attempting to connect...")
	} else {
		taskCtx, taskCancel = chromedp.NewContext(allocCtx)
	}
	defer taskCancel()

	// Test the connection with a simple navigation to about:blank before proceeding
	if opts.verbose {
		log.Println("Testing Chrome connection with about:blank...")
	}

	// Use a longer timeout for the connection test
	testCtx, testCancel := context.WithTimeout(taskCtx, 180*time.Second)
	defer testCancel()

	testErr := chromedp.Run(testCtx, chromedp.Navigate("about:blank"))
	if testErr != nil {
		if opts.verbose {
			log.Printf("Chrome connection test failed: %v", testErr)
			log.Println("You can try the following:")
			log.Println("1. Increase timeout with -timeout=300")
			log.Println("2. Try a different debug port with -debug-port=9222")
			log.Println("3. Close any other Chrome instances that may be running")
			log.Println("4. Try with -headless flag")
		}
		return chromeErrors.Wrap(testErr, chromeErrors.ChromeConnectionError, "Chrome connection test failed")
	}

	if opts.verbose {
		log.Printf("Successfully connected to Chrome browser")
	}

	// Create recorder
	rec, err := recorder.New(
		recorder.WithVerbose(opts.verbose),
		recorder.WithStreaming(opts.streaming),
		recorder.WithFilter(opts.filter),
		recorder.WithTemplate(opts.template),
	)
	if err != nil {
		return chromeErrors.Wrap(err, chromeErrors.NetworkRecordError, "failed to create network recorder")
	}

	// Enable network events
	if err := chromedp.Run(taskCtx,
		network.Enable(),
		chromedp.ActionFunc(func(ctx context.Context) error {
			chromedp.ListenTarget(ctx, rec.HandleNetworkEvent(ctx))
			return nil
		}),
	); err != nil {
		return chromeErrors.Wrap(err, chromeErrors.NetworkError, "failed to enable network monitoring")
	}

	// Set up blocking if enabled
	var blockingEngine *blocking.BlockingEngine
	if opts.blockingEnabled {
		blockingConfig := &blocking.Config{
			Verbose:       opts.blockingVerbose,
			Enabled:       opts.blockingEnabled,
			URLPatterns:   splitAndTrim(opts.blockURLPatterns, ","),
			Domains:       splitAndTrim(opts.blockDomains, ","),
			RegexPatterns: splitAndTrim(opts.blockRegexPatterns, ","),
			AllowURLs:     splitAndTrim(opts.allowURLs, ","),
			AllowDomains:  splitAndTrim(opts.allowDomains, ","),
			RuleFile:      opts.blockingRuleFile,
		}

		var err error
		blockingEngine, err = blocking.NewBlockingEngine(blockingConfig)
		if err != nil {
			return errors.Wrap(err, "creating blocking engine")
		}

		// Add common blocking rules if requested
		if opts.blockCommonAds {
			if err := blockingEngine.AddCommonAdBlockRules(); err != nil {
				return errors.Wrap(err, "adding common ad blocking rules")
			}
		}

		if opts.blockCommonTracking {
			if err := blockingEngine.AddCommonTrackingBlockRules(); err != nil {
				return errors.Wrap(err, "adding common tracking blocking rules")
			}
		}

		// Enable fetch domain for request interception
		if err := chromedp.Run(taskCtx, fetch.Enable()); err != nil {
			return errors.Wrap(err, "enabling fetch domain for blocking")
		}

		// Set up the request interceptor
		chromedp.ListenTarget(taskCtx, func(ev interface{}) {
			switch e := ev.(type) {
			case *fetch.EventRequestPaused:
				go handleBlockingRequest(taskCtx, blockingEngine, e, opts.verbose)
			}
		})

		if opts.verbose {
			log.Printf("Blocking engine initialized with %d rules", len(blockingEngine.ListRules()))
		}
	}

	// Navigate if URL specified
	if opts.startURL != "" {
		if opts.verbose {
			log.Printf("Attempting to navigate to: %s", opts.startURL)
		}

		// Add a timeout specifically for navigation
		navCtx, navCancel := context.WithTimeout(taskCtx, 45*time.Second)
		defer navCancel()

		if err := chromedp.Run(navCtx, chromedp.Navigate(opts.startURL)); err != nil {
			return chromeErrors.WithContext(
				chromeErrors.Wrap(err, chromeErrors.ChromeNavigationError, "failed to navigate to URL"),
				"url", opts.startURL,
			)
		}

		if opts.verbose {
			log.Printf("Successfully navigated to: %s", opts.startURL)
		}

		// Wait for stability if requested
		if opts.waitForStability {
			if err := waitForEnhancedStability(taskCtx, opts); err != nil {
				if opts.verbose {
					log.Printf("Stability detection failed: %v", err)
				}
				// Don't fail on stability timeout, just log it
			}
		} else if opts.waitStable {
			// Legacy stability detection
			if err := waitForLegacyStability(taskCtx, opts); err != nil {
				if opts.verbose {
					log.Printf("Legacy stability detection failed: %v", err)
				}
			}
		}

		// Wait for specific selector if requested
		if opts.waitSelector != "" {
			selectorCtx, selectorCancel := context.WithTimeout(taskCtx, time.Duration(opts.stableTimeout)*time.Second)
			defer selectorCancel()

			if err := chromedp.Run(selectorCtx, chromedp.WaitVisible(opts.waitSelector)); err != nil {
				if opts.verbose {
					log.Printf("Failed to wait for selector '%s': %v", opts.waitSelector, err)
				}
				// Don't fail on selector timeout, just log it
			} else if opts.verbose {
				log.Printf("Successfully waited for selector: %s", opts.waitSelector)
			}
		}
	}

	// Interactive mode handling
	if opts.interactiveMode {
		return runInteractiveMode(taskCtx, opts.verbose)
	}

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Create a channel for Ctrl+D (EOF) detection
	eofChan := make(chan bool)
	go func() {
		buf := make([]byte, 1)
		for {
			_, err := os.Stdin.Read(buf)
			if err != nil {
				eofChan <- true
				return
			}
		}
	}()

	if opts.verbose {
		log.Println("Recording network activity. Press Ctrl+D to stop...")
	}

	// Wait for either signal or EOF
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-sigChan:
		if opts.verbose {
			log.Println("Received interrupt signal")
		}
	case <-eofChan:
		if opts.verbose {
			log.Println("Received EOF (Ctrl+D)")
		}
	}

	if !opts.streaming {
		if err := rec.WriteHAR(opts.outputFile); err != nil {
			return chromeErrors.WithContext(
				chromeErrors.FileError("write", opts.outputFile, err),
				"format", "har",
			)
		}
	}

	return nil
}

func runInteractiveMode(ctx context.Context, verbose bool) error {
	fmt.Println("Interactive CLI Mode. Type commands to execute JavaScript in the browser.")
	fmt.Println("Type 'exit' or 'quit' to exit. Press Ctrl+C to terminate.")
	fmt.Println("Examples:")
	fmt.Println("  document.title")
	fmt.Println("  window.location.href")
	fmt.Println("  document.querySelector('button').click()")

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		cmd := strings.TrimSpace(scanner.Text())
		if cmd == "" {
			continue
		}

		if cmd == "exit" || cmd == "quit" {
			if verbose {
				fmt.Println("Exiting interactive mode")
			}
			break
		}

		// Execute the JavaScript in the browser
		var result string
		err := chromedp.Run(ctx,
			chromedp.Evaluate(cmd, &result),
		)

		if err != nil {
			if execErr, ok := err.(*runtime.ExceptionDetails); ok {
				fmt.Printf("Error: %s\n", execErr.Text)
			} else {
				fmt.Printf("Error: %v\n", err)
			}
			continue
		}

		fmt.Println(result)
	}

	return nil
}

func splitAndTrim(s, sep string) []string {
	if s == "" {
		return nil
	}
	parts := make([]string, 0)
	for _, p := range strings.Split(s, sep) {
		if p = strings.TrimSpace(p); p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

// waitForEnhancedStability uses the new stability detection system
func waitForEnhancedStability(ctx context.Context, opts options) error {
	// Create a simple enhanced stability detection using chromedp directly
	// This is a simplified version since we're working with chromedp context directly

	stableCtx, cancel := context.WithTimeout(ctx, time.Duration(opts.stableTimeout)*time.Second)
	defer cancel()

	if opts.verbose {
		log.Println("Starting enhanced stability detection...")
	}

	// Enable network domain for monitoring
	if err := chromedp.Run(stableCtx, network.Enable()); err != nil {
		return chromeErrors.Wrap(err, chromeErrors.NetworkError, "failed to enable network domain")
	}

	// Wait for DOM to be ready first
	if err := chromedp.Run(stableCtx, chromedp.WaitReady("body")); err != nil {
		return chromeErrors.Wrap(err, chromeErrors.ChromeTimeoutError, "timed out waiting for DOM ready")
	}

	// Network idle detection
	if err := waitForNetworkIdle(stableCtx, opts); err != nil {
		return chromeErrors.Wrap(err, chromeErrors.NetworkIdleError, "network idle detection failed")
	}

	// Wait for resources if requested
	if opts.waitForImages || opts.waitForFonts || opts.waitForStylesheets || opts.waitForScripts {
		if err := waitForResources(stableCtx, opts); err != nil {
			if opts.verbose {
				log.Printf("Resource loading check failed: %v", err)
			}
			// Don't fail on resource timeout, just log it
		}
	}

	// Wait for JavaScript execution to complete
	if err := waitForJSExecution(stableCtx, opts); err != nil {
		if opts.verbose {
			log.Printf("JS execution check failed: %v", err)
		}
		// Don't fail on JS timeout, just log it
	}

	if opts.verbose {
		log.Println("Enhanced stability detection completed")
	}

	return nil
}

// waitForNetworkIdle waits for network activity to become idle
func waitForNetworkIdle(ctx context.Context, opts options) error {
	if opts.verbose {
		log.Println("Waiting for network idle...")
	}

	// Use a simplified network idle detection
	// This waits for the network idle timeout duration
	idleTimeout := time.Duration(opts.networkIdleTimeout) * time.Millisecond

	// Wait for the specified idle timeout
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(idleTimeout):
		if opts.verbose {
			log.Printf("Network idle timeout reached (%v)", idleTimeout)
		}
		return nil
	}
}

// waitForResources waits for page resources to load
func waitForResources(ctx context.Context, opts options) error {
	if opts.verbose {
		log.Println("Waiting for resources to load...")
	}

	resourceCtx, cancel := context.WithTimeout(ctx, time.Duration(opts.resourceTimeout)*time.Second)
	defer cancel()

	checks := []struct {
		enabled bool
		name    string
		script  string
	}{
		{
			opts.waitForImages,
			"images",
			`Array.from(document.images).every(img => img.complete && img.naturalHeight !== 0)`,
		},
		{
			opts.waitForStylesheets,
			"stylesheets",
			`Array.from(document.styleSheets).every(sheet => {
				try { return sheet.cssRules !== null; } catch(e) { return true; }
			})`,
		},
		{
			opts.waitForFonts,
			"fonts",
			`document.fonts ? document.fonts.ready.then(() => true) : true`,
		},
		{
			opts.waitForScripts,
			"scripts",
			`Array.from(document.scripts).every(script => !script.src || script.readyState === 'complete' || !script.readyState)`,
		},
	}

	for _, check := range checks {
		if !check.enabled {
			continue
		}

		if opts.verbose {
			log.Printf("Checking %s...", check.name)
		}

		// Use a polling approach to check resource loading
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-resourceCtx.Done():
				return errors.Errorf("timeout waiting for %s", check.name)
			case <-ticker.C:
				var result bool
				if err := chromedp.Run(resourceCtx, chromedp.Evaluate(check.script, &result)); err == nil && result {
					if opts.verbose {
						log.Printf("All %s loaded", check.name)
					}
					goto nextCheck
				}
			}
		}
	nextCheck:
	}

	return nil
}

// waitForJSExecution waits for JavaScript execution to complete
func waitForJSExecution(ctx context.Context, opts options) error {
	if opts.verbose {
		log.Println("Waiting for JS execution to complete...")
	}

	jsCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Wait for next animation frame
	var frameComplete bool
	if err := chromedp.Run(jsCtx, chromedp.Evaluate(`new Promise(resolve => requestAnimationFrame(() => resolve(true)))`, &frameComplete)); err != nil {
		return errors.Wrap(err, "waiting for animation frame")
	}

	// Wait for idle callback if available
	var idleComplete bool
	if err := chromedp.Run(jsCtx, chromedp.Evaluate(`new Promise(resolve => {
		if ('requestIdleCallback' in window) {
			requestIdleCallback(() => resolve(true), { timeout: 1000 });
		} else {
			setTimeout(() => resolve(true), 0);
		}
	})`, &idleComplete)); err != nil {
		return errors.Wrap(err, "waiting for idle callback")
	}

	if opts.verbose {
		log.Println("JS execution completed")
	}

	return nil
}

// waitForLegacyStability implements basic stability detection
func waitForLegacyStability(ctx context.Context, opts options) error {
	// Simple implementation that waits for page load event and then a fixed delay
	stableCtx, cancel := context.WithTimeout(ctx, time.Duration(opts.stableTimeout)*time.Second)
	defer cancel()

	// Wait for page load
	if err := chromedp.Run(stableCtx, chromedp.WaitReady("body")); err != nil {
		return errors.Wrap(err, "waiting for page to be ready")
	}

	// Wait for a fixed delay to allow dynamic content to load
	time.Sleep(2 * time.Second)

	if opts.verbose {
		log.Println("Legacy stability detection completed")
	}

	return nil
}

// runDifferentialMode handles differential capture mode operations
func runDifferentialMode(opts options) error {
	// Set up work directory
	workDir := opts.diffWorkDir
	if workDir == "" {
		workDir = filepath.Join(os.TempDir(), "chrome-to-har-diff")
	}

	// Create differential controller
	diffOpts := &differential.DifferentialOptions{
		WorkDir:          workDir,
		TrackResources:   opts.trackResources,
		TrackPerformance: opts.trackPerformance,
		TrackStates:      opts.trackStates,
		Verbose:          opts.verbose,
	}

	controller, err := differential.NewDifferentialController(diffOpts)
	if err != nil {
		return errors.Wrap(err, "creating differential controller")
	}
	defer controller.Cleanup()

	// Handle list captures
	if opts.listCaptures {
		captures := controller.ListCaptures()
		if len(captures) == 0 {
			fmt.Println("No captures found.")
			return nil
		}

		fmt.Printf("Available captures (%d):\n", len(captures))
		for _, capture := range captures {
			fmt.Printf("  ID: %s\n", capture.ID)
			fmt.Printf("  Name: %s\n", capture.Name)
			fmt.Printf("  URL: %s\n", capture.URL)
			fmt.Printf("  Timestamp: %s\n", capture.Timestamp.Format("2006-01-02 15:04:05"))
			fmt.Printf("  Status: %s\n", capture.Status)
			fmt.Printf("  Entries: %d\n", capture.EntryCount)
			fmt.Printf("  Size: %s\n", formatBytes(capture.Size))
			if len(capture.Labels) > 0 {
				fmt.Printf("  Labels: ")
				for k, v := range capture.Labels {
					fmt.Printf("%s=%s ", k, v)
				}
				fmt.Println()
			}
			fmt.Println()
		}
		return nil
	}

	// Handle delete capture
	if opts.deleteCapture != "" {
		if err := controller.DeleteCapture(opts.deleteCapture); err != nil {
			return errors.Wrap(err, "deleting capture")
		}
		fmt.Printf("Capture %s deleted successfully\n", opts.deleteCapture)
		return nil
	}

	// Handle comparison
	if opts.compareWith != "" {
		if opts.baselineCapture == "" {
			return fmt.Errorf("baseline capture must be specified when comparing")
		}

		// Perform comparison
		result, err := controller.CompareCapturesByID(opts.baselineCapture, opts.compareWith)
		if err != nil {
			return errors.Wrap(err, "comparing captures")
		}

		// Generate report
		if opts.diffOutput == "" {
			opts.diffOutput = fmt.Sprintf("diff-report-%s.%s",
				time.Now().Format("20060102-150405"), opts.diffFormat)
		}

		// Parse significance level
		var minSig differential.DiffSignificance
		switch strings.ToLower(opts.minSignificance) {
		case "low":
			minSig = differential.DiffSignificanceLow
		case "medium":
			minSig = differential.DiffSignificanceMedium
		case "high":
			minSig = differential.DiffSignificanceHigh
		default:
			minSig = differential.DiffSignificanceLow
		}

		// Parse report format
		var format differential.ReportFormat
		switch strings.ToLower(opts.diffFormat) {
		case "json":
			format = differential.ReportFormatJSON
		case "html":
			format = differential.ReportFormatHTML
		case "text":
			format = differential.ReportFormatText
		case "csv":
			format = differential.ReportFormatCSV
		default:
			format = differential.ReportFormatHTML
		}

		reportOpts := &differential.ReportOptions{
			Format:          format,
			OutputPath:      opts.diffOutput,
			IncludeDetails:  true,
			IncludeGraphs:   true,
			ThemeColor:      "#007bff",
			Title:           "Differential Capture Report",
			Description:     fmt.Sprintf("Comparison between %s and %s", opts.baselineCapture, opts.compareWith),
			ShowUnchanged:   false,
			MinSignificance: minSig,
		}

		if err := controller.GenerateReport(result, reportOpts); err != nil {
			return errors.Wrap(err, "generating report")
		}

		fmt.Printf("Comparison completed successfully\n")
		fmt.Printf("Report generated: %s\n", opts.diffOutput)
		fmt.Printf("Summary: %d added, %d removed, %d modified requests\n",
			result.Summary.AddedRequests, result.Summary.RemovedRequests, result.Summary.ModifiedRequests)

		return nil
	}

	// Handle differential mode (normal capture with differential features)
	if opts.diffMode {
		fmt.Println("Differential mode enabled")

		// Parse labels
		labels := make(map[string]string)
		if opts.captureLabels != "" {
			pairs := strings.Split(opts.captureLabels, ",")
			for _, pair := range pairs {
				kv := strings.Split(pair, "=")
				if len(kv) == 2 {
					labels[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
				}
			}
		}

		// Create capture name if not provided
		captureName := opts.captureName
		if captureName == "" {
			captureName = fmt.Sprintf("capture-%s", time.Now().Format("20060102-150405"))
		}

		// Create a new capture
		capture, err := controller.CreateBaselineCapture(context.Background(), captureName, opts.startURL, "Differential mode capture", labels)
		if err != nil {
			return errors.Wrap(err, "creating capture")
		}

		fmt.Printf("Created capture: %s (ID: %s)\n", capture.Name, capture.ID)
		fmt.Printf("Capture will be completed by the normal HAR recording process\n")
		fmt.Printf("To compare with another capture, use: -compare-with <capture-id> -baseline %s\n", capture.ID)

		return nil
	}

	return fmt.Errorf("no differential operation specified")
}

// formatBytes formats bytes into a human-readable string
func formatBytes(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	}
	if bytes < 1024*1024 {
		return fmt.Sprintf("%.2f KB", float64(bytes)/1024)
	}
	return fmt.Sprintf("%.2f MB", float64(bytes)/(1024*1024))
}

// handleBlockingRequest processes intercepted requests for blocking
func handleBlockingRequest(ctx context.Context, blockingEngine *blocking.BlockingEngine, ev *fetch.EventRequestPaused, verbose bool) {
	// Check if the request should be blocked
	if blockingEngine != nil && blockingEngine.ShouldBlock(ev.Request.URL) {
		// Block the request
		if err := chromedp.Run(ctx, fetch.FailRequest(ev.RequestID, network.ErrorReasonAccessDenied)); err != nil {
			if verbose {
				log.Printf("Error blocking request %s: %v", ev.Request.URL, err)
			}
		}
		return
	}

	// Continue the request as normal
	if err := chromedp.Run(ctx, fetch.ContinueRequest(ev.RequestID)); err != nil {
		if verbose {
			log.Printf("Error continuing request %s: %v", ev.Request.URL, err)
		}
	}
}
