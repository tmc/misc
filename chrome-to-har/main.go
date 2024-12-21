package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/pkg/errors"
	"github.com/tmc/misc/chrome-to-har/internal/chromeprofiles"
	"github.com/tmc/misc/chrome-to-har/internal/recorder"

	_ "embed"
)

//go:embed doc.go
var usage string

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

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s\n", usage)
		flag.PrintDefaults()
	}
	flag.Parse()

	if opts.listProfiles {
		if err := listAvailableProfiles(opts.verbose); err != nil {
			log.Fatal(err)
		}
		return
	}

	if err := run(opts); err != nil {
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

func run(opts options) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	done := make(chan struct{})

	go func() {
		select {
		case <-sigChan:
			log.Println("Received interrupt signal, shutting down...")
			cancel()
		case <-ctx.Done():
		}
		close(done)
	}()

	pm, err := chromeprofiles.NewProfileManager(
		chromeprofiles.WithVerbose(opts.verbose),
	)
	if err != nil {
		return err
	}
	if err := pm.SetupWorkdir(); err != nil {
		return err
	}
	defer pm.Cleanup()

	if opts.profileDir != "" {
		var cookieDomains []string
		if opts.cookieDomains != "" {
			cookieDomains = strings.Split(opts.cookieDomains, ",")
			for i := range cookieDomains {
				cookieDomains[i] = strings.TrimSpace(cookieDomains[i])
			}
		}

		if err := pm.CopyProfile(opts.profileDir, cookieDomains); err != nil {
			return errors.Wrap(err, "copying profile")
		}
	}

	options := []chromedp.ExecAllocatorOption{
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Flag("enable-automation", false),
		chromedp.Flag("restore-last-session", opts.restoreSession),
		chromedp.Flag("suppress-message-center-popups", !opts.restoreSession),
		chromedp.Flag("disable-session-crashed-bubble", !opts.restoreSession),
		chromedp.UserDataDir(pm.WorkDir()),
	}

	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, options...)
	defer allocCancel()

	chromeCtx, chromeCancel := chromedp.NewContext(allocCtx,
		chromedp.WithLogf(func(format string, args ...interface{}) {
			if opts.verbose {
				log.Printf(format, args...)
			}
		}),
	)
	defer chromeCancel()

	if err := chromedp.Run(chromeCtx); err != nil {
		return errors.Wrap(err, "starting chrome")
	}

	rec, err := recorder.New(
		recorder.WithVerbose(opts.verbose),
		recorder.WithCookiePattern(opts.cookiePattern),
		recorder.WithURLPattern(opts.urlPattern),
		recorder.WithBlockPattern(opts.blockPattern),
		recorder.WithOmitPattern(opts.omitPattern),
	)
	if err != nil {
		return err
	}

	if err := chromedp.Run(chromeCtx, network.Enable()); err != nil {
		return errors.Wrap(err, "enabling network monitoring")
	}

	if err := chromedp.Run(chromeCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		cookies, err := network.GetCookies().Do(ctx)
		if err != nil {
			return err
		}
		rec.SetCookies(cookies)
		return nil
	})); err != nil {
		return errors.Wrap(err, "getting cookies")
	}

	chromedp.ListenTarget(chromeCtx, rec.HandleNetworkEvent(chromeCtx))

	if opts.startURL != "" {
		log.Printf("Navigating to start URL: %s", opts.startURL)
		if err := chromedp.Run(chromeCtx, chromedp.Navigate(opts.startURL)); err != nil {
			return errors.Wrap(err, "navigating to start URL")
		}
	}

	ctrlDChan := make(chan struct{})
	go func() {
		reader := bufio.NewReader(os.Stdin)
		for {
			char, err := reader.ReadByte()
			if err != nil {
				if err != io.EOF {
					log.Printf("Error reading input: %v", err)
				}
				close(ctrlDChan)
				return
			}
			if char == 4 { // Ctrl+D
				log.Printf("Received Ctrl+D, writing HAR file to %s...", opts.outputFile)
				if err := rec.WriteHAR(opts.outputFile); err != nil {
					log.Printf("Error writing HAR: %v", err)
				} else {
					log.Printf("Successfully wrote HAR file to: %s", opts.outputFile)
				}
				close(ctrlDChan)
				return
			}
		}
	}()

	fmt.Printf("Chrome started. Press Ctrl+D to capture HAR to %s (Ctrl+C to exit without saving)\n", opts.outputFile)

	select {
	case <-chromeCtx.Done():
		return chromeCtx.Err()
	case <-ctrlDChan:
		cancel()
		return nil
	case <-done:
		return nil
	}
}
