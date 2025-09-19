package differential

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chromedp/cdproto/har"
)

func TestCaptureManager(t *testing.T) {
	t.Parallel()
	// Create temporary directory for testing
	tmpDir := filepath.Join(os.TempDir(), "chrome-to-har-test")
	defer os.RemoveAll(tmpDir)

	// Create capture manager
	cm, err := NewCaptureManager(tmpDir, true)
	if err != nil {
		t.Fatalf("Failed to create capture manager: %v", err)
	}

	// Test creating a capture
	labels := map[string]string{
		"test": "true",
		"env":  "development",
	}

	capture, err := cm.CreateCapture("test-capture", "https://example.com", "Test capture", labels)
	if err != nil {
		t.Fatalf("Failed to create capture: %v", err)
	}

	if capture.Name != "test-capture" {
		t.Errorf("Expected name 'test-capture', got %s", capture.Name)
	}

	if capture.URL != "https://example.com" {
		t.Errorf("Expected URL 'https://example.com', got %s", capture.URL)
	}

	if capture.Status != CaptureStatusPending {
		t.Errorf("Expected status 'pending', got %s", capture.Status)
	}

	// Test listing captures
	captures := cm.ListCaptures()
	if len(captures) != 1 {
		t.Errorf("Expected 1 capture, got %d", len(captures))
	}

	// Test finding captures by label
	found := cm.FindCapturesByLabel("test", "true")
	if len(found) != 1 {
		t.Errorf("Expected 1 capture with label 'test=true', got %d", len(found))
	}

	// Test completing a capture
	testHAR := createTestHAR()
	err = cm.CompleteCapture(capture.ID, testHAR)
	if err != nil {
		t.Fatalf("Failed to complete capture: %v", err)
	}

	// Check that the capture is now completed
	updated, err := cm.GetCapture(capture.ID)
	if err != nil {
		t.Fatalf("Failed to get capture: %v", err)
	}

	if updated.Status != CaptureStatusCompleted {
		t.Errorf("Expected status 'completed', got %s", updated.Status)
	}

	if updated.EntryCount != 2 {
		t.Errorf("Expected 2 entries, got %d", updated.EntryCount)
	}

	// Test loading HAR
	loadedHAR, err := cm.LoadHAR(capture.ID)
	if err != nil {
		t.Fatalf("Failed to load HAR: %v", err)
	}

	if len(loadedHAR.Log.Entries) != 2 {
		t.Errorf("Expected 2 entries in loaded HAR, got %d", len(loadedHAR.Log.Entries))
	}

	// Test deleting capture
	err = cm.DeleteCapture(capture.ID)
	if err != nil {
		t.Fatalf("Failed to delete capture: %v", err)
	}

	// Check that it's gone
	_, err = cm.GetCapture(capture.ID)
	if err == nil {
		t.Error("Expected error when getting deleted capture, got nil")
	}
}

func TestDiffEngine(t *testing.T) {
	t.Parallel()
	de := NewDiffEngine(true)

	// Create test HAR data
	baselineHAR := createTestHAR()
	compareHAR := createTestHAR()

	// Modify compare HAR to create differences
	compareHAR.Log.Entries = append(compareHAR.Log.Entries, &har.Entry{
		Request: &har.Request{
			Method: "GET",
			URL:    "https://example.com/new-resource",
		},
		Response: &har.Response{
			Status:     200,
			StatusText: "OK",
			Content: &har.Content{
				Size: 1024,
			},
		},
		Time: 150.0,
	})

	// Create metadata
	baselineMetadata := &CaptureMetadata{
		ID:         "baseline-1",
		Name:       "baseline",
		URL:        "https://example.com",
		Timestamp:  time.Now(),
		EntryCount: 2,
	}

	compareMetadata := &CaptureMetadata{
		ID:         "compare-1",
		Name:       "compare",
		URL:        "https://example.com",
		Timestamp:  time.Now(),
		EntryCount: 3,
	}

	// Compare captures
	result, err := de.CompareCaptures(baselineMetadata, compareMetadata, baselineHAR, compareHAR)
	if err != nil {
		t.Fatalf("Failed to compare captures: %v", err)
	}

	if result.Summary.AddedRequests != 1 {
		t.Errorf("Expected 1 added request, got %d", result.Summary.AddedRequests)
	}

	if len(result.NetworkDiffs) != 1 {
		t.Errorf("Expected 1 network diff, got %d", len(result.NetworkDiffs))
	}

	if result.NetworkDiffs[0].Type != DiffTypeAdded {
		t.Errorf("Expected diff type 'added', got %s", result.NetworkDiffs[0].Type)
	}
}

