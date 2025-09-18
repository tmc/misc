package differential

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// ReportFormat represents the format of the diff report
type ReportFormat string

const (
	ReportFormatJSON ReportFormat = "json"
	ReportFormatHTML ReportFormat = "html"
	ReportFormatText ReportFormat = "text"
	ReportFormatCSV  ReportFormat = "csv"
)

// ReportOptions configures report generation
type ReportOptions struct {
	Format          ReportFormat `json:"format"`
	OutputPath      string       `json:"output_path"`
	IncludeDetails  bool         `json:"include_details"`
	IncludeGraphs   bool         `json:"include_graphs"`
	ThemeColor      string       `json:"theme_color"`
	Title           string       `json:"title"`
	Description     string       `json:"description"`
	ShowUnchanged   bool         `json:"show_unchanged"`
	MinSignificance DiffSignificance `json:"min_significance"`
}

// ReportGenerator generates diff reports in various formats
type ReportGenerator struct {
	verbose bool
}

// NewReportGenerator creates a new report generator
func NewReportGenerator(verbose bool) *ReportGenerator {
	return &ReportGenerator{
		verbose: verbose,
	}
}

// GenerateReport generates a diff report in the specified format
func (rg *ReportGenerator) GenerateReport(result *DiffResult, options *ReportOptions) error {
	if rg.verbose {
		fmt.Printf("Generating %s report to %s\n", options.Format, options.OutputPath)
	}

	switch options.Format {
	case ReportFormatJSON:
		return rg.generateJSONReport(result, options)
	case ReportFormatHTML:
		return rg.generateHTMLReport(result, options)
	case ReportFormatText:
		return rg.generateTextReport(result, options)
	case ReportFormatCSV:
		return rg.generateCSVReport(result, options)
	default:
		return fmt.Errorf("unsupported report format: %s", options.Format)
	}
}

// generateJSONReport generates a JSON report
func (rg *ReportGenerator) generateJSONReport(result *DiffResult, options *ReportOptions) error {
	// Filter results based on options
	filteredResult := rg.filterResult(result, options)

	var jsonBytes []byte
	var err error

	if options.IncludeDetails {
		jsonBytes, err = json.MarshalIndent(filteredResult, "", "  ")
	} else {
		// Create a summary-only version
		summary := map[string]interface{}{
			"baseline_capture": filteredResult.BaselineCapture,
			"compare_capture":  filteredResult.CompareCapture,
			"summary":          filteredResult.Summary,
			"performance_diff": filteredResult.PerformanceDiff,
			"timestamp":        filteredResult.Timestamp,
		}
		jsonBytes, err = json.MarshalIndent(summary, "", "  ")
	}

	if err != nil {
		return errors.Wrap(err, "marshaling JSON report")
	}

	if err := os.WriteFile(options.OutputPath, jsonBytes, 0644); err != nil {
		return errors.Wrap(err, "writing JSON report")
	}

	return nil
}

// generateHTMLReport generates an HTML report
func (rg *ReportGenerator) generateHTMLReport(result *DiffResult, options *ReportOptions) error {
	filteredResult := rg.filterResult(result, options)

	tmpl := template.Must(template.New("report").Funcs(template.FuncMap{
		"formatTime":    rg.formatTime,
		"formatSize":    rg.formatSize,
		"formatPercent": rg.formatPercent,
		"styleClass":    rg.getStyleClass,
		"upper":         strings.ToUpper,
		"title":         strings.Title,
		"toString": func(v interface{}) string {
			if stringer, ok := v.(fmt.Stringer); ok {
				return stringer.String()
			}
			return fmt.Sprintf("%v", v)
		},
	}).Parse(htmlTemplate))

	var buf bytes.Buffer
	data := map[string]interface{}{
		"Result":  filteredResult,
		"Options": options,
		"Title":   options.Title,
		"Generated": time.Now().Format("January 2, 2006 at 3:04 PM"),
	}

	if err := tmpl.Execute(&buf, data); err != nil {
		return errors.Wrap(err, "executing HTML template")
	}

	if err := os.WriteFile(options.OutputPath, buf.Bytes(), 0644); err != nil {
		return errors.Wrap(err, "writing HTML report")
	}

	return nil
}

