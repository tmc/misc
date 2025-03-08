package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/pkg/errors"
	"github.com/tmc/misc/chrome-to-har/internal/chromeprofiles"
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
	debugPort       int       // Chrome debug port
	timeout         int       // Global timeout in seconds
	chromePath      string    // Path to Chrome executable
	debugMode       bool      // Run Chrome debug diagnostics
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
		return err
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
			return errors.Wrap(err, "listing profiles")
		}
		if len(profiles) == 0 {
			return errors.New("no Chrome profiles found")
		}
		opts.profileDir = profiles[0]
		if opts.verbose {
			log.Printf("Auto-selected profile: %s", opts.profileDir)
		}
	}

	// Verify profile exists
	profiles, err := pm.ListProfiles()
	if err != nil {
		return errors.Wrap(err, "listing profiles")
	}
	profileExists := false
	for _, p := range profiles {
		if p == opts.profileDir {
			profileExists = true
			break
		}
	}
	if !profileExists {
		return errors.Errorf("profile not found: %s", opts.profileDir)
	}

	runner := NewRunner(pm)
	return runner.Run(ctx, opts)
}

func (r *Runner) Run(ctx context.Context, opts options) error {
	if err := r.pm.SetupWorkdir(); err != nil {
		return errors.Wrap(err, "setting up working directory")
	}
	defer r.pm.Cleanup()

	var cookieDomains []string
	if opts.cookieDomains != "" {
		cookieDomains = splitAndTrim(opts.cookieDomains, ",")
	}

	if err := r.pm.CopyProfile(opts.profileDir, cookieDomains); err != nil {
		return errors.Wrap(err, "copying profile")
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
		// Use default Chrome path for macOS as fallback
		defaultPath := "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
		if _, err := os.Stat(defaultPath); err == nil {
			copts = append(copts, chromedp.ExecPath(defaultPath))
			if opts.verbose {
				log.Printf("Using default Chrome path: %s", defaultPath)
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
		return errors.Wrap(testErr, "testing Chrome connection failed")
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
		return errors.Wrap(err, "creating recorder")
	}

	// Enable network events
	if err := chromedp.Run(taskCtx,
		network.Enable(),
		chromedp.ActionFunc(func(ctx context.Context) error {
			chromedp.ListenTarget(ctx, rec.HandleNetworkEvent(ctx))
			return nil
		}),
	); err != nil {
		return errors.Wrap(err, "enabling network monitoring")
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
			return errors.Wrap(err, "navigating to URL")
		}
		
		if opts.verbose {
			log.Printf("Successfully navigated to: %s", opts.startURL)
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
			return errors.Wrap(err, "writing HAR file")
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