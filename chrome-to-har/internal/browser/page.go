package browser

import (
	"context"
	"fmt"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	"github.com/pkg/errors"
)

// Page represents a browser page/tab with high-level interaction methods
type Page struct {
	ctx               context.Context
	cancel            context.CancelFunc
	targetID          target.ID
	browser           *Browser
	networkManager    *NetworkManager
	stabilityDetector *StabilityDetector
}

// NewPage creates a new page in the browser
func (b *Browser) NewPage() (*Page, error) {
	if b.ctx == nil {
		return nil, errors.New("browser not launched")
	}

	// Create new tab
	newCtx, cancel := chromedp.NewContext(b.ctx)

	p := &Page{
		ctx:     newCtx,
		cancel:  cancel,
		browser: b,
	}

	// Navigate to blank page to initialize
	if err := chromedp.Run(p.ctx, chromedp.Navigate("about:blank")); err != nil {
		cancel()
		return nil, errors.Wrap(err, "initializing page")
	}

	return p, nil
}

// AttachToTarget attaches to an existing tab/page by target ID
func (b *Browser) AttachToTarget(targetID string) (*Page, error) {
	if b.ctx == nil {
		return nil, errors.New("browser not launched")
	}

	// Get target info
	targets, err := target.GetTargets().Do(b.ctx)
	if err != nil {
		return nil, errors.Wrap(err, "getting targets")
	}

	var targetInfo *target.Info
	for _, t := range targets {
		if string(t.TargetID) == targetID {
			targetInfo = t
			break
		}
	}

	if targetInfo == nil {
		return nil, fmt.Errorf("target not found: %s", targetID)
	}

	// Create context for the target
	ctx, cancel := chromedp.NewContext(b.ctx, chromedp.WithTargetID(target.ID(targetID)))

	p := &Page{
		ctx:      ctx,
		cancel:   cancel,
		targetID: target.ID(targetID),
		browser:  b,
	}

	return p, nil
}

// Pages returns all pages in the browser
func (b *Browser) Pages() ([]*Page, error) {
	if b.ctx == nil {
		return nil, errors.New("browser not launched")
	}

	targets, err := target.GetTargets().Do(b.ctx)
	if err != nil {
		return nil, errors.Wrap(err, "getting targets")
	}

	var pages []*Page
	for _, t := range targets {
		if t.Type == "page" {
			p, err := b.AttachToTarget(string(t.TargetID))
			if err != nil {
				continue
			}
			pages = append(pages, p)
		}
	}

	return pages, nil
}

// Context returns the page's context
func (p *Page) Context() context.Context {
	return p.ctx
}

// Close closes the page
func (p *Page) Close() error {
	if p.cancel != nil {
		p.cancel()
	}
	return nil
}

// Navigate navigates to a URL
func (p *Page) Navigate(url string, opts ...NavigateOption) error {
	options := &NavigateOptions{
		Timeout:   30 * time.Second,
		WaitUntil: "load",
	}

	for _, opt := range opts {
		opt(options)
	}

	ctx, cancel := context.WithTimeout(p.ctx, options.Timeout)
	defer cancel()

	if err := chromedp.Run(ctx, chromedp.Navigate(url)); err != nil {
		return errors.Wrap(err, "navigating")
	}

	// Wait for load state
	switch options.WaitUntil {
	case "domcontentloaded":
		if err := chromedp.Run(ctx, chromedp.WaitReady("body")); err != nil {
			return errors.Wrap(err, "waiting for DOM")
		}
	case "networkidle":
		// Use the new stability detector for network idle
		if err := p.WaitForLoadState(ctx, LoadStateNetworkIdle); err != nil {
			return errors.Wrap(err, "waiting for network idle")
		}
	case "stable":
		// Wait for full page stability (network, DOM, resources)
		if err := p.WaitForStability(ctx, nil); err != nil {
			return errors.Wrap(err, "waiting for page stability")
		}
	default: // "load"
		// Default chromedp behavior
	}

	return nil
}

// Title returns the page title
func (p *Page) Title() (string, error) {
	var title string
	if err := chromedp.Run(p.ctx, chromedp.Title(&title)); err != nil {
		return "", errors.Wrap(err, "getting title")
	}
	return title, nil
}

