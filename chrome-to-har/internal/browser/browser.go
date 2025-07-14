// Package browser provides abstractions for managing Chrome browser instances.
package browser

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/pkg/errors"
	"github.com/tmc/misc/chrome-to-har/internal/chromeprofiles"
)

// Browser represents a managed Chrome browser instance
type Browser struct {
	ctx        context.Context
	cancelFunc context.CancelFunc
	opts       *Options
	profileMgr chromeprofiles.ProfileManager
}

// New creates a new Browser with the provided options
func New(ctx context.Context, profileMgr chromeprofiles.ProfileManager, opts ...Option) (*Browser, error) {
	// Create default options
	options := defaultOptions()

	// Apply all option functions
	for _, opt := range opts {
		if err := opt(options); err != nil {
			return nil, errors.Wrap(err, "applying browser option")
		}
	}

	browser := &Browser{
		opts:       options,
		profileMgr: profileMgr,
	}

	return browser, nil
}

// Launch starts the browser or connects to a running instance
func (b *Browser) Launch(ctx context.Context) error {
	// If using remote Chrome, connect to it instead of launching
	if b.opts.UseRemote {
		if b.opts.RemoteHost == "" {
			b.opts.RemoteHost = "localhost"
		}
		
		if b.opts.RemoteTabID != "" {
			// Connect to specific tab
			return b.ConnectToTab(ctx, b.opts.RemoteHost, b.opts.RemotePort, b.opts.RemoteTabID)
		} else {
			// Connect to browser instance
			return b.ConnectToRunningChrome(ctx, b.opts.RemoteHost, b.opts.RemotePort)
		}
	}

	// Set up the profile directory if needed
	if b.opts.UseProfile && b.profileMgr != nil {
		if err := b.profileMgr.SetupWorkdir(); err != nil {
			return errors.Wrap(err, "setting up working directory")
		}

		if err := b.profileMgr.CopyProfile(b.opts.ProfileName, b.opts.CookieDomains); err != nil {
			return errors.Wrap(err, "copying profile")
		}
	}

	// Set up the Chrome options
	chromeLaunchOpts := []chromedp.ExecAllocatorOption{
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.DisableGPU,
		// Stability flags
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
		chromedp.WSURLReadTimeout(180 * time.Second),
	}

	// If headless mode is enabled
	if b.opts.Headless {
		chromeLaunchOpts = append(chromeLaunchOpts, chromedp.Headless)
	}

	// If a profile is being used
	if b.opts.UseProfile && b.profileMgr != nil {
		chromeLaunchOpts = append(chromeLaunchOpts, chromedp.UserDataDir(b.profileMgr.WorkDir()))
	}

	// Add Chrome path if specified
	if b.opts.ChromePath != "" {
		chromeLaunchOpts = append(chromeLaunchOpts, chromedp.ExecPath(b.opts.ChromePath))
	}

	// Add remote debugging port if specified
	if b.opts.DebugPort > 0 {
		portStr := fmt.Sprintf("%d", b.opts.DebugPort)
		chromeLaunchOpts = append(chromeLaunchOpts, chromedp.Flag("remote-debugging-port", portStr))
	}

	// Enable logging if verbose
	if b.opts.Verbose {
		chromeLaunchOpts = append(chromeLaunchOpts, chromedp.CombinedOutput(os.Stdout))
	}

	// Add custom Chrome flags
	for _, flag := range b.opts.ChromeFlags {
		chromeLaunchOpts = append(chromeLaunchOpts, chromedp.Flag(flag, true))
	}

	// Create the allocator context
	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, chromeLaunchOpts...)
	allocCtx, allocCancel = context.WithTimeout(allocCtx, time.Duration(b.opts.Timeout)*time.Second)

	// Create the browser context
	var browserCtx context.Context
	var browserCancel context.CancelFunc

	if b.opts.Verbose {
		browserCtx, browserCancel = chromedp.NewContext(
			allocCtx,
			chromedp.WithLogf(log.Printf),
		)
	} else {
		browserCtx, browserCancel = chromedp.NewContext(allocCtx)
	}

	// Store context and cancel functions
	b.ctx = browserCtx
	b.cancelFunc = func() {
		browserCancel()
		allocCancel()
	}

	// Test connection with navigation to about:blank to ensure browser launches properly
	if b.opts.Verbose {
		log.Println("Testing Chrome connection with about:blank...")
	}

	testCtx, testCancel := context.WithTimeout(browserCtx, 30*time.Second)
	defer testCancel()

	if err := chromedp.Run(testCtx, chromedp.Navigate("about:blank")); err != nil {
		b.cancelFunc()
		return errors.Wrap(err, "testing Chrome connection")
	}

	if b.opts.Verbose {
		log.Printf("Successfully launched Chrome browser")
	}

	return nil
}