// generateTextReport generates a text report
func (rg *ReportGenerator) generateTextReport(result *DiffResult, options *ReportOptions) error {
	filteredResult := rg.filterResult(result, options)

	var buf bytes.Buffer

	// Header
	buf.WriteString("DIFFERENTIAL CAPTURE REPORT\n")
	buf.WriteString("===========================\n\n")

	if options.Title != "" {
		buf.WriteString(fmt.Sprintf("Title: %s\n", options.Title))
	}
	if options.Description != "" {
		buf.WriteString(fmt.Sprintf("Description: %s\n", options.Description))
	}
	buf.WriteString(fmt.Sprintf("Generated: %s\n\n", time.Now().Format("January 2, 2006 at 3:04 PM")))

	// Capture Information
	buf.WriteString("CAPTURE INFORMATION\n")
	buf.WriteString("-------------------\n")
	buf.WriteString(fmt.Sprintf("Baseline: %s (ID: %s)\n", filteredResult.BaselineCapture.Name, filteredResult.BaselineCapture.ID))
	buf.WriteString(fmt.Sprintf("  URL: %s\n", filteredResult.BaselineCapture.URL))
	buf.WriteString(fmt.Sprintf("  Timestamp: %s\n", filteredResult.BaselineCapture.Timestamp.Format("2006-01-02 15:04:05")))
	buf.WriteString(fmt.Sprintf("  Entries: %d\n", filteredResult.BaselineCapture.EntryCount))
	buf.WriteString(fmt.Sprintf("Compare: %s (ID: %s)\n", filteredResult.CompareCapture.Name, filteredResult.CompareCapture.ID))
	buf.WriteString(fmt.Sprintf("  URL: %s\n", filteredResult.CompareCapture.URL))
	buf.WriteString(fmt.Sprintf("  Timestamp: %s\n", filteredResult.CompareCapture.Timestamp.Format("2006-01-02 15:04:05")))
	buf.WriteString(fmt.Sprintf("  Entries: %d\n\n", filteredResult.CompareCapture.EntryCount))

	// Summary
	buf.WriteString("SUMMARY\n")
	buf.WriteString("-------\n")
	buf.WriteString(fmt.Sprintf("Total Requests: %d\n", filteredResult.Summary.TotalRequests))
	buf.WriteString(fmt.Sprintf("Added Requests: %d\n", filteredResult.Summary.AddedRequests))
	buf.WriteString(fmt.Sprintf("Removed Requests: %d\n", filteredResult.Summary.RemovedRequests))
	buf.WriteString(fmt.Sprintf("Modified Requests: %d\n", filteredResult.Summary.ModifiedRequests))
	buf.WriteString(fmt.Sprintf("Unchanged Requests: %d\n", filteredResult.Summary.UnchangedRequests))
	buf.WriteString(fmt.Sprintf("Significant Changes: %d\n\n", filteredResult.Summary.SignificantChanges))

	// Performance Summary
	if filteredResult.PerformanceDiff != nil {
		buf.WriteString("PERFORMANCE COMPARISON\n")
		buf.WriteString("----------------------\n")
		
		if filteredResult.PerformanceDiff.TotalLoadTime != nil {
			buf.WriteString(fmt.Sprintf("Total Load Time: %.2fms → %.2fms (%+.2f%%)\n", 
				filteredResult.PerformanceDiff.TotalLoadTime.Baseline,
				filteredResult.PerformanceDiff.TotalLoadTime.Compare,
				filteredResult.PerformanceDiff.TotalLoadTime.Percentage))
		}
		
		if filteredResult.PerformanceDiff.TotalSize != nil {
			buf.WriteString(fmt.Sprintf("Total Size: %s → %s (%+.2f%%)\n", 
				rg.formatSize(int64(filteredResult.PerformanceDiff.TotalSize.Baseline)),
				rg.formatSize(int64(filteredResult.PerformanceDiff.TotalSize.Compare)),
				filteredResult.PerformanceDiff.TotalSize.Percentage))
		}
		
		if filteredResult.PerformanceDiff.RequestCount != nil {
			buf.WriteString(fmt.Sprintf("Request Count: %.0f → %.0f (%+.2f%%)\n", 
				filteredResult.PerformanceDiff.RequestCount.Baseline,
				filteredResult.PerformanceDiff.RequestCount.Compare,
				filteredResult.PerformanceDiff.RequestCount.Percentage))
		}
		
		if filteredResult.PerformanceDiff.AverageResponse != nil {
			buf.WriteString(fmt.Sprintf("Average Response Time: %.2fms → %.2fms (%+.2f%%)\n", 
				filteredResult.PerformanceDiff.AverageResponse.Baseline,
				filteredResult.PerformanceDiff.AverageResponse.Compare,
				filteredResult.PerformanceDiff.AverageResponse.Percentage))
		}
		
		buf.WriteString("\n")
	}

	// Network Changes
	if len(filteredResult.NetworkDiffs) > 0 {
		buf.WriteString("NETWORK CHANGES\n")
		buf.WriteString("---------------\n")
		
		// Group by type
		added := make([]*NetworkDiff, 0)
		removed := make([]*NetworkDiff, 0)
		modified := make([]*NetworkDiff, 0)
		
		for _, diff := range filteredResult.NetworkDiffs {
			switch diff.Type {
			case DiffTypeAdded:
				added = append(added, diff)
			case DiffTypeRemoved:
				removed = append(removed, diff)
			case DiffTypeModified:
				modified = append(modified, diff)
			}
		}
		
		// Added requests
		if len(added) > 0 {
			buf.WriteString(fmt.Sprintf("\nAdded Requests (%d):\n", len(added)))
			for _, diff := range added {
				buf.WriteString(fmt.Sprintf("  + %s %s [%s]\n", diff.Method, diff.URL, diff.Significance))
			}
		}
		
		// Removed requests
		if len(removed) > 0 {
			buf.WriteString(fmt.Sprintf("\nRemoved Requests (%d):\n", len(removed)))
			for _, diff := range removed {
				buf.WriteString(fmt.Sprintf("  - %s %s [%s]\n", diff.Method, diff.URL, diff.Significance))
			}
		}
		
		// Modified requests
		if len(modified) > 0 {
			buf.WriteString(fmt.Sprintf("\nModified Requests (%d):\n", len(modified)))
			for _, diff := range modified {
				buf.WriteString(fmt.Sprintf("  ~ %s %s [%s]\n", diff.Method, diff.URL, diff.Significance))
				for _, change := range diff.Changes {
					buf.WriteString(fmt.Sprintf("    %s\n", change))
				}
			}
		}
		
		buf.WriteString("\n")
	}

	// Resource Changes
	if len(filteredResult.ResourceDiffs) > 0 {
		buf.WriteString("RESOURCE CHANGES\n")
		buf.WriteString("----------------\n")
		
		for _, diff := range filteredResult.ResourceDiffs {
			buf.WriteString(fmt.Sprintf("%s Resources [%s]:\n", strings.Title(diff.ResourceType), diff.Significance))
			for _, change := range diff.Changes {
				buf.WriteString(fmt.Sprintf("  %s\n", change))
			}
		}
		
		buf.WriteString("\n")
	}

	if err := os.WriteFile(options.OutputPath, buf.Bytes(), 0644); err != nil {
		return errors.Wrap(err, "writing text report")
	}

	return nil
}

