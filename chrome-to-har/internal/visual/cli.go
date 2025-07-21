package visual

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/tmc/misc/chrome-to-har/internal/browser"
	"github.com/tmc/misc/chrome-to-har/internal/chromeprofiles"
)

// CLIConfig holds configuration for CLI commands
type CLIConfig struct {
	BaselineDir         string
	ActualDir           string
	DiffDir             string
	ReportDir           string
	ConfigFile          string
	Verbose             bool
	OutputFormat        string
	Headless            bool
	UseProfile          bool
	ProfileName         string
	ChromePath          string
	Timeout             time.Duration
	Retries             int
	Threshold           float64
	IgnoreAntialiasing  bool
	EnableFuzzyMatching bool
	FuzzyThreshold      float64
	MaxDiffPixels       int
	Quality             int
	Format              string
	FullPage            bool
	ViewportWidth       int
	ViewportHeight      int
	DeviceScaleFactor   float64
	EmulateDevice       string
	WaitForFonts        bool
	WaitForImages       bool
	WaitForAnimations   bool
	HideCursor          bool
	HideScrollbars      bool
	CustomCSS           string
	ExcludeSelectors    []string
	MaskSelectors       []string
	MaskColor           string
}

// VisualCLI provides CLI interface for visual regression testing
type VisualCLI struct {
	config *CLIConfig
	tester *VisualRegressionTester
}

// NewVisualCLI creates a new visual CLI instance
func NewVisualCLI(config *CLIConfig) *VisualCLI {
	return &VisualCLI{
		config: config,
		tester: NewVisualRegressionTester(config.Verbose),
	}
}

// CaptureCommand captures a screenshot and saves it as a baseline
func (vcli *VisualCLI) CaptureCommand(ctx context.Context, testName, url string) error {
	if vcli.config.Verbose {
		fmt.Printf("Capturing baseline for test: %s\n", testName)
		fmt.Printf("URL: %s\n", url)
	}

	// Create browser instance
	browser, err := vcli.createBrowser(ctx)
	if err != nil {
		return errors.Wrap(err, "creating browser")
	}
	defer browser.Close()

	// Launch browser
	if err := browser.Launch(ctx); err != nil {
		return errors.Wrap(err, "launching browser")
	}

	// Get current page
	page := browser.GetCurrentPage()
	if page == nil {
		return errors.New("failed to get current page")
	}

	// Navigate to URL
	if err := page.Navigate(url); err != nil {
		return errors.Wrap(err, "navigating to URL")
	}

	// Create screenshot options
	opts := vcli.createScreenshotOptions()

	// Capture screenshot
	screenshot, err := vcli.tester.CaptureScreenshot(ctx, page, opts)
	if err != nil {
		return errors.Wrap(err, "capturing screenshot")
	}

	// Create baseline metadata
	metadata := &BaselineMetadata{
		Version:     "1.0.0",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		URL:         url,
		TestName:    testName,
		Environment: vcli.config.BaselineDir,
		ViewportSize: ViewportSize{
			Width:  vcli.config.ViewportWidth,
			Height: vcli.config.ViewportHeight,
		},
	}

	// Save baseline
	visualConfig := vcli.createVisualTestConfig()
	if err := vcli.tester.SaveBaseline(testName, screenshot, metadata, visualConfig); err != nil {
		return errors.Wrap(err, "saving baseline")
	}

	if vcli.config.Verbose {
		fmt.Printf("Baseline saved successfully: %s\n", testName)
	}

	return nil
}

// TestCommand runs a visual regression test
func (vcli *VisualCLI) TestCommand(ctx context.Context, testName, url string) error {
	if vcli.config.Verbose {
		fmt.Printf("Running visual test: %s\n", testName)
		fmt.Printf("URL: %s\n", url)
	}

	// Create browser instance
	browser, err := vcli.createBrowser(ctx)
	if err != nil {
		return errors.Wrap(err, "creating browser")
	}
	defer browser.Close()

	// Launch browser
	if err := browser.Launch(ctx); err != nil {
		return errors.Wrap(err, "launching browser")
	}

	// Get current page
	page := browser.GetCurrentPage()
	if page == nil {
		return errors.New("failed to get current page")
	}

	// Navigate to URL
	if err := page.Navigate(url); err != nil {
		return errors.Wrap(err, "navigating to URL")
	}

	// Run visual test
	visualConfig := vcli.createVisualTestConfig()
	opts := vcli.createScreenshotOptions()
	
	result, err := vcli.tester.RunVisualTest(ctx, testName, page, visualConfig, opts)
	if err != nil {
		return errors.Wrap(err, "running visual test")
	}

	// Output result
	if err := vcli.outputResult(result); err != nil {
		return errors.Wrap(err, "outputting result")
	}

	// Exit with appropriate code
	if !result.Passed {
		os.Exit(1)
	}

	return nil
}

