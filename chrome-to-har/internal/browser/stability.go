package browser

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/pkg/errors"
)

// StabilityDetector monitors various page loading signals to determine when a page is stable
type StabilityDetector struct {
	page     *Page
	config   *StabilityConfig
	metrics  *StabilityMetrics
	mu       sync.RWMutex
	started  bool
	stopChan chan struct{}
}

// StabilityConfig configures stability detection behavior
type StabilityConfig struct {
	// Network idle configuration
	NetworkIdleThreshold    int           // Number of concurrent network requests to consider idle (default: 0)
	NetworkIdleTimeout      time.Duration // Time to wait with network at idle threshold (default: 500ms)
	NetworkIdleWatchWindow  time.Duration // Time window to monitor network activity (default: 5s)
	
	// DOM stability configuration
	DOMStableThreshold      int           // Number of DOM mutations to consider stable (default: 0)
	DOMStableTimeout        time.Duration // Time to wait with DOM at stable threshold (default: 500ms)
	DOMWatchWindow          time.Duration // Time window to monitor DOM mutations (default: 3s)
	
	// Resource loading configuration
	WaitForImages           bool          // Wait for all images to load (default: true)
	WaitForFonts            bool          // Wait for all fonts to load (default: true)
	WaitForStylesheets      bool          // Wait for all stylesheets to load (default: true)
	WaitForScripts          bool          // Wait for all scripts to load (default: true)
	ResourceTimeout         time.Duration // Max time to wait for resources (default: 10s)
	
	// JavaScript execution configuration
	WaitForAnimationFrame   bool          // Wait for animation frame to complete (default: true)
	WaitForIdleCallback     bool          // Wait for request idle callback (default: true)
	JSExecutionTimeout      time.Duration // Max time to wait for JS execution (default: 5s)
	
	// Overall stability configuration
	MaxStabilityWait        time.Duration // Maximum time to wait for stability (default: 30s)
	RetryAttempts           int           // Number of retry attempts if stability check fails (default: 3)
	RetryDelay              time.Duration // Delay between retry attempts (default: 1s)
	
	// Custom stability checks
	CustomChecks            []StabilityCheck // Custom JavaScript stability checks
	
	// Logging
	Verbose                 bool          // Enable verbose logging
}

// StabilityCheck represents a custom JavaScript stability check
type StabilityCheck struct {
	Name       string
	Expression string // JavaScript expression that should return true when stable
	Timeout    time.Duration
}

// StabilityMetrics tracks stability detection metrics
type StabilityMetrics struct {
	NetworkRequests      int32
	PendingRequests      map[network.RequestID]time.Time
	DOMModifications     int32
	LastDOMModification  time.Time
	LoadedResources      map[string]bool
	StabilityChecks      map[string]bool
	mu                   sync.RWMutex
}

// DefaultStabilityConfig returns a default stability configuration
func DefaultStabilityConfig() *StabilityConfig {
	return &StabilityConfig{
		NetworkIdleThreshold:   0,
		NetworkIdleTimeout:     500 * time.Millisecond,
		NetworkIdleWatchWindow: 5 * time.Second,
		
		DOMStableThreshold:     0,
		DOMStableTimeout:       500 * time.Millisecond,
		DOMWatchWindow:         3 * time.Second,
		
		WaitForImages:          true,
		WaitForFonts:           true,
		WaitForStylesheets:     true,
		WaitForScripts:         true,
		ResourceTimeout:        10 * time.Second,
		
		WaitForAnimationFrame:  true,
		WaitForIdleCallback:    true,
		JSExecutionTimeout:     5 * time.Second,
		
		MaxStabilityWait:       30 * time.Second,
		RetryAttempts:          3,
		RetryDelay:             1 * time.Second,
		
		CustomChecks:           []StabilityCheck{},
		Verbose:                false,
	}
}

// NewStabilityDetector creates a new stability detector
func NewStabilityDetector(page *Page, config *StabilityConfig) *StabilityDetector {
	if config == nil {
		config = DefaultStabilityConfig()
	}
	
	return &StabilityDetector{
		page:   page,
		config: config,
		metrics: &StabilityMetrics{
			PendingRequests: make(map[network.RequestID]time.Time),
			LoadedResources: make(map[string]bool),
			StabilityChecks: make(map[string]bool),
		},
		stopChan: make(chan struct{}),
	}
}