// URL returns the current URL
func (p *Page) URL() (string, error) {
	var url string
	if err := chromedp.Run(p.ctx, chromedp.Location(&url)); err != nil {
		return "", errors.Wrap(err, "getting URL")
	}
	return url, nil
}

// Content returns the page HTML
func (p *Page) Content() (string, error) {
	var html string
	if err := chromedp.Run(p.ctx, chromedp.OuterHTML("html", &html)); err != nil {
		return "", errors.Wrap(err, "getting content")
	}
	return html, nil
}

// Click clicks an element
func (p *Page) Click(selector string, opts ...ClickOption) error {
	options := &ClickOptions{
		Button:  "left",
		Count:   1,
		Delay:   0,
		Timeout: 30 * time.Second,
	}

	for _, opt := range opts {
		opt(options)
	}

	ctx, cancel := context.WithTimeout(p.ctx, options.Timeout)
	defer cancel()

	// Wait for element and click
	if err := chromedp.Run(ctx,
		chromedp.WaitVisible(selector),
		chromedp.Click(selector),
	); err != nil {
		return errors.Wrapf(err, "clicking %s", selector)
	}

	return nil
}

// Type types text into an element
func (p *Page) Type(selector string, text string, opts ...TypeOption) error {
	options := &TypeOptions{
		Delay:   0,
		Timeout: 30 * time.Second,
	}

	for _, opt := range opts {
		opt(options)
	}

	ctx, cancel := context.WithTimeout(p.ctx, options.Timeout)
	defer cancel()

	// Clear existing text and type new text
	if err := chromedp.Run(ctx,
		chromedp.WaitVisible(selector),
		chromedp.Clear(selector),
		chromedp.SendKeys(selector, text),
	); err != nil {
		return errors.Wrapf(err, "typing into %s", selector)
	}

	return nil
}

// WaitForSelector waits for a selector to appear
func (p *Page) WaitForSelector(selector string, opts ...WaitOption) error {
	options := &WaitOptions{
		State:   "visible",
		Timeout: 30 * time.Second,
	}

	for _, opt := range opts {
		opt(options)
	}

	ctx, cancel := context.WithTimeout(p.ctx, options.Timeout)
	defer cancel()

	switch options.State {
	case "attached":
		return chromedp.Run(ctx, chromedp.WaitReady(selector))
	case "visible":
		return chromedp.Run(ctx, chromedp.WaitVisible(selector))
	case "hidden":
		return chromedp.Run(ctx, chromedp.WaitNotVisible(selector))
	case "detached":
		return chromedp.Run(ctx, chromedp.WaitNotPresent(selector))
	default:
		return fmt.Errorf("unknown state: %s", options.State)
	}
}

// Evaluate evaluates JavaScript in the page context
func (p *Page) Evaluate(expression string, result interface{}) error {
	if result == nil {
		// No return value expected
		return chromedp.Run(p.ctx, chromedp.Evaluate(expression, nil))
	}
	return chromedp.Run(p.ctx, chromedp.Evaluate(expression, result))
}

// EvaluateHandle evaluates JavaScript and returns a handle
func (p *Page) EvaluateHandle(expression string) (*runtime.RemoteObject, error) {
	var obj *runtime.RemoteObject
	if err := chromedp.Run(p.ctx, chromedp.EvaluateAsDevTools(expression, &obj)); err != nil {
		return nil, errors.Wrap(err, "evaluating handle")
	}
	return obj, nil
}

// Screenshot takes a screenshot
func (p *Page) Screenshot(opts ...ScreenshotOption) ([]byte, error) {
	options := &ScreenshotOptions{
		FullPage: false,
		Quality:  90,
		Type:     "png",
	}

	for _, opt := range opts {
		opt(options)
	}

	var buf []byte
	var action chromedp.Action

	if options.FullPage {
		action = chromedp.FullScreenshot(&buf, int(options.Quality))
	} else if options.Selector != "" {
		action = chromedp.Screenshot(options.Selector, &buf, chromedp.NodeVisible)
	} else {
		action = chromedp.CaptureScreenshot(&buf)
	}

	if err := chromedp.Run(p.ctx, action); err != nil {
		return nil, errors.Wrap(err, "taking screenshot")
	}

	return buf, nil
}

