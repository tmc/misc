# Basic Usage Examples

This directory contains simple examples to get you started with chrome-to-har and churl.

## Examples

### 1. Simple HAR Capture (`simple-capture.go`)
Demonstrates the most basic usage of chrome-to-har library to capture a HAR file.

**Usage:**
```bash
go run simple-capture.go https://example.com > example.har
```

**What it does:**
- Creates a Chrome browser context
- Navigates to the specified URL
- Captures all network traffic as HAR
- Outputs the HAR data to stdout

### 2. Basic churl Usage (`churl-basic.go`)
Shows how to use the churl command for simple HTTP requests through Chrome.

**Usage:**
```bash
go run churl-basic.go https://example.com
```

**What it does:**
- Uses the churl command to fetch a webpage
- Handles JavaScript rendering
- Returns the final HTML content

## Shell Script Examples

### Quick HAR Capture
```bash
#!/bin/bash
# quick-har.sh
chrome-to-har --output output.har "$1"
echo "HAR saved to output.har"
```

### Quick HTML Fetch
```bash
#!/bin/bash
# quick-html.sh
churl "$1" > output.html
echo "HTML saved to output.html"
```

## Common Use Cases

1. **Basic website testing**: Capture HAR files to analyze network requests
2. **Content extraction**: Use churl to get rendered HTML from SPAs
3. **Performance analysis**: Monitor network activity and timing
4. **API testing**: Test endpoints that require browser context

## Next Steps

- Check out the web scraping examples in `../02-web-scraping/`
- Learn about SPA testing in `../03-spa-testing/`
- Explore performance monitoring in `../05-performance/`