// Start begins monitoring page stability
func (sd *StabilityDetector) Start() error {
	sd.mu.Lock()
	defer sd.mu.Unlock()
	
	if sd.started {
		return nil
	}
	
	// Enable necessary Chrome DevTools domains
	if err := chromedp.Run(sd.page.ctx,
		network.Enable(),
		dom.Enable(),
		page.Enable(),
		runtime.Enable(),
	); err != nil {
		return errors.Wrap(err, "enabling DevTools domains for stability detection")
	}
	
	// Set up event listeners
	chromedp.ListenTarget(sd.page.ctx, sd.handleEvent)
	
	// Inject DOM mutation observer
	if err := sd.injectDOMMutationObserver(); err != nil {
		return errors.Wrap(err, "injecting DOM mutation observer")
	}
	
	sd.started = true
	return nil
}

// Stop stops monitoring page stability
func (sd *StabilityDetector) Stop() {
	sd.mu.Lock()
	defer sd.mu.Unlock()
	
	if sd.started {
		close(sd.stopChan)
		sd.started = false
	}
}

// WaitForStability waits for the page to reach a stable state
func (sd *StabilityDetector) WaitForStability(ctx context.Context) error {
	if !sd.started {
		if err := sd.Start(); err != nil {
			return err
		}
	}
	
	// Create a timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, sd.config.MaxStabilityWait)
	defer cancel()
	
	// Try stability check with retries
	for attempt := 0; attempt <= sd.config.RetryAttempts; attempt++ {
		if attempt > 0 {
			if sd.config.Verbose {
				log.Printf("Stability check attempt %d/%d", attempt, sd.config.RetryAttempts)
			}
			time.Sleep(sd.config.RetryDelay)
		}
		
		err := sd.checkStability(timeoutCtx)
		if err == nil {
			if sd.config.Verbose {
				log.Println("Page reached stable state")
			}
			return nil
		}
		
		if err == context.DeadlineExceeded {
			continue // Retry on timeout
		}
		
		return err // Return on other errors
	}
	
	return errors.New("failed to detect page stability after all retry attempts")
}

// checkStability performs all stability checks
func (sd *StabilityDetector) checkStability(ctx context.Context) error {
	checks := []struct {
		name  string
		check func(context.Context) error
	}{
		{"network idle", sd.waitForNetworkIdle},
		{"DOM stability", sd.waitForDOMStability},
		{"resource loading", sd.waitForResourceLoading},
		{"JavaScript execution", sd.waitForJSExecution},
		{"custom checks", sd.runCustomChecks},
	}
	
	// Run all checks in parallel
	var wg sync.WaitGroup
	errChan := make(chan error, len(checks))
	
	for _, c := range checks {
		check := c
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			if sd.config.Verbose {
				log.Printf("Starting stability check: %s", check.name)
			}
			
			if err := check.check(ctx); err != nil {
				if sd.config.Verbose {
					log.Printf("Stability check failed: %s - %v", check.name, err)
				}
				errChan <- errors.Wrapf(err, "%s check failed", check.name)
			} else if sd.config.Verbose {
				log.Printf("Stability check passed: %s", check.name)
			}
		}()
	}
	
	// Wait for all checks to complete
	go func() {
		wg.Wait()
		close(errChan)
	}()
	
	// Collect any errors
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}
	
	if len(errs) > 0 {
		return errors.Errorf("stability checks failed: %v", errs)
	}
	
	return nil
}

