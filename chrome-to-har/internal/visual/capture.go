package visual

import (
	"context"
	"fmt"
	"image"
	"image/png"
	"log"
	"strings"
	"time"

	"github.com/chromedp/cdproto/emulation"
	cdppage "github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/pkg/errors"
	"github.com/tmc/misc/chrome-to-har/internal/browser"
)

// ScreenshotCapture handles advanced screenshot capture functionality
type ScreenshotCapture struct {
	verbose bool
}

// NewScreenshotCapture creates a new screenshot capture instance
func NewScreenshotCapture(verbose bool) *ScreenshotCapture {
	return &ScreenshotCapture{
		verbose: verbose,
	}
}

// CaptureScreenshot captures a screenshot with advanced options
func (sc *ScreenshotCapture) CaptureScreenshot(ctx context.Context, page *browser.Page, opts *ScreenshotOptions) ([]byte, error) {
	if opts == nil {
		opts = DefaultScreenshotOptions()
	}

	if sc.verbose {
		log.Printf("Capturing screenshot with options: fullPage=%v, selector=%s, viewport=%dx%d", 
			opts.FullPage, opts.Selector, opts.ViewportWidth, opts.ViewportHeight)
	}

	// Set up viewport if specified
	if opts.ViewportWidth > 0 && opts.ViewportHeight > 0 {
		if err := sc.setViewport(ctx, page, opts.ViewportWidth, opts.ViewportHeight, opts.DeviceScaleFactor); err != nil {
			return nil, errors.Wrap(err, "setting viewport")
		}
	}

	// Set up device emulation if specified
	if opts.EmulateDevice != "" {
		if err := sc.emulateDevice(ctx, page, opts.EmulateDevice); err != nil {
			return nil, errors.Wrap(err, "emulating device")
		}
	}

	// Inject custom CSS if specified
	if opts.CustomCSS != "" {
		if err := sc.injectCSS(ctx, page, opts.CustomCSS); err != nil {
			return nil, errors.Wrap(err, "injecting CSS")
		}
	}

	// Hide cursor if requested
	if opts.HideCursor {
		if err := sc.hideCursor(ctx, page); err != nil && sc.verbose {
			log.Printf("Warning: failed to hide cursor: %v", err)
		}
	}

	// Hide scrollbars if requested
	if opts.HideScrollbars {
		if err := sc.hideScrollbars(ctx, page); err != nil && sc.verbose {
			log.Printf("Warning: failed to hide scrollbars: %v", err)
		}
	}

	// Wait for resources to load if requested
	if err := sc.waitForResources(ctx, page, opts); err != nil {
		return nil, errors.Wrap(err, "waiting for resources")
	}

	// Mask/exclude elements if specified
	if err := sc.processElementMasking(ctx, page, opts); err != nil {
		return nil, errors.Wrap(err, "processing element masking")
	}

	// Wait for page stability
	if opts.StabilityWaitTime > 0 {
		if err := sc.waitForStability(ctx, page, opts.StabilityWaitTime); err != nil && sc.verbose {
			log.Printf("Warning: stability wait failed: %v", err)
		}
	}

	// Capture the screenshot
	var screenshot []byte
	var err error

	if opts.FullPage {
		screenshot, err = sc.captureFullPage(ctx, page, opts)
	} else if opts.Selector != "" {
		screenshot, err = sc.captureElement(ctx, page, opts.Selector, opts)
	} else {
		screenshot, err = sc.captureViewport(ctx, page, opts)
	}

	if err != nil {
		return nil, errors.Wrap(err, "capturing screenshot")
	}

	if sc.verbose {
		log.Printf("Screenshot captured successfully, size: %d bytes", len(screenshot))
	}

	return screenshot, nil
}

// setViewport sets the viewport size and device scale factor
func (sc *ScreenshotCapture) setViewport(ctx context.Context, page *browser.Page, width, height int, deviceScaleFactor float64) error {
	if deviceScaleFactor == 0 {
		deviceScaleFactor = 1.0
	}

	return chromedp.Run(ctx,
		emulation.SetDeviceMetricsOverride(int64(width), int64(height), deviceScaleFactor, false),
		chromedp.EmulateViewport(int64(width), int64(height)),
	)
}