// generateCSVReport generates a CSV report
func (rg *ReportGenerator) generateCSVReport(result *DiffResult, options *ReportOptions) error {
	filteredResult := rg.filterResult(result, options)

	var buf bytes.Buffer

	// CSV Header
	buf.WriteString("Type,Method,URL,Significance,Baseline_Status,Compare_Status,Baseline_Size,Compare_Size,Baseline_Time,Compare_Time,Changes\n")

	// Write network diffs
	for _, diff := range filteredResult.NetworkDiffs {
		buf.WriteString(fmt.Sprintf("%s,%s,\"%s\",%s,", 
			diff.Type, diff.Method, diff.URL, diff.Significance))
		
		if diff.Baseline != nil {
			buf.WriteString(fmt.Sprintf("%d,%d,%.2f,", 
				diff.Baseline.Status, diff.Baseline.Size, diff.Baseline.Time))
		} else {
			buf.WriteString(",,,")
		}
		
		if diff.Compare != nil {
			buf.WriteString(fmt.Sprintf("%d,%d,%.2f,", 
				diff.Compare.Status, diff.Compare.Size, diff.Compare.Time))
		} else {
			buf.WriteString(",,,")
		}
		
		buf.WriteString(fmt.Sprintf("\"%s\"\n", strings.Join(diff.Changes, "; ")))
	}

	if err := os.WriteFile(options.OutputPath, buf.Bytes(), 0644); err != nil {
		return errors.Wrap(err, "writing CSV report")
	}

	return nil
}

