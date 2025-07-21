# Differential Capture Mode

The differential capture mode allows you to capture and compare different states of web pages, providing valuable insights into changes between captures.

## Features

### 1. **Baseline Capture**
- Create baseline HAR captures for comparison
- Store capture metadata with labels and descriptions
- Manage multiple captures with unique identifiers

### 2. **Delta Detection**
- Identify differences between captures
- Track added, removed, and modified requests
- Detect changes in resource types and sizes

### 3. **State Comparison**
- Compare different page states (before/after interactions)
- Track DOM changes, storage modifications, and viewport changes
- Monitor performance metrics between captures

### 4. **Resource Diff**
- Track resource changes between captures
- Monitor file size changes and loading times
- Identify new or removed resources

### 5. **Performance Diff**
- Compare performance metrics between captures
- Track load times, response times, and resource sizes
- Identify performance regressions or improvements

## Command-Line Interface

### Basic Usage

```bash
# Enable differential mode
chrome-to-har --diff-mode --url https://example.com --capture-name "baseline"

# Compare two captures
chrome-to-har --baseline <baseline-id> --compare-with <compare-id> --diff-output report.html

# List all captures
chrome-to-har --list-captures

# Delete a capture
chrome-to-har --delete-capture <capture-id>
```

### Advanced Options

```bash
# Create capture with labels
chrome-to-har --diff-mode --url https://example.com \
  --capture-name "login-test" \
  --capture-labels "env=prod,feature=login"

# Generate different report formats
chrome-to-har --baseline <baseline-id> --compare-with <compare-id> \
  --diff-output report.json --diff-format json

# Filter by significance level
chrome-to-har --baseline <baseline-id> --compare-with <compare-id> \
  --diff-output report.html --min-significance medium

# Enable state tracking
chrome-to-har --diff-mode --url https://example.com \
  --track-states --track-resources --track-performance
```

## Command-Line Flags

### Differential Mode Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--diff-mode` | Enable differential capture mode | `false` |
| `--baseline` | Baseline capture name or ID for comparison | `""` |
| `--compare-with` | Compare with this capture name or ID | `""` |
| `--diff-output` | Diff report output path | `""` |
| `--diff-format` | Report format (json, html, text, csv) | `"html"` |
| `--track-resources` | Track resource changes | `true` |
| `--track-performance` | Track performance changes | `true` |
| `--track-states` | Track page state changes | `false` |
| `--capture-name` | Name for the current capture | `""` |
| `--capture-labels` | Labels (key=value,key2=value2) | `""` |
| `--diff-work-dir` | Working directory for captures | `""` |
| `--list-captures` | List available captures | `false` |
| `--delete-capture` | Delete a capture by ID | `""` |
| `--min-significance` | Minimum significance level (low, medium, high) | `"low"` |

## Applications

### 1. **A/B Testing Analysis**
Compare different versions of a page to understand the impact of changes:

```bash
# Capture baseline (Version A)
chrome-to-har --diff-mode --url https://example.com/version-a \
  --capture-name "version-a" --capture-labels "version=a,test=homepage"

# Capture comparison (Version B)
chrome-to-har --diff-mode --url https://example.com/version-b \
  --capture-name "version-b" --capture-labels "version=b,test=homepage"

# Generate comparison report
chrome-to-har --baseline version-a --compare-with version-b \
  --diff-output ab-test-report.html
```

### 2. **Performance Regression Detection**
Monitor performance changes between deployments:

```bash
# Capture before deployment
chrome-to-har --diff-mode --url https://app.example.com \
  --capture-name "pre-deploy" --capture-labels "env=prod,deploy=before"

# Capture after deployment
chrome-to-har --diff-mode --url https://app.example.com \
  --capture-name "post-deploy" --capture-labels "env=prod,deploy=after"

# Generate performance comparison
chrome-to-har --baseline pre-deploy --compare-with post-deploy \
  --diff-output performance-regression.html --min-significance medium
```

### 3. **Content Change Monitoring**
Track changes in dynamic content:

```bash
# Capture initial state
chrome-to-har --diff-mode --url https://news.example.com \
  --capture-name "morning-news" --track-states

# Capture later state
chrome-to-har --diff-mode --url https://news.example.com \
  --capture-name "evening-news" --track-states

# Compare content changes
chrome-to-har --baseline morning-news --compare-with evening-news \
  --diff-output content-changes.html
```

### 4. **Feature Impact Analysis**
Understand the impact of new features:

```bash
# Capture before feature
chrome-to-har --diff-mode --url https://app.example.com/dashboard \
  --capture-name "before-feature" --capture-labels "feature=dashboard-v1"

# Capture after feature
chrome-to-har --diff-mode --url https://app.example.com/dashboard \
  --capture-name "after-feature" --capture-labels "feature=dashboard-v2"

# Analyze feature impact
chrome-to-har --baseline before-feature --compare-with after-feature \
  --diff-output feature-impact.html --track-performance
```

### 5. **Security Audit Comparisons**
Compare security configurations:

```bash
# Capture before security changes
chrome-to-har --diff-mode --url https://secure.example.com \
  --capture-name "security-before" --capture-labels "security=audit"

# Capture after security changes
chrome-to-har --diff-mode --url https://secure.example.com \
  --capture-name "security-after" --capture-labels "security=audit"

# Generate security comparison
chrome-to-har --baseline security-before --compare-with security-after \
  --diff-output security-audit.html --min-significance high
```

## Report Formats