// CompareCommand compares two screenshots
func (vcli *VisualCLI) CompareCommand(ctx context.Context, baseline, actual string) error {
	if vcli.config.Verbose {
		fmt.Printf("Comparing images:\n")
		fmt.Printf("Baseline: %s\n", baseline)
		fmt.Printf("Actual: %s\n", actual)
	}

	// Read images
	baselineData, err := os.ReadFile(baseline)
	if err != nil {
		return errors.Wrap(err, "reading baseline image")
	}

	actualData, err := os.ReadFile(actual)
	if err != nil {
		return errors.Wrap(err, "reading actual image")
	}

	// Compare images
	visualConfig := vcli.createVisualTestConfig()
	comparison, err := vcli.tester.CompareImages(baselineData, actualData, visualConfig)
	if err != nil {
		return errors.Wrap(err, "comparing images")
	}

	// Output comparison result
	if err := vcli.outputComparison(comparison); err != nil {
		return errors.Wrap(err, "outputting comparison result")
	}

	// Exit with appropriate code
	if !comparison.Passed {
		os.Exit(1)
	}

	return nil
}

// ListCommand lists all available baselines
func (vcli *VisualCLI) ListCommand(ctx context.Context) error {
	visualConfig := vcli.createVisualTestConfig()
	baselines, err := vcli.tester.ListBaselines(visualConfig)
	if err != nil {
		return errors.Wrap(err, "listing baselines")
	}

	if vcli.config.Verbose {
		fmt.Printf("Found %d baselines:\n", len(baselines))
	}

	for _, baseline := range baselines {
		if vcli.config.OutputFormat == "json" {
			metadata, err := vcli.tester.GetBaselineMetadata(baseline, visualConfig)
			if err == nil {
				data, _ := json.MarshalIndent(metadata, "", "  ")
				fmt.Printf("%s\n", data)
			} else {
				fmt.Printf(`{"name": "%s", "error": "%s"}`, baseline, err.Error())
				fmt.Println()
			}
		} else {
			fmt.Printf("%s\n", baseline)
		}
	}

	return nil
}

// UpdateCommand updates a baseline
func (vcli *VisualCLI) UpdateCommand(ctx context.Context, testName, url string) error {
	if vcli.config.Verbose {
		fmt.Printf("Updating baseline for test: %s\n", testName)
		fmt.Printf("URL: %s\n", url)
	}

	// Create browser instance
	browser, err := vcli.createBrowser(ctx)
	if err != nil {
		return errors.Wrap(err, "creating browser")
	}
	defer browser.Close()

	// Launch browser
	if err := browser.Launch(ctx); err != nil {
		return errors.Wrap(err, "launching browser")
	}

	// Get current page
	page := browser.GetCurrentPage()
	if page == nil {
		return errors.New("failed to get current page")
	}

	// Navigate to URL
	if err := page.Navigate(url); err != nil {
		return errors.Wrap(err, "navigating to URL")
	}

	// Create screenshot options
	opts := vcli.createScreenshotOptions()

	// Capture screenshot
	screenshot, err := vcli.tester.CaptureScreenshot(ctx, page, opts)
	if err != nil {
		return errors.Wrap(err, "capturing screenshot")
	}

	// Update baseline metadata
	metadata := &BaselineMetadata{
		UpdatedAt:   time.Now(),
		URL:         url,
		TestName:    testName,
		Environment: vcli.config.BaselineDir,
		ViewportSize: ViewportSize{
			Width:  vcli.config.ViewportWidth,
			Height: vcli.config.ViewportHeight,
		},
	}

	// Update baseline
	visualConfig := vcli.createVisualTestConfig()
	if err := vcli.tester.UpdateBaseline(testName, screenshot, metadata, visualConfig); err != nil {
		return errors.Wrap(err, "updating baseline")
	}

	if vcli.config.Verbose {
		fmt.Printf("Baseline updated successfully: %s\n", testName)
	}

	return nil
}

