// Package visual provides comprehensive visual regression testing capabilities
package visual

import (
	"context"
	"image"
	"time"

	"github.com/tmc/misc/chrome-to-har/internal/browser"
)

// VisualTestConfig holds configuration for visual regression testing
type VisualTestConfig struct {
	// BaselineDir is the directory where baseline images are stored
	BaselineDir string
	// ActualDir is the directory where actual screenshots are stored
	ActualDir string
	// DiffDir is the directory where diff images are stored
	DiffDir string
	// Threshold is the pixel difference threshold (0-1)
	Threshold float64
	// IgnoreAntialiasing ignores antialiasing differences
	IgnoreAntialiasing bool
	// EnableFuzzyMatching enables fuzzy matching for tolerance
	EnableFuzzyMatching bool
	// FuzzyThreshold is the fuzzy matching threshold (0-1)
	FuzzyThreshold float64
	// MaxDiffPixels is the maximum allowed different pixels
	MaxDiffPixels int
	// Quality is the screenshot quality (1-100)
	Quality int
	// Format is the screenshot format (png, jpg, webp)
	Format string
	// RetryCount is the number of retries for flaky tests
	RetryCount int
	// RetryDelay is the delay between retries
	RetryDelay time.Duration
}

// ScreenshotOptions configures screenshot capture
type ScreenshotOptions struct {
	// FullPage captures the entire page
	FullPage bool
	// Selector captures only the specified element
	Selector string
	// ViewportWidth sets the viewport width
	ViewportWidth int
	// ViewportHeight sets the viewport height
	ViewportHeight int
	// Quality is the image quality (1-100)
	Quality int
	// Format is the image format (png, jpg, webp)
	Format string
	// HideCursor hides the mouse cursor
	HideCursor bool
	// HideScrollbars hides scrollbars
	HideScrollbars bool
	// WaitForFonts waits for fonts to load
	WaitForFonts bool
	// WaitForImages waits for images to load
	WaitForImages bool
	// WaitForAnimations waits for animations to complete
	WaitForAnimations bool
	// AnimationWaitTime is the time to wait for animations
	AnimationWaitTime time.Duration
	// StabilityWaitTime is the time to wait for page stability
	StabilityWaitTime time.Duration
	// DeviceScaleFactor sets the device scale factor
	DeviceScaleFactor float64
	// EmulateDevice emulates a specific device
	EmulateDevice string
	// ClipToViewport clips the screenshot to the viewport
	ClipToViewport bool
	// OmitBackground omits the background
	OmitBackground bool
	// CustomCSS injects custom CSS before taking screenshot
	CustomCSS string
	// ExcludeSelectors excludes elements from the screenshot
	ExcludeSelectors []string
	// MaskSelectors masks elements in the screenshot
	MaskSelectors []string
	// MaskColor is the color to use for masking
	MaskColor string
}

// ViewportSize represents a viewport size configuration
type ViewportSize struct {
	Width  int
	Height int
	Name   string
}

// DeviceEmulation represents device emulation settings
type DeviceEmulation struct {
	Name              string
	ViewportSize      ViewportSize
	DeviceScaleFactor float64
	IsMobile          bool
	HasTouch          bool
	UserAgent         string
	ScreenWidth       int
	ScreenHeight      int
	ScreenOrientation string
}

// ComparisonResult represents the result of image comparison
type ComparisonResult struct {
	// Passed indicates if the comparison passed
	Passed bool
	// DiffPixels is the number of different pixels
	DiffPixels int
	// TotalPixels is the total number of pixels
	TotalPixels int
	// DifferencePercentage is the percentage of different pixels
	DifferencePercentage float64
	// DiffImage is the difference image
	DiffImage image.Image
	// MatchingScore is the overall matching score (0-1)
	MatchingScore float64
	// Regions are the different regions
	Regions []DiffRegion
	// Metadata contains comparison metadata
	Metadata map[string]interface{}
}

// DiffRegion represents a region with differences
type DiffRegion struct {
	X      int
	Y      int
	Width  int
	Height int
	Score  float64
}

// BaselineMetadata contains metadata for baseline images
type BaselineMetadata struct {
	// Version is the baseline version
	Version string
	// CreatedAt is when the baseline was created
	CreatedAt time.Time
	// UpdatedAt is when the baseline was last updated
	UpdatedAt time.Time
	// URL is the URL that was captured
	URL string
	// ViewportSize is the viewport size used
	ViewportSize ViewportSize
	// DeviceEmulation is the device emulation used
	DeviceEmulation *DeviceEmulation
	// BrowserInfo contains browser information
	BrowserInfo BrowserInfo
	// TestName is the name of the test
	TestName string
	// TestSuite is the test suite name
	TestSuite string
	// Environment is the testing environment
	Environment string
	// Branch is the git branch
	Branch string
	// Commit is the git commit hash
	Commit string
	// Tags are additional tags
	Tags []string
	// Notes are additional notes
	Notes string
	// ChecksumMD5 is the MD5 checksum of the image
	ChecksumMD5 string
	// ChecksumSHA256 is the SHA256 checksum of the image
	ChecksumSHA256 string
	// FileSize is the size of the image file
	FileSize int64
	// ImageDimensions contains image dimensions
	ImageDimensions ImageDimensions
}