// waitForNetworkIdle waits for network activity to settle
func (sd *StabilityDetector) waitForNetworkIdle(ctx context.Context) error {
	watchCtx, cancel := context.WithTimeout(ctx, sd.config.NetworkIdleWatchWindow)
	defer cancel()
	
	idleStart := time.Time{}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-watchCtx.Done():
			return errors.New("network idle timeout")
		case <-ticker.C:
			sd.metrics.mu.RLock()
			pendingCount := len(sd.metrics.PendingRequests)
			sd.metrics.mu.RUnlock()
			
			if pendingCount <= sd.config.NetworkIdleThreshold {
				if idleStart.IsZero() {
					idleStart = time.Now()
					if sd.config.Verbose {
						log.Printf("Network idle detected, waiting %v for stability", sd.config.NetworkIdleTimeout)
					}
				} else if time.Since(idleStart) >= sd.config.NetworkIdleTimeout {
					return nil // Network has been idle long enough
				}
			} else {
				idleStart = time.Time{} // Reset idle timer
				if sd.config.Verbose {
					log.Printf("Network active: %d pending requests", pendingCount)
				}
			}
		}
	}
}

// waitForDOMStability waits for DOM modifications to settle
func (sd *StabilityDetector) waitForDOMStability(ctx context.Context) error {
	watchCtx, cancel := context.WithTimeout(ctx, sd.config.DOMWatchWindow)
	defer cancel()
	
	// Reset DOM modification counter
	atomic.StoreInt32(&sd.metrics.DOMModifications, 0)
	
	// Re-inject mutation observer to ensure it's active
	if err := sd.injectDOMMutationObserver(); err != nil {
		return err
	}
	
	stableStart := time.Time{}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-watchCtx.Done():
			return errors.New("DOM stability timeout")
		case <-ticker.C:
			modifications := atomic.LoadInt32(&sd.metrics.DOMModifications)
			
			sd.metrics.mu.RLock()
			lastMod := sd.metrics.LastDOMModification
			sd.metrics.mu.RUnlock()
			
			timeSinceLastMod := time.Since(lastMod)
			
			if modifications <= int32(sd.config.DOMStableThreshold) || timeSinceLastMod >= sd.config.DOMStableTimeout {
				if stableStart.IsZero() {
					stableStart = time.Now()
					if sd.config.Verbose {
						log.Printf("DOM stable detected, waiting %v for confirmation", sd.config.DOMStableTimeout)
					}
				} else if time.Since(stableStart) >= sd.config.DOMStableTimeout {
					return nil // DOM has been stable long enough
				}
			} else {
				stableStart = time.Time{} // Reset stable timer
				atomic.StoreInt32(&sd.metrics.DOMModifications, 0) // Reset counter
				if sd.config.Verbose {
					log.Printf("DOM active: %d modifications in last %v", modifications, timeSinceLastMod)
				}
			}
		}
	}
}

// waitForResourceLoading waits for all resources to load
func (sd *StabilityDetector) waitForResourceLoading(ctx context.Context) error {
	resourceCtx, cancel := context.WithTimeout(ctx, sd.config.ResourceTimeout)
	defer cancel()
	
	checks := []struct {
		enabled  bool
		name     string
		script   string
	}{
		{
			sd.config.WaitForImages,
			"images",
			`Array.from(document.images).every(img => img.complete && img.naturalHeight !== 0)`,
		},
		{
			sd.config.WaitForStylesheets,
			"stylesheets",
			`Array.from(document.styleSheets).every(sheet => {
				try { return sheet.cssRules !== null; } catch(e) { return true; }
			})`,
		},
		{
			sd.config.WaitForFonts,
			"fonts",
			`document.fonts ? document.fonts.ready.then(() => true) : true`,
		},
		{
			sd.config.WaitForScripts,
			"scripts",
			`Array.from(document.scripts).every(script => !script.src || script.readyState === 'complete' || !script.readyState)`,
		},
	}
	
	for _, check := range checks {
		if !check.enabled {
			continue
		}
		
		if sd.config.Verbose {
			log.Printf("Waiting for %s to load", check.name)
		}
		
		err := sd.page.WaitForFunction(check.script, sd.config.ResourceTimeout)
		if err != nil {
			if resourceCtx.Err() != nil {
				return errors.Wrapf(resourceCtx.Err(), "timeout waiting for %s", check.name)
			}
			return errors.Wrapf(err, "waiting for %s", check.name)
		}
		
		if sd.config.Verbose {
			log.Printf("All %s loaded", check.name)
		}
	}
	
	return nil
}