// emulateDevice emulates a specific device
func (sc *ScreenshotCapture) emulateDevice(ctx context.Context, page *browser.Page, deviceName string) error {
	device, exists := DefaultDeviceEmulations[deviceName]
	if !exists {
		return fmt.Errorf("unknown device: %s", deviceName)
	}

	return chromedp.Run(ctx,
		emulation.SetDeviceMetricsOverride(
			int64(device.ViewportSize.Width),
			int64(device.ViewportSize.Height),
			device.DeviceScaleFactor,
			device.IsMobile,
		),
		emulation.SetTouchEmulationEnabled(device.HasTouch),
		emulation.SetUserAgentOverride(device.UserAgent),
	)
}

// injectCSS injects custom CSS into the page
func (sc *ScreenshotCapture) injectCSS(ctx context.Context, page *browser.Page, css string) error {
	script := fmt.Sprintf(`
		(function() {
			const style = document.createElement('style');
			style.textContent = %s;
			document.head.appendChild(style);
		})();
	`, "`"+css+"`")

	return chromedp.Run(ctx, chromedp.Evaluate(script, nil))
}

// hideCursor hides the mouse cursor
func (sc *ScreenshotCapture) hideCursor(ctx context.Context, page *browser.Page) error {
	hideCSS := `
		* {
			cursor: none !important;
		}
	`
	return sc.injectCSS(ctx, page, hideCSS)
}

// hideScrollbars hides scrollbars
func (sc *ScreenshotCapture) hideScrollbars(ctx context.Context, page *browser.Page) error {
	hideCSS := `
		::-webkit-scrollbar {
			display: none !important;
		}
		html {
			scrollbar-width: none !important;
		}
		body {
			scrollbar-width: none !important;
		}
	`
	return sc.injectCSS(ctx, page, hideCSS)
}

// waitForResources waits for various resources to load
func (sc *ScreenshotCapture) waitForResources(ctx context.Context, page *browser.Page, opts *ScreenshotOptions) error {
	var waitFunctions []string

	if opts.WaitForImages {
		waitFunctions = append(waitFunctions, `
			// Wait for images to load
			await Promise.all(Array.from(document.images).map(img => {
				if (img.complete) return Promise.resolve();
				return new Promise(resolve => {
					img.onload = resolve;
					img.onerror = resolve;
				});
			}));
		`)
	}

	if opts.WaitForFonts {
		waitFunctions = append(waitFunctions, `
			// Wait for fonts to load
			if (document.fonts && document.fonts.ready) {
				await document.fonts.ready;
			}
		`)
	}

	if opts.WaitForAnimations {
		waitFunctions = append(waitFunctions, fmt.Sprintf(`
			// Wait for animations to complete
			await new Promise(resolve => setTimeout(resolve, %d));
		`, int(opts.AnimationWaitTime.Milliseconds())))
	}

	if len(waitFunctions) > 0 {
		script := fmt.Sprintf(`
			(async function() {
				try {
					%s
				} catch (e) {
					console.warn('Error waiting for resources:', e);
				}
			})();
		`, strings.Join(waitFunctions, "\n"))

		return chromedp.Run(ctx, chromedp.Evaluate(script, nil))
	}

	return nil
}

// processElementMasking masks or excludes elements from the screenshot
func (sc *ScreenshotCapture) processElementMasking(ctx context.Context, page *browser.Page, opts *ScreenshotOptions) error {
	var operations []string

	// Exclude elements by setting display: none
	for _, selector := range opts.ExcludeSelectors {
		operations = append(operations, fmt.Sprintf(`
			document.querySelectorAll('%s').forEach(el => {
				el.style.display = 'none';
			});
		`, selector))
	}

	// Mask elements by setting background color
	for _, selector := range opts.MaskSelectors {
		operations = append(operations, fmt.Sprintf(`
			document.querySelectorAll('%s').forEach(el => {
				el.style.backgroundColor = '%s';
				el.style.color = '%s';
			});
		`, selector, opts.MaskColor, opts.MaskColor))
	}

	if len(operations) > 0 {
		script := fmt.Sprintf(`
			(function() {
				try {
					%s
				} catch (e) {
					console.warn('Error processing element masking:', e);
				}
			})();
		`, strings.Join(operations, "\n"))

		return chromedp.Run(ctx, chromedp.Evaluate(script, nil))
	}

	return nil
}