// BrowserInfo contains browser information
type BrowserInfo struct {
	Name    string
	Version string
	Major   int
	Minor   int
	Patch   int
	Build   string
}

// ImageDimensions contains image dimensions
type ImageDimensions struct {
	Width  int
	Height int
}

// TestResult represents the result of a visual test
type TestResult struct {
	// TestName is the name of the test
	TestName string
	// TestSuite is the test suite name
	TestSuite string
	// Passed indicates if the test passed
	Passed bool
	// ComparisonResult is the detailed comparison result
	ComparisonResult *ComparisonResult
	// BaselineMetadata is the baseline metadata
	BaselineMetadata *BaselineMetadata
	// ActualScreenshot is the actual screenshot data
	ActualScreenshot []byte
	// BaselineScreenshot is the baseline screenshot data
	BaselineScreenshot []byte
	// DiffScreenshot is the diff screenshot data
	DiffScreenshot []byte
	// Error is any error that occurred
	Error error
	// Duration is how long the test took
	Duration time.Duration
	// Timestamp is when the test was run
	Timestamp time.Time
	// Retries is the number of retries performed
	Retries int
	// Environment is the testing environment
	Environment string
	// URL is the URL that was tested
	URL string
	// Config is the test configuration used
	Config *VisualTestConfig
	// ScreenshotOptions are the options used for screenshot
	ScreenshotOptions *ScreenshotOptions
	// Metadata contains additional metadata
	Metadata map[string]interface{}
}

// TestSuiteResult represents the result of a test suite
type TestSuiteResult struct {
	// SuiteName is the name of the test suite
	SuiteName string
	// Results are the individual test results
	Results []*TestResult
	// Passed indicates if all tests passed
	Passed bool
	// Duration is the total duration
	Duration time.Duration
	// Timestamp is when the suite was run
	Timestamp time.Time
	// Environment is the testing environment
	Environment string
	// Summary contains summary statistics
	Summary TestSummary
	// Config is the suite configuration
	Config *VisualTestConfig
	// Metadata contains additional metadata
	Metadata map[string]interface{}
}

// TestSummary contains summary statistics
type TestSummary struct {
	// TotalTests is the total number of tests
	TotalTests int
	// PassedTests is the number of passed tests
	PassedTests int
	// FailedTests is the number of failed tests
	FailedTests int
	// SkippedTests is the number of skipped tests
	SkippedTests int
	// PassRate is the pass rate (0-1)
	PassRate float64
	// TotalDuration is the total duration
	TotalDuration time.Duration
	// AverageDuration is the average duration per test
	AverageDuration time.Duration
}

// VisualTester is the main interface for visual regression testing
type VisualTester interface {
	// CaptureScreenshot captures a screenshot with the given options
	CaptureScreenshot(ctx context.Context, page *browser.Page, opts *ScreenshotOptions) ([]byte, error)

	// CompareImages compares two images and returns the result
	CompareImages(baseline, actual image.Image, config *VisualTestConfig) (*ComparisonResult, error)

	// RunVisualTest runs a visual test
	RunVisualTest(ctx context.Context, testName string, page *browser.Page, config *VisualTestConfig, opts *ScreenshotOptions) (*TestResult, error)

	// RunTestSuite runs a test suite
	RunTestSuite(ctx context.Context, suiteName string, tests []VisualTest, config *VisualTestConfig) (*TestSuiteResult, error)

	// SaveBaseline saves a baseline image
	SaveBaseline(testName string, screenshot []byte, metadata *BaselineMetadata, config *VisualTestConfig) error

	// LoadBaseline loads a baseline image
	LoadBaseline(testName string, config *VisualTestConfig) ([]byte, *BaselineMetadata, error)

	// GetBaselineMetadata gets metadata for a baseline
	GetBaselineMetadata(testName string, config *VisualTestConfig) (*BaselineMetadata, error)

	// UpdateBaseline updates a baseline image
	UpdateBaseline(testName string, screenshot []byte, metadata *BaselineMetadata, config *VisualTestConfig) error

	// DeleteBaseline deletes a baseline image
	DeleteBaseline(testName string, config *VisualTestConfig) error

	// ListBaselines lists all baselines
	ListBaselines(config *VisualTestConfig) ([]string, error)

	// GenerateReport generates a visual test report
	GenerateReport(results []*TestResult, config *VisualTestConfig) (*TestReport, error)
}