// filterResult filters the result based on report options
func (rg *ReportGenerator) filterResult(result *DiffResult, options *ReportOptions) *DiffResult {
	filtered := &DiffResult{
		BaselineCapture: result.BaselineCapture,
		CompareCapture:  result.CompareCapture,
		Summary:         result.Summary,
		PerformanceDiff: result.PerformanceDiff,
		Timestamp:       result.Timestamp,
	}

	// Filter network diffs
	for _, diff := range result.NetworkDiffs {
		if rg.shouldIncludeDiff(diff.Significance, options.MinSignificance) {
			filtered.NetworkDiffs = append(filtered.NetworkDiffs, diff)
		}
	}

	// Filter resource diffs
	for _, diff := range result.ResourceDiffs {
		if rg.shouldIncludeDiff(diff.Significance, options.MinSignificance) {
			filtered.ResourceDiffs = append(filtered.ResourceDiffs, diff)
		}
	}

	return filtered
}

// shouldIncludeDiff determines if a diff should be included based on significance
func (rg *ReportGenerator) shouldIncludeDiff(significance, minSignificance DiffSignificance) bool {
	significanceOrder := map[DiffSignificance]int{
		DiffSignificanceLow:    1,
		DiffSignificanceMedium: 2,
		DiffSignificanceHigh:   3,
	}

	return significanceOrder[significance] >= significanceOrder[minSignificance]
}

// formatTime formats time in milliseconds
func (rg *ReportGenerator) formatTime(ms float64) string {
	if ms < 1000 {
		return fmt.Sprintf("%.2fms", ms)
	}
	return fmt.Sprintf("%.2fs", ms/1000)
}

// formatSize formats size in bytes
func (rg *ReportGenerator) formatSize(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	}
	if bytes < 1024*1024 {
		return fmt.Sprintf("%.2f KB", float64(bytes)/1024)
	}
	return fmt.Sprintf("%.2f MB", float64(bytes)/(1024*1024))
}

// formatPercent formats a percentage
func (rg *ReportGenerator) formatPercent(percent float64) string {
	if percent > 0 {
		return fmt.Sprintf("+%.1f%%", percent)
	}
	return fmt.Sprintf("%.1f%%", percent)
}