// DeleteCommand deletes a baseline
func (vcli *VisualCLI) DeleteCommand(ctx context.Context, testName string) error {
	if vcli.config.Verbose {
		fmt.Printf("Deleting baseline: %s\n", testName)
	}

	visualConfig := vcli.createVisualTestConfig()
	if err := vcli.tester.DeleteBaseline(testName, visualConfig); err != nil {
		return errors.Wrap(err, "deleting baseline")
	}

	if vcli.config.Verbose {
		fmt.Printf("Baseline deleted successfully: %s\n", testName)
	}

	return nil
}

// ResponsiveCommand runs responsive visual tests
func (vcli *VisualCLI) ResponsiveCommand(ctx context.Context, testName, url string) error {
	if vcli.config.Verbose {
		fmt.Printf("Running responsive visual test: %s\n", testName)
		fmt.Printf("URL: %s\n", url)
	}

	// Create responsive tester
	responsiveTester := NewResponsiveVisualTester(vcli.config.Verbose)
	
	// Create responsive config
	visualConfig := vcli.createVisualTestConfig()
	responsiveConfig := CreateResponsiveTestConfig(visualConfig)

	// Run responsive test
	result, err := responsiveTester.RunResponsiveTest(ctx, testName, url, responsiveConfig)
	if err != nil {
		return errors.Wrap(err, "running responsive test")
	}

	// Output result
	if err := vcli.outputResponsiveResult(result); err != nil {
		return errors.Wrap(err, "outputting responsive result")
	}

	// Exit with appropriate code
	if !result.Passed {
		os.Exit(1)
	}

	return nil
}

// ReportCommand generates a visual regression report
func (vcli *VisualCLI) ReportCommand(ctx context.Context, reportPath string) error {
	if vcli.config.Verbose {
		fmt.Printf("Generating visual regression report: %s\n", reportPath)
	}

	// This would load test results and generate a report
	// For now, just create a placeholder
	report := &TestReport{
		Title:       "Visual Regression Test Report",
		GeneratedAt: time.Now(),
		Environment: vcli.config.BaselineDir,
		Config:      vcli.createVisualTestConfig(),
		SuiteResults: []*TestSuiteResult{},
	}

	// Save report
	if err := vcli.saveReport(report, reportPath); err != nil {
		return errors.Wrap(err, "saving report")
	}

	if vcli.config.Verbose {
		fmt.Printf("Report saved successfully: %s\n", reportPath)
	}

	return nil
}

// Helper methods

// createBrowser creates a browser instance with the configured options
func (vcli *VisualCLI) createBrowser(ctx context.Context) (*browser.Browser, error) {
	var profileMgr chromeprofiles.ProfileManager
	if vcli.config.UseProfile {
		profileMgr = chromeprofiles.NewChromeProfileManager()
	}

	opts := []browser.Option{
		browser.WithHeadless(vcli.config.Headless),
		browser.WithVerbose(vcli.config.Verbose),
	}

	if vcli.config.ChromePath != "" {
		opts = append(opts, browser.WithChromePath(vcli.config.ChromePath))
	}

	if vcli.config.UseProfile && vcli.config.ProfileName != "" {
		opts = append(opts, browser.WithProfile(vcli.config.ProfileName))
	}

	return browser.New(ctx, profileMgr, opts...)
}

// createScreenshotOptions creates screenshot options from CLI config
func (vcli *VisualCLI) createScreenshotOptions() *ScreenshotOptions {
	opts := DefaultScreenshotOptions()
	
	opts.FullPage = vcli.config.FullPage
	opts.ViewportWidth = vcli.config.ViewportWidth
	opts.ViewportHeight = vcli.config.ViewportHeight
	opts.Quality = vcli.config.Quality
	opts.Format = vcli.config.Format
	opts.DeviceScaleFactor = vcli.config.DeviceScaleFactor
	opts.EmulateDevice = vcli.config.EmulateDevice
	opts.WaitForFonts = vcli.config.WaitForFonts
	opts.WaitForImages = vcli.config.WaitForImages
	opts.WaitForAnimations = vcli.config.WaitForAnimations
	opts.HideCursor = vcli.config.HideCursor
	opts.HideScrollbars = vcli.config.HideScrollbars
	opts.CustomCSS = vcli.config.CustomCSS
	opts.ExcludeSelectors = vcli.config.ExcludeSelectors
	opts.MaskSelectors = vcli.config.MaskSelectors
	opts.MaskColor = vcli.config.MaskColor

	return opts
}

