# Chrome-to-HAR Examples

This directory contains comprehensive examples and usage patterns for the chrome-to-har toolkit, demonstrating its full capabilities across various use cases.

## 📁 Directory Structure

### [01-basic/](01-basic/) - Getting Started
Simple examples to get you started with chrome-to-har and churl.

- **`simple-capture.go`** - Basic HAR capture using the library
- **`churl-basic.go`** - Simple HTTP requests through Chrome
- **Shell scripts** - Quick HAR capture and HTML fetch utilities

### [02-web-scraping/](02-web-scraping/) - Web Scraping
Complete web scraping workflows for different types of websites.

- **`news-scraper.go`** - Extract articles from news websites
- **`ecommerce-scraper.go`** - Scrape product information
- **`social-media-scraper.go`** - Extract social media posts and metrics
- **Anti-detection techniques** and **best practices**

### [03-spa-testing/](03-spa-testing/) - Single Page Application Testing
Specialized testing for modern JavaScript frameworks.

- **`react-app-tester.go`** - React application testing with component analysis
- **`vue-app-tester.go`** - Vue.js testing with Vuex state extraction
- **`angular-app-tester.go`** - Angular testing with service detection
- **Framework-specific patterns** and **state management testing**

### [04-api-testing/](04-api-testing/) - API Testing
Test various APIs with browser context for authentication and cookies.

- **`rest-api-tester.go`** - Comprehensive REST API testing suite
- **`graphql-tester.go`** - GraphQL query, mutation, and subscription testing
- **`websocket-tester.go`** - Real-time WebSocket connection testing
- **Authentication patterns** and **performance validation**

### [05-performance/](05-performance/) - Performance Monitoring
Monitor and measure web performance metrics.

- **`core-web-vitals.go`** - Measure Google's Core Web Vitals (LCP, FID, CLS)
- **Performance budgets** and **regression detection**
- **Continuous monitoring** and **alerting**

### [06-automation/](06-automation/) - CI/CD Integration
Integrate chrome-to-har into automated testing and deployment pipelines.

- **`ci-cd-integration.go`** - CI/CD pipeline integration example
- **GitHub Actions workflows** and **Docker deployment**
- **Automated testing patterns** and **report generation**

### [11-integration/](11-integration/) - Integration Examples
Integration with other tools and platforms.

- **`docker-example.go`** - Docker containerization example
- **`Dockerfile`** - Production-ready Docker configuration
- **Language bindings** and **external tool integration**

### [scripts/](scripts/) - Utility Scripts
Helpful shell scripts for common tasks.

- **`quick-har.sh`** - Quick HAR capture utility
- **`quick-html.sh`** - Quick HTML fetch utility
- **Batch processing** and **monitoring scripts**

## 🚀 Quick Start

### Installation
```bash
# Install chrome-to-har
go install github.com/tmc/misc/chrome-to-har@latest

# Install churl
go install github.com/tmc/misc/chrome-to-har/cmd/churl@latest
```

### Basic Usage
```bash
# Capture HAR file
chrome-to-har --output example.har https://example.com

# Fetch HTML with JavaScript rendering
churl https://example.com > example.html

# Run a simple example
cd examples/01-basic
go run simple-capture.go https://example.com
```

## 📊 Use Case Categories

### 🔍 Web Scraping & Data Extraction
- **News aggregation** - Extract articles from news sites
- **E-commerce monitoring** - Track product prices and availability
- **Social media analysis** - Gather posts and engagement metrics
- **Content monitoring** - Detect changes and updates

### 🧪 Testing & Quality Assurance
- **SPA testing** - Test React, Vue, Angular applications
- **API testing** - REST, GraphQL, WebSocket endpoint testing
- **Performance testing** - Core Web Vitals and load time monitoring
- **Regression testing** - Automated UI and performance regression detection

### 🚀 DevOps & Automation
- **CI/CD integration** - Automated testing in pipelines
- **Health monitoring** - Continuous website health checks
- **Performance budgets** - Enforce performance standards
- **Alerting systems** - Notify on failures or regressions

### 🔒 Security & Compliance
- **Security testing** - Vulnerability scanning and authentication testing
- **Compliance monitoring** - GDPR, accessibility, and other compliance checks
- **Authentication flows** - Test complex login and session management

## 🛠️ Advanced Features

### Network Analysis
```go
// Capture and analyze network requests
rec := recorder.New()
chromedp.Run(ctx,
    rec.Start(),
    chromedp.Navigate(url),
    rec.Stop(),
)
harData, _ := rec.HAR()
// Analyze timing, sizes, status codes
```

### Performance Monitoring
```go
// Measure Core Web Vitals
result := measureWebVitals(ctx, url)
fmt.Printf("LCP: %.2f ms\n", result.LCP)
fmt.Printf("FID: %.2f ms\n", result.FID)
fmt.Printf("CLS: %.3f\n", result.CLS)
```

