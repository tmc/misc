package visual

import (
	"context"
	"fmt"
	"image"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/tmc/misc/chrome-to-har/internal/browser"
)

// VisualRegressionTester implements the main visual regression testing functionality
type VisualRegressionTester struct {
	capture    *ScreenshotCapture
	baseline   *BaselineManager
	comparison *ImageComparison
	verbose    bool
}

// NewVisualRegressionTester creates a new visual regression tester
func NewVisualRegressionTester(verbose bool) *VisualRegressionTester {
	return &VisualRegressionTester{
		capture:    NewScreenshotCapture(verbose),
		baseline:   NewBaselineManager(verbose),
		comparison: NewImageComparison(verbose),
		verbose:    verbose,
	}
}

// CaptureScreenshot captures a screenshot with the given options
func (vrt *VisualRegressionTester) CaptureScreenshot(ctx context.Context, page *browser.Page, opts *ScreenshotOptions) ([]byte, error) {
	if err := ValidateScreenshotOptions(opts); err != nil {
		return nil, errors.Wrap(err, "invalid screenshot options")
	}

	return vrt.capture.CaptureScreenshot(ctx, page, opts)
}

// CompareImages compares two images and returns the result
func (vrt *VisualRegressionTester) CompareImages(baseline, actual []byte, config *VisualTestConfig) (*ComparisonResult, error) {
	return vrt.comparison.CompareScreenshots(baseline, actual, config)
}

// RunVisualTest runs a single visual regression test
func (vrt *VisualRegressionTester) RunVisualTest(ctx context.Context, testName string, page *browser.Page, config *VisualTestConfig, opts *ScreenshotOptions) (*TestResult, error) {
	startTime := time.Now()

	result := &TestResult{
		TestName:          testName,
		Timestamp:         startTime,
		Environment:       config.BaselineDir, // Use baseline dir as environment indicator
		Config:            config,
		ScreenshotOptions: opts,
		Metadata:          make(map[string]interface{}),
	}

	if vrt.verbose {
		log.Printf("Running visual test: %s", testName)
	}

	// Capture actual screenshot
	actualScreenshot, err := vrt.CaptureScreenshot(ctx, page, opts)
	if err != nil {
		result.Error = errors.Wrap(err, "capturing screenshot")
		result.Duration = time.Since(startTime)
		return result, nil
	}

	result.ActualScreenshot = actualScreenshot

	// Load baseline
	baselineScreenshot, baselineMetadata, err := vrt.baseline.LoadBaseline(testName, config)
	if err != nil {
		// If baseline doesn't exist, create it
		if strings.Contains(err.Error(), "baseline image not found") || strings.Contains(err.Error(), "no such file") {
			if vrt.verbose {
				log.Printf("Baseline not found for %s, creating new baseline", testName)
			}

			// Create metadata for new baseline
			metadata := &BaselineMetadata{
				Version:     "1.0.0",
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
				TestName:    testName,
				Environment: config.BaselineDir,
			}

			// Get current URL from page
			if currentURL, err := page.URL(); err == nil {
				metadata.URL = currentURL
			}

			// Save baseline
			if err := vrt.baseline.SaveBaseline(testName, actualScreenshot, metadata, config); err != nil {
				result.Error = errors.Wrap(err, "saving new baseline")
				result.Duration = time.Since(startTime)
				return result, nil
			}

			result.Passed = true
			result.BaselineMetadata = metadata
			result.BaselineScreenshot = actualScreenshot
			result.Duration = time.Since(startTime)
			result.Metadata["baseline_created"] = true

			return result, nil
		}

		result.Error = errors.Wrap(err, "loading baseline")
		result.Duration = time.Since(startTime)
		return result, nil
	}

	result.BaselineScreenshot = baselineScreenshot
	result.BaselineMetadata = baselineMetadata

	// Compare images
	comparison, err := vrt.CompareImages(baselineScreenshot, actualScreenshot, config)
	if err != nil {
		result.Error = errors.Wrap(err, "comparing images")
		result.Duration = time.Since(startTime)
		return result, nil
	}

	result.ComparisonResult = comparison
	result.Passed = comparison.Passed

	// Save diff image if test failed
	if !comparison.Passed {
		diffFilename := fmt.Sprintf("%s_diff.png", testName)
		diffPath := filepath.Join(config.DiffDir, diffFilename)

		if err := vrt.saveDiffImage(comparison.DiffImage, diffPath); err != nil {
			if vrt.verbose {
				log.Printf("Warning: failed to save diff image: %v", err)
			}
		} else {
			result.Metadata["diff_image_path"] = diffPath
		}
	}

	result.Duration = time.Since(startTime)

	if vrt.verbose {
		status := "PASSED"
		if !result.Passed {
			status = "FAILED"
		}
		log.Printf("Test %s: %s (%.2f%% difference, %.2fs)", testName, status, comparison.DifferencePercentage, result.Duration.Seconds())
	}

	return result, nil
}

