package differential

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/chromedp/cdproto/har"
	"github.com/pkg/errors"
	"github.com/tmc/misc/chrome-to-har/internal/recorder"
)

// DifferentialController manages the entire differential capture process
type DifferentialController struct {
	captureManager  *CaptureManager
	diffEngine      *DiffEngine
	reportGenerator *ReportGenerator
	stateTracker    *StateTracker
	recorder        *recorder.Recorder
	workDir         string
	verbose         bool
}

// DifferentialOptions configures the differential capture process
type DifferentialOptions struct {
	WorkDir          string                   `json:"work_dir"`
	BaselineName     string                   `json:"baseline_name"`
	BaselineURL      string                   `json:"baseline_url"`
	CompareName      string                   `json:"compare_name"`
	CompareURL       string                   `json:"compare_url"`
	Labels           map[string]string        `json:"labels"`
	TrackResources   bool                     `json:"track_resources"`
	TrackPerformance bool                     `json:"track_performance"`
	TrackStates      bool                     `json:"track_states"`
	StateOptions     *StateTrackingOptions    `json:"state_options"`
	ReportOptions    *ReportOptions           `json:"report_options"`
	AutoCapture      bool                     `json:"auto_capture"`
	CaptureInterval  time.Duration            `json:"capture_interval"`
	MaxCaptures      int                      `json:"max_captures"`
	Verbose          bool                     `json:"verbose"`
}

// NewDifferentialController creates a new differential controller
func NewDifferentialController(options *DifferentialOptions) (*DifferentialController, error) {
	// Create work directory if it doesn't exist
	if options.WorkDir == "" {
		options.WorkDir = filepath.Join(os.TempDir(), "chrome-to-har-diff")
	}
	if err := os.MkdirAll(options.WorkDir, 0755); err != nil {
		return nil, errors.Wrap(err, "creating work directory")
	}

	// Create capture manager
	captureManager, err := NewCaptureManager(options.WorkDir, options.Verbose)
	if err != nil {
		return nil, errors.Wrap(err, "creating capture manager")
	}

	// Create diff engine
	diffEngine := NewDiffEngine(options.Verbose)

	// Create report generator
	reportGenerator := NewReportGenerator(options.Verbose)

	// Create state tracker if enabled
	var stateTracker *StateTracker
	if options.TrackStates {
		stateTracker = NewStateTracker(options.Verbose, options.StateOptions)
	}

	return &DifferentialController{
		captureManager:  captureManager,
		diffEngine:      diffEngine,
		reportGenerator: reportGenerator,
		stateTracker:    stateTracker,
		workDir:         options.WorkDir,
		verbose:         options.Verbose,
	}, nil
}

// CreateBaselineCapture creates a baseline capture for comparison
func (dc *DifferentialController) CreateBaselineCapture(ctx context.Context, name, url, description string, labels map[string]string) (*CaptureMetadata, error) {
	if dc.verbose {
		fmt.Printf("Creating baseline capture: %s\n", name)
	}

	// Create capture metadata
	metadata, err := dc.captureManager.CreateCapture(name, url, description, labels)
	if err != nil {
		return nil, errors.Wrap(err, "creating baseline capture")
	}

	if err := dc.captureManager.StartCapture(metadata.ID); err != nil {
		return nil, errors.Wrap(err, "starting baseline capture")
	}

	return metadata, nil
}

// CreateCompareCapture creates a comparison capture
func (dc *DifferentialController) CreateCompareCapture(ctx context.Context, name, url, description string, labels map[string]string) (*CaptureMetadata, error) {
	if dc.verbose {
		fmt.Printf("Creating compare capture: %s\n", name)
	}

	// Create capture metadata
	metadata, err := dc.captureManager.CreateCapture(name, url, description, labels)
	if err != nil {
		return nil, errors.Wrap(err, "creating compare capture")
	}

	if err := dc.captureManager.StartCapture(metadata.ID); err != nil {
		return nil, errors.Wrap(err, "starting compare capture")
	}

	return metadata, nil
}