func TestReportGenerator(t *testing.T) {
	t.Parallel()
	rg := NewReportGenerator(true)

	// Create test diff result
	result := &DiffResult{
		BaselineCapture: &CaptureMetadata{
			ID:   "baseline-1",
			Name: "baseline",
			URL:  "https://example.com",
		},
		CompareCapture: &CaptureMetadata{
			ID:   "compare-1",
			Name: "compare",
			URL:  "https://example.com",
		},
		Summary: &DiffSummary{
			TotalRequests:     3,
			AddedRequests:     1,
			RemovedRequests:   0,
			ModifiedRequests:  0,
			UnchangedRequests: 2,
		},
		NetworkDiffs: []*NetworkDiff{
			{
				Type:   DiffTypeAdded,
				URL:    "https://example.com/new-resource",
				Method: "GET",
				Compare: &NetworkReq{
					Method: "GET",
					URL:    "https://example.com/new-resource",
					Status: 200,
					Size:   1024,
				},
				Significance: DiffSignificanceMedium,
			},
		},
		Timestamp: time.Now(),
	}

	// Test JSON report
	tmpDir := filepath.Join(os.TempDir(), "chrome-to-har-test-reports")
	defer os.RemoveAll(tmpDir)
	os.MkdirAll(tmpDir, 0755)

	jsonPath := filepath.Join(tmpDir, "test-report.json")
	options := &ReportOptions{
		Format:          ReportFormatJSON,
		OutputPath:      jsonPath,
		IncludeDetails:  true,
		MinSignificance: DiffSignificanceLow,
	}

	err := rg.GenerateReport(result, options)
	if err != nil {
		t.Fatalf("Failed to generate JSON report: %v", err)
	}

	// Check that file was created
	if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
		t.Error("JSON report file was not created")
	}

	// Test HTML report
	htmlPath := filepath.Join(tmpDir, "test-report.html")
	options.Format = ReportFormatHTML
	options.OutputPath = htmlPath
	options.Title = "Test Report"
	options.Description = "This is a test report"

	err = rg.GenerateReport(result, options)
	if err != nil {
		t.Fatalf("Failed to generate HTML report: %v", err)
	}

	// Check that file was created
	if _, err := os.Stat(htmlPath); os.IsNotExist(err) {
		t.Error("HTML report file was not created")
	}

	// Test text report
	textPath := filepath.Join(tmpDir, "test-report.txt")
	options.Format = ReportFormatText
	options.OutputPath = textPath

	err = rg.GenerateReport(result, options)
	if err != nil {
		t.Fatalf("Failed to generate text report: %v", err)
	}

	// Check that file was created
	if _, err := os.Stat(textPath); os.IsNotExist(err) {
		t.Error("Text report file was not created")
	}
}

func TestStateTracker(t *testing.T) {
	t.Parallel()
	// Note: This test would require a real Chrome context to work properly
	// For now, we'll test the basic functionality without Chrome

	options := &StateTrackingOptions{
		TrackDOM:          true,
		TrackLocalStorage: true,
		TrackCookies:      true,
		TrackViewport:     true,
		TrackPerformance:  true,
		MaxSnapshots:      10,
	}

	st := NewStateTracker(true, options)

	// Test that state tracker is initialized correctly
	if st.options.MaxSnapshots != 10 {
		t.Errorf("Expected max snapshots 10, got %d", st.options.MaxSnapshots)
	}

	if !st.options.TrackDOM {
		t.Error("Expected DOM tracking to be enabled")
	}

	// Test state comparison
	state1 := &PageState{
		ID:        "state-1",
		URL:       "https://example.com",
		Title:     "Example",
		Timestamp: time.Now(),
		DOM: &DOMState{
			ElementCount:  100,
			ContentLength: 5000,
			Checksum:      "abc123",
		},
	}

	state2 := &PageState{
		ID:        "state-2",
		URL:       "https://example.com/page2",
		Title:     "Example Page 2",
		Timestamp: time.Now(),
		DOM: &DOMState{
			ElementCount:  120,
			ContentLength: 6000,
			Checksum:      "def456",
		},
	}

	differences := st.CompareStates(state1, state2)
	if len(differences) == 0 {
		t.Error("Expected differences between states, got none")
	}

	// Check that URL difference is detected
	found := false
	for _, diff := range differences {
		if diff == "URL: https://example.com -> https://example.com/page2" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected URL difference to be detected")
	}
}