// RunTestSuite runs a suite of visual regression tests
func (vrt *VisualRegressionTester) RunTestSuite(ctx context.Context, suiteName string, tests []VisualTest, config *VisualTestConfig) (*TestSuiteResult, error) {
	startTime := time.Now()

	suiteResult := &TestSuiteResult{
		SuiteName:   suiteName,
		Timestamp:   startTime,
		Environment: config.BaselineDir,
		Config:      config,
		Results:     make([]*TestResult, 0, len(tests)),
		Metadata:    make(map[string]interface{}),
	}

	if vrt.verbose {
		log.Printf("Running test suite: %s (%d tests)", suiteName, len(tests))
	}

	// Ensure directories exist
	if err := vrt.ensureDirectories(config); err != nil {
		return nil, errors.Wrap(err, "ensuring directories exist")
	}

	var totalTests, passedTests, failedTests, skippedTests int

	for i, test := range tests {
		if test.Skip {
			skippedTests++
			if vrt.verbose {
				log.Printf("Skipping test %d/%d: %s (%s)", i+1, len(tests), test.Name, test.SkipReason)
			}
			continue
		}

		totalTests++

		if vrt.verbose {
			log.Printf("Running test %d/%d: %s", i+1, len(tests), test.Name)
		}

		// Run the test with retries
		var result *TestResult
		var err error

		for retry := 0; retry <= config.RetryCount; retry++ {
			if retry > 0 {
				if vrt.verbose {
					log.Printf("Retrying test %s (attempt %d/%d)", test.Name, retry+1, config.RetryCount+1)
				}
				time.Sleep(config.RetryDelay)
			}

			result, err = vrt.runSingleTest(ctx, test, config)
			if err == nil && result.Passed {
				break
			}

			if result != nil {
				result.Retries = retry
			}
		}

		if err != nil {
			// Create a failed result
			result = &TestResult{
				TestName:    test.Name,
				Passed:      false,
				Error:       err,
				Duration:    0,
				Timestamp:   time.Now(),
				Environment: config.BaselineDir,
				Config:      config,
				Retries:     config.RetryCount,
				Metadata:    make(map[string]interface{}),
			}
		}

		suiteResult.Results = append(suiteResult.Results, result)

		if result.Passed {
			passedTests++
		} else {
			failedTests++
		}
	}

	suiteResult.Duration = time.Since(startTime)
	suiteResult.Passed = failedTests == 0

	// Calculate summary
	suiteResult.Summary = TestSummary{
		TotalTests:      totalTests,
		PassedTests:     passedTests,
		FailedTests:     failedTests,
		SkippedTests:    skippedTests,
		PassRate:        float64(passedTests) / float64(totalTests),
		TotalDuration:   suiteResult.Duration,
		AverageDuration: suiteResult.Duration / time.Duration(totalTests),
	}

	if vrt.verbose {
		log.Printf("Test suite completed: %d/%d passed (%.1f%%), %d skipped, %.2fs total",
			passedTests, totalTests, suiteResult.Summary.PassRate*100, skippedTests, suiteResult.Duration.Seconds())
	}

	return suiteResult, nil
}

