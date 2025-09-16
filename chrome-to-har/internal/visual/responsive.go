package visual

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/pkg/errors"
	"github.com/tmc/misc/chrome-to-har/internal/browser"
)

// ResponsiveVisualTester handles responsive visual regression testing
type ResponsiveVisualTester struct {
	tester  *VisualRegressionTester
	verbose bool
}

// NewResponsiveVisualTester creates a new responsive visual tester
func NewResponsiveVisualTester(verbose bool) *ResponsiveVisualTester {
	return &ResponsiveVisualTester{
		tester:  NewVisualRegressionTester(verbose),
		verbose: verbose,
	}
}

// ResponsiveTestConfig configures responsive visual testing
type ResponsiveTestConfig struct {
	// BaseConfig is the base visual test configuration
	BaseConfig *VisualTestConfig
	// Viewports are the viewports to test
	Viewports []ViewportSize
	// Devices are the devices to emulate
	Devices []string
	// OrientationTest enables orientation testing
	OrientationTest bool
	// PixelDensities are the pixel densities to test
	PixelDensities []float64
	// BreakpointTest enables breakpoint testing
	BreakpointTest bool
	// Breakpoints are custom breakpoints to test
	Breakpoints []int
}

// ResponsiveTestResult contains results from responsive testing
type ResponsiveTestResult struct {
	TestName  string
	URL       string
	Results   map[string]*TestResult // keyed by viewport/device name
	Summary   ResponsiveTestSummary
	Timestamp time.Time
	Duration  time.Duration
	Passed    bool
	Config    *ResponsiveTestConfig
	Metadata  map[string]interface{}
}

// ResponsiveTestSummary contains summary statistics for responsive testing
type ResponsiveTestSummary struct {
	TotalViewports    int
	PassedViewports   int
	FailedViewports   int
	WorstViewport     string
	WorstDifference   float64
	BestViewport      string
	BestDifference    float64
	AverageDifference float64
}

// RunResponsiveTest runs a responsive visual test across multiple viewports
func (rvt *ResponsiveVisualTester) RunResponsiveTest(ctx context.Context, testName, url string, config *ResponsiveTestConfig) (*ResponsiveTestResult, error) {
	startTime := time.Now()

	result := &ResponsiveTestResult{
		TestName:  testName,
		URL:       url,
		Results:   make(map[string]*TestResult),
		Timestamp: startTime,
		Config:    config,
		Metadata:  make(map[string]interface{}),
	}

	if rvt.verbose {
		log.Printf("Running responsive test: %s", testName)
	}

	// Test viewports
	if err := rvt.testViewports(ctx, testName, url, config, result); err != nil {
		return nil, errors.Wrap(err, "testing viewports")
	}

	// Test devices
	if err := rvt.testDevices(ctx, testName, url, config, result); err != nil {
		return nil, errors.Wrap(err, "testing devices")
	}

	// Test orientations
	if config.OrientationTest {
		if err := rvt.testOrientations(ctx, testName, url, config, result); err != nil {
			return nil, errors.Wrap(err, "testing orientations")
		}
	}

	// Test pixel densities
	if err := rvt.testPixelDensities(ctx, testName, url, config, result); err != nil {
		return nil, errors.Wrap(err, "testing pixel densities")
	}

	// Test breakpoints
	if config.BreakpointTest {
		if err := rvt.testBreakpoints(ctx, testName, url, config, result); err != nil {
			return nil, errors.Wrap(err, "testing breakpoints")
		}
	}

	// Calculate summary
	result.Summary = rvt.calculateSummary(result.Results)
	result.Duration = time.Since(startTime)
	result.Passed = result.Summary.FailedViewports == 0

	if rvt.verbose {
		log.Printf("Responsive test completed: %d/%d viewports passed, %.2fs",
			result.Summary.PassedViewports, result.Summary.TotalViewports, result.Duration.Seconds())
	}

	return result, nil
}