func TestDifferentialController(t *testing.T) {
	t.Parallel()
	// Create temporary directory for testing
	tmpDir := filepath.Join(os.TempDir(), "chrome-to-har-controller-test")
	defer os.RemoveAll(tmpDir)

	options := &DifferentialOptions{
		WorkDir:          tmpDir,
		TrackResources:   true,
		TrackPerformance: true,
		TrackStates:      false,
		Verbose:          true,
	}

	controller, err := NewDifferentialController(options)
	if err != nil {
		t.Fatalf("Failed to create differential controller: %v", err)
	}
	defer controller.Cleanup()

	// Test creating a baseline capture
	ctx := context.Background()
	labels := map[string]string{"test": "true"}

	capture, err := controller.CreateBaselineCapture(ctx, "test-baseline", "https://example.com", "Test baseline", labels)
	if err != nil {
		t.Fatalf("Failed to create baseline capture: %v", err)
	}

	if capture.Name != "test-baseline" {
		t.Errorf("Expected name 'test-baseline', got %s", capture.Name)
	}

	// Test listing captures
	captures := controller.ListCaptures()
	if len(captures) != 1 {
		t.Errorf("Expected 1 capture, got %d", len(captures))
	}

	// Test finding captures by label
	found := controller.FindCapturesByLabel("test", "true")
	if len(found) != 1 {
		t.Errorf("Expected 1 capture with label 'test=true', got %d", len(found))
	}

	// Test completing the capture
	testHAR := createTestHAR()
	err = controller.CompleteCapture(capture.ID, testHAR)
	if err != nil {
		t.Fatalf("Failed to complete capture: %v", err)
	}

	// Create a second capture for comparison
	capture2, err := controller.CreateCompareCapture(ctx, "test-compare", "https://example.com", "Test compare", labels)
	if err != nil {
		t.Fatalf("Failed to create compare capture: %v", err)
	}

	// Complete the second capture with modified data
	testHAR2 := createTestHAR()
	// Add an extra entry to create a difference
	testHAR2.Log.Entries = append(testHAR2.Log.Entries, &har.Entry{
		Request: &har.Request{
			Method: "GET",
			URL:    "https://example.com/new-resource",
		},
		Response: &har.Response{
			Status:     200,
			StatusText: "OK",
			Content: &har.Content{
				Size: 1024,
			},
		},
		Time: 150.0,
	})

	err = controller.CompleteCapture(capture2.ID, testHAR2)
	if err != nil {
		t.Fatalf("Failed to complete compare capture: %v", err)
	}

	// Test comparison
	result, err := controller.CompareCapturesByID(capture.ID, capture2.ID)
	if err != nil {
		t.Fatalf("Failed to compare captures: %v", err)
	}

	if result.Summary.AddedRequests != 1 {
		t.Errorf("Expected 1 added request, got %d", result.Summary.AddedRequests)
	}

	// Test report generation
	reportPath := filepath.Join(tmpDir, "test-report.json")
	reportOptions := &ReportOptions{
		Format:          ReportFormatJSON,
		OutputPath:      reportPath,
		IncludeDetails:  true,
		MinSignificance: DiffSignificanceLow,
	}

	err = controller.GenerateReport(result, reportOptions)
	if err != nil {
		t.Fatalf("Failed to generate report: %v", err)
	}

	// Check that report file was created
	if _, err := os.Stat(reportPath); os.IsNotExist(err) {
		t.Error("Report file was not created")
	}

	// Test deleting a capture
	err = controller.DeleteCapture(capture.ID)
	if err != nil {
		t.Fatalf("Failed to delete capture: %v", err)
	}

	// Check that it's gone
	captures = controller.ListCaptures()
	if len(captures) != 1 {
		t.Errorf("Expected 1 capture after deletion, got %d", len(captures))
	}
}

// Helper function to create test HAR data
func createTestHAR() *har.HAR {
	return &har.HAR{
		Log: &har.Log{
			Version: "1.2",
			Creator: &har.Creator{
				Name:    "test",
				Version: "1.0",
			},
			Entries: []*har.Entry{
				{
					Request: &har.Request{
						Method: "GET",
						URL:    "https://example.com",
						Headers: []*har.NameValuePair{
							{Name: "User-Agent", Value: "test-agent"},
						},
					},
					Response: &har.Response{
						Status:     200,
						StatusText: "OK",
						Content: &har.Content{
							Size:     2048,
							MimeType: "text/html",
						},
					},
					Time: 100.0,
				},
				{
					Request: &har.Request{
						Method: "GET",
						URL:    "https://example.com/style.css",
					},
					Response: &har.Response{
						Status:     200,
						StatusText: "OK",
						Content: &har.Content{
							Size:     512,
							MimeType: "text/css",
						},
					},
					Time: 50.0,
				},
			},
		},
	}
}
