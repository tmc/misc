# Chrome Tools

This repository contains Chrome-based tools for HTTP traffic capture, web page interaction, and more.

## Tools Available

- **chrome-to-har**: Capture network activity from Chrome to HAR format
- **churl**: A curl-like tool that runs through Chrome to properly handle JavaScript/SPAs
- **cdp**: Interactive CLI for direct Chrome DevTools Protocol interaction

## Implementation Status

- **chrome-to-har**: ✅ Functional
- **churl**: ✅ Initial implementation complete
- **cdp**: ✅ Fully functional
- **Refactoring**: 🔄 Work in progress
- **Advanced Features**: 🔄 Work in progress
- **Documentation**: ✅ Complete

See [implementation notes](docs/implementation.md) for details on what's done and what's next.

## Installation

```bash
# Install chrome-to-har
go install github.com/tmc/misc/chrome-to-har@latest

# Install churl
go install github.com/tmc/misc/chrome-to-har/cmd/churl@latest

# Install cdp
go install github.com/tmc/misc/chrome-to-har/cmd/cdp@latest
```

Alternatively, build from source:

```bash
git clone https://github.com/tmc/misc/chrome-to-har.git
cd chrome-to-har
go build -o chrome-to-har .
go build -o churl ./cmd/churl
go build -o cdp ./cmd/cdp
```

# chrome-to-har

Launches a Chrome browser and captures network activity to HAR format, with optional interactive JavaScript CLI mode.

## Troubleshooting

If you get a "context deadline exceeded" error, Chrome is having trouble starting or connecting. Try:

1. Close all running Chrome instances before starting
2. Use `-headless` flag to run Chrome without UI
3. Increase the timeout with `-timeout=300`
4. Restart your computer if the issue persists

## Usage

```
# Basic HAR capture mode
chrome-to-har -profile=/path/to/chrome/profile -output=output.har [-url=https://example.com] [-verbose]

# Interactive JavaScript CLI mode
chrome-to-har -interactive [-url=https://aistudio.google.com/live]

# For complex sites or slow connections (RECOMMENDED APPROACH)
chrome-to-har -interactive -timeout=300 -headless
```

### Options

- `-profile`: Chrome profile directory to use
- `-output`: Output HAR file (default: output.har)
- `-diff`: Enable differential HAR capture
- `-verbose`: Enable verbose logging
- `-url`: Starting URL to navigate to
- `-cookies`: Regular expression to filter cookies (default: capture all)
- `-urls`: Regular expression to filter URLs (default: capture all)
- `-stream`: Stream HAR entries as they are captured (outputs NDJSON)
- `-filter`: JQ expression to filter HAR entries (e.g., 'select(.response.status < 400)')
- `-template`: Go template to transform HAR entries (e.g., '{{.request.url}} {{.response.status}}')
- `-interactive`: Run in interactive JavaScript CLI mode
- `-headless`: Run Chrome in headless mode
- `-debug-port`: Use specific port for Chrome DevTools (e.g., 9222)
- `-timeout`: Global timeout in seconds (default: 180)

For HAR capture mode, press Ctrl+D to capture the HAR file.

# churl

A curl alternative that runs requests through Chrome, allowing it to properly handle JavaScript and Single Page Applications.

## Usage

```bash
# Basic fetch, output to stdout
churl https://example.com

# Save output to file
churl -o output.html https://example.com

# Output as JSON with page info
churl --output-format=json https://example.com

# Wait for specific element to appear
churl --wait-for "#content" https://example.com

# Set custom headers
churl -H "User-Agent: Chrome/90" -H "Cookie: session=123" https://example.com

# Output HAR format
churl --output-format=har https://example.com > output.har
```

See [churl documentation](docs/churl.md) for more details and examples.

# cdp

An interactive CLI tool for working directly with the Chrome DevTools Protocol.

## Usage

```bash
# Start in interactive mode
cdp

# Connect to a specific URL
cdp -url https://example.com

# Run in headless mode
cdp -headless

# Connect to existing Chrome instance
cdp -debug-port 9222

# Run from a script file
cdp -script commands.txt

# Save output to file
cdp -output results.txt
```

In interactive mode, you can use raw CDP commands or aliases:

```
cdp> Page.navigate {"url": "https://example.com"}
cdp> Runtime.evaluate {"expression": "document.title"}
cdp> goto https://google.com
cdp> screenshot
cdp> html
```

See [cdp documentation](docs/cdp.md) for comprehensive documentation.

## Interactive CLI Mode

The interactive CLI mode allows you to run JavaScript commands directly in the Chrome browser from the command line:

```
# Run in interactive mode with Google AI Studio (default URL)
chrome-to-har -interactive

# Run in interactive mode with custom URL
chrome-to-har -interactive -url="https://your-url-here"
```

In interactive mode, you can type JavaScript commands to control the browser:

```
> document.title                  # Get page title
> window.location.href            # Get current URL
> document.querySelector('button').click()  # Click a button
> exit                            # Exit interactive mode
```

## Streaming Format

When using `-stream`, entries are output in NDJSON (Newline Delimited JSON) format. Each line contains a single HAR entry:

```jsonl
{"startedDateTime":"2024-01-01T00:00:00Z","request":{"method":"GET","url":"https://example.com"},"response":{"status":200}}
{"startedDateTime":"2024-01-01T00:00:01Z","request":{"method":"POST","url":"https://api.example.com"},"response":{"status":201}}
```

## Filtering Examples

Filter out successful responses:
```bash
chrome-to-har -filter='select(.response.status >= 400)'
```

Only include specific domains:
```bash
chrome-to-har -filter='select(.request.url | contains("api.example.com"))'
```

Custom template output:
```bash
chrome-to-har -template='{{.startedDateTime}} {{.request.method}} {{.request.url}} {{.response.status}}'
```

Combine streaming and filtering:
```bash
chrome-to-har -stream -filter='select(.response.status < 500) | {url: .request.url, status: .response.status}'
```

## Features

- Captures all network requests and responses
- Interactive JavaScript CLI mode for browser control
- Includes cookies from Chrome profile
- Supports verbose logging
- Can start with a specific URL
- Preserves authentication and session data from profile
- Filtering support for cookies and URLs
- Optional streaming mode for real-time HAR entry output
- JQ-style filtering expressions
- Go template support for custom output formats