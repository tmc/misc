# chrome-to-har

Launches a headed Chrome browser and captures network activity to HAR format.

## Usage

```
chrome-to-har -profile=/path/to/chrome/profile -output=output.har [-url=https://example.com] [-verbose] [-cookies=regexp] [-urls=regexp] [-stream] [-filter='jq expr']
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

Press Ctrl+D to capture the HAR file.

### Streaming Format

When using `-stream`, entries are output in NDJSON (Newline Delimited JSON) format. Each line contains a single HAR entry:

```jsonl
{"startedDateTime":"2024-01-01T00:00:00Z","request":{"method":"GET","url":"https://example.com"},"response":{"status":200}}
{"startedDateTime":"2024-01-01T00:00:01Z","request":{"method":"POST","url":"https://api.example.com"},"response":{"status":201}}
```

### Filtering Examples

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
- Includes cookies from Chrome profile
- Supports verbose logging
- Can start with a specific URL
- Preserves authentication and session data from profile
- Filtering support for cookies and URLs
- Optional streaming mode for real-time HAR entry output
- JQ-style filtering expressions
- Go template support for custom output formats
```

