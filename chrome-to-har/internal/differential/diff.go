package differential

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/chromedp/cdproto/har"
)

// DiffResult represents the result of comparing two HAR captures
type DiffResult struct {
	BaselineCapture *CaptureMetadata `json:"baseline_capture"`
	CompareCapture  *CaptureMetadata `json:"compare_capture"`
	Summary         *DiffSummary     `json:"summary"`
	NetworkDiffs    []*NetworkDiff   `json:"network_diffs"`
	ResourceDiffs   []*ResourceDiff  `json:"resource_diffs"`
	PerformanceDiff *PerformanceDiff `json:"performance_diff"`
	Timestamp       time.Time        `json:"timestamp"`
}

// DiffSummary provides high-level statistics about the differences
type DiffSummary struct {
	TotalRequests        int `json:"total_requests"`
	AddedRequests        int `json:"added_requests"`
	RemovedRequests      int `json:"removed_requests"`
	ModifiedRequests     int `json:"modified_requests"`
	UnchangedRequests    int `json:"unchanged_requests"`
	TotalResourceChanges int `json:"total_resource_changes"`
	SignificantChanges   int `json:"significant_changes"`
}

// NetworkDiff represents a difference in network requests
type NetworkDiff struct {
	Type         DiffType         `json:"type"`
	URL          string           `json:"url"`
	Method       string           `json:"method"`
	Baseline     *NetworkReq      `json:"baseline,omitempty"`
	Compare      *NetworkReq      `json:"compare,omitempty"`
	Changes      []string         `json:"changes,omitempty"`
	Significance DiffSignificance `json:"significance"`
}

// NetworkReq represents a simplified network request for comparison
type NetworkReq struct {
	Method       string            `json:"method"`
	URL          string            `json:"url"`
	Status       int64             `json:"status"`
	Size         int64             `json:"size"`
	Time         float64           `json:"time"`
	MimeType     string            `json:"mime_type"`
	Headers      map[string]string `json:"headers"`
	ResponseCode int64             `json:"response_code"`
}

// ResourceDiff represents changes in resource loading
type ResourceDiff struct {
	Type         DiffType         `json:"type"`
	ResourceType string           `json:"resource_type"`
	URL          string           `json:"url"`
	Baseline     *ResourceMetrics `json:"baseline,omitempty"`
	Compare      *ResourceMetrics `json:"compare,omitempty"`
	Changes      []string         `json:"changes,omitempty"`
	Significance DiffSignificance `json:"significance"`
}

// ResourceMetrics represents resource loading metrics
type ResourceMetrics struct {
	LoadTime     float64 `json:"load_time"`
	Size         int64   `json:"size"`
	Compressed   int64   `json:"compressed"`
	CacheStatus  string  `json:"cache_status"`
	ResponseTime float64 `json:"response_time"`
}

// PerformanceDiff represents performance metrics comparison
type PerformanceDiff struct {
	TotalLoadTime   *MetricDiff `json:"total_load_time"`
	TotalSize       *MetricDiff `json:"total_size"`
	RequestCount    *MetricDiff `json:"request_count"`
	AverageResponse *MetricDiff `json:"average_response"`
	LargestRequest  *MetricDiff `json:"largest_request"`
	SlowestRequest  *MetricDiff `json:"slowest_request"`
}

// MetricDiff represents a numerical difference between two metrics
type MetricDiff struct {
	Baseline   float64 `json:"baseline"`
	Compare    float64 `json:"compare"`
	Difference float64 `json:"difference"`
	Percentage float64 `json:"percentage"`
	Improved   bool    `json:"improved"`
}

// DiffType represents the type of difference
type DiffType string

const (
	DiffTypeAdded    DiffType = "added"
	DiffTypeRemoved  DiffType = "removed"
	DiffTypeModified DiffType = "modified"
)

// String returns the string representation of DiffType
func (d DiffType) String() string {
	return string(d)
}

// DiffSignificance represents how significant a change is
type DiffSignificance string