// CompleteCapture completes a capture with HAR data
func (dc *DifferentialController) CompleteCapture(captureID string, harData *har.HAR) error {
	if dc.verbose {
		fmt.Printf("Completing capture: %s\n", captureID)
	}

	return dc.captureManager.CompleteCapture(captureID, harData)
}

// CompareCapturesByID compares two captures by their IDs
func (dc *DifferentialController) CompareCapturesByID(baselineID, compareID string) (*DiffResult, error) {
	if dc.verbose {
		fmt.Printf("Comparing captures: %s vs %s\n", baselineID, compareID)
	}

	// Get capture metadata
	baselineMetadata, err := dc.captureManager.GetCapture(baselineID)
	if err != nil {
		return nil, errors.Wrap(err, "getting baseline capture")
	}

	compareMetadata, err := dc.captureManager.GetCapture(compareID)
	if err != nil {
		return nil, errors.Wrap(err, "getting compare capture")
	}

	// Load HAR data
	baselineHAR, err := dc.captureManager.LoadHAR(baselineID)
	if err != nil {
		return nil, errors.Wrap(err, "loading baseline HAR")
	}

	compareHAR, err := dc.captureManager.LoadHAR(compareID)
	if err != nil {
		return nil, errors.Wrap(err, "loading compare HAR")
	}

	// Compare captures
	return dc.diffEngine.CompareCaptures(baselineMetadata, compareMetadata, baselineHAR, compareHAR)
}

// CompareCaptures compares two captures directly
func (dc *DifferentialController) CompareCaptures(baseline, compare *CaptureMetadata, baselineHAR, compareHAR *har.HAR) (*DiffResult, error) {
	return dc.diffEngine.CompareCaptures(baseline, compare, baselineHAR, compareHAR)
}

// GenerateReport generates a diff report
func (dc *DifferentialController) GenerateReport(result *DiffResult, options *ReportOptions) error {
	if dc.verbose {
		fmt.Printf("Generating %s report\n", options.Format)
	}

	return dc.reportGenerator.GenerateReport(result, options)
}

// ListCaptures lists all available captures
func (dc *DifferentialController) ListCaptures() []*CaptureMetadata {
	return dc.captureManager.ListCaptures()
}

// FindCapturesByLabel finds captures with specific label
func (dc *DifferentialController) FindCapturesByLabel(key, value string) []*CaptureMetadata {
	return dc.captureManager.FindCapturesByLabel(key, value)
}

// DeleteCapture deletes a capture
func (dc *DifferentialController) DeleteCapture(captureID string) error {
	return dc.captureManager.DeleteCapture(captureID)
}

// CaptureCurrentState captures the current page state (if state tracking is enabled)
func (dc *DifferentialController) CaptureCurrentState(ctx context.Context, description string, labels map[string]string) (*PageState, error) {
	if dc.stateTracker == nil {
		return nil, fmt.Errorf("state tracking is not enabled")
	}

	return dc.stateTracker.CaptureCurrentState(ctx, description, labels)
}

// RecordInteraction records a user interaction (if state tracking is enabled)
func (dc *DifferentialController) RecordInteraction(ctx context.Context, interactionType InteractionType, target string, data map[string]interface{}) (*InteractionEvent, error) {
	if dc.stateTracker == nil {
		return nil, fmt.Errorf("state tracking is not enabled")
	}

	return dc.stateTracker.RecordInteraction(ctx, interactionType, target, data)
}

// CompleteInteraction completes an interaction (if state tracking is enabled)
func (dc *DifferentialController) CompleteInteraction(ctx context.Context, event *InteractionEvent) error {
	if dc.stateTracker == nil {
		return fmt.Errorf("state tracking is not enabled")
	}

	return dc.stateTracker.CompleteInteraction(ctx, event)
}