// testViewports tests multiple viewports
func (rvt *ResponsiveVisualTester) testViewports(ctx context.Context, testName, url string, config *ResponsiveTestConfig, result *ResponsiveTestResult) error {
	viewports := config.Viewports
	if len(viewports) == 0 {
		viewports = DefaultViewportSizes
	}

	for _, viewport := range viewports {
		if rvt.verbose {
			log.Printf("Testing viewport: %s (%dx%d)", viewport.Name, viewport.Width, viewport.Height)
		}

		viewportTestName := fmt.Sprintf("%s_%s", testName, viewport.Name)

		// Create screenshot options for this viewport
		opts := DefaultScreenshotOptions()
		opts.ViewportWidth = viewport.Width
		opts.ViewportHeight = viewport.Height

		// Run the test
		testResult, err := rvt.runSingleViewportTest(ctx, viewportTestName, url, opts, config.BaseConfig)
		if err != nil {
			return errors.Wrapf(err, "testing viewport %s", viewport.Name)
		}

		result.Results[viewport.Name] = testResult
	}

	return nil
}

// testDevices tests device emulation
func (rvt *ResponsiveVisualTester) testDevices(ctx context.Context, testName, url string, config *ResponsiveTestConfig, result *ResponsiveTestResult) error {
	for _, deviceName := range config.Devices {
		if rvt.verbose {
			log.Printf("Testing device: %s", deviceName)
		}

		device, exists := DefaultDeviceEmulations[deviceName]
		if !exists {
			if rvt.verbose {
				log.Printf("Warning: unknown device %s, skipping", deviceName)
			}
			continue
		}

		deviceTestName := fmt.Sprintf("%s_%s", testName, deviceName)

		// Create screenshot options for this device
		opts := DefaultScreenshotOptions()
		opts.EmulateDevice = deviceName
		opts.ViewportWidth = device.ViewportSize.Width
		opts.ViewportHeight = device.ViewportSize.Height
		opts.DeviceScaleFactor = device.DeviceScaleFactor

		// Run the test
		testResult, err := rvt.runSingleViewportTest(ctx, deviceTestName, url, opts, config.BaseConfig)
		if err != nil {
			return errors.Wrapf(err, "testing device %s", deviceName)
		}

		result.Results[deviceName] = testResult
	}

	return nil
}

// testOrientations tests portrait and landscape orientations
func (rvt *ResponsiveVisualTester) testOrientations(ctx context.Context, testName, url string, config *ResponsiveTestConfig, result *ResponsiveTestResult) error {
	orientations := []struct {
		name   string
		width  int
		height int
	}{
		{"portrait", 768, 1024},
		{"landscape", 1024, 768},
	}

	for _, orientation := range orientations {
		if rvt.verbose {
			log.Printf("Testing orientation: %s (%dx%d)", orientation.name, orientation.width, orientation.height)
		}

		orientationTestName := fmt.Sprintf("%s_%s", testName, orientation.name)

		// Create screenshot options for this orientation
		opts := DefaultScreenshotOptions()
		opts.ViewportWidth = orientation.width
		opts.ViewportHeight = orientation.height

		// Run the test
		testResult, err := rvt.runSingleViewportTest(ctx, orientationTestName, url, opts, config.BaseConfig)
		if err != nil {
			return errors.Wrapf(err, "testing orientation %s", orientation.name)
		}

		result.Results[orientation.name] = testResult
	}

	return nil
}

// testPixelDensities tests different pixel densities
func (rvt *ResponsiveVisualTester) testPixelDensities(ctx context.Context, testName, url string, config *ResponsiveTestConfig, result *ResponsiveTestResult) error {
	densities := config.PixelDensities
	if len(densities) == 0 {
		densities = []float64{1.0, 2.0, 3.0}
	}

	for _, density := range densities {
		if rvt.verbose {
			log.Printf("Testing pixel density: %.1fx", density)
		}

		densityTestName := fmt.Sprintf("%s_%.1fx", testName, density)

		// Create screenshot options for this density
		opts := DefaultScreenshotOptions()
		opts.DeviceScaleFactor = density

		// Run the test
		testResult, err := rvt.runSingleViewportTest(ctx, densityTestName, url, opts, config.BaseConfig)
		if err != nil {
			return errors.Wrapf(err, "testing pixel density %.1fx", density)
		}

		result.Results[fmt.Sprintf("%.1fx", density)] = testResult
	}

	return nil
}

