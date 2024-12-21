# chrome-to-har

Launches a headed Chrome browser and captures network activity to HAR format.

## Usage

```
chrome-to-har -profile=/path/to/chrome/profile -output=output.har [-url=https://example.com] [-verbose] [-cookies=regexp] [-urls=regexp]
```

### Options

- `-profile`: Chrome profile directory to use
- `-output`: Output HAR file (default: output.har)
- `-diff`: Enable differential HAR capture
- `-verbose`: Enable verbose logging
- `-url`: Starting URL to navigate to
- `-cookies`: Regular expression to filter cookies (default: capture all)
- `-urls`: Regular expression to filter URLs (default: capture all)

Press Ctrl+D to capture the HAR file.

## Features

- Captures all network requests and responses
- Includes cookies from Chrome profile
- Supports verbose logging
- Can start with a specific URL
- Preserves authentication and session data from profile
- Filtering support for cookies and URLs
```