// ExportStates exports all captured states to a file
func (dc *DifferentialController) ExportStates(filename string) error {
	if dc.stateTracker == nil {
		return fmt.Errorf("state tracking is not enabled")
	}

	return dc.stateTracker.ExportStates(filename)
}

// CreateFullDiffReport creates a complete diff report including all sections
func (dc *DifferentialController) CreateFullDiffReport(baselineID, compareID string, reportPath string, format ReportFormat) error {
	// Compare captures
	result, err := dc.CompareCapturesByID(baselineID, compareID)
	if err != nil {
		return errors.Wrap(err, "comparing captures")
	}

	// Setup report options
	reportOptions := &ReportOptions{
		Format:          format,
		OutputPath:      reportPath,
		IncludeDetails:  true,
		IncludeGraphs:   true,
		ThemeColor:      "#007bff",
		Title:           "Differential Capture Analysis",
		Description:     fmt.Sprintf("Comparison between %s and %s", result.BaselineCapture.Name, result.CompareCapture.Name),
		ShowUnchanged:   false,
		MinSignificance: DiffSignificanceLow,
	}

	// Generate report
	return dc.GenerateReport(result, reportOptions)
}

// RunDifferentialComparison runs a complete differential comparison workflow
func (dc *DifferentialController) RunDifferentialComparison(ctx context.Context, options *DifferentialOptions) (*DiffResult, error) {
	if dc.verbose {
		fmt.Println("Starting differential comparison workflow")
	}

	// Find or create baseline capture
	var baselineCapture *CaptureMetadata
	if options.BaselineName != "" {
		captures := dc.FindCapturesByLabel("name", options.BaselineName)
		if len(captures) > 0 {
			baselineCapture = captures[0]
			if dc.verbose {
				fmt.Printf("Using existing baseline: %s\n", baselineCapture.Name)
			}
		}
	}

	// If no baseline found, this would require integration with the capture process
	// For now, we'll assume captures are created externally and we're just comparing

	// Find comparison capture
	var compareCapture *CaptureMetadata
	if options.CompareName != "" {
		captures := dc.FindCapturesByLabel("name", options.CompareName)
		if len(captures) > 0 {
			compareCapture = captures[0]
			if dc.verbose {
				fmt.Printf("Using comparison capture: %s\n", compareCapture.Name)
			}
		}
	}

	if baselineCapture == nil || compareCapture == nil {
		return nil, fmt.Errorf("baseline or comparison capture not found")
	}

	// Perform comparison
	result, err := dc.CompareCapturesByID(baselineCapture.ID, compareCapture.ID)
	if err != nil {
		return nil, errors.Wrap(err, "performing comparison")
	}

	// Generate report if requested
	if options.ReportOptions != nil {
		if err := dc.GenerateReport(result, options.ReportOptions); err != nil {
			return nil, errors.Wrap(err, "generating report")
		}
	}

	if dc.verbose {
		fmt.Printf("Differential comparison completed: %d changes detected\n", 
			result.Summary.AddedRequests+result.Summary.RemovedRequests+result.Summary.ModifiedRequests)
	}

	return result, nil
}

// Cleanup cleans up temporary files and resources
func (dc *DifferentialController) Cleanup() error {
	if dc.captureManager != nil {
		return dc.captureManager.Cleanup()
	}
	return nil
}

// GetWorkDir returns the work directory path
func (dc *DifferentialController) GetWorkDir() string {
	return dc.workDir
}

// GetCaptureManager returns the capture manager
func (dc *DifferentialController) GetCaptureManager() *CaptureManager {
	return dc.captureManager
}

// GetDiffEngine returns the diff engine
func (dc *DifferentialController) GetDiffEngine() *DiffEngine {
	return dc.diffEngine
}

// GetReportGenerator returns the report generator
func (dc *DifferentialController) GetReportGenerator() *ReportGenerator {
	return dc.reportGenerator
}

// GetStateTracker returns the state tracker
func (dc *DifferentialController) GetStateTracker() *StateTracker {
	return dc.stateTracker
}