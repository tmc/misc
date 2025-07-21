# Performance Monitoring Examples

This directory contains examples for monitoring web performance using chrome-to-har.

## Examples

### 1. Core Web Vitals Monitor (`core-web-vitals.go`)
Measures Google's Core Web Vitals metrics for website performance optimization.

**Usage:**
```bash
go run core-web-vitals.go https://example.com
```

**Features:**
- Measures LCP (Largest Contentful Paint)
- Measures FID (First Input Delay)  
- Measures CLS (Cumulative Layout Shift)
- Additional metrics: FCP, TTFB, Total Load Time
- Resource analysis and performance grading
- Detailed JSON reports

**Output:**
- Console report with performance grades
- `web-vitals-report-{timestamp}.json` with detailed metrics
- Performance recommendations

## Shell Script Helpers

### Performance Monitoring Script
```bash
#!/bin/bash
# monitor-performance.sh
URL="$1"
INTERVAL=300  # 5 minutes

while true; do
    echo "Performance check at $(date)"
    go run core-web-vitals.go "$URL"
    echo "Sleeping for $INTERVAL seconds..."
    sleep $INTERVAL
done
```

### Multi-URL Performance Check
```bash
#!/bin/bash
# check-multiple-sites.sh
declare -a urls=(
    "https://example.com"
    "https://example.com/about"
    "https://example.com/products"
    "https://example.com/contact"
)

for url in "${urls[@]}"; do
    echo "Checking performance for $url"
    go run core-web-vitals.go "$url"
    echo "---"
done
```

## Core Web Vitals Explained

### Largest Contentful Paint (LCP)
- **What it measures**: Loading performance
- **Good**: ≤ 2.5 seconds
- **Needs Improvement**: ≤ 4 seconds  
- **Poor**: > 4 seconds

### First Input Delay (FID)
- **What it measures**: Interactivity
- **Good**: ≤ 100 milliseconds
- **Needs Improvement**: ≤ 300 milliseconds
- **Poor**: > 300 milliseconds

### Cumulative Layout Shift (CLS)
- **What it measures**: Visual stability
- **Good**: ≤ 0.1
- **Needs Improvement**: ≤ 0.25
- **Poor**: > 0.25

## Additional Metrics

### First Contentful Paint (FCP)
- **What it measures**: Time to first visible content
- **Good**: ≤ 1.8 seconds
- **Needs Improvement**: ≤ 3 seconds
- **Poor**: > 3 seconds

### Time to First Byte (TTFB)
- **What it measures**: Server response time
- **Good**: ≤ 800 milliseconds
- **Needs Improvement**: ≤ 1.8 seconds
- **Poor**: > 1.8 seconds

## Performance Optimization Tips

### Improving LCP
```bash
# Optimize images
# Use next-gen formats (WebP, AVIF)
# Implement lazy loading
# Use CDN for static assets
# Preload critical resources
```

### Improving FID
```bash
# Minimize JavaScript execution time
# Use code splitting
# Remove unused JavaScript
# Implement service workers
# Use web workers for heavy computations
```

### Improving CLS
```bash
# Set dimensions on images and videos
# Use CSS aspect ratio for dynamic content
# Preload fonts to avoid font swapping
# Avoid inserting content above existing content
# Use transform animations instead of layout changes
```

## Advanced Performance Testing

### Custom Performance Metrics
```go
// Measure custom metrics
chromedp.Evaluate(`
    (function() {
        const customMetrics = {};
        
        // Measure time to interactive
        const observer = new PerformanceObserver((list) => {
            for (const entry of list.getEntries()) {
                if (entry.entryType === 'measure' && entry.name === 'TTI') {
                    customMetrics.tti = entry.duration;
                }
            }
        });
        
        observer.observe({ entryTypes: ['measure'] });
        
        return customMetrics;
    })()
`, &customMetrics)
```

### Network Performance Analysis
```go
// Analyze network requests
chromedp.Evaluate(`
    (function() {
        const resources = performance.getEntriesByType('resource');
        const analysis = {
            totalRequests: resources.length,
            slowRequests: resources.filter(r => r.duration > 1000).length,
            failedRequests: resources.filter(r => r.transferSize === 0).length,
            avgResponseTime: resources.reduce((sum, r) => sum + r.duration, 0) / resources.length
        };
        
        return analysis;
    })()
`, &networkAnalysis)
```

### Memory Usage Monitoring
```go
// Monitor memory usage
chromedp.Evaluate(`
    (function() {
        if (performance.memory) {
            return {
                usedJSHeapSize: performance.memory.usedJSHeapSize,
                totalJSHeapSize: performance.memory.totalJSHeapSize,
                jsHeapSizeLimit: performance.memory.jsHeapSizeLimit,
                memoryUsagePercent: (performance.memory.usedJSHeapSize / performance.memory.jsHeapSizeLimit) * 100
            };
        }
        return null;
    })()
`, &memoryStats)
```

## Performance Budgets

