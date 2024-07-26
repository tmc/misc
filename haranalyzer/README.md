# HAR Analyzer

HAR Analyzer is a powerful command-line tool for analyzing HTTP Archive (HAR) files with advanced filtering capabilities. It allows you to process large HAR files, apply complex filters, and generate insightful summaries of network activity.

## Features

- Parse and analyze large HAR files
- Advanced filtering options (URL, HTTP method, status code, response time, response size, content type)
- Custom query language for complex filtering
- Sort entries by time, size, status code, or URL
- Chunk-based analysis with configurable chunk size
- Multiple output formats (text, JSON, CSV)
- Optional AI-powered analysis using the Anthropic API

## Installation

1. Ensure you have Go 1.20 or later installed on your system.
2. Clone this repository:
   ```
   git clone https://github.com/tmc/haranalyzer.git
   ```
3. Change to the project directory:
   ```
   cd haranalyzer
   ```
4. Build the project:
   ```
   go build
   ```

## Usage

```
haranalyzer [flags]
```

### Flags

- `-i, --input string`: Input HAR file (required)
- `-o, --output string`: Output format (text, json, csv) (default "text")
- `-s, --sort string`: Sort entries by (time, size, status, url) (default "time")
- `-c, --chunk int`: Chunk size for analysis (default 100)
- `-q, --query string`: Query string for filtering
- `-a, --ai`: Perform AI analysis
- `--anthropic-key string`: Anthropic API key

### Examples

1. Basic usage:
   ```
   ./haranalyzer -i example.har
   ```

2. Use JSON output and sort by response size:
   ```
   ./haranalyzer -i example.har -o json -s size
   ```

3. Apply a complex filter:
   ```
   ./haranalyzer -i example.har -q "method:GET AND status:200-299 AND time>1000"
   ```

4. Perform AI analysis (requires Anthropic API key):
   ```
   ./haranalyzer -i example.har -a --anthropic-key YOUR_API_KEY
   ```

## Query Language

The query language allows you to create complex filters using the following syntax:

- `key:value`: Exact match (e.g., `method:GET`)
- `key:value1-value2`: Range match (e.g., `status:200-299`)
- `key>value` or `key<value`: Greater than or less than (e.g., `time>1000`)
- `key~value`: Contains match (e.g., `url~example.com`)
- `key/regex/`: Regular expression match (e.g., `url/^https:\/\/api\./)

You can combine multiple conditions using `AND` and `OR` operators.

Example complex query:
```
method:GET AND status:200-299 AND time>1000 AND (url~example.com OR url/^https:\/\/api\./)
```

This query filters for GET requests with status codes between 200 and 299, response times greater than 1000ms, and URLs that either contain "example.com" or match the regex `^https:\/\/api\.`.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