// createVisualTestConfig creates visual test config from CLI config
func (vcli *VisualCLI) createVisualTestConfig() *VisualTestConfig {
	config := DefaultConfig()
	
	config.BaselineDir = vcli.config.BaselineDir
	config.ActualDir = vcli.config.ActualDir
	config.DiffDir = vcli.config.DiffDir
	config.Threshold = vcli.config.Threshold
	config.IgnoreAntialiasing = vcli.config.IgnoreAntialiasing
	config.EnableFuzzyMatching = vcli.config.EnableFuzzyMatching
	config.FuzzyThreshold = vcli.config.FuzzyThreshold
	config.MaxDiffPixels = vcli.config.MaxDiffPixels
	config.Quality = vcli.config.Quality
	config.Format = vcli.config.Format
	config.RetryCount = vcli.config.Retries
	config.RetryDelay = time.Second

	return config
}

// outputResult outputs a test result
func (vcli *VisualCLI) outputResult(result *TestResult) error {
	switch vcli.config.OutputFormat {
	case "json":
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return errors.Wrap(err, "marshaling result")
		}
		fmt.Printf("%s\n", data)
	default:
		status := "PASSED"
		if !result.Passed {
			status = "FAILED"
		}
		
		fmt.Printf("Test: %s\n", result.TestName)
		fmt.Printf("Status: %s\n", status)
		fmt.Printf("Duration: %.2fs\n", result.Duration.Seconds())
		
		if result.ComparisonResult != nil {
			fmt.Printf("Difference: %.2f%%\n", result.ComparisonResult.DifferencePercentage)
			fmt.Printf("Different pixels: %d/%d\n", result.ComparisonResult.DiffPixels, result.ComparisonResult.TotalPixels)
			fmt.Printf("Matching score: %.2f\n", result.ComparisonResult.MatchingScore)
		}
		
		if result.Error != nil {
			fmt.Printf("Error: %s\n", result.Error.Error())
		}
	}
	
	return nil
}

// outputComparison outputs a comparison result
func (vcli *VisualCLI) outputComparison(comparison *ComparisonResult) error {
	switch vcli.config.OutputFormat {
	case "json":
		data, err := json.MarshalIndent(comparison, "", "  ")
		if err != nil {
			return errors.Wrap(err, "marshaling comparison")
		}
		fmt.Printf("%s\n", data)
	default:
		status := "PASSED"
		if !comparison.Passed {
			status = "FAILED"
		}
		
		fmt.Printf("Comparison: %s\n", status)
		fmt.Printf("Difference: %.2f%%\n", comparison.DifferencePercentage)
		fmt.Printf("Different pixels: %d/%d\n", comparison.DiffPixels, comparison.TotalPixels)
		fmt.Printf("Matching score: %.2f\n", comparison.MatchingScore)
		fmt.Printf("Diff regions: %d\n", len(comparison.Regions))
	}
	
	return nil
}

// outputResponsiveResult outputs a responsive test result
func (vcli *VisualCLI) outputResponsiveResult(result *ResponsiveTestResult) error {
	switch vcli.config.OutputFormat {
	case "json":
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return errors.Wrap(err, "marshaling responsive result")
		}
		fmt.Printf("%s\n", data)
	default:
		status := "PASSED"
		if !result.Passed {
			status = "FAILED"
		}
		
		fmt.Printf("Responsive Test: %s\n", result.TestName)
		fmt.Printf("Status: %s\n", status)
		fmt.Printf("Duration: %.2fs\n", result.Duration.Seconds())
		fmt.Printf("Viewports: %d/%d passed\n", result.Summary.PassedViewports, result.Summary.TotalViewports)
		
		if result.Summary.WorstViewport != "" {
			fmt.Printf("Worst viewport: %s (%.2f%% difference)\n", result.Summary.WorstViewport, result.Summary.WorstDifference)
		}
		
		if result.Summary.BestViewport != "" {
			fmt.Printf("Best viewport: %s (%.2f%% difference)\n", result.Summary.BestViewport, result.Summary.BestDifference)
		}
		
		fmt.Printf("Average difference: %.2f%%\n", result.Summary.AverageDifference)
		
		// Show details for each viewport
		if vcli.config.Verbose {
			fmt.Printf("\nViewport details:\n")
			for viewport, testResult := range result.Results {
				status := "PASSED"
				diff := 0.0
				if testResult.ComparisonResult != nil {
					diff = testResult.ComparisonResult.DifferencePercentage
				}
				if !testResult.Passed {
					status = "FAILED"
				}
				fmt.Printf("  %s: %s (%.2f%%)\n", viewport, status, diff)
			}
		}
	}
	
	return nil
}