### Setting Performance Budgets
```go
type PerformanceBudget struct {
    MaxLCP    float64 `json:"max_lcp_ms"`
    MaxFID    float64 `json:"max_fid_ms"`
    MaxCLS    float64 `json:"max_cls"`
    MaxFCP    float64 `json:"max_fcp_ms"`
    MaxTTFB   float64 `json:"max_ttfb_ms"`
    MaxSize   int64   `json:"max_size_bytes"`
    MaxRequests int   `json:"max_requests"`
}

func validateBudget(result WebVitalsResult, budget PerformanceBudget) []string {
    var violations []string
    
    if result.LCP > budget.MaxLCP {
        violations = append(violations, fmt.Sprintf("LCP exceeds budget: %.2f > %.2f", result.LCP, budget.MaxLCP))
    }
    
    if result.FID > budget.MaxFID {
        violations = append(violations, fmt.Sprintf("FID exceeds budget: %.2f > %.2f", result.FID, budget.MaxFID))
    }
    
    // ... more validations
    
    return violations
}
```

## Continuous Performance Monitoring

### CI/CD Integration
```yaml
# .github/workflows/performance.yml
name: Performance Monitoring
on:
  push:
    branches: [main]
  schedule:
    - cron: '0 */6 * * *'  # Every 6 hours

jobs:
  performance:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - name: Setup Go
        uses: actions/setup-go@v2
        with:
          go-version: 1.21
      - name: Run Performance Tests
        run: |
          go run examples/05-performance/core-web-vitals.go ${{ secrets.SITE_URL }}
      - name: Upload Performance Report
        uses: actions/upload-artifact@v2
        with:
          name: performance-report
          path: "web-vitals-report-*.json"
```

### Performance Alerting
```bash
#!/bin/bash
# performance-alert.sh
URL="$1"
THRESHOLD_LCP=2500
THRESHOLD_FID=100
THRESHOLD_CLS=0.1

# Run performance test
go run core-web-vitals.go "$URL" > performance-output.txt

# Check if thresholds are exceeded
if grep -q "Poor" performance-output.txt; then
    echo "Performance alert: Core Web Vitals thresholds exceeded!"
    # Send alert (email, Slack, etc.)
    curl -X POST -H 'Content-type: application/json' \
        --data '{"text":"Performance alert for '$URL'"}' \
        "$SLACK_WEBHOOK_URL"
fi
```

## Performance Regression Detection

### Baseline Comparison
```go
func compareWithBaseline(current WebVitalsResult, baseline WebVitalsResult) RegressionReport {
    report := RegressionReport{
        HasRegression: false,
        Changes: make(map[string]float64),
    }
    
    // Compare LCP
    lcpChange := ((current.LCP - baseline.LCP) / baseline.LCP) * 100
    report.Changes["LCP"] = lcpChange
    
    if lcpChange > 10 { // 10% regression threshold
        report.HasRegression = true
        report.Regressions = append(report.Regressions, "LCP regressed by %.2f%%", lcpChange)
    }
    
    // Compare other metrics...
    
    return report
}
```

### Trend Analysis
```go
func analyzeTrend(results []WebVitalsResult) TrendAnalysis {
    analysis := TrendAnalysis{
        Period: len(results),
        Trends: make(map[string]string),
    }
    
    // Calculate LCP trend
    if len(results) >= 2 {
        firstLCP := results[0].LCP
        lastLCP := results[len(results)-1].LCP
        
        if lastLCP > firstLCP*1.1 {
            analysis.Trends["LCP"] = "Degrading"
        } else if lastLCP < firstLCP*0.9 {
            analysis.Trends["LCP"] = "Improving"
        } else {
            analysis.Trends["LCP"] = "Stable"
        }
    }
    
    return analysis
}
```

## Best Practices

### 1. Regular Monitoring
- Set up automated performance monitoring
- Monitor key user journeys and pages
- Track performance over time

### 2. Performance Budgets
- Define clear performance budgets
- Integrate budget validation into CI/CD
- Alert on budget violations

### 3. Real User Monitoring (RUM)
- Combine synthetic testing with RUM data
- Monitor performance across different devices and networks
- Track user experience metrics

### 4. Performance Testing Strategy
- Test on different devices and network conditions
- Include performance tests in your testing suite
- Monitor third-party script impact

### 5. Optimization Workflow
- Measure before optimizing
- Focus on user-centric metrics
- Test optimizations thoroughly
- Monitor post-deployment performance

## Troubleshooting

### Common Issues

1. **Inconsistent Results**: Network conditions, server load
   - Solution: Run multiple tests and average results

2. **High LCP**: Large images, slow server responses
   - Solution: Optimize images, use CDN, improve server performance

3. **High FID**: Heavy JavaScript execution
   - Solution: Code splitting, reduce JavaScript, use web workers

4. **High CLS**: Unsized images, dynamic content insertion
   - Solution: Set image dimensions, reserve space for dynamic content

### Debug Performance Issues

1. **Use Chrome DevTools**: Lighthouse, Performance tab
2. **Analyze HAR files**: Network timing, resource sizes
3. **Monitor resource loading**: Waterfall charts, critical path
4. **Test different conditions**: Slow networks, low-end devices

## Reporting and Visualization

### Performance Dashboard
```go
func generatePerformanceDashboard(results []WebVitalsResult) {
    // Generate HTML dashboard with charts
    // Include trend analysis, alerts, and recommendations
}
```

### Integration with Monitoring Tools
- Send metrics to DataDog, New Relic, or similar
- Set up alerts and notifications
- Create performance dashboards
- Track performance SLAs