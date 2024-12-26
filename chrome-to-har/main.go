package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"text/tabwriter"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/pkg/errors"
	"github.com/tmc/misc/chrome-to-har/internal/chromeprofiles"
	"github.com/tmc/misc/chrome-to-har/internal/recorder"
	"github.com/tmc/misc/chrome-to-har/internal/termmd"
)

type options struct {
	profileDir     string
	outputFile     string
	differential   bool
	verbose        bool
	startURL       string
	cookiePattern  string
	urlPattern     string
	blockPattern   string
	omitPattern    string
	cookieDomains  string
	listProfiles   bool
	restoreSession bool
	streaming      bool
	headless       bool
	filter         string
	template       string
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
		fmt.Fprintf(w, "Version: %s\n\n", Version)
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

		doc, err := termmd.RenderMarkdown(GetUsageDoc())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error rendering documentation: %v\n", err)
			return
		}

		fmt.Fprintf(os.Stderr, "\nDetailed Documentation:\n\n%s\n", doc)
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

	flag.Parse()

	if opts.listProfiles {
		if err := listAvailableProfiles(opts.verbose); err != nil {
			log.Fatal(err)
		}
		return
	}

	ctx := context.Background()
	pm, err := chromeprofiles.NewProfileManager(
		chromeprofiles.WithVerbose(opts.verbose),
	)
	if err != nil {
		log.Fatal(err)
	}

	if err := run(ctx, pm, opts); err != nil {
		log.Fatal(err)
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
		chromedp.UserDataDir(r.pm.WorkDir()),
	}

	if opts.headless {
		copts = append(copts, chromedp.Headless)
	}

	// Create Chrome instance
	allocCtx, cancel := chromedp.NewExecAllocator(ctx, copts...)
	defer cancel()

	taskCtx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

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
		if err := chromedp.Run(taskCtx, chromedp.Navigate(opts.startURL)); err != nil {
			return errors.Wrap(err, "navigating to URL")
		}
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