const (
	DiffSignificanceHigh   DiffSignificance = "high"
	DiffSignificanceMedium DiffSignificance = "medium"
	DiffSignificanceLow    DiffSignificance = "low"
)

// String returns the string representation of DiffSignificance
func (d DiffSignificance) String() string {
	return string(d)
}

// DiffEngine compares HAR captures and generates difference reports
type DiffEngine struct {
	verbose bool
}

// NewDiffEngine creates a new diff engine
func NewDiffEngine(verbose bool) *DiffEngine {
	return &DiffEngine{
		verbose: verbose,
	}
}

// CompareCaptures compares two HAR captures and returns differences
func (de *DiffEngine) CompareCaptures(baseline, compare *CaptureMetadata, baselineHAR, compareHAR *har.HAR) (*DiffResult, error) {
	if de.verbose {
		fmt.Printf("Comparing captures: %s vs %s\n", baseline.Name, compare.Name)
	}

	// Extract network requests
	baselineReqs := de.extractNetworkRequests(baselineHAR)
	compareReqs := de.extractNetworkRequests(compareHAR)

	// Generate network diffs
	networkDiffs, summary := de.compareNetworkRequests(baselineReqs, compareReqs)

	// Generate resource diffs
	resourceDiffs := de.compareResources(baselineReqs, compareReqs)

	// Generate performance diff
	performanceDiff := de.comparePerformance(baselineReqs, compareReqs)

	result := &DiffResult{
		BaselineCapture: baseline,
		CompareCapture:  compare,
		Summary:         summary,
		NetworkDiffs:    networkDiffs,
		ResourceDiffs:   resourceDiffs,
		PerformanceDiff: performanceDiff,
		Timestamp:       time.Now(),
	}

	if de.verbose {
		fmt.Printf("Diff complete: %d added, %d removed, %d modified requests\n",
			summary.AddedRequests, summary.RemovedRequests, summary.ModifiedRequests)
	}

	return result, nil
}

// extractNetworkRequests extracts network requests from HAR data
func (de *DiffEngine) extractNetworkRequests(harData *har.HAR) map[string]*NetworkReq {
	requests := make(map[string]*NetworkReq)

	if harData == nil || harData.Log == nil {
		return requests
	}

	for _, entry := range harData.Log.Entries {
		if entry.Request == nil || entry.Response == nil {
			continue
		}

		// Create a unique key for the request
		key := fmt.Sprintf("%s:%s", entry.Request.Method, entry.Request.URL)

		// Extract headers
		headers := make(map[string]string)
		for _, header := range entry.Request.Headers {
			headers[header.Name] = header.Value
		}

		// Calculate response size
		var size int64
		if entry.Response.Content != nil {
			size = entry.Response.Content.Size
		}

		requests[key] = &NetworkReq{
			Method:       entry.Request.Method,
			URL:          entry.Request.URL,
			Status:       entry.Response.Status,
			Size:         size,
			Time:         entry.Time,
			MimeType:     entry.Response.Content.MimeType,
			Headers:      headers,
			ResponseCode: entry.Response.Status,
		}
	}

	return requests
}

