# churl - Chrome-powered curl for Modern Web

`churl` is a powerful command-line tool that combines the simplicity of curl with the full rendering capabilities of Chrome. Unlike traditional HTTP clients, churl executes JavaScript, handles SPAs (Single Page Applications), and captures fully rendered content, making it ideal for modern web scraping and testing.

## Table of Contents

- [Installation](#installation)
- [Quick Start](#quick-start)
- [Core Features](#core-features)
- [Command-Line Reference](#command-line-reference)
- [Usage Examples](#usage-examples)
- [Advanced Features](#advanced-features)
- [Remote Chrome Integration](#remote-chrome-integration)
- [Common Use Cases](#common-use-cases)
- [Troubleshooting](#troubleshooting)
- [Integration Examples](#integration-examples)

## Installation

### Using Go Install

```bash
go install github.com/tmc/misc/chrome-to-har/cmd/churl@latest
```

### Building from Source

```bash
git clone https://github.com/tmc/misc/chrome-to-har.git
cd chrome-to-har
go build -o churl ./cmd/churl
```

### Prerequisites

churl requires a Chromium-based browser installed on your system. It automatically detects:
- Google Chrome
- Chromium
- Brave Browser
- Microsoft Edge
- Opera
- Vivaldi

## Quick Start

```bash
# Basic usage - fetch and output HTML
churl https://example.com

# Save to file
churl -o page.html https://example.com

# Extract text content
churl --output-format=text https://example.com

# Capture network activity as HAR
churl --output-format=har https://example.com > capture.har

# Wait for specific element
churl --wait-for "#app-loaded" https://spa-app.com
```

## Core Features

### JavaScript Execution
- Full JavaScript engine support via Chrome
- Handles dynamic content and SPAs
- Waits for asynchronous operations to complete

### Network Control
- Custom headers and authentication
- Support for all HTTP methods (GET, POST, PUT, DELETE, etc.)
- Cookie management through Chrome profiles

### Output Formats
- **HTML**: Fully rendered DOM after JavaScript execution
- **HAR**: Complete network activity log
- **Text**: Extracted text content
- **JSON**: Structured page information

### Wait Strategies
- Network idle detection
- CSS selector appearance
- Custom timeouts for stability

## Command-Line Reference

### Basic Syntax

```bash
churl [options] URL
```

### Output Options

| Flag | Type | Description | Default |
|------|------|-------------|---------|
| `-o` | string | Output file (instead of stdout) | - |
| `--output-format` | string | Output format: html, har, text, json | html |

### Chrome Browser Options

| Flag | Type | Description | Default |
|------|------|-------------|---------|
| `--chrome-path` | string | Path to Chrome executable | auto-detected |
| `--profile` | string | Chrome profile directory to use | - |
| `--headless` | bool | Run Chrome in headless mode | true |
| `--debug-port` | int | Chrome DevTools port (0 for auto) | 0 |
| `--timeout` | int | Global timeout in seconds | 180 |
| `--verbose` | bool | Enable verbose logging | false |

### Remote Chrome Options

| Flag | Type | Description | Default |
|------|------|-------------|---------|
| `--remote-host` | string | Connect to remote Chrome host | - |
| `--remote-port` | int | Remote Chrome debugging port | 9222 |
| `--remote-tab` | string | Connect to specific tab ID or URL | - |
| `--list-tabs` | bool | List available tabs on remote Chrome | false |

### Wait Strategy Options

| Flag | Type | Description | Default |
|------|------|-------------|---------|
| `--wait-network-idle` | bool | Wait for network activity to stop | true |
| `--wait-for` | string | Wait for CSS selector to appear | - |
| `--stable-timeout` | int | Max seconds to wait for stability | 30 |

### HTTP Request Options

| Flag | Type | Description | Default |
|------|------|-------------|---------|
| `-H` | string | Add request header (repeatable) | - |
| `-X` | string | HTTP method | GET |
| `-d` | string | Request body data | - |
| `-L` | bool | Follow redirects | true |
| `-u` | string | Basic auth (username:password) | - |

### Script Injection Options

| Flag | Type | Description | Default |
|------|------|-------------|---------|
| `--script-before` | string | JavaScript to run before page load (repeatable) | - |
| `--script-after` | string | JavaScript to run after page load (repeatable) | - |
| `--script-file-before` | string | JS file to run before page load (repeatable) | - |
| `--script-file-after` | string | JS file to run after page load (repeatable) | - |

## Usage Examples

### Basic Web Scraping

```bash
# Fetch a JavaScript-heavy page
churl https://react-app.example.com

# Extract text content from a SPA
churl --output-format=text --wait-for ".content-loaded" https://spa.example.com

# Save rendered HTML for offline analysis
churl -o rendered.html https://dynamic-site.com
```

### HTTP Methods and Headers

```bash
# POST request with JSON data
churl -X POST -H "Content-Type: application/json" \
  -d '{"name":"John","age":30}' \
  https://api.example.com/users

# PUT request with custom headers
churl -X PUT \
  -H "Authorization: Bearer token123" \
  -H "X-Custom-Header: value" \
  -d '{"status":"updated"}' \
  https://api.example.com/resource/123

# DELETE request
churl -X DELETE -H "Authorization: Bearer token123" \
  https://api.example.com/resource/123

# Multiple headers
churl -H "Accept: application/json" \
  -H "User-Agent: churl/1.0" \
  -H "X-Request-ID: 12345" \
  https://api.example.com
```

### Authentication Examples

```bash
# Basic authentication
churl -u john:password123 https://protected.example.com

# Bearer token via header
churl -H "Authorization: Bearer eyJhbGc..." https://api.example.com

# Using Chrome profile with saved cookies
churl --profile "Default" https://authenticated-app.com
```

### Script Injection

```bash
# Inject inline JavaScript before page load
churl --script-before "window.DEBUG = true" https://example.com

# Inject multiple scripts
churl --script-before "window.startTime = Date.now()" \
  --script-after "console.log('Load time:', Date.now() - window.startTime)" \
  https://example.com

# Use script files
churl --script-file-before setup.js \
  --script-file-after analyze.js \
  https://example.com
```

Example `setup.js`:
```javascript
// Set up monitoring before page loads
window.requests = [];
const originalFetch = window.fetch;
window.fetch = function(...args) {
    window.requests.push({url: args[0], time: Date.now()});
    return originalFetch.apply(this, args);
};
```

Example `analyze.js`:
```javascript
// Analyze page after loading
console.log('Total requests:', window.requests.length);
console.log('Page title:', document.title);
console.log('Images found:', document.images.length);
```

### Network Analysis with HAR

```bash
# Capture full network activity
churl --output-format=har https://example.com > capture.har

# Analyze the HAR file
cat capture.har | jq '.log.entries | length'  # Count requests
cat capture.har | jq '.log.entries[].request.url'  # List all URLs

# Filter for specific requests
cat capture.har | jq '.log.entries[] | select(.response.status == 404)'
```

### Working with SPAs

```bash
# Wait for React app to load
churl --wait-for "[data-app-ready='true']" https://react-app.com

# Wait for Angular app
churl --wait-for "app-root.ng-star-inserted" https://angular-app.com

# Wait for specific text to appear
churl --script-after "
  const checkText = () => document.body.textContent.includes('Welcome');
  if (!checkText()) {
    await new Promise(r => {
      const observer = new MutationObserver(() => {
        if (checkText()) { observer.disconnect(); r(); }
      });
      observer.observe(document.body, {childList: true, subtree: true});
    });
  }
" https://dynamic-app.com
```

## Advanced Features

### Chrome Profile Management

```bash
# List available Chrome profiles
chrome-to-har --list-profiles

# Use specific profile (maintains cookies, localStorage, etc.)
churl --profile "Default" https://example.com
churl --profile "Profile 1" https://example.com

# Create a temporary profile for isolation
TMPDIR=$(mktemp -d)
churl --profile "$TMPDIR" https://example.com
rm -rf "$TMPDIR"
```

### Debug Mode

```bash
# Run with visible browser window
churl --headless=false https://example.com

# Enable verbose logging
churl --verbose https://example.com

# Connect to DevTools on specific port
churl --debug-port=9222 --headless=false https://example.com
# Then open chrome://inspect in another Chrome instance
```

### Custom Chrome Flags

```bash
# Disable images for faster loading
churl --chrome-path="/usr/bin/google-chrome" \
  --script-before "
    const style = document.createElement('style');
    style.textContent = 'img { display: none !important; }';
    document.head.appendChild(style);
  " https://image-heavy-site.com

# Use specific window size
churl --headless=false --verbose https://example.com
```

## Remote Chrome Integration

### Connecting to Remote Chrome

```bash
# Start Chrome with remote debugging on a server
google-chrome --headless --remote-debugging-port=9222 --remote-debugging-address=0.0.0.0

# Connect from another machine
churl --remote-host=server.example.com --remote-port=9222 https://example.com

# List available tabs
churl --remote-host=server.example.com --list-tabs

# Connect to specific tab
churl --remote-host=server.example.com --remote-tab="CDC7F2B..." https://example.com
```

### Docker Integration

```dockerfile
# Dockerfile for remote Chrome
FROM chromium:latest
EXPOSE 9222
CMD ["chromium", "--headless", "--no-sandbox", \
     "--remote-debugging-port=9222", \
     "--remote-debugging-address=0.0.0.0"]
```

```bash
# Run Chrome in Docker
docker run -d -p 9222:9222 --name chrome-headless chrome-image

# Use with churl
churl --remote-host=localhost --remote-port=9222 https://example.com
```

## Common Use Cases

### Web Scraping Pipeline

```bash
#!/bin/bash
# Scrape product data from SPA

# 1. Fetch rendered page
churl --wait-for ".products-loaded" \
  --output-format=html \
  -o products.html \
  https://shop.example.com/products

# 2. Extract data with script
churl --script-after "
  const products = Array.from(document.querySelectorAll('.product')).map(p => ({
    name: p.querySelector('.name')?.textContent,
    price: p.querySelector('.price')?.textContent,
    image: p.querySelector('img')?.src
  }));
  console.log(JSON.stringify(products, null, 2));
" https://shop.example.com/products 2>&1 | tail -n +2 > products.json
```

### API Testing with Browser Context

```bash
#!/bin/bash
# Test API that requires browser environment

# Login first
churl -X POST \
  -d '{"username":"test","password":"pass"}' \
  --profile "TestProfile" \
  https://app.example.com/api/login

# Use saved session for subsequent requests
churl --profile "TestProfile" \
  --output-format=json \
  https://app.example.com/api/user/data
```

### Performance Monitoring

```bash
# Monitor page load performance
churl --script-before "window.performance.mark('churl-start')" \
  --script-after "
    window.performance.mark('churl-end');
    window.performance.measure('churl-load', 'churl-start', 'churl-end');
    const measure = window.performance.getEntriesByName('churl-load')[0];
    console.log('Load time:', measure.duration, 'ms');
    console.log('DOM nodes:', document.getElementsByTagName('*').length);
  " \
  --output-format=har \
  https://example.com > performance.har
```

### Automated Testing

```bash
# Test if login works
LOGIN_RESULT=$(churl -X POST \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"test123"}' \
  --output-format=json \
  https://app.example.com/api/login)

if echo "$LOGIN_RESULT" | jq -e '.token' > /dev/null; then
  echo "Login successful"
else
  echo "Login failed"
  exit 1
fi
```

## Troubleshooting

### Common Issues

**Chrome not found**
```bash
# Specify Chrome path explicitly
churl --chrome-path="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" https://example.com

# Or set environment variable
export CHROME_PATH="/usr/bin/google-chrome"
churl https://example.com
```

**Timeout errors**
```bash
# Increase timeout for slow sites
churl --timeout=300 --stable-timeout=60 https://slow-site.com

# Disable network idle wait for sites with continuous polling
churl --wait-network-idle=false https://polling-site.com
```

**JavaScript errors**
```bash
# Debug with visible browser
churl --headless=false --verbose https://problematic-site.com

# Check console errors with script
churl --script-after "
  const errors = window.__errors || [];
  console.log('JS Errors:', JSON.stringify(errors));
" https://example.com
```

**SSL/Certificate issues**
```bash
# For development/testing only - ignore certificate errors
churl --verbose https://self-signed.local
```

### Debug Output

```bash
# Maximum debugging information
churl --verbose \
  --headless=false \
  --debug-port=9222 \
  --script-before "console.log('Starting navigation')" \
  --script-after "console.log('Navigation complete')" \
  https://example.com 2>&1 | tee debug.log
```

## Integration Examples

### With jq for JSON Processing

```bash
# Extract specific data from JSON output
churl --output-format=json https://example.com | \
  jq -r '.title'

# Process dynamic content
churl --wait-for ".data-table" --script-after "
  const data = Array.from(document.querySelectorAll('tr')).map(row => 
    Array.from(row.cells).map(cell => cell.textContent)
  );
  console.log(JSON.stringify(data));
" https://example.com 2>&1 | tail -1 | jq -r '.[]'
```

### With grep/awk for Text Processing

```bash
# Find all email addresses
churl --output-format=text https://contact.example.com | \
  grep -E -o "[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}"

# Extract prices
churl --output-format=text https://shop.example.com | \
  grep -oP '\$\d+\.\d{2}' | sort -u
```

### In Shell Scripts

```bash
#!/bin/bash
# monitor.sh - Monitor website changes

URL="https://example.com"
SELECTOR=".main-content"

# Fetch current content
CURRENT=$(churl --wait-for "$SELECTOR" --output-format=text "$URL" | md5sum)

# Compare with previous
if [ -f last_check.md5 ]; then
  PREVIOUS=$(cat last_check.md5)
  if [ "$CURRENT" != "$PREVIOUS" ]; then
    echo "Website changed!"
    # Send notification, email, etc.
  fi
fi

echo "$CURRENT" > last_check.md5
```

### With Python

```python
import subprocess
import json

def fetch_with_churl(url, wait_for=None):
    cmd = ['churl', '--output-format=json']
    if wait_for:
        cmd.extend(['--wait-for', wait_for])
    cmd.append(url)
    
    result = subprocess.run(cmd, capture_output=True, text=True)
    return json.loads(result.stdout)

# Usage
data = fetch_with_churl('https://api.example.com', wait_for='#data')
print(f"Title: {data['title']}")
print(f"URL: {data['url']}")
```

### CI/CD Integration

```yaml
# .github/workflows/test.yml
name: Integration Tests
on: [push]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      
      - name: Setup Chrome
        uses: browser-actions/setup-chrome@latest
      
      - name: Install churl
        run: go install github.com/tmc/misc/chrome-to-har/cmd/churl@latest
      
      - name: Test website availability
        run: |
          churl --timeout=60 https://myapp.example.com
          
      - name: Test API endpoints
        run: |
          churl -X POST -d '{"test": true}' https://myapp.example.com/api/health
```

## Best Practices

1. **Use appropriate timeouts**: Adjust timeouts based on site complexity
2. **Profile management**: Use separate profiles for different authentication contexts  
3. **Script validation**: Test scripts in browser console before using
4. **Resource optimization**: Disable images/styles when only text is needed
5. **Error handling**: Always check exit codes in scripts
6. **Debugging**: Start with `--headless=false` when troubleshooting

## Comparison with Traditional Tools

| Feature | curl | wget | churl |
|---------|------|------|-------|
| JavaScript execution | ❌ | ❌ | ✅ |
| SPA support | ❌ | ❌ | ✅ |
| Cookie persistence | Limited | ✅ | ✅ |
| Network analysis | Limited | ❌ | ✅ (HAR) |
| Visual debugging | ❌ | ❌ | ✅ |
| Wait strategies | ❌ | ❌ | ✅ |
| Browser profiles | ❌ | ❌ | ✅ |

## Security Considerations

- **Script injection**: Only inject scripts you trust
- **Profile isolation**: Use separate profiles for different security contexts
- **Remote connections**: Secure remote Chrome instances appropriately
- **Credential handling**: Use environment variables for sensitive data

## Performance Tips

1. **Headless mode**: Always use unless debugging
2. **Network idle**: Disable for sites with continuous polling
3. **Selective waiting**: Use specific selectors instead of generic timeouts
4. **Resource blocking**: Block unnecessary resources via scripts
5. **Profile reuse**: Reuse profiles to avoid repeated logins

## Conclusion

churl bridges the gap between traditional command-line HTTP tools and modern web applications. By leveraging Chrome's rendering engine, it provides reliable access to JavaScript-heavy sites while maintaining the simplicity of command-line tools.