// getStyleClass returns CSS class for significance level
func (rg *ReportGenerator) getStyleClass(significance DiffSignificance) string {
	switch significance {
	case DiffSignificanceHigh:
		return "high-significance"
	case DiffSignificanceMedium:
		return "medium-significance"
	case DiffSignificanceLow:
		return "low-significance"
	default:
		return ""
	}
}

// HTML template for the report
const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}} - Differential Capture Report</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            line-height: 1.6;
            margin: 0;
            padding: 20px;
            background-color: #f8f9fa;
        }
        .container {
            max-width: 1200px;
            margin: 0 auto;
            background: white;
            padding: 30px;
            border-radius: 8px;
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
        }
        h1, h2, h3 {
            color: #333;
            margin-top: 30px;
        }
        h1 {
            border-bottom: 3px solid #007bff;
            padding-bottom: 10px;
        }
        .summary {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 20px;
            margin: 20px 0;
        }
        .metric {
            background: #f8f9fa;
            padding: 15px;
            border-radius: 5px;
            text-align: center;
        }
        .metric-value {
            font-size: 2em;
            font-weight: bold;
            color: #007bff;
        }
        .metric-label {
            font-size: 0.9em;
            color: #666;
        }
        .capture-info {
            background: #e9ecef;
            padding: 15px;
            border-radius: 5px;
            margin: 10px 0;
        }
        .diff-item {
            border: 1px solid #dee2e6;
            border-radius: 5px;
            margin: 10px 0;
            padding: 15px;
        }
        .high-significance {
            border-left: 4px solid #dc3545;
        }
        .medium-significance {
            border-left: 4px solid #ffc107;
        }
        .low-significance {
            border-left: 4px solid #28a745;
        }
        .added {
            background-color: #d4edda;
            border-color: #c3e6cb;
        }
        .removed {
            background-color: #f8d7da;
            border-color: #f5c6cb;
        }
        .modified {
            background-color: #fff3cd;
            border-color: #ffeaa7;
        }
        .changes {
            margin-top: 10px;
            padding-left: 20px;
        }
        .changes li {
            margin: 5px 0;
            font-family: monospace;
            font-size: 0.9em;
        }
        .performance-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
            gap: 15px;
            margin: 20px 0;
        }
        .performance-item {
            background: #f8f9fa;
            padding: 15px;
            border-radius: 5px;
            border: 1px solid #dee2e6;
        }
        .improvement {
            color: #28a745;
        }
        .regression {
            color: #dc3545;
        }
        .footer {
            text-align: center;
            margin-top: 40px;
            color: #666;
            font-size: 0.9em;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>{{.Title}}</h1>
        <p><strong>Generated:</strong> {{.Generated}}</p>
        {{if .Options.Description}}
        <p><strong>Description:</strong> {{.Options.Description}}</p>
        {{end}}

        <h2>Capture Information</h2>
        <div class="capture-info">
            <h3>Baseline Capture</h3>
            <p><strong>Name:</strong> {{.Result.BaselineCapture.Name}} (ID: {{.Result.BaselineCapture.ID}})</p>
            <p><strong>URL:</strong> {{.Result.BaselineCapture.URL}}</p>
            <p><strong>Timestamp:</strong> {{.Result.BaselineCapture.Timestamp.Format "2006-01-02 15:04:05"}}</p>
            <p><strong>Entries:</strong> {{.Result.BaselineCapture.EntryCount}}</p>
        </div>
        
        <div class="capture-info">
            <h3>Compare Capture</h3>
            <p><strong>Name:</strong> {{.Result.CompareCapture.Name}} (ID: {{.Result.CompareCapture.ID}})</p>
            <p><strong>URL:</strong> {{.Result.CompareCapture.URL}}</p>
            <p><strong>Timestamp:</strong> {{.Result.CompareCapture.Timestamp.Format "2006-01-02 15:04:05"}}</p>
            <p><strong>Entries:</strong> {{.Result.CompareCapture.EntryCount}}</p>
        </div>

        <h2>Summary</h2>
        <div class="summary">
            <div class="metric">
                <div class="metric-value">{{.Result.Summary.TotalRequests}}</div>
                <div class="metric-label">Total Requests</div>
            </div>
            <div class="metric">
                <div class="metric-value">{{.Result.Summary.AddedRequests}}</div>
                <div class="metric-label">Added</div>
            </div>
            <div class="metric">
                <div class="metric-value">{{.Result.Summary.RemovedRequests}}</div>
                <div class="metric-label">Removed</div>
            </div>
            <div class="metric">
                <div class="metric-value">{{.Result.Summary.ModifiedRequests}}</div>
                <div class="metric-label">Modified</div>
            </div>
            <div class="metric">
                <div class="metric-value">{{.Result.Summary.SignificantChanges}}</div>
                <div class="metric-label">Significant</div>
            </div>
        </div>

        {{if .Result.PerformanceDiff}}
        <h2>Performance Comparison</h2>
        <div class="performance-grid">
            {{if .Result.PerformanceDiff.TotalLoadTime}}
            <div class="performance-item">
                <strong>Total Load Time</strong><br>
                {{formatTime .Result.PerformanceDiff.TotalLoadTime.Baseline}} → {{formatTime .Result.PerformanceDiff.TotalLoadTime.Compare}}
                <span class="{{if .Result.PerformanceDiff.TotalLoadTime.Improved}}improvement{{else}}regression{{end}}">
                    ({{formatPercent .Result.PerformanceDiff.TotalLoadTime.Percentage}})
                </span>
            </div>
            {{end}}
            {{if .Result.PerformanceDiff.TotalSize}}
            <div class="performance-item">
                <strong>Total Size</strong><br>
                {{formatSize .Result.PerformanceDiff.TotalSize.Baseline}} → {{formatSize .Result.PerformanceDiff.TotalSize.Compare}}
                <span class="{{if .Result.PerformanceDiff.TotalSize.Improved}}improvement{{else}}regression{{end}}">
                    ({{formatPercent .Result.PerformanceDiff.TotalSize.Percentage}})
                </span>
            </div>
            {{end}}
            {{if .Result.PerformanceDiff.RequestCount}}
            <div class="performance-item">
                <strong>Request Count</strong><br>
                {{.Result.PerformanceDiff.RequestCount.Baseline}} → {{.Result.PerformanceDiff.RequestCount.Compare}}
                <span class="{{if .Result.PerformanceDiff.RequestCount.Improved}}improvement{{else}}regression{{end}}">
                    ({{formatPercent .Result.PerformanceDiff.RequestCount.Percentage}})
                </span>
            </div>
            {{end}}
        </div>
        {{end}}

        {{if .Result.NetworkDiffs}}
        <h2>Network Changes</h2>
        {{range .Result.NetworkDiffs}}
        <div class="diff-item {{toString .Type}} {{styleClass .Significance}}">
            <strong>{{title (upper (toString .Type))}}:</strong> {{.Method}} {{.URL}} 
            <span class="significance">[{{.Significance}}]</span>
            {{if .Changes}}
            <ul class="changes">
                {{range .Changes}}
                <li>{{.}}</li>
                {{end}}
            </ul>
            {{end}}
        </div>
        {{end}}
        {{end}}

        {{if .Result.ResourceDiffs}}
        <h2>Resource Changes</h2>
        {{range .Result.ResourceDiffs}}
        <div class="diff-item {{styleClass .Significance}}">
            <strong>{{title .ResourceType}} Resources</strong> 
            <span class="significance">[{{.Significance}}]</span>
            {{if .Changes}}
            <ul class="changes">
                {{range .Changes}}
                <li>{{.}}</li>
                {{end}}
            </ul>
            {{end}}
        </div>
        {{end}}
        {{end}}

        <div class="footer">
            <p>Report generated by chrome-to-har differential capture system</p>
        </div>
    </div>
</body>
</html>`