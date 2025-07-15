package browser

import "time"

// NavigateOptions configures page navigation
type NavigateOptions struct {
	Timeout   time.Duration
	WaitUntil string // "load", "domcontentloaded", "networkidle"
}

// NavigateOption is a function that modifies NavigateOptions
type NavigateOption func(*NavigateOptions)

// WithNavigateTimeout sets navigation timeout
func WithNavigateTimeout(timeout time.Duration) NavigateOption {
	return func(o *NavigateOptions) {
		o.Timeout = timeout
	}
}

// WithWaitUntil sets what to wait for
func WithWaitUntil(state string) NavigateOption {
	return func(o *NavigateOptions) {
		o.WaitUntil = state
	}
}

// ClickOptions configures click behavior
type ClickOptions struct {
	Button  string        // "left", "right", "middle"
	Count   int           // Number of clicks
	Delay   time.Duration // Delay between clicks
	Timeout time.Duration
}

// ClickOption is a function that modifies ClickOptions
type ClickOption func(*ClickOptions)

// WithClickButton sets mouse button
func WithClickButton(button string) ClickOption {
	return func(o *ClickOptions) {
		o.Button = button
	}
}

// WithClickCount sets click count
func WithClickCount(count int) ClickOption {
	return func(o *ClickOptions) {
		o.Count = count
	}
}

// WithClickDelay sets delay between clicks
func WithClickDelay(delay time.Duration) ClickOption {
	return func(o *ClickOptions) {
		o.Delay = delay
	}
}

// WithClickTimeout sets click timeout
func WithClickTimeout(timeout time.Duration) ClickOption {
	return func(o *ClickOptions) {
		o.Timeout = timeout
	}
}

// TypeOptions configures typing behavior
type TypeOptions struct {
	Delay   time.Duration // Delay between keystrokes
	Timeout time.Duration
}

// TypeOption is a function that modifies TypeOptions
type TypeOption func(*TypeOptions)

// WithTypeDelay sets typing delay
func WithTypeDelay(delay time.Duration) TypeOption {
	return func(o *TypeOptions) {
		o.Delay = delay
	}
}

// WithTypeTimeout sets type timeout
func WithTypeTimeout(timeout time.Duration) TypeOption {
	return func(o *TypeOptions) {
		o.Timeout = timeout
	}
}

// WaitOptions configures wait behavior
type WaitOptions struct {
	State   string // "attached", "detached", "visible", "hidden"
	Timeout time.Duration
}

// WaitOption is a function that modifies WaitOptions
type WaitOption func(*WaitOptions)

// WithWaitState sets what state to wait for
func WithWaitState(state string) WaitOption {
	return func(o *WaitOptions) {
		o.State = state
	}
}

// WithWaitTimeout sets wait timeout
func WithWaitTimeout(timeout time.Duration) WaitOption {
	return func(o *WaitOptions) {
		o.Timeout = timeout
	}
}

// ScreenshotOptions configures screenshot behavior
type ScreenshotOptions struct {
	FullPage bool
	Selector string
	Quality  int    // JPEG quality 0-100
	Type     string // "png" or "jpeg"
	Clip     *Clip
}

// Clip defines screenshot clipping area
type Clip struct {
	X      float64
	Y      float64
	Width  float64
	Height float64
}

// ScreenshotOption is a function that modifies ScreenshotOptions
type ScreenshotOption func(*ScreenshotOptions)

// WithFullPage captures full page
func WithFullPage() ScreenshotOption {
	return func(o *ScreenshotOptions) {
		o.FullPage = true
	}
}

// WithScreenshotSelector captures specific element
func WithScreenshotSelector(selector string) ScreenshotOption {
	return func(o *ScreenshotOptions) {
		o.Selector = selector
	}
}

// WithScreenshotQuality sets JPEG quality
func WithScreenshotQuality(quality int) ScreenshotOption {
	return func(o *ScreenshotOptions) {
		o.Quality = quality
		o.Type = "jpeg"
	}
}

// WithScreenshotType sets image type
func WithScreenshotType(imgType string) ScreenshotOption {
	return func(o *ScreenshotOptions) {
		o.Type = imgType
	}
}

// WithScreenshotClip sets clipping area
func WithScreenshotClip(x, y, width, height float64) ScreenshotOption {
	return func(o *ScreenshotOptions) {
		o.Clip = &Clip{X: x, Y: y, Width: width, Height: height}
	}
}

// Compatibility functions for existing tests

// NavigateWithTimeout creates a navigate option with timeout (compatibility)
func NavigateWithTimeout(timeout time.Duration) NavigateOption {
	return WithNavigateTimeout(timeout)
}

// ClickWithTimeout creates a click option with timeout (compatibility)
func ClickWithTimeout(timeout time.Duration) ClickOption {
	return WithClickTimeout(timeout)
}

// TypeWithTimeout creates a type option with timeout (compatibility)
func TypeWithTimeout(timeout time.Duration) TypeOption {
	return WithTypeTimeout(timeout)
}

// WaitWithTimeout creates a wait option with timeout (compatibility)
func WaitWithTimeout(timeout time.Duration) WaitOption {
	return WithWaitTimeout(timeout)
}

// WaitWithState creates a wait option with state (compatibility)
func WaitWithState(state string) WaitOption {
	return WithWaitState(state)
}

// ScreenshotFullPage creates a screenshot option for full page (compatibility)
func ScreenshotFullPage(fullPage bool) ScreenshotOption {
	if fullPage {
		return WithFullPage()
	}
	return func(*ScreenshotOptions) {} // No-op if false
}

// ScreenshotSelector creates a screenshot option for element selector (compatibility)
func ScreenshotSelector(selector string) ScreenshotOption {
	return WithScreenshotSelector(selector)
}

// PDFOptions configures PDF generation
type PDFOptions struct {
	Format          string // A4, Letter, etc
	Landscape       bool
	Scale           float64
	PrintBackground bool
	MarginTop       float64
	MarginBottom    float64
	MarginLeft      float64
	MarginRight     float64
}

// PDFOption is a function that modifies PDFOptions
type PDFOption func(*PDFOptions)

// WithPDFFormat sets paper format
func WithPDFFormat(format string) PDFOption {
	return func(o *PDFOptions) {
		o.Format = format
	}
}

// WithPDFLandscape sets landscape orientation
func WithPDFLandscape() PDFOption {
	return func(o *PDFOptions) {
		o.Landscape = true
	}
}

// WithPDFScale sets scale
func WithPDFScale(scale float64) PDFOption {
	return func(o *PDFOptions) {
		o.Scale = scale
	}
}

// WithPDFBackground includes background
func WithPDFBackground() PDFOption {
	return func(o *PDFOptions) {
		o.PrintBackground = true
	}
}

// WithPDFMargins sets margins
func WithPDFMargins(top, bottom, left, right float64) PDFOption {
	return func(o *PDFOptions) {
		o.MarginTop = top
		o.MarginBottom = bottom
		o.MarginLeft = left
		o.MarginRight = right
	}
}