// testBreakpoints tests CSS breakpoints
func (rvt *ResponsiveVisualTester) testBreakpoints(ctx context.Context, testName, url string, config *ResponsiveTestConfig, result *ResponsiveTestResult) error {
	breakpoints := config.Breakpoints
	if len(breakpoints) == 0 {
		breakpoints = []int{320, 768, 1024, 1200, 1920}
	}

	for _, breakpoint := range breakpoints {
		if rvt.verbose {
			log.Printf("Testing breakpoint: %dpx", breakpoint)
		}

		breakpointTestName := fmt.Sprintf("%s_%dpx", testName, breakpoint)

		// Create screenshot options for this breakpoint
		opts := DefaultScreenshotOptions()
		opts.ViewportWidth = breakpoint
		opts.ViewportHeight = 1080 // Fixed height for breakpoint testing

		// Run the test
		testResult, err := rvt.runSingleViewportTest(ctx, breakpointTestName, url, opts, config.BaseConfig)
		if err != nil {
			return errors.Wrapf(err, "testing breakpoint %dpx", breakpoint)
		}

		result.Results[fmt.Sprintf("%dpx", breakpoint)] = testResult
	}

	return nil
}

// runSingleViewportTest runs a single viewport test
func (rvt *ResponsiveVisualTester) runSingleViewportTest(ctx context.Context, testName, url string, opts *ScreenshotOptions, config *VisualTestConfig) (*TestResult, error) {
	// Create a new browser instance
	browser, err := browser.New(ctx, nil)
	if err != nil {
		return nil, errors.Wrap(err, "creating browser")
	}
	defer browser.Close()

	// Launch browser
	if err := browser.Launch(ctx); err != nil {
		return nil, errors.Wrap(err, "launching browser")
	}

	// Get current page
	page := browser.GetCurrentPage()
	if page == nil {
		return nil, errors.New("failed to get current page")
	}

	// Navigate to URL
	if err := page.Navigate(url); err != nil {
		return nil, errors.Wrap(err, "navigating to URL")
	}

	// Run the visual test
	return rvt.tester.RunVisualTest(ctx, testName, page, config, opts)
}

// calculateSummary calculates summary statistics
func (rvt *ResponsiveVisualTester) calculateSummary(results map[string]*TestResult) ResponsiveTestSummary {
	summary := ResponsiveTestSummary{
		TotalViewports:  len(results),
		WorstDifference: -1,
		BestDifference:  101,
	}

	var totalDifference float64

	for viewport, result := range results {
		if result.Passed {
			summary.PassedViewports++
		} else {
			summary.FailedViewports++
		}

		// Get difference percentage
		var difference float64
		if result.ComparisonResult != nil {
			difference = result.ComparisonResult.DifferencePercentage
		}

		totalDifference += difference

		// Track worst viewport
		if difference > summary.WorstDifference {
			summary.WorstDifference = difference
			summary.WorstViewport = viewport
		}

		// Track best viewport
		if difference < summary.BestDifference {
			summary.BestDifference = difference
			summary.BestViewport = viewport
		}
	}

	if summary.TotalViewports > 0 {
		summary.AverageDifference = totalDifference / float64(summary.TotalViewports)
	}

	return summary
}

// RunResponsiveTestSuite runs a suite of responsive tests
func (rvt *ResponsiveVisualTester) RunResponsiveTestSuite(ctx context.Context, suiteName string, tests []ResponsiveTest, config *ResponsiveTestConfig) (*ResponsiveTestSuiteResult, error) {
	startTime := time.Now()

	suiteResult := &ResponsiveTestSuiteResult{
		SuiteName: suiteName,
		Timestamp: startTime,
		Config:    config,
		Results:   make([]*ResponsiveTestResult, 0, len(tests)),
		Metadata:  make(map[string]interface{}),
	}

	if rvt.verbose {
		log.Printf("Running responsive test suite: %s (%d tests)", suiteName, len(tests))
	}

	var totalTests, passedTests, failedTests int

	for i, test := range tests {
		if rvt.verbose {
			log.Printf("Running responsive test %d/%d: %s", i+1, len(tests), test.Name)
		}

		result, err := rvt.RunResponsiveTest(ctx, test.Name, test.URL, config)
		if err != nil {
			return nil, errors.Wrapf(err, "running responsive test %s", test.Name)
		}

		suiteResult.Results = append(suiteResult.Results, result)

		totalTests++
		if result.Passed {
			passedTests++
		} else {
			failedTests++
		}
	}

	suiteResult.Duration = time.Since(startTime)
	suiteResult.Passed = failedTests == 0

	// Calculate summary
	suiteResult.Summary = ResponsiveTestSuiteSummary{
		TotalTests:      totalTests,
		PassedTests:     passedTests,
		FailedTests:     failedTests,
		PassRate:        float64(passedTests) / float64(totalTests),
		TotalDuration:   suiteResult.Duration,
		AverageDuration: suiteResult.Duration / time.Duration(totalTests),
	}

	if rvt.verbose {
		log.Printf("Responsive test suite completed: %d/%d passed (%.1f%%), %.2fs total",
			passedTests, totalTests, suiteResult.Summary.PassRate*100, suiteResult.Duration.Seconds())
	}

	return suiteResult, nil
}

