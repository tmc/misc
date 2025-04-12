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

	// Run the churl command
	if err := run(ctx, pm, url, opts); err != nil {
		if err == context.DeadlineExceeded {
			log.Fatal("Operation timed out. Try increasing the timeout value.")
		} else {
			log.Fatal(err)
		}
	}
}

func run(ctx context.Context, pm chromeprofiles.ProfileManager, url string, opts options) error {
	// Parse request headers
	headers := make(map[string]string)
	for _, h := range opts.headers {
		parts := strings.SplitN(h, ":", 2)
		if len(parts) != 2 {
			return errors.Errorf("invalid header format: %s", h)
		}
		name := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		headers[name] = value
	}

	// Verify profile if provided
	if opts.profileDir != "" {
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

	// Create and launch browser
	b, err := browser.New(ctx, pm, browserOpts...)
	if err != nil {
		return errors.Wrap(err, "creating browser")
	}

	if err := b.Launch(ctx); err != nil {
		return errors.Wrap(err, "launching browser")
	}
	defer b.Close()

	// Set up request headers
	if len(headers) > 0 {
		if err := b.SetRequestHeaders(headers); err != nil {
			return errors.Wrap(err, "setting request headers")
		}
	}

	// Set up basic auth if provided
	if opts.username != "" && opts.password != "" {
		if err := b.SetBasicAuth(opts.username, opts.password); err != nil {
			return errors.Wrap(err, "setting basic authentication")
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
			return errors.Wrap(err, "creating recorder")
		}

		// Enable network monitoring
		if err := network.Enable().Do(b.Context()); err != nil {
			return errors.Wrap(err, "enabling network monitoring")
		}

		// Set up event listener for network events
		chromedp.ListenTarget(b.Context(), rec.HandleNetworkEvent(b.Context()))
	}

	// Navigate to the URL
	if err := b.Navigate(url); err != nil {
		return errors.Wrap(err, "navigating to URL")
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
			return errors.New("recorder not initialized for HAR output")
		}

		// Write HAR to a temporary file and read it back
		tmpFile, err := os.CreateTemp("", "churl-*.har")
		if err != nil {
			return errors.Wrap(err, "creating temp file")
		}
		defer os.Remove(tmpFile.Name())

		if err := rec.WriteHAR(tmpFile.Name()); err != nil {
			return errors.Wrap(err, "writing HAR")
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
			return errors.Wrap(outputErr, "getting URL")
		}

		info.Title, outputErr = b.GetTitle()
		if outputErr != nil {
			return errors.Wrap(outputErr, "getting title")
		}

		info.Content, outputErr = b.GetHTML()
		if outputErr != nil {
			return errors.Wrap(outputErr, "getting HTML")
		}

		output, outputErr = json.MarshalIndent(info, "", "  ")

	default:
		return errors.Errorf("unsupported output format: %s", opts.outputFormat)
	}

	if outputErr != nil {
		return errors.Wrap(outputErr, "getting output")
	}

	// Write the output
	var outWriter io.Writer = os.Stdout
	if opts.outputFile != "" {
		file, err := os.Create(opts.outputFile)
		if err != nil {
			return errors.Wrap(err, "creating output file")
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
