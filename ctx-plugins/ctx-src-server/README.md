# ctx-src-server

A server that fetches source code from GitHub repositories using ctx-src and provides an HTTP API to access it. It uses gcsfuse for caching repositories in Google Cloud Storage.

## Features

- HTTP API for fetching source code from GitHub repositories
- Caching repositories locally or in Google Cloud Storage via gcsfuse
- Concurrent repository processing with configurable limits
- Supports specific git references (branches, tags, or commits)
- Flexible path filtering with include/exclude patterns
- XML or plain text output formats

## Installation

### Local Build

```bash
go get github.com/tmc/misc/ctx-plugins/ctx-src-server
cd $GOPATH/src/github.com/tmc/misc/ctx-plugins/ctx-src-server
go build
```

### Docker

```bash
docker build -t ctx-src-server .
```

### Cloud Run Deployment

We provide a simple script to deploy to Google Cloud Run:

```bash
# Set your Google Cloud project ID
export PROJECT_ID=your-project-id

# Make the deployment script executable
chmod +x deploy-to-cloudrun.sh

# Basic deployment using local cache
./deploy-to-cloudrun.sh --project $PROJECT_ID --region us-central1

# Deployment with Google Cloud Storage for caching
./deploy-to-cloudrun.sh --project $PROJECT_ID --region us-central1 --create-gcs-bucket
```

This script handles:
1. Building and pushing the Docker image to Google Container Registry
2. Creating a service account with necessary permissions
3. Setting up a Google Cloud Storage bucket for caching (optional)
4. Deploying the service to Cloud Run with appropriate settings
5. Configuring public access (you can modify this in the script)

For advanced configuration, the script supports several options:
```bash
./deploy-to-cloudrun.sh \
  --project my-project \
  --region us-central1 \
  --service-name ctx-src-server \
  --create-gcs-bucket \
  --gcs-bucket my-custom-bucket-name \
  --gcs-mount /mnt/ctx-src-cache \
  --max-concurrent 3 \
  --clone-timeout 5m \
  --default-branch main
```

Run `./deploy-to-cloudrun.sh --help` for a complete list of options.

## Usage

### Starting the Server

```bash
# Using local cache
./ctx-src-server --addr=:8080 --cache-dir=/tmp/ctx-src-cache

# Using GCS bucket for cache
./ctx-src-server --addr=:8080 --gcs-bucket=my-cache-bucket --gcs-mount=/mnt/ctx-src-cache
```

### Docker Example

```bash
# Using local cache
docker run -p 8080:8080 ctx-src-server

# Using GCS bucket with service account credentials
docker run -p 8080:8080 \
  -v /path/to/credentials.json:/credentials.json \
  -e GOOGLE_APPLICATION_CREDENTIALS=/credentials.json \
  ctx-src-server --gcs-bucket=my-cache-bucket
```

## Command Line Options

- `--addr`: Address to listen on (default ":8080")
- `--cache-dir`: Directory for caching repositories (default "/tmp/ctx-src-cache")
- `--gcs-bucket`: GCS bucket name for gcsfuse cache (if empty, local cache will be used)
- `--gcs-mount`: Mount point for gcsfuse bucket (default "/mnt/ctx-src-cache")
- `--clone-timeout`: Timeout for cloning repositories (default 5m)
- `--max-concurrent`: Maximum number of concurrent git operations (default 5)
- `--verbose`: Enable verbose logging
- `--ctx-src-path`: Path to ctx-src binary (if empty, assumed to be in PATH)
- `--default-branch`: Default branch to use if none specified (default "main")

## API

### GET /healthz

Health check endpoint that returns "ok" if the server is running.

### GET /metrics

Returns JSON-formatted metrics about the server's performance and usage:

```json
{
  "uptime": "3h2m5.123456789s",
  "request_count": 120,
  "success_count": 118,
  "error_count": 2,
  "success_rate": 98.33333333333333,
  "avg_processing_time_ms": 325.5,
  "cache_hits": 80,
  "cache_misses": 40,
  "cache_hit_rate": 66.66666666666666,
  "total_bytes_served": 5236418,
  "current_goroutines": 8,
  "num_cpu": 8,
  "go_version": "go1.20",
  "concurrent_git_operations": 1
}
```

### POST /src

Fetch source code from a GitHub repository.

#### Request Body

```json
{
  "owner": "username",          // GitHub username or organization (required)
  "repo": "repository-name",    // GitHub repository name (required)
  "ref": "main",                // Git reference (branch, tag, or commit hash)
  "paths": [                    // Array of paths to include (optional)
    "*.go",
    "cmd/**/*.go"
  ],
  "excludes": [                 // Array of paths to exclude (optional)
    "vendor/**",
    "**/*_test.go"
  ],
  "no_xml": false               // If true, returns plain text without XML tags
}
```

#### Response

The response is a text content containing the source code from the repository, either in XML format or plain text depending on the `no_xml` flag.

XML format example:
```xml
<src path="/tmp/ctx-src-cache/username/repository-name">
  <file path="main.go">
    package main
    
    import "fmt"
    
    func main() {
        fmt.Println("Hello, World!")
    }
  </file>
  <file path="utils/helper.go">
    package utils
    
    func Helper() string {
        return "I'm a helper function"
    }
  </file>
</src>
```

## Testing and Using the Deployed Service

### Using the Test Script

We provide a simple test script to verify your deployment:

```bash
# Make the test script executable
chmod +x test-deployment.sh

# Test with the URL provided after deployment
./test-deployment.sh --url https://ctx-src-server-abc123xyz.a.run.app \
  --owner tmc \
  --repo misc \
  --path "ctx-plugins/**/*.go" \
  --exclude "**/vendor/**" \
  --ref main \
  --output output.txt
```

This script will make a request to your deployed service and save or display the response.

### Example Curl Commands

Fetch source code from a repository:
```bash
curl -X POST https://your-service-url.a.run.app/src \
  -H "Content-Type: application/json" \
  -d '{
    "owner": "tmc",
    "repo": "misc",
    "ref": "main",
    "paths": ["ctx-plugins/**/*.go"],
    "excludes": ["**/vendor/**"]
  }'
```

View server metrics:
```bash
curl https://your-service-url.a.run.app/metrics | jq .
```

### Using the Client Example

```bash
go build -o ctx-src-client client/main.go
./ctx-src-client \
  --server https://your-service-url.a.run.app \
  --repo tmc/misc \
  --paths ctx-plugins/**/*.go \
  --excludes **/vendor/** \
  --output output.txt
```

## License

This project is licensed under the MIT License.