// PDF generates a PDF
func (p *Page) PDF(opts ...PDFOption) ([]byte, error) {
	options := &PDFOptions{
		Format:          "A4",
		Landscape:       false,
		Scale:           1.0,
		PrintBackground: true,
	}

	for _, opt := range opts {
		opt(options)
	}

	var buf []byte
	if err := chromedp.Run(p.ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		params := page.PrintToPDF()
		params = params.WithPrintBackground(options.PrintBackground).
			WithScale(options.Scale).
			WithLandscape(options.Landscape)

		if options.Format != "" {
			params = params.WithPaperWidth(8.5).WithPaperHeight(11) // A4 default
		}

		data, _, err := params.Do(ctx)
		if err != nil {
			return err
		}
		buf = data
		return nil
	})); err != nil {
		return nil, errors.Wrap(err, "generating PDF")
	}

	return buf, nil
}

// GetText gets text content of an element
func (p *Page) GetText(selector string) (string, error) {
	var text string
	if err := chromedp.Run(p.ctx,
		chromedp.WaitVisible(selector),
		chromedp.Text(selector, &text),
	); err != nil {
		return "", errors.Wrapf(err, "getting text from %s", selector)
	}
	return text, nil
}

// GetAttribute gets an attribute value
func (p *Page) GetAttribute(selector, attribute string) (string, error) {
	var value string
	if err := chromedp.Run(p.ctx,
		chromedp.WaitReady(selector),
		chromedp.AttributeValue(selector, attribute, &value, nil),
	); err != nil {
		return "", errors.Wrapf(err, "getting attribute %s from %s", attribute, selector)
	}
	return value, nil
}

// SetViewport sets the viewport size
func (p *Page) SetViewport(width, height int) error {
	return chromedp.Run(p.ctx,
		chromedp.EmulateViewport(int64(width), int64(height)),
	)
}

// Focus focuses an element
func (p *Page) Focus(selector string) error {
	return chromedp.Run(p.ctx,
		chromedp.WaitVisible(selector),
		chromedp.Focus(selector),
	)
}

// Hover hovers over an element
func (p *Page) Hover(selector string) error {
	// Get element position first
	var nodes []*cdp.Node
	if err := chromedp.Run(p.ctx,
		chromedp.WaitVisible(selector),
		chromedp.Nodes(selector, &nodes),
	); err != nil {
		return err
	}

	if len(nodes) == 0 {
		return fmt.Errorf("element not found: %s", selector)
	}

	// Get box model to find element center
	box, err := p.getElementBox(nodes[0].NodeID)
	if err != nil {
		return err
	}

	// Move mouse to element center
	centerX := box.Content[0] + (box.Content[4]-box.Content[0])/2
	centerY := box.Content[1] + (box.Content[5]-box.Content[1])/2

	return chromedp.Run(p.ctx,
		chromedp.MouseEvent(input.MouseMoved, centerX, centerY),
	)
}

// getElementBox gets the box model for an element
func (p *Page) getElementBox(nodeID cdp.NodeID) (*dom.BoxModel, error) {
	var box *dom.BoxModel
	if err := chromedp.Run(p.ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			box, err = dom.GetBoxModel().WithNodeID(nodeID).Do(ctx)
			return err
		}),
	); err != nil {
		return nil, err
	}
	return box, nil
}

// SelectOption selects options in a select element
func (p *Page) SelectOption(selector string, values ...string) error {
	return chromedp.Run(p.ctx,
		chromedp.WaitVisible(selector),
		chromedp.SetAttributeValue(selector, "value", values[0]),
		chromedp.Evaluate(fmt.Sprintf(
			`document.querySelector('%s').dispatchEvent(new Event('change', {bubbles: true}))`,
			selector,
		), nil),
	)
}

// Press presses a key
func (p *Page) Press(key string) error {
	return chromedp.Run(p.ctx,
		chromedp.KeyEvent(key),
	)
}