// waitForJSExecution waits for JavaScript execution to complete
func (sd *StabilityDetector) waitForJSExecution(ctx context.Context) error {
	jsCtx, cancel := context.WithTimeout(ctx, sd.config.JSExecutionTimeout)
	defer cancel()
	
	if sd.config.WaitForAnimationFrame {
		if sd.config.Verbose {
			log.Println("Waiting for animation frame")
		}
		
		// Wait for next animation frame
		var frameComplete bool
		err := chromedp.Run(jsCtx,
			chromedp.Evaluate(`new Promise(resolve => requestAnimationFrame(() => resolve(true)))`, &frameComplete),
		)
		if err != nil {
			return errors.Wrap(err, "waiting for animation frame")
		}
	}
	
	if sd.config.WaitForIdleCallback {
		if sd.config.Verbose {
			log.Println("Waiting for idle callback")
		}
		
		// Wait for browser idle time
		var idleComplete bool
		err := chromedp.Run(jsCtx,
			chromedp.Evaluate(`new Promise(resolve => {
				if ('requestIdleCallback' in window) {
					requestIdleCallback(() => resolve(true), { timeout: 1000 });
				} else {
					setTimeout(() => resolve(true), 0);
				}
			})`, &idleComplete),
		)
		if err != nil {
			return errors.Wrap(err, "waiting for idle callback")
		}
	}
	
	return nil
}

// runCustomChecks runs any custom stability checks
func (sd *StabilityDetector) runCustomChecks(ctx context.Context) error {
	for _, check := range sd.config.CustomChecks {
		checkCtx, cancel := context.WithTimeout(ctx, check.Timeout)
		defer cancel()
		
		if sd.config.Verbose {
			log.Printf("Running custom stability check: %s", check.Name)
		}
		
		err := sd.page.WaitForFunction(check.Expression, check.Timeout)
		if err != nil {
			if checkCtx.Err() != nil {
				return errors.Wrapf(checkCtx.Err(), "custom check '%s' timeout", check.Name)
			}
			return errors.Wrapf(err, "custom check '%s' failed", check.Name)
		}
		
		sd.metrics.mu.Lock()
		sd.metrics.StabilityChecks[check.Name] = true
		sd.metrics.mu.Unlock()
	}
	
	return nil
}

// handleEvent processes Chrome DevTools events for stability detection
func (sd *StabilityDetector) handleEvent(ev interface{}) {
	switch ev := ev.(type) {
	case *network.EventRequestWillBeSent:
		sd.handleRequestStart(ev.RequestID)
	case *network.EventResponseReceived:
		sd.handleRequestEnd(ev.RequestID)
	case *network.EventLoadingFailed:
		sd.handleRequestEnd(ev.RequestID)
	case *network.EventLoadingFinished:
		sd.handleRequestEnd(ev.RequestID)
	case *runtime.EventConsoleAPICalled:
		// Handle DOM mutation events from our injected observer
		if ev.Type == runtime.APITypeLog {
			for _, arg := range ev.Args {
				if arg.Value != nil && len(arg.Value) > 0 {
					// Convert to string to check for our mutation marker
					str := string(arg.Value)
					if str == `"__DOM_MUTATION__"` {
						sd.handleDOMMutation()
					}
				}
			}
		}
	}
}

// handleRequestStart records a new network request
func (sd *StabilityDetector) handleRequestStart(requestID network.RequestID) {
	sd.metrics.mu.Lock()
	defer sd.metrics.mu.Unlock()
	
	sd.metrics.PendingRequests[requestID] = time.Now()
	atomic.AddInt32(&sd.metrics.NetworkRequests, 1)
	
	if sd.config.Verbose {
		log.Printf("Network request started: %s (total pending: %d)", requestID, len(sd.metrics.PendingRequests))
	}
}

// handleRequestEnd records the completion of a network request
func (sd *StabilityDetector) handleRequestEnd(requestID network.RequestID) {
	sd.metrics.mu.Lock()
	defer sd.metrics.mu.Unlock()
	
	if _, exists := sd.metrics.PendingRequests[requestID]; exists {
		delete(sd.metrics.PendingRequests, requestID)
		
		if sd.config.Verbose {
			log.Printf("Network request finished: %s (remaining: %d)", requestID, len(sd.metrics.PendingRequests))
		}
	}
}

