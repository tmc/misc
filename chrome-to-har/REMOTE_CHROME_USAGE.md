# Remote Chrome Connection Support

The chrome-to-har tools now support connecting to running Chrome instances with remote debugging enabled.

## Starting Chrome with Remote Debugging

First, start Chrome (or any Chromium-based browser) with remote debugging enabled:

```bash
# macOS - Chrome
"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" --remote-debugging-port=9222

# macOS - Brave
"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser" --remote-debugging-port=9222

# macOS - Chrome Canary
"/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary" --remote-debugging-port=9222

# Linux
google-chrome --remote-debugging-port=9222

# Windows
"C:\Program Files\Google\Chrome\Application\chrome.exe" --remote-debugging-port=9222
```

## Using cdp with Remote Chrome

### List available tabs
```bash
./cdp --remote-host localhost --list-tabs
```

### Connect to the browser (first tab)
```bash
./cdp --remote-host localhost
```

### Connect to a specific tab by ID
```bash
./cdp --remote-host localhost --remote-tab "1A2B3C4D-5E6F-7890-ABCD-EF1234567890"
```

### Connect to a specific tab by URL
```bash
./cdp --remote-host localhost --remote-tab "https://example.com"
```

### Example CDP session
```bash
$ ./cdp --remote-host localhost
Connected to remote Chrome at localhost:9222
Connected to Chrome. Type commands or 'help' for assistance.
Examples: 'goto https://example.com', 'title', 'screenshot'
cdp> title
Result: Example Domain
cdp> url
Result: https://example.com/
cdp> screenshot
Screenshot saved to: screenshot-1736365123.png
cdp> exit
Exiting...
```

## Using churl with Remote Chrome

### Fetch a page using remote Chrome
```bash
./churl --remote-host localhost https://example.com
```

### List tabs and fetch from specific tab
```bash
# List tabs
./churl --remote-host localhost --list-tabs

# Fetch from specific tab
./churl --remote-host localhost --remote-tab "tab-id-here" https://example.com
```

### Save HAR file from remote Chrome
```bash
./churl --remote-host localhost --output-format har -o output.har https://example.com
```

### Examples with different output formats
```bash
# HTML (default)
./churl --remote-host localhost https://example.com > page.html

# HAR format
./churl --remote-host localhost --output-format har -o page.har https://example.com

# JSON format
./churl --remote-host localhost --output-format json https://example.com

# Text extraction
./churl --remote-host localhost --output-format text https://example.com
```

## Advanced Usage

### Connect to remote Chrome on different host
```bash
./cdp --remote-host 192.168.1.100 --remote-port 9222
```

### Use with verbose logging
```bash
./cdp --remote-host localhost --verbose
./churl --remote-host localhost --verbose https://example.com
```

### Combine with other options
```bash
# Wait for specific selector
./churl --remote-host localhost --wait-for "#content" https://example.com

# Set custom headers
./churl --remote-host localhost -H "User-Agent: CustomBot" https://example.com

# Use with authentication
./churl --remote-host localhost -u user:password https://protected.example.com
```

## Benefits of Remote Chrome Connection

1. **Reuse existing browser session** - Connect to your regular browser with logged-in sessions
2. **Debug live pages** - Interact with pages you're already viewing
3. **Share browser across tools** - Multiple tools can connect to the same browser
4. **Avoid browser startup overhead** - Faster for repeated operations
5. **Use browser extensions** - Leverage installed extensions and settings

## Troubleshooting

If you can't connect to remote Chrome:

1. Ensure Chrome is started with `--remote-debugging-port=9222`
2. Check if the port is accessible: `curl http://localhost:9222/json/version`
3. Try using `--remote-host 127.0.0.1` instead of `localhost`
4. Check firewall settings if connecting to remote host
5. Make sure no other application is using port 9222

## Security Considerations

- Remote debugging exposes Chrome to network connections
- Only enable on trusted networks
- Consider using SSH tunneling for remote connections
- The debugging port allows full browser control