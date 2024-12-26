# chrome-to-har Usage Guide

`chrome-to-har` launches a headed Chrome browser and captures network activity to HAR format.

## Basic Usage

```bash
chrome-to-har [-profile=/path/to/chrome/profile] [-output=output.har] [-url=https://example.com] [-verbose] [-stream]
```

If no profile is specified, the first available Chrome profile will be automatically selected. Use `-verbose` to see which profile was selected.

## Options

- `-profile`: Chrome profile directory to use (defaults to first available profile)
- `-output`: Output HAR file (default: output.har)
- `-diff`: Enable differential HAR capture
- `-verbose`: Enable verbose logging
- `-url`: Starting URL to navigate to
- `-cookies`: Regular expression to filter cookies (default: capture all)
- `-urls`: Regular expression to filter URLs (default: capture all)
- `-stream`: Stream HAR entries as they are captured (outputs NDJSON)
- `-filter`: JQ expression to filter HAR entries
- `-template`: Go template to transform HAR entries
- `-block`: Regular expression of URLs to block from loading
- `-omit`: Regular expression of URLs to omit from HAR output
- `-cookie-domains`: Comma-separated list of domains to include cookies from
- `-headless`: Run Chrome in headless mode

## Examples

Basic usage with auto-selected profile:
```bash
chrome-to-har -verbose
```

Specify a profile explicitly:
```bash
chrome-to-har -profile="Profile 1"
```

Stream with auto-selected profile:
```bash
chrome-to-har -verbose -stream
```

## Streaming Mode

When using `-stream`, entries are output in NDJSON (Newline Delimited JSON) format as they are captured:

```jsonl
{"startedDateTime":"2024-01-01T00:00:00Z","request":{"method":"GET","url":"https://example.com"},"response":{"status":200}}
{"startedDateTime":"2024-01-01T00:00:01Z","request":{"method":"POST","url":"https://api.example.com"},"response":{"status":201}}
```

### Streaming Use Cases

1. Real-time monitoring:
   ```bash
   chrome-to-har -stream -filter='select(.response.status >= 400)' | tee errors.log
   ```

2. Live metrics collection:
   ```bash
   chrome-to-har -stream -template='{{.request.url}},{{.response.status}},{{.time}}' >> metrics.csv
   ```

3. API debugging:
   ```bash
   chrome-to-har -stream -urls='api\.example\.com' -filter='select(.request.method == "POST")'
   ```

## Filtering

### JQ Filters

Filter entries using JQ expressions:

```bash
# Only errors
chrome-to-har -filter='select(.response.status >= 400)'

# Specific domains
chrome-to-har -filter='select(.request.url | contains("api.example.com"))'

# Complex conditions
chrome-to-har -filter='select(.request.method == "POST" and .response.status < 300)'
```

### Template Transformations

Transform entries using Go templates:

```bash
# Basic request log
chrome-to-har -template='{{.startedDateTime}} {{.request.method}} {{.request.url}} {{.response.status}}'

# JSON transformation
chrome-to-har -template='{"url":"{{.request.url}}","duration":{{.time}}}'

# CSV format
chrome-to-har -template='{{.request.method}},{{.request.url}},{{.response.status}},{{.time}}'
```

## Common Use Cases

1. Authentication debugging:
   ```bash
   chrome-to-har -urls='auth|login' -cookies='session|token'
   ```

2. Performance monitoring:
   ```bash
   chrome-to-har -stream -template='{"url":"{{.request.url}}","time":{{.time}}}' | jq -c 'select(.time > 1000)'
   ```

3. API testing:
   ```bash
   chrome-to-har -urls='api' -filter='select(.request.headers[] | select(.name == "Authorization"))' -stream
   ```

4. Security scanning:
   ```bash
   chrome-to-har -filter='select(.response.headers[] | select(.name|test("^X-";"i")))' -stream
   ```

5. Load testing:
   ```bash
   chrome-to-har -template='{{.time}}' -urls='critical-endpoint' | awk '{sum+=$1} END {print "Avg:",sum/NR}'
   ```

## Advanced Features

### Cookie Management

Filter and manage cookies:

```bash
# Include specific domains
chrome-to-har -cookie-domains=example.com,api.example.com

# Filter by cookie name
chrome-to-har -cookies='session|auth|token'
```

### URL Control

Control which URLs are processed:

```bash
# Block tracking scripts
chrome-to-har -block='google-analytics|facebook|tracking'

# Omit from output
chrome-to-har -omit='\.png$|\.jpg$'

# Focus on specific paths
chrome-to-har -urls='/api/v[0-9]+'
```

### Differential Capture

Capture changes between runs:

```bash
# First run
chrome-to-har -output=baseline.har -url=https://example.com

# Compare with baseline
chrome-to-har -diff -output=diff.har -url=https://example.com
```

Would you like me to continue with the remaining files?