// waitForStability waits for page stability
func (sc *ScreenshotCapture) waitForStability(ctx context.Context, page *browser.Page, duration time.Duration) error {
	waitCtx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()

	// Use the page's stability detection if available
	if err := page.WaitForStability(waitCtx, nil); err != nil {
		// Fall back to a simple wait
		time.Sleep(duration)
	}

	return nil
}

// captureFullPage captures the full page
func (sc *ScreenshotCapture) captureFullPage(ctx context.Context, page *browser.Page, opts *ScreenshotOptions) ([]byte, error) {
	var buf []byte
	
	var action chromedp.Action
	if opts.OmitBackground {
		action = chromedp.ActionFunc(func(ctx context.Context) error {
			params := cdppage.CaptureScreenshot().
				WithFormat(cdppage.CaptureScreenshotFormatPng).
				WithQuality(int64(opts.Quality)).
				WithCaptureBeyondViewport(true).
				WithFromSurface(true)
			data, err := params.Do(ctx)
			if err != nil {
				return err
			}
			buf = data
			return nil
		})
	} else {
		action = chromedp.FullScreenshot(&buf, int(opts.Quality))
	}

	if err := chromedp.Run(ctx, action); err != nil {
		return nil, errors.Wrap(err, "capturing full page screenshot")
	}

	return buf, nil
}

// captureElement captures a specific element
func (sc *ScreenshotCapture) captureElement(ctx context.Context, page *browser.Page, selector string, opts *ScreenshotOptions) ([]byte, error) {
	var buf []byte
	
	action := chromedp.Screenshot(selector, &buf, chromedp.NodeVisible)
	if err := chromedp.Run(ctx, action); err != nil {
		return nil, errors.Wrapf(err, "capturing element screenshot for selector: %s", selector)
	}

	return buf, nil
}

// captureViewport captures the current viewport
func (sc *ScreenshotCapture) captureViewport(ctx context.Context, page *browser.Page, opts *ScreenshotOptions) ([]byte, error) {
	var buf []byte
	
	action := chromedp.CaptureScreenshot(&buf)
	if opts.OmitBackground {
		action = chromedp.ActionFunc(func(ctx context.Context) error {
			params := cdppage.CaptureScreenshot().
				WithFormat(cdppage.CaptureScreenshotFormatPng).
				WithQuality(int64(opts.Quality)).
				WithFromSurface(true)
			data, err := params.Do(ctx)
			if err != nil {
				return err
			}
			buf = data
			return nil
		})
	}

	if err := chromedp.Run(ctx, action); err != nil {
		return nil, errors.Wrap(err, "capturing viewport screenshot")
	}

	return buf, nil
}

// CaptureMultipleViewports captures screenshots at multiple viewport sizes
func (sc *ScreenshotCapture) CaptureMultipleViewports(ctx context.Context, page *browser.Page, viewports []ViewportSize, opts *ScreenshotOptions) (map[string][]byte, error) {
	results := make(map[string][]byte)
	
	for _, viewport := range viewports {
		if sc.verbose {
			log.Printf("Capturing screenshot for viewport: %s (%dx%d)", viewport.Name, viewport.Width, viewport.Height)
		}

		// Create a copy of options for this viewport
		viewportOpts := *opts
		viewportOpts.ViewportWidth = viewport.Width
		viewportOpts.ViewportHeight = viewport.Height

		screenshot, err := sc.CaptureScreenshot(ctx, page, &viewportOpts)
		if err != nil {
			return nil, errors.Wrapf(err, "capturing screenshot for viewport %s", viewport.Name)
		}

		results[viewport.Name] = screenshot
	}

	return results, nil
}