// saveReport saves a test report
func (vcli *VisualCLI) saveReport(report *TestReport, path string) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return errors.Wrap(err, "creating report directory")
	}

	var data []byte
	var err error

	if strings.HasSuffix(path, ".json") {
		data, err = json.MarshalIndent(report, "", "  ")
	} else {
		// Generate HTML report
		data, err = vcli.generateHTMLReport(report)
	}

	if err != nil {
		return errors.Wrap(err, "generating report data")
	}

	return os.WriteFile(path, data, 0644)
}

// generateHTMLReport generates an HTML report
func (vcli *VisualCLI) generateHTMLReport(report *TestReport) ([]byte, error) {
	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>%s</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .header { background: #f5f5f5; padding: 20px; border-radius: 5px; }
        .summary { margin: 20px 0; }
        .test-result { margin: 10px 0; padding: 10px; border: 1px solid #ddd; border-radius: 5px; }
        .passed { background: #d4edda; }
        .failed { background: #f8d7da; }
        .timestamp { color: #666; font-size: 0.9em; }
    </style>
</head>
<body>
    <div class="header">
        <h1>%s</h1>
        <p class="timestamp">Generated: %s</p>
        <p>Environment: %s</p>
    </div>
    
    <div class="summary">
        <h2>Summary</h2>
        <p>Total Tests: %d</p>
        <p>Passed: %d</p>
        <p>Failed: %d</p>
        <p>Pass Rate: %.1f%%</p>
    </div>
    
    <div class="results">
        <h2>Test Results</h2>
        <!-- Test results would be populated here -->
    </div>
</body>
</html>`,
		report.Title,
		report.Title,
		report.GeneratedAt.Format("2006-01-02 15:04:05"),
		report.Environment,
		report.Summary.TotalTests,
		report.Summary.PassedTests,
		report.Summary.FailedTests,
		report.Summary.PassRate*100,
	)

	return []byte(html), nil
}

// LoadConfig loads CLI configuration from file
func LoadConfig(configPath string) (*CLIConfig, error) {
	if configPath == "" {
		return DefaultCLIConfig(), nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, errors.Wrap(err, "reading config file")
	}

	var config CLIConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, errors.Wrap(err, "parsing config file")
	}

	return &config, nil
}

// SaveConfig saves CLI configuration to file
func SaveConfig(config *CLIConfig, configPath string) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return errors.Wrap(err, "marshaling config")
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return errors.Wrap(err, "creating config directory")
	}

	return os.WriteFile(configPath, data, 0644)
}

// DefaultCLIConfig returns default CLI configuration
func DefaultCLIConfig() *CLIConfig {
	return &CLIConfig{
		BaselineDir:         "visual-baselines",
		ActualDir:           "visual-actual",
		DiffDir:             "visual-diffs",
		ReportDir:           "visual-reports",
		Verbose:             false,
		OutputFormat:        "text",
		Headless:            true,
		UseProfile:          false,
		ProfileName:         "",
		ChromePath:          "",
		Timeout:             30 * time.Second,
		Retries:             3,
		Threshold:           0.1,
		IgnoreAntialiasing:  true,
		EnableFuzzyMatching: true,
		FuzzyThreshold:      0.05,
		MaxDiffPixels:       1000,
		Quality:             90,
		Format:              "png",
		FullPage:            false,
		ViewportWidth:       1920,
		ViewportHeight:      1080,
		DeviceScaleFactor:   1.0,
		EmulateDevice:       "",
		WaitForFonts:        true,
		WaitForImages:       true,
		WaitForAnimations:   true,
		HideCursor:          true,
		HideScrollbars:      true,
		CustomCSS:           "",
		ExcludeSelectors:    []string{},
		MaskSelectors:       []string{},
		MaskColor:           "#000000",
	}
}