### Browser Automation
```go
// Advanced browser control
chromedp.Run(ctx,
    chromedp.Navigate(url),
    chromedp.WaitVisible("selector", chromedp.ByQuery),
    chromedp.Click("button", chromedp.ByQuery),
    chromedp.Screenshot("full-page", chromedp.FullScreenshot),
)
```

### Data Extraction
```go
// Extract structured data
chromedp.Evaluate(`
    Array.from(document.querySelectorAll('.item')).map(item => ({
        title: item.querySelector('h2').textContent,
        price: item.querySelector('.price').textContent,
        url: item.querySelector('a').href
    }))
`, &extractedData)
```

## 🔧 Configuration & Customization

### Chrome Options
```go
opts := append(chromedp.DefaultExecAllocatorOptions[:],
    chromedp.Flag("headless", true),
    chromedp.Flag("no-sandbox", true),
    chromedp.UserAgent("Custom-Agent/1.0"),
    chromedp.WindowSize(1920, 1080),
)
```

### Environment Variables
```bash
export CHROME_PATH=/path/to/chrome
export HEADLESS=true
export DEBUG=true
export TIMEOUT=30s
```

### Docker Configuration
```dockerfile
FROM golang:1.21-alpine
RUN apk add --no-cache chromium
ENV CHROME_PATH=/usr/bin/chromium-browser
COPY . .
RUN go build -o app
CMD ["./app"]
```

## 📈 Best Practices

### 1. Error Handling
```go
// Implement retry logic
for attempts := 0; attempts < 3; attempts++ {
    err := chromedp.Run(ctx, actions...)
    if err == nil {
        break
    }
    time.Sleep(time.Duration(attempts+1) * time.Second)
}
```

### 2. Resource Management
```go
// Proper context management
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

// Clean up resources
allocCtx, cancel := chromedp.NewExecAllocator(ctx, opts...)
defer cancel()
```

### 3. Rate Limiting
```go
// Respect rate limits
time.Sleep(time.Duration(rand.Intn(3)+1) * time.Second)

// Implement exponential backoff
backoff := time.Duration(math.Pow(2, float64(attempts))) * time.Second
time.Sleep(backoff)
```

### 4. Data Validation
```go
// Validate extracted data
if title := strings.TrimSpace(title); title != "" && len(title) > 3 {
    // Process valid data
}
```

### 5. Testing Strategy
```go
// Test-driven development
func TestWebScraping(t *testing.T) {
    result := scrapeWebsite(testURL)
    assert.NotEmpty(t, result.Title)
    assert.True(t, result.Success)
}
```

## 🐛 Troubleshooting

### Common Issues

1. **Chrome not found**: Set `CHROME_PATH` environment variable
2. **Permission denied**: Use `--no-sandbox` flag in Docker
3. **Memory issues**: Increase container memory limits
4. **Network timeouts**: Adjust timeout values
5. **Element not found**: Add explicit waits

### Debug Mode
```bash
# Enable debug logging
export DEBUG=true
go run example.go

# Capture screenshots for debugging
chromedp.Screenshot("debug.png", chromedp.FullScreenshot)
```

### Performance Optimization
```go
// Optimize for speed
chromedp.Flag("disable-images", true)
chromedp.Flag("disable-plugins", true)
chromedp.Flag("disable-extensions", true)
```

## 📚 Learning Resources

### Documentation
- [Chrome DevTools Protocol](https://chromedevtools.github.io/devtools-protocol/)
- [ChromeDP Documentation](https://github.com/chromedp/chromedp)
- [HAR Format Specification](https://w3c.github.io/web-performance/specs/HAR/Overview.html)

### Tutorials
- [Web Scraping with Go](./02-web-scraping/README.md)
- [SPA Testing Guide](./03-spa-testing/README.md)
- [API Testing Patterns](./04-api-testing/README.md)
- [Performance Monitoring](./05-performance/README.md)

### Community
- [GitHub Issues](https://github.com/tmc/misc/issues)
- [Discussions](https://github.com/tmc/misc/discussions)
- [Contributing Guide](../CONTRIBUTING.md)

## 🤝 Contributing

We welcome contributions! Please see our [Contributing Guide](../CONTRIBUTING.md) for details.

### Adding Examples
1. Create a new directory under the appropriate category
2. Include a README.md with usage instructions
3. Add comprehensive comments and error handling
4. Include test cases where applicable
5. Update this main README with your example

### Example Template
```go
// Example Title
// Description of what this example demonstrates
package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "time"
    
    "github.com/chromedp/chromedp"
    "github.com/tmc/misc/chrome-to-har/internal/recorder"
)

func main() {
    // Implementation with proper error handling
    // and comprehensive comments
}
```

## 📄 License

This project is licensed under the MIT License. See the [LICENSE](../LICENSE) file for details.

## 🙏 Acknowledgments

- Chrome DevTools team for the CDP protocol
- ChromeDP contributors for the Go bindings
- Go community for the excellent tooling

---

**Happy Testing!** 🎉

For questions or support, please open an issue on [GitHub](https://github.com/tmc/misc/issues).