// runSingleTest runs a single visual test with proper setup/teardown
func (vrt *VisualRegressionTester) runSingleTest(ctx context.Context, test VisualTest, config *VisualTestConfig) (*TestResult, error) {
	// Create a new browser instance for the test
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

	// Set viewport if specified
	if test.Viewport != nil {
		if err := page.SetViewport(test.Viewport.Width, test.Viewport.Height); err != nil {
			return nil, errors.Wrap(err, "setting viewport")
		}
	}

	// Navigate to URL
	if test.URL != "" {
		if err := page.Navigate(test.URL); err != nil {
			return nil, errors.Wrap(err, "navigating to URL")
		}
	}

	// Run setup if provided
	if test.Setup != nil {
		if err := test.Setup(ctx, page); err != nil {
			return nil, errors.Wrap(err, "running test setup")
		}
	}

	// Run test action if provided
	if test.Action != nil {
		if err := test.Action(ctx, page); err != nil {
			return nil, errors.Wrap(err, "running test action")
		}
	}

	// Run the visual test
	result, err := vrt.RunVisualTest(ctx, test.Name, page, config, test.Options)
	if err != nil {
		return nil, errors.Wrap(err, "running visual test")
	}

	// Set additional metadata
	if result.Metadata == nil {
		result.Metadata = make(map[string]interface{})
	}
	result.Metadata["test_url"] = test.URL
	result.Metadata["test_tags"] = test.Tags
	if test.Viewport != nil {
		result.Metadata["viewport"] = fmt.Sprintf("%dx%d", test.Viewport.Width, test.Viewport.Height)
	}

	// Run cleanup if provided
	if test.Cleanup != nil {
		if err := test.Cleanup(ctx, page); err != nil {
			if vrt.verbose {
				log.Printf("Warning: test cleanup failed: %v", err)
			}
		}
	}

	return result, nil
}

// SaveBaseline saves a baseline image
func (vrt *VisualRegressionTester) SaveBaseline(testName string, screenshot []byte, metadata *BaselineMetadata, config *VisualTestConfig) error {
	return vrt.baseline.SaveBaseline(testName, screenshot, metadata, config)
}

// LoadBaseline loads a baseline image
func (vrt *VisualRegressionTester) LoadBaseline(testName string, config *VisualTestConfig) ([]byte, *BaselineMetadata, error) {
	return vrt.baseline.LoadBaseline(testName, config)
}

// GetBaselineMetadata gets metadata for a baseline
func (vrt *VisualRegressionTester) GetBaselineMetadata(testName string, config *VisualTestConfig) (*BaselineMetadata, error) {
	return vrt.baseline.GetBaselineMetadata(testName, config)
}

// UpdateBaseline updates a baseline image
func (vrt *VisualRegressionTester) UpdateBaseline(testName string, screenshot []byte, metadata *BaselineMetadata, config *VisualTestConfig) error {
	return vrt.baseline.UpdateBaseline(testName, screenshot, metadata, config)
}

// DeleteBaseline deletes a baseline image
func (vrt *VisualRegressionTester) DeleteBaseline(testName string, config *VisualTestConfig) error {
	return vrt.baseline.DeleteBaseline(testName, config)
}

// ListBaselines lists all baselines
func (vrt *VisualRegressionTester) ListBaselines(config *VisualTestConfig) ([]string, error) {
	return vrt.baseline.ListBaselines(config)
}