// compareNetworkRequests compares network requests between two captures
func (de *DiffEngine) compareNetworkRequests(baseline, compare map[string]*NetworkReq) ([]*NetworkDiff, *DiffSummary) {
	var diffs []*NetworkDiff
	summary := &DiffSummary{}

	// Find added requests
	for key, req := range compare {
		if _, exists := baseline[key]; !exists {
			diffs = append(diffs, &NetworkDiff{
				Type:         DiffTypeAdded,
				URL:          req.URL,
				Method:       req.Method,
				Compare:      req,
				Significance: de.calculateSignificance(req, nil),
			})
			summary.AddedRequests++
		}
	}

	// Find removed requests
	for key, req := range baseline {
		if _, exists := compare[key]; !exists {
			diffs = append(diffs, &NetworkDiff{
				Type:         DiffTypeRemoved,
				URL:          req.URL,
				Method:       req.Method,
				Baseline:     req,
				Significance: de.calculateSignificance(req, nil),
			})
			summary.RemovedRequests++
		}
	}

	// Find modified requests
	for key, baseReq := range baseline {
		if compReq, exists := compare[key]; exists {
			changes := de.findRequestChanges(baseReq, compReq)
			if len(changes) > 0 {
				diffs = append(diffs, &NetworkDiff{
					Type:         DiffTypeModified,
					URL:          baseReq.URL,
					Method:       baseReq.Method,
					Baseline:     baseReq,
					Compare:      compReq,
					Changes:      changes,
					Significance: de.calculateSignificance(baseReq, compReq),
				})
				summary.ModifiedRequests++
			} else {
				summary.UnchangedRequests++
			}
		}
	}

	summary.TotalRequests = len(baseline) + len(compare)
	summary.SignificantChanges = de.countSignificantChanges(diffs)

	return diffs, summary
}

// findRequestChanges identifies specific changes between two requests
func (de *DiffEngine) findRequestChanges(baseline, compare *NetworkReq) []string {
	var changes []string

	if baseline.Status != compare.Status {
		changes = append(changes, fmt.Sprintf("Status: %d -> %d", baseline.Status, compare.Status))
	}

	if baseline.Size != compare.Size {
		changes = append(changes, fmt.Sprintf("Size: %d -> %d bytes", baseline.Size, compare.Size))
	}

	timeDiff := compare.Time - baseline.Time
	if timeDiff > 100 || timeDiff < -100 { // Significant time difference (100ms)
		changes = append(changes, fmt.Sprintf("Time: %.2fms -> %.2fms", baseline.Time, compare.Time))
	}

	if baseline.MimeType != compare.MimeType {
		changes = append(changes, fmt.Sprintf("MimeType: %s -> %s", baseline.MimeType, compare.MimeType))
	}

	// Check for significant header changes
	headerChanges := de.findHeaderChanges(baseline.Headers, compare.Headers)
	changes = append(changes, headerChanges...)

	return changes
}

// findHeaderChanges identifies changes in request headers
func (de *DiffEngine) findHeaderChanges(baseline, compare map[string]string) []string {
	var changes []string

	// Check for important header changes
	importantHeaders := []string{"User-Agent", "Accept", "Content-Type", "Authorization", "Cache-Control"}

	for _, header := range importantHeaders {
		baseValue, baseExists := baseline[header]
		compValue, compExists := compare[header]

		if baseExists && compExists && baseValue != compValue {
			changes = append(changes, fmt.Sprintf("Header %s: %s -> %s", header, baseValue, compValue))
		} else if baseExists && !compExists {
			changes = append(changes, fmt.Sprintf("Header %s: removed", header))
		} else if !baseExists && compExists {
			changes = append(changes, fmt.Sprintf("Header %s: added", header))
		}
	}

	return changes
}

// compareResources compares resource loading between captures
func (de *DiffEngine) compareResources(baseline, compare map[string]*NetworkReq) []*ResourceDiff {
	var diffs []*ResourceDiff

	// Group requests by resource type
	baselineResources := de.groupByResourceType(baseline)
	compareResources := de.groupByResourceType(compare)

	// Compare each resource type
	for resourceType, baseReqs := range baselineResources {
		compReqs := compareResources[resourceType]

		// Calculate metrics for each group
		baseMetrics := de.calculateResourceMetrics(baseReqs)
		compMetrics := de.calculateResourceMetrics(compReqs)

		// Find significant changes
		changes := de.findResourceChanges(baseMetrics, compMetrics)
		if len(changes) > 0 {
			diffs = append(diffs, &ResourceDiff{
				Type:         DiffTypeModified,
				ResourceType: resourceType,
				Baseline:     baseMetrics,
				Compare:      compMetrics,
				Changes:      changes,
				Significance: de.calculateResourceSignificance(baseMetrics, compMetrics),
			})
		}
	}

	return diffs
}