// CaptureWithDeviceEmulation captures screenshots with device emulation
func (sc *ScreenshotCapture) CaptureWithDeviceEmulation(ctx context.Context, page *browser.Page, devices []string, opts *ScreenshotOptions) (map[string][]byte, error) {
	results := make(map[string][]byte)
	
	for _, deviceName := range devices {
		if sc.verbose {
			log.Printf("Capturing screenshot for device: %s", deviceName)
		}

		// Create a copy of options for this device
		deviceOpts := *opts
		deviceOpts.EmulateDevice = deviceName

		screenshot, err := sc.CaptureScreenshot(ctx, page, &deviceOpts)
		if err != nil {
			return nil, errors.Wrapf(err, "capturing screenshot for device %s", deviceName)
		}

		results[deviceName] = screenshot
	}

	return results, nil
}

// CaptureElementScreenshots captures screenshots of multiple elements
func (sc *ScreenshotCapture) CaptureElementScreenshots(ctx context.Context, page *browser.Page, selectors []string, opts *ScreenshotOptions) (map[string][]byte, error) {
	results := make(map[string][]byte)
	
	for _, selector := range selectors {
		if sc.verbose {
			log.Printf("Capturing screenshot for element: %s", selector)
		}

		// Create a copy of options for this element
		elementOpts := *opts
		elementOpts.Selector = selector
		elementOpts.FullPage = false

		screenshot, err := sc.CaptureScreenshot(ctx, page, &elementOpts)
		if err != nil {
			return nil, errors.Wrapf(err, "capturing screenshot for element %s", selector)
		}

		results[selector] = screenshot
	}

	return results, nil
}

// ValidateScreenshotOptions validates screenshot options
func ValidateScreenshotOptions(opts *ScreenshotOptions) error {
	if opts == nil {
		return errors.New("screenshot options cannot be nil")
	}

	if opts.ViewportWidth < 0 || opts.ViewportHeight < 0 {
		return errors.New("viewport dimensions must be non-negative")
	}

	if opts.Quality < 1 || opts.Quality > 100 {
		return errors.New("quality must be between 1 and 100")
	}

	if opts.DeviceScaleFactor < 0 {
		return errors.New("device scale factor must be non-negative")
	}

	if opts.Format != "" && opts.Format != "png" && opts.Format != "jpg" && opts.Format != "jpeg" && opts.Format != "webp" {
		return errors.New("format must be png, jpg, jpeg, or webp")
	}

	return nil
}

// GetScreenshotDimensions returns the dimensions of a screenshot
func GetScreenshotDimensions(screenshot []byte) (*ImageDimensions, error) {
	img, _, err := image.Decode(strings.NewReader(string(screenshot)))
	if err != nil {
		return nil, errors.Wrap(err, "decoding screenshot")
	}

	bounds := img.Bounds()
	return &ImageDimensions{
		Width:  bounds.Max.X - bounds.Min.X,
		Height: bounds.Max.Y - bounds.Min.Y,
	}, nil
}

// ConvertScreenshotFormat converts a screenshot from one format to another
func ConvertScreenshotFormat(screenshot []byte, targetFormat string) ([]byte, error) {
	img, _, err := image.Decode(strings.NewReader(string(screenshot)))
	if err != nil {
		return nil, errors.Wrap(err, "decoding screenshot")
	}

	var buf strings.Builder
	switch targetFormat {
	case "png":
		err = png.Encode(&buf, img)
	default:
		return nil, fmt.Errorf("unsupported target format: %s", targetFormat)
	}

	if err != nil {
		return nil, errors.Wrapf(err, "encoding to %s", targetFormat)
	}

	return []byte(buf.String()), nil
}

// GetImageHash calculates a hash of the image for comparison
func GetImageHash(img image.Image) (string, error) {
	bounds := img.Bounds()
	width := bounds.Max.X - bounds.Min.X
	height := bounds.Max.Y - bounds.Min.Y
	
	// Simple hash based on image dimensions and sample pixels
	hash := fmt.Sprintf("%dx%d", width, height)
	
	// Sample some pixels for a more detailed hash
	for y := 0; y < height; y += height / 10 {
		for x := 0; x < width; x += width / 10 {
			r, g, b, a := img.At(x, y).RGBA()
			hash += fmt.Sprintf("_%d_%d_%d_%d", r, g, b, a)
		}
	}
	
	return hash, nil
}