### 1. **HTML Report**
Interactive HTML report with visual comparisons:
- Summary statistics
- Network change details
- Resource comparison charts
- Performance metrics

### 2. **JSON Report**
Machine-readable JSON format for automated analysis:
- Complete diff data
- Structured comparison results
- API integration friendly

### 3. **Text Report**
Human-readable text format for console output:
- Concise summary
- Key changes highlighted
- Easy to read in terminal

### 4. **CSV Report**
Tabular format for data analysis:
- Request-level changes
- Importable into spreadsheets
- Statistical analysis friendly

## Significance Levels

### **High Significance**
- Status code changes (4xx, 5xx errors)
- Large size changes (>50% difference)
- New or removed critical resources

### **Medium Significance**
- Moderate size changes (25-50% difference)
- Significant timing changes (>100% difference)
- New or removed non-critical resources

### **Low Significance**
- Minor size changes (<25% difference)
- Small timing changes (<100% difference)
- Header modifications

## Best Practices

### 1. **Capture Management**
- Use descriptive capture names
- Apply consistent labeling schemes
- Clean up old captures regularly

### 2. **Comparison Strategy**
- Ensure similar conditions for captures
- Use appropriate significance levels
- Focus on meaningful changes

### 3. **Performance Monitoring**
- Establish baseline metrics
- Monitor trends over time
- Set up automated comparisons

### 4. **Report Analysis**
- Review high-significance changes first
- Investigate performance regressions
- Document important findings

## Integration Examples

### Automated Testing Pipeline

```bash
#!/bin/bash
# Automated A/B testing script

# Capture baseline
BASELINE_ID=$(chrome-to-har --diff-mode --url "$BASE_URL" \
  --capture-name "baseline-$(date +%Y%m%d)" \
  --capture-labels "env=$ENV,branch=$BRANCH" \
  --output /dev/null | grep "ID:" | cut -d' ' -f2)

# Capture comparison
COMPARE_ID=$(chrome-to-har --diff-mode --url "$COMPARE_URL" \
  --capture-name "compare-$(date +%Y%m%d)" \
  --capture-labels "env=$ENV,branch=$BRANCH" \
  --output /dev/null | grep "ID:" | cut -d' ' -f2)

# Generate report
chrome-to-har --baseline "$BASELINE_ID" --compare-with "$COMPARE_ID" \
  --diff-output "reports/comparison-$(date +%Y%m%d).html" \
  --min-significance medium

echo "Comparison report generated: reports/comparison-$(date +%Y%m%d).html"
```

### CI/CD Integration

```yaml
# GitHub Actions example
name: Performance Comparison
on:
  pull_request:
    branches: [main]

jobs:
  performance-check:
    runs-on: ubuntu-latest
    steps:
      - name: Capture baseline
        run: |
          chrome-to-har --diff-mode --url "${{ env.PROD_URL }}" \
            --capture-name "baseline-pr-${{ github.event.number }}" \
            --capture-labels "env=prod,pr=${{ github.event.number }}"
      
      - name: Capture PR version
        run: |
          chrome-to-har --diff-mode --url "${{ env.PREVIEW_URL }}" \
            --capture-name "pr-${{ github.event.number }}" \
            --capture-labels "env=preview,pr=${{ github.event.number }}"
      
      - name: Generate comparison
        run: |
          chrome-to-har --baseline "baseline-pr-${{ github.event.number }}" \
            --compare-with "pr-${{ github.event.number }}" \
            --diff-output "performance-report.html" \
            --min-significance medium
      
      - name: Upload report
        uses: actions/upload-artifact@v2
        with:
          name: performance-report
          path: performance-report.html
```

## API Usage

The differential capture system can also be used programmatically:

```go
package main

import (
    "context"
    "github.com/tmc/misc/chrome-to-har/internal/differential"
)

func main() {
    // Create controller
    options := &differential.DifferentialOptions{
        WorkDir:          "/tmp/captures",
        TrackResources:   true,
        TrackPerformance: true,
        Verbose:          true,
    }
    
    controller, err := differential.NewDifferentialController(options)
    if err != nil {
        panic(err)
    }
    defer controller.Cleanup()
    
    // Create baseline capture
    ctx := context.Background()
    baseline, err := controller.CreateBaselineCapture(ctx, 
        "baseline", "https://example.com", "Test baseline", nil)
    if err != nil {
        panic(err)
    }
    
    // Complete capture (after HAR recording)
    // controller.CompleteCapture(baseline.ID, harData)
    
    // Compare captures
    // result, err := controller.CompareCapturesByID(baseline.ID, compare.ID)
    
    // Generate report
    // controller.GenerateReport(result, reportOptions)
}
```

## Troubleshooting

### Common Issues

1. **Capture Not Found**
   - Check capture ID spelling
   - Use `--list-captures` to verify existence
   - Ensure correct work directory

2. **Empty Diff Report**
   - Verify captures have data
   - Check significance level settings
   - Ensure captures are comparable

3. **Performance Issues**
   - Limit capture size with filters
   - Use appropriate work directory
   - Clean up old captures

4. **Report Generation Errors**
   - Check output path permissions
   - Verify report format support
   - Ensure sufficient disk space

### Debug Mode

Enable verbose logging for troubleshooting:

```bash
chrome-to-har --diff-mode --verbose --url https://example.com
```

This comprehensive differential capture system provides powerful tools for analyzing web page changes, performance impacts, and content modifications, making it valuable for A/B testing, performance monitoring, and security auditing.