// groupByResourceType groups requests by their resource type
func (de *DiffEngine) groupByResourceType(requests map[string]*NetworkReq) map[string][]*NetworkReq {
	groups := make(map[string][]*NetworkReq)

	for _, req := range requests {
		resourceType := de.getResourceType(req)
		groups[resourceType] = append(groups[resourceType], req)
	}

	return groups
}

// getResourceType determines the resource type from a request
func (de *DiffEngine) getResourceType(req *NetworkReq) string {
	// Parse URL to get file extension
	u, err := url.Parse(req.URL)
	if err != nil {
		return "other"
	}

	path := u.Path
	if strings.Contains(path, ".") {
		parts := strings.Split(path, ".")
		ext := strings.ToLower(parts[len(parts)-1])

		switch ext {
		case "js", "mjs":
			return "script"
		case "css":
			return "stylesheet"
		case "png", "jpg", "jpeg", "gif", "svg", "webp":
			return "image"
		case "woff", "woff2", "ttf", "otf":
			return "font"
		case "json", "xml":
			return "data"
		case "html", "htm":
			return "document"
		default:
			return "other"
		}
	}

	// Check MIME type
	mimeType := strings.ToLower(req.MimeType)
	switch {
	case strings.Contains(mimeType, "javascript"):
		return "script"
	case strings.Contains(mimeType, "css"):
		return "stylesheet"
	case strings.Contains(mimeType, "image"):
		return "image"
	case strings.Contains(mimeType, "font"):
		return "font"
	case strings.Contains(mimeType, "json"), strings.Contains(mimeType, "xml"):
		return "data"
	case strings.Contains(mimeType, "html"):
		return "document"
	default:
		return "other"
	}
}

// calculateResourceMetrics calculates metrics for a group of resources
func (de *DiffEngine) calculateResourceMetrics(requests []*NetworkReq) *ResourceMetrics {
	if len(requests) == 0 {
		return &ResourceMetrics{}
	}

	var totalSize, totalTime, totalResponseTime int64
	var loadTime float64

	for _, req := range requests {
		totalSize += req.Size
		totalTime += int64(req.Time)
		totalResponseTime += int64(req.Time)
		if req.Time > loadTime {
			loadTime = req.Time
		}
	}

	return &ResourceMetrics{
		LoadTime:     loadTime,
		Size:         totalSize,
		ResponseTime: float64(totalResponseTime) / float64(len(requests)),
	}
}

// findResourceChanges identifies changes in resource metrics
func (de *DiffEngine) findResourceChanges(baseline, compare *ResourceMetrics) []string {
	var changes []string

	if baseline.LoadTime != compare.LoadTime {
		changes = append(changes, fmt.Sprintf("Load time: %.2fms -> %.2fms", baseline.LoadTime, compare.LoadTime))
	}

	if baseline.Size != compare.Size {
		changes = append(changes, fmt.Sprintf("Total size: %d -> %d bytes", baseline.Size, compare.Size))
	}

	if baseline.ResponseTime != compare.ResponseTime {
		changes = append(changes, fmt.Sprintf("Average response time: %.2fms -> %.2fms", baseline.ResponseTime, compare.ResponseTime))
	}

	return changes
}

// comparePerformance compares overall performance metrics
func (de *DiffEngine) comparePerformance(baseline, compare map[string]*NetworkReq) *PerformanceDiff {
	baseMetrics := de.calculatePerformanceMetrics(baseline)
	compMetrics := de.calculatePerformanceMetrics(compare)

	return &PerformanceDiff{
		TotalLoadTime:   de.createMetricDiff(baseMetrics["total_load_time"], compMetrics["total_load_time"], false),
		TotalSize:       de.createMetricDiff(baseMetrics["total_size"], compMetrics["total_size"], false),
		RequestCount:    de.createMetricDiff(baseMetrics["request_count"], compMetrics["request_count"], false),
		AverageResponse: de.createMetricDiff(baseMetrics["average_response"], compMetrics["average_response"], false),
		LargestRequest:  de.createMetricDiff(baseMetrics["largest_request"], compMetrics["largest_request"], false),
		SlowestRequest:  de.createMetricDiff(baseMetrics["slowest_request"], compMetrics["slowest_request"], false),
	}
}