// GenerateReport generates a visual test report
func (vrt *VisualRegressionTester) GenerateReport(results []*TestResult, config *VisualTestConfig) (*TestReport, error) {
	report := &TestReport{
		Title:       "Visual Regression Test Report",
		GeneratedAt: time.Now(),
		Environment: config.BaselineDir,
		Config:      config,
		Metadata:    make(map[string]interface{}),
	}

	// Group results by test suite
	suiteMap := make(map[string][]*TestResult)
	for _, result := range results {
		suite := result.TestSuite
		if suite == "" {
			suite = "Default"
		}
		suiteMap[suite] = append(suiteMap[suite], result)
	}

	// Create suite results
	var totalTests, passedTests, failedTests, skippedTests int
	var totalDuration time.Duration

	for suiteName, suiteResults := range suiteMap {
		suiteResult := &TestSuiteResult{
			SuiteName:   suiteName,
			Results:     suiteResults,
			Timestamp:   time.Now(),
			Environment: config.BaselineDir,
			Config:      config,
			Metadata:    make(map[string]interface{}),
		}

		// Calculate suite summary
		var suitePassed, suiteFailed, suiteSkipped int
		var suiteDuration time.Duration

		for _, result := range suiteResults {
			suiteDuration += result.Duration
			if result.Passed {
				suitePassed++
			} else {
				suiteFailed++
			}
		}

		suiteResult.Duration = suiteDuration
		suiteResult.Passed = suiteFailed == 0
		suiteResult.Summary = TestSummary{
			TotalTests:      len(suiteResults),
			PassedTests:     suitePassed,
			FailedTests:     suiteFailed,
			SkippedTests:    suiteSkipped,
			PassRate:        float64(suitePassed) / float64(len(suiteResults)),
			TotalDuration:   suiteDuration,
			AverageDuration: suiteDuration / time.Duration(len(suiteResults)),
		}

		report.SuiteResults = append(report.SuiteResults, suiteResult)

		totalTests += len(suiteResults)
		passedTests += suitePassed
		failedTests += suiteFailed
		skippedTests += suiteSkipped
		totalDuration += suiteDuration
	}

	// Calculate overall summary
	report.Summary = TestSummary{
		TotalTests:      totalTests,
		PassedTests:     passedTests,
		FailedTests:     failedTests,
		SkippedTests:    skippedTests,
		PassRate:        float64(passedTests) / float64(totalTests),
		TotalDuration:   totalDuration,
		AverageDuration: totalDuration / time.Duration(totalTests),
	}

	return report, nil
}

// Helper methods

// ensureDirectories ensures all required directories exist
func (vrt *VisualRegressionTester) ensureDirectories(config *VisualTestConfig) error {
	dirs := []string{
		config.BaselineDir,
		config.ActualDir,
		config.DiffDir,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return errors.Wrapf(err, "creating directory %s", dir)
		}
	}

	return nil
}

// saveDiffImage saves a diff image to the filesystem
func (vrt *VisualRegressionTester) saveDiffImage(diffImage image.Image, path string) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return errors.Wrap(err, "creating diff directory")
	}

	return vrt.comparison.SaveDiffImage(diffImage, path)
}

// CreateVisualTestFromURL creates a visual test from a URL
func CreateVisualTestFromURL(name, url string, opts *ScreenshotOptions) VisualTest {
	if opts == nil {
		opts = DefaultScreenshotOptions()
	}

	return VisualTest{
		Name:    name,
		URL:     url,
		Options: opts,
		Timeout: 30 * time.Second,
		Retries: 3,
		Tags:    []string{"url-test"},
		Metadata: map[string]interface{}{
			"created_at": time.Now(),
			"type":       "url",
		},
	}
}

// CreateResponsiveVisualTest creates a visual test that runs on multiple viewports
func CreateResponsiveVisualTest(name, url string, viewports []ViewportSize) []VisualTest {
	var tests []VisualTest

	for _, viewport := range viewports {
		opts := DefaultScreenshotOptions()
		opts.ViewportWidth = viewport.Width
		opts.ViewportHeight = viewport.Height

		test := VisualTest{
			Name:     fmt.Sprintf("%s_%s", name, viewport.Name),
			URL:      url,
			Options:  opts,
			Viewport: &viewport,
			Timeout:  30 * time.Second,
			Retries:  3,
			Tags:     []string{"responsive", viewport.Name},
			Metadata: map[string]interface{}{
				"created_at": time.Now(),
				"type":       "responsive",
				"viewport":   viewport.Name,
			},
		}

		tests = append(tests, test)
	}

	return tests
}

// CreateElementVisualTest creates a visual test for a specific element
func CreateElementVisualTest(name, url, selector string) VisualTest {
	opts := DefaultScreenshotOptions()
	opts.Selector = selector

	return VisualTest{
		Name:    name,
		URL:     url,
		Options: opts,
		Timeout: 30 * time.Second,
		Retries: 3,
		Tags:    []string{"element-test"},
		Metadata: map[string]interface{}{
			"created_at": time.Now(),
			"type":       "element",
			"selector":   selector,
		},
	}
}