// handleDOMMutation records a DOM modification
func (sd *StabilityDetector) handleDOMMutation() {
	atomic.AddInt32(&sd.metrics.DOMModifications, 1)
	
	sd.metrics.mu.Lock()
	sd.metrics.LastDOMModification = time.Now()
	sd.metrics.mu.Unlock()
	
	if sd.config.Verbose {
		count := atomic.LoadInt32(&sd.metrics.DOMModifications)
		log.Printf("DOM mutation detected (total: %d)", count)
	}
}

// injectDOMMutationObserver injects a MutationObserver into the page
func (sd *StabilityDetector) injectDOMMutationObserver() error {
	script := `
		if (!window.__stabilityMutationObserver) {
			window.__domMutationCount = 0;
			window.__stabilityMutationObserver = new MutationObserver((mutations) => {
				window.__domMutationCount += mutations.length;
				console.log('__DOM_MUTATION__');
			});
			
			window.__stabilityMutationObserver.observe(document.documentElement, {
				childList: true,
				subtree: true,
				attributes: true,
				characterData: true
			});
		}
	`
	
	return chromedp.Run(sd.page.ctx, chromedp.Evaluate(script, nil))
}

// GetMetrics returns current stability metrics
func (sd *StabilityDetector) GetMetrics() StabilityMetrics {
	sd.metrics.mu.RLock()
	defer sd.metrics.mu.RUnlock()
	
	// Create a copy of the metrics
	metrics := StabilityMetrics{
		NetworkRequests:     atomic.LoadInt32(&sd.metrics.NetworkRequests),
		DOMModifications:    atomic.LoadInt32(&sd.metrics.DOMModifications),
		LastDOMModification: sd.metrics.LastDOMModification,
		PendingRequests:     make(map[network.RequestID]time.Time),
		LoadedResources:     make(map[string]bool),
		StabilityChecks:     make(map[string]bool),
	}
	
	for k, v := range sd.metrics.PendingRequests {
		metrics.PendingRequests[k] = v
	}
	
	for k, v := range sd.metrics.LoadedResources {
		metrics.LoadedResources[k] = v
	}
	
	for k, v := range sd.metrics.StabilityChecks {
		metrics.StabilityChecks[k] = v
	}
	
	return metrics
}

// StabilityOption is a function that modifies StabilityConfig
type StabilityOption func(*StabilityConfig)

// WithNetworkIdleThreshold sets the network idle threshold
func WithNetworkIdleThreshold(threshold int) StabilityOption {
	return func(c *StabilityConfig) {
		c.NetworkIdleThreshold = threshold
	}
}

// WithNetworkIdleTimeout sets the network idle timeout
func WithNetworkIdleTimeout(timeout time.Duration) StabilityOption {
	return func(c *StabilityConfig) {
		c.NetworkIdleTimeout = timeout
	}
}

// WithDOMStableTimeout sets the DOM stable timeout
func WithDOMStableTimeout(timeout time.Duration) StabilityOption {
	return func(c *StabilityConfig) {
		c.DOMStableTimeout = timeout
	}
}

// WithResourceWaiting configures which resources to wait for
func WithResourceWaiting(images, fonts, stylesheets, scripts bool) StabilityOption {
	return func(c *StabilityConfig) {
		c.WaitForImages = images
		c.WaitForFonts = fonts
		c.WaitForStylesheets = stylesheets
		c.WaitForScripts = scripts
	}
}

// WithMaxStabilityWait sets the maximum time to wait for stability
func WithMaxStabilityWait(timeout time.Duration) StabilityOption {
	return func(c *StabilityConfig) {
		c.MaxStabilityWait = timeout
	}
}

// WithCustomCheck adds a custom stability check
func WithCustomCheck(name, expression string, timeout time.Duration) StabilityOption {
	return func(c *StabilityConfig) {
		c.CustomChecks = append(c.CustomChecks, StabilityCheck{
			Name:       name,
			Expression: expression,
			Timeout:    timeout,
		})
	}
}

// WithVerboseLogging enables verbose logging
func WithVerboseLogging(verbose bool) StabilityOption {
	return func(c *StabilityConfig) {
		c.Verbose = verbose
	}
}