// ElementExists checks if an element exists
func (p *Page) ElementExists(selector string) (bool, error) {
	var nodes []*cdp.Node
	if err := chromedp.Run(p.ctx,
		chromedp.Nodes(selector, &nodes),
	); err != nil {
		return false, nil // Element doesn't exist
	}
	return len(nodes) > 0, nil
}

// ElementVisible checks if an element is visible
func (p *Page) ElementVisible(selector string) (bool, error) {
	var visible bool
	if err := chromedp.Run(p.ctx,
		chromedp.Evaluate(fmt.Sprintf(`
			(() => {
				const el = document.querySelector('%s');
				if (!el) return false;
				const style = window.getComputedStyle(el);
				return style.display !== 'none' && style.visibility !== 'hidden' && style.opacity !== '0';
			})()
		`, selector), &visible),
	); err != nil {
		return false, errors.Wrap(err, "checking visibility")
	}
	return visible, nil
}

// WaitForFunction waits for a JavaScript function to return true
func (p *Page) WaitForFunction(expression string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(p.ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return errors.New("timeout waiting for function")
		case <-ticker.C:
			var result bool
			if err := chromedp.Run(ctx, chromedp.Evaluate(expression, &result)); err == nil && result {
				return nil
			}
		}
	}
}

// LoadState represents different page load states
type LoadState string

const (
	LoadStateLoad             LoadState = "load"
	LoadStateDOMContentLoaded LoadState = "domcontentloaded"
	LoadStateNetworkIdle      LoadState = "networkidle"
	LoadStateNetworkIdle0     LoadState = "networkidle0" // No network requests for 500ms
	LoadStateNetworkIdle2     LoadState = "networkidle2" // At most 2 network requests for 500ms
)

// WaitForLoadState waits for a specific load state
func (p *Page) WaitForLoadState(ctx context.Context, state LoadState) error {
	switch state {
	case LoadStateDOMContentLoaded:
		return chromedp.Run(ctx, chromedp.WaitReady("body"))
	
	case LoadStateNetworkIdle, LoadStateNetworkIdle0:
		config := DefaultStabilityConfig()
		config.NetworkIdleThreshold = 0
		config.DOMStableThreshold = -1 // Disable DOM check for network idle
		config.WaitForImages = false
		config.WaitForFonts = false
		config.WaitForStylesheets = false
		config.WaitForScripts = false
		config.WaitForAnimationFrame = false
		config.WaitForIdleCallback = false
		
		detector := NewStabilityDetector(p, config)
		return detector.WaitForStability(ctx)
	
	case LoadStateNetworkIdle2:
		config := DefaultStabilityConfig()
		config.NetworkIdleThreshold = 2
		config.DOMStableThreshold = -1 // Disable DOM check for network idle
		config.WaitForImages = false
		config.WaitForFonts = false
		config.WaitForStylesheets = false
		config.WaitForScripts = false
		config.WaitForAnimationFrame = false
		config.WaitForIdleCallback = false
		
		detector := NewStabilityDetector(p, config)
		return detector.WaitForStability(ctx)
	
	default: // LoadStateLoad
		// Default chromedp behavior - waits for load event
		return nil
	}
}

// WaitForStability waits for the page to reach a stable state using custom configuration
func (p *Page) WaitForStability(ctx context.Context, config *StabilityConfig) error {
	if p.stabilityDetector == nil {
		p.stabilityDetector = NewStabilityDetector(p, config)
	}
	
	return p.stabilityDetector.WaitForStability(ctx)
}

// ConfigureStability configures stability detection options
func (p *Page) ConfigureStability(opts ...StabilityOption) {
	config := DefaultStabilityConfig()
	for _, opt := range opts {
		opt(config)
	}
	
	p.stabilityDetector = NewStabilityDetector(p, config)
}

// GetStabilityMetrics returns current stability metrics
func (p *Page) GetStabilityMetrics() *StabilityMetrics {
	if p.stabilityDetector == nil {
		return nil
	}
	
	metrics := p.stabilityDetector.GetMetrics()
	return &metrics
}