// CreateResponsiveTestConfig creates a default responsive test configuration
func CreateResponsiveTestConfig(baseConfig *VisualTestConfig) *ResponsiveTestConfig {
	return &ResponsiveTestConfig{
		BaseConfig:      baseConfig,
		Viewports:       DefaultViewportSizes,
		Devices:         []string{"desktop", "tablet", "mobile"},
		OrientationTest: true,
		PixelDensities:  []float64{1.0, 2.0, 3.0},
		BreakpointTest:  true,
		Breakpoints:     []int{320, 768, 1024, 1200, 1920},
	}
}

// Supporting types

// ResponsiveTest represents a responsive visual test
type ResponsiveTest struct {
	Name     string
	URL      string
	Setup    func(ctx context.Context, page *browser.Page) error
	Action   func(ctx context.Context, page *browser.Page) error
	Cleanup  func(ctx context.Context, page *browser.Page) error
	Tags     []string
	Metadata map[string]interface{}
}

// ResponsiveTestSuiteResult contains results from a responsive test suite
type ResponsiveTestSuiteResult struct {
	SuiteName string
	Results   []*ResponsiveTestResult
	Summary   ResponsiveTestSuiteSummary
	Timestamp time.Time
	Duration  time.Duration
	Passed    bool
	Config    *ResponsiveTestConfig
	Metadata  map[string]interface{}
}

// ResponsiveTestSuiteSummary contains summary statistics for a responsive test suite
type ResponsiveTestSuiteSummary struct {
	TotalTests      int
	PassedTests     int
	FailedTests     int
	PassRate        float64
	TotalDuration   time.Duration
	AverageDuration time.Duration
}

// GetViewportFailures returns viewports that failed across all tests
func (rtsr *ResponsiveTestSuiteResult) GetViewportFailures() map[string]int {
	failures := make(map[string]int)

	for _, result := range rtsr.Results {
		for viewport, testResult := range result.Results {
			if !testResult.Passed {
				failures[viewport]++
			}
		}
	}

	return failures
}

// GetWorstViewports returns the viewports with the highest failure rate
func (rtsr *ResponsiveTestSuiteResult) GetWorstViewports(limit int) []ViewportFailure {
	failures := rtsr.GetViewportFailures()

	var viewportFailures []ViewportFailure
	for viewport, count := range failures {
		viewportFailures = append(viewportFailures, ViewportFailure{
			Viewport:     viewport,
			FailureCount: count,
			FailureRate:  float64(count) / float64(len(rtsr.Results)),
		})
	}

	// Sort by failure count (descending)
	for i := 0; i < len(viewportFailures)-1; i++ {
		for j := i + 1; j < len(viewportFailures); j++ {
			if viewportFailures[i].FailureCount < viewportFailures[j].FailureCount {
				viewportFailures[i], viewportFailures[j] = viewportFailures[j], viewportFailures[i]
			}
		}
	}

	if limit > 0 && len(viewportFailures) > limit {
		viewportFailures = viewportFailures[:limit]
	}

	return viewportFailures
}

// ViewportFailure represents viewport failure statistics
type ViewportFailure struct {
	Viewport     string
	FailureCount int
	FailureRate  float64
}