// VisualTest represents a single visual test
type VisualTest struct {
	// Name is the test name
	Name string
	// URL is the URL to test
	URL string
	// Setup is a function to set up the test (optional)
	Setup func(ctx context.Context, page *browser.Page) error
	// Action is a function to perform actions before screenshot (optional)
	Action func(ctx context.Context, page *browser.Page) error
	// Cleanup is a function to clean up after the test (optional)
	Cleanup func(ctx context.Context, page *browser.Page) error
	// Options are the screenshot options
	Options *ScreenshotOptions
	// Viewport is the viewport size
	Viewport *ViewportSize
	// DeviceEmulation is the device emulation
	DeviceEmulation *DeviceEmulation
	// Timeout is the test timeout
	Timeout time.Duration
	// Retries is the number of retries
	Retries int
	// Skip indicates if the test should be skipped
	Skip bool
	// SkipReason is the reason for skipping
	SkipReason string
	// Tags are test tags
	Tags []string
	// Metadata contains additional metadata
	Metadata map[string]interface{}
}

// TestReport represents a visual test report
type TestReport struct {
	// Title is the report title
	Title string
	// SuiteResults are the suite results
	SuiteResults []*TestSuiteResult
	// Summary is the overall summary
	Summary TestSummary
	// GeneratedAt is when the report was generated
	GeneratedAt time.Time
	// Environment is the testing environment
	Environment string
	// Config is the configuration used
	Config *VisualTestConfig
	// Metadata contains additional metadata
	Metadata map[string]interface{}
}

// Default values
var (
	DefaultViewportSizes = []ViewportSize{
		{Width: 1920, Height: 1080, Name: "Desktop"},
		{Width: 1366, Height: 768, Name: "Laptop"},
		{Width: 768, Height: 1024, Name: "Tablet"},
		{Width: 414, Height: 896, Name: "Mobile"},
		{Width: 375, Height: 667, Name: "iPhone"},
		{Width: 360, Height: 640, Name: "Android"},
	}

	DefaultDeviceEmulations = map[string]*DeviceEmulation{
		"desktop": {
			Name:              "Desktop",
			ViewportSize:      ViewportSize{Width: 1920, Height: 1080, Name: "Desktop"},
			DeviceScaleFactor: 1.0,
			IsMobile:          false,
			HasTouch:          false,
			UserAgent:         "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
			ScreenWidth:       1920,
			ScreenHeight:      1080,
			ScreenOrientation: "landscape",
		},
		"tablet": {
			Name:              "Tablet",
			ViewportSize:      ViewportSize{Width: 768, Height: 1024, Name: "Tablet"},
			DeviceScaleFactor: 2.0,
			IsMobile:          true,
			HasTouch:          true,
			UserAgent:         "Mozilla/5.0 (iPad; CPU OS 14_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.0 Mobile/15E148 Safari/604.1",
			ScreenWidth:       768,
			ScreenHeight:      1024,
			ScreenOrientation: "portrait",
		},
		"mobile": {
			Name:              "Mobile",
			ViewportSize:      ViewportSize{Width: 414, Height: 896, Name: "Mobile"},
			DeviceScaleFactor: 3.0,
			IsMobile:          true,
			HasTouch:          true,
			UserAgent:         "Mozilla/5.0 (iPhone; CPU iPhone OS 14_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.0 Mobile/15E148 Safari/604.1",
			ScreenWidth:       414,
			ScreenHeight:      896,
			ScreenOrientation: "portrait",
		},
	}
)

// DefaultConfig returns a default visual test configuration
func DefaultConfig() *VisualTestConfig {
	return &VisualTestConfig{
		BaselineDir:         "visual-baselines",
		ActualDir:           "visual-actual",
		DiffDir:             "visual-diffs",
		Threshold:           0.1,
		IgnoreAntialiasing:  true,
		EnableFuzzyMatching: true,
		FuzzyThreshold:      0.05,
		MaxDiffPixels:       1000,
		Quality:             90,
		Format:              "png",
		RetryCount:          3,
		RetryDelay:          time.Second,
	}
}

// DefaultScreenshotOptions returns default screenshot options
func DefaultScreenshotOptions() *ScreenshotOptions {
	return &ScreenshotOptions{
		FullPage:          false,
		ViewportWidth:     1920,
		ViewportHeight:    1080,
		Quality:           90,
		Format:            "png",
		HideCursor:        true,
		HideScrollbars:    true,
		WaitForFonts:      true,
		WaitForImages:     true,
		WaitForAnimations: true,
		AnimationWaitTime: 1 * time.Second,
		StabilityWaitTime: 2 * time.Second,
		DeviceScaleFactor: 1.0,
		ClipToViewport:    false,
		OmitBackground:    false,
		MaskColor:         "#000000",
	}
}
