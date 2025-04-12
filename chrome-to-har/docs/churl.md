# churl - Chrome-powered curl

`churl` is a command line tool similar to curl but which runs through Chrome, allowing it to handle JavaScript and SPAs (Single Page Applications) properly. It captures the fully rendered page after JavaScript execution.

## Installation

```bash
go install github.com/tmc/misc/chrome-to-har/cmd/churl@latest
```

Alternatively, build from source:

```bash
git clone https://github.com/tmc/misc/chrome-to-har.git
cd chrome-to-har
go build -o churl ./cmd/churl
```

## Usage

```bash
churl [options] URL
```

## Basic Examples

```bash
# Basic fetch, output to stdout
churl https://example.com

# Save output to file
churl -o output.html https://example.com

# Output as JSON with page info
churl --output-format=json https://example.com

# Custom user agent
churl -H "User-Agent: Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36" https://example.com

# Basic authentication
churl -u username:password https://example.com

# Wait for specific element to appear
churl --wait-for "#content" https://example.com

# Output HAR format
churl --output-format=har https://example.com > output.har

# Use non-headless Chrome (shows browser UI)
churl --headless=false https://example.com

# POST request with JSON data
churl -X POST -d '{"key":"value"}' https://api.example.com
```

## Options

### Output Options

- `-o FILE` - Output to file instead of stdout
- `--output-format FORMAT` - Output format: html, har, text, json (default: html)

### Chrome Options

- `--profile PROFILE` - Chrome profile directory to use
- `--headless` - Run Chrome in headless mode (default: true)
- `--debug-port PORT` - Use specific port for Chrome DevTools (0 for auto)
- `--timeout SECONDS` - Global timeout in seconds (default: 180)
- `--chrome-path PATH` - Path to Chrome executable
- `--verbose` - Enable verbose logging

### Wait Options

- `--wait-network-idle` - Wait until network activity becomes idle (default: true)
- `--wait-for SELECTOR` - Wait for specific CSS selector to appear
- `--stable-timeout SECONDS` - Max time in seconds to wait for stability (default: 30)

### Request Options

- `-H "Name: Value"` - Add request header (can be used multiple times)
- `-X METHOD` - HTTP method to use (default: GET)
- `-d DATA` - Data to send for POST/PUT
- `-L` - Follow redirects (default: true)

### Authentication

- `-u USER:PASS` - Username and password for basic auth

## Output Formats

### HTML

The default output format. Returns the raw HTML of the fully rendered page after JavaScript execution.

```bash
churl https://example.com
```

### HAR (HTTP Archive)

Outputs a complete HAR file with all network requests and responses made during page load.

```bash
churl --output-format=har https://example.com > output.har
```

### Text

Attempts to extract plain text content from the page.

```bash
churl --output-format=text https://example.com
```

### JSON

Returns a JSON object with URL, title, and HTML content.

```bash
churl --output-format=json https://example.com
```

Output structure:

```json
{
  "url": "https://example.com",
  "title": "Example Domain",
  "content": "<!DOCTYPE html><html>..."
}
```

## Using Chrome Profiles

`churl` can use existing Chrome profiles to maintain sessions, cookies, and authentication:

```bash
# List available profiles
chrome-to-har --list-profiles

# Use a specific profile
churl --profile "Default" https://authenticated-site.com
```

## Use Cases

- Capture JavaScript-rendered content for scraping
- Test SPAs from the command line
- Create HAR files for network analysis
- Automated page capture in scripts
- Extract fully rendered HTML after JS execution