// Navigate visits the specified URL
func (b *Browser) Navigate(url string) error {
	if b.ctx == nil {
		return errors.New("browser not launched, call Launch() first")
	}

	if b.opts.Verbose {
		log.Printf("Navigating to: %s", url)
	}

	navCtx, navCancel := context.WithTimeout(b.ctx, time.Duration(b.opts.NavigationTimeout)*time.Second)
	defer navCancel()

	// Enable network events if we need to wait for network idle
	if b.opts.WaitNetworkIdle {
		if err := chromedp.Run(navCtx, network.Enable()); err != nil {
			return errors.Wrap(err, "enabling network events")
		}
	}

	// Navigate to the URL
	if err := chromedp.Run(navCtx, chromedp.Navigate(url)); err != nil {
		return errors.Wrap(err, "navigating to URL")
	}

	// If waiting for network idle is requested
	if b.opts.WaitNetworkIdle {
		waitCtx, waitCancel := context.WithTimeout(b.ctx, time.Duration(b.opts.StableTimeout)*time.Second)
		defer waitCancel()

		if err := chromedp.Run(waitCtx, chromedp.ActionFunc(func(ctx context.Context) error {
			// This will wait until there are no more than 2 network connections for at least 500ms
			ch := make(chan struct{})
			lctx, cancel := context.WithCancel(ctx)
			chromedp.ListenTarget(lctx, func(ev interface{}) {
				switch ev.(type) {
				case *network.EventLoadingFinished, *network.EventLoadingFailed,
					*page.EventLoadEventFired, *page.EventDomContentEventFired:
					select {
					case ch <- struct{}{}:
					default:
					}
				}
			})

			// Wait for idle using a timer
			idleTimer := time.NewTimer(500 * time.Millisecond)
			defer cancel()

			for {
				select {
				case <-waitCtx.Done():
					return waitCtx.Err()
				case <-idleTimer.C:
					// We've been idle for 500ms
					return nil
				case <-ch:
					// Reset the timer when any network event occurs
					if !idleTimer.Stop() {
						<-idleTimer.C
					}
					idleTimer.Reset(500 * time.Millisecond)
				}
			}
		})); err != nil {
			return errors.Wrap(err, "waiting for network idle")
		}
	}

	// If waiting for a specific CSS selector
	if b.opts.WaitSelector != "" {
		waitCtx, waitCancel := context.WithTimeout(b.ctx, time.Duration(b.opts.StableTimeout)*time.Second)
		defer waitCancel()

		if err := chromedp.Run(waitCtx, chromedp.WaitVisible(b.opts.WaitSelector, chromedp.ByQuery)); err != nil {
			return errors.Wrap(err, "waiting for selector: "+b.opts.WaitSelector)
		}
	}

	return nil
}

// GetHTML returns the current page's HTML content
func (b *Browser) GetHTML() (string, error) {
	if b.ctx == nil {
		return "", errors.New("browser not launched, call Launch() first")
	}

	var html string
	if err := chromedp.Run(b.ctx, chromedp.OuterHTML("html", &html)); err != nil {
		return "", errors.Wrap(err, "getting page HTML")
	}

	return html, nil
}

// GetCurrentPage returns a Page wrapper for the current browser context
// This is useful when connected to a remote tab
func (b *Browser) GetCurrentPage() *Page {
	if b.ctx == nil {
		return nil
	}
	
	return &Page{
		ctx:     b.ctx,
		cancel:  func() {}, // Browser owns the context
		browser: b,
	}
}

// Context returns the browser's context
func (b *Browser) Context() context.Context {
	return b.ctx
}

// Close shuts down the browser
func (b *Browser) Close() error {
	if b.cancelFunc != nil {
		b.cancelFunc()
	}

	if b.profileMgr != nil {
		if err := b.profileMgr.Cleanup(); err != nil {
			return errors.Wrap(err, "cleaning up profile")
		}
	}

	return nil
}

// Context returns the browser's context
func (b *Browser) Context() context.Context {
	return b.ctx
}

// WaitForSelector waits for a CSS selector to be visible
func (b *Browser) WaitForSelector(selector string, timeout time.Duration) error {
	if b.ctx == nil {
		return errors.New("browser not launched, call Launch() first")
	}

	waitCtx, waitCancel := context.WithTimeout(b.ctx, timeout)
	defer waitCancel()

	return chromedp.Run(waitCtx, chromedp.WaitVisible(selector, chromedp.ByQuery))
}

// ExecuteScript runs JavaScript in the browser and returns the result
func (b *Browser) ExecuteScript(script string) (interface{}, error) {
	if b.ctx == nil {
		return nil, errors.New("browser not launched, call Launch() first")
	}

	var result interface{}
	err := chromedp.Run(b.ctx, chromedp.Evaluate(script, &result))
	if err != nil {
		return nil, errors.Wrap(err, "executing script")
	}

	return result, nil
}

// GetTitle returns the page title
func (b *Browser) GetTitle() (string, error) {
	if b.ctx == nil {
		return "", errors.New("browser not launched, call Launch() first")
	}

	var title string
	if err := chromedp.Run(b.ctx, chromedp.Title(&title)); err != nil {
		return "", errors.Wrap(err, "getting page title")
	}

	return title, nil
}

// GetURL returns the current page URL
func (b *Browser) GetURL() (string, error) {
	if b.ctx == nil {
		return "", errors.New("browser not launched, call Launch() first")
	}

	var url string
	if err := chromedp.Run(b.ctx, chromedp.Location(&url)); err != nil {
		return "", errors.Wrap(err, "getting page URL")
	}

	return url, nil
}

// SetRequestHeaders sets custom headers for all subsequent requests
func (b *Browser) SetRequestHeaders(headers map[string]string) error {
	if b.ctx == nil {
		return errors.New("browser not launched, call Launch() first")
	}

	// Convert to the format expected by CDP
	cdpHeaders := make(map[string]interface{})
	for k, v := range headers {
		cdpHeaders[k] = v
	}

	return chromedp.Run(b.ctx, network.SetExtraHTTPHeaders(network.Headers(cdpHeaders)))
}

// SetBasicAuth sets basic authentication headers
func (b *Browser) SetBasicAuth(username, password string) error {
	auth := username + ":" + password
	encodedAuth := base64.StdEncoding.EncodeToString([]byte(auth))
	return b.SetRequestHeaders(map[string]string{
		"Authorization": "Basic " + encodedAuth,
	})
}
