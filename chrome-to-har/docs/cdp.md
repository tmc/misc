# CDP - Chrome DevTools Protocol CLI Tool

CDP is a powerful command-line tool for interacting with Chrome using the Chrome DevTools Protocol (CDP). It provides a REPL (Read-Eval-Print Loop) interface to execute CDP commands directly, which is useful for browser automation, debugging, and exploring the CDP API.

## Installation

```bash
go install github.com/tmc/misc/chrome-to-har/cmd/cdp@latest
```

Or build from source:

```bash
cd chrome-to-har
go build -o cdp ./cmd/cdp
```

## Basic Usage

```bash
# Start an interactive CDP session
cdp

# Start with a specific URL
cdp -url https://example.com

# Use in headless mode
cdp -headless

# Connect to an existing Chrome instance
cdp -debug-port 9222

# Run a script file
cdp -script commands.txt

# Save output to a file
cdp -output results.txt
```

## Interactive Mode Commands

In interactive mode, CDP presents a `cdp>` prompt. You can enter CDP commands directly or use predefined aliases:

### Raw CDP Commands

CDP commands follow the format: `Domain.method {"param":"value"}`. For example:

```
cdp> Page.navigate {"url": "https://example.com"}
cdp> Runtime.evaluate {"expression": "document.title"}
cdp> Debugger.setBreakpoint {"location": {"scriptId": "123", "lineNumber": 42}}
```

### Aliases

CDP provides numerous aliases for common operations:

#### Navigation
- `goto https://example.com` - Navigate to URL
- `reload` - Reload current page
- `back` - Go back in history 
- `forward` - Go forward in history

#### DOM Interaction
- `click '#button'` - Click element matching CSS selector
- `focus '#input'` - Focus element matching CSS selector
- `type 'Hello'` - Insert text at current focus

#### Page Info
- `title` - Get page title
- `url` - Get current URL
- `cookies` - Get all cookies
- `html` - Get page HTML

#### Screenshots & PDF
- `screenshot` - Take a screenshot
- `screenshot-full` - Take a full-page screenshot
- `pdf` - Generate PDF of current page

#### Device Emulation
- `mobile` - Emulate mobile device
- `desktop` - Emulate desktop device
- `clear-emulation` - Clear emulation settings

#### Network Conditions
- `offline` - Simulate offline mode
- `online` - Restore normal connection
- `slow-3g` - Simulate slow 3G connection
- `fast-3g` - Simulate fast 3G connection

#### Debugging
- `pause` - Pause JavaScript execution
- `resume` / `cont` - Resume execution
- `step` / `stepinto` - Step into function call
- `next` / `stepover` - Step over function call
- `out` / `stepout` - Step out of current function

#### Coverage
- JavaScript coverage: `covjs_start`, `covjs_take`, `covjs_stop`
- CSS coverage: `covcss_start`, `covcss_take`, `covcss_stop`

#### Browser Management
- `targets` - List browser targets
- `info` - Get browser version info
- `domains` - List available CDP domains

### Help Commands

- `help` - Show general help
- `help aliases` - Show all command aliases
- `help domain Page` - Show help for a specific domain
- `help screenshot` - Show help for a specific command/alias

## Script Mode

CDP can execute commands from a script file. Each line in the script is treated as a separate command. Use `#` or `//` for comments:

```
# Example CDP script
# Navigate to a page
goto https://example.com

# Wait a bit and take a screenshot
Page.captureScreenshot {}

# Click a button
click '#submit-button'

# Get the resulting HTML
html
```

Run with:

```bash
cdp -script my_script.txt
```

## Output Format

CDP prints responses in pretty-formatted JSON. Event messages are prefixed with `<-- Event:` and special events like Debugger paused/resumed are highlighted.

To save output to a file:

```bash
cdp -output results.txt
```

## Advanced Features

### Connection to Existing Chrome Instance

1. Launch Chrome with remote debugging enabled:
   ```
   chrome --remote-debugging-port=9222
   ```

2. Connect CDP to this instance:
   ```
   cdp -debug-port 9222
   ```

### Using Chrome Profiles

To use an existing Chrome profile, which includes cookies, extensions, and settings:

```
cdp -profile Default
```

### Custom Chrome Path

If Chrome is installed in a non-standard location:

```
cdp -chrome-path /path/to/chrome
```

## Common Use Cases

### Web Page Analysis

```
cdp -url https://example.com
cdp> html
cdp> cookies
```

### JavaScript Debugging

```
cdp -url https://example.com
cdp> Debugger.setBreakpointByUrl {"url": "https://example.com/script.js", "lineNumber": 123}
cdp> step
cdp> Runtime.evaluate {"expression": "someVariable"}
```

### Performance Testing

```
cdp -headless
cdp> goto https://example.com
cdp> Performance.enable {}
cdp> Performance.getMetrics {}
```

### Coverage Analysis

```
cdp> covjs_start
# Interact with page
cdp> covjs_take
```

### Web Scraping

```
cdp -headless
cdp> goto https://example.com
cdp> Runtime.evaluate {"expression": "Array.from(document.querySelectorAll('h1')).map(h => h.textContent)"}
```

## Example Script for Automated Screenshot

```
# screenshot.txt
goto https://example.com
# Wait for page to fully load
Runtime.evaluate {"expression": "new Promise(resolve => setTimeout(resolve, 1000))"}
# Take a screenshot
screenshot
# Switch to mobile mode
mobile
# Take another screenshot
screenshot-full
```

Run with:
```
cdp -headless -script screenshot.txt -output screenshots.log
```