// calculatePerformanceMetrics calculates performance metrics for a set of requests
func (de *DiffEngine) calculatePerformanceMetrics(requests map[string]*NetworkReq) map[string]float64 {
	metrics := make(map[string]float64)

	if len(requests) == 0 {
		return metrics
	}

	var totalSize int64
	var maxTime, maxSize float64
	var responseSum float64

	for _, req := range requests {
		totalSize += req.Size
		responseSum += req.Time
		if req.Time > maxTime {
			maxTime = req.Time
		}
		if float64(req.Size) > maxSize {
			maxSize = float64(req.Size)
		}
	}

	count := float64(len(requests))
	metrics["total_load_time"] = maxTime
	metrics["total_size"] = float64(totalSize)
	metrics["request_count"] = count
	metrics["average_response"] = responseSum / count
	metrics["largest_request"] = maxSize
	metrics["slowest_request"] = maxTime

	return metrics
}

// createMetricDiff creates a metric difference object
func (de *DiffEngine) createMetricDiff(baseline, compare float64, lowerIsBetter bool) *MetricDiff {
	diff := compare - baseline
	var percentage float64
	if baseline != 0 {
		percentage = (diff / baseline) * 100
	}

	var improved bool
	if lowerIsBetter {
		improved = diff < 0
	} else {
		improved = diff > 0
	}

	return &MetricDiff{
		Baseline:   baseline,
		Compare:    compare,
		Difference: diff,
		Percentage: percentage,
		Improved:   improved,
	}
}

// calculateSignificance determines the significance of a change
func (de *DiffEngine) calculateSignificance(baseline, compare *NetworkReq) DiffSignificance {
	if baseline == nil || compare == nil {
		return DiffSignificanceMedium
	}

	// Consider status code changes as high significance
	if baseline.Status != compare.Status {
		return DiffSignificanceHigh
	}

	// Consider large size changes as high significance
	sizeDiff := float64(compare.Size - baseline.Size)
	if baseline.Size > 0 {
		sizePercentage := (sizeDiff / float64(baseline.Size)) * 100
		if sizePercentage > 50 || sizePercentage < -50 {
			return DiffSignificanceHigh
		}
	}

	// Consider large time changes as medium significance
	timeDiff := compare.Time - baseline.Time
	if baseline.Time > 0 {
		timePercentage := (timeDiff / baseline.Time) * 100
		if timePercentage > 100 || timePercentage < -50 {
			return DiffSignificanceMedium
		}
	}

	return DiffSignificanceLow
}

// calculateResourceSignificance determines the significance of resource changes
func (de *DiffEngine) calculateResourceSignificance(baseline, compare *ResourceMetrics) DiffSignificance {
	if baseline == nil || compare == nil {
		return DiffSignificanceMedium
	}

	// Check for significant size changes
	if baseline.Size > 0 {
		sizeDiff := float64(compare.Size - baseline.Size)
		sizePercentage := (sizeDiff / float64(baseline.Size)) * 100
		if sizePercentage > 25 || sizePercentage < -25 {
			return DiffSignificanceHigh
		}
	}

	// Check for significant time changes
	if baseline.LoadTime > 0 {
		timeDiff := compare.LoadTime - baseline.LoadTime
		timePercentage := (timeDiff / baseline.LoadTime) * 100
		if timePercentage > 50 || timePercentage < -30 {
			return DiffSignificanceMedium
		}
	}

	return DiffSignificanceLow
}

// countSignificantChanges counts changes marked as significant
func (de *DiffEngine) countSignificantChanges(diffs []*NetworkDiff) int {
	count := 0
	for _, diff := range diffs {
		if diff.Significance == DiffSignificanceHigh || diff.Significance == DiffSignificanceMedium {
			count++
		}
	}
	return count
}
