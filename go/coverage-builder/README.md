# Go Coverage Builder

A web service that dynamically generates Go version wrappers with coverage instrumentation enabled. It automatically discovers available Go versions from go.dev/dl and creates custom wrappers that build Go from source with coverage support.

## Features

- **Dynamic Discovery**: Automatically fetches the latest Go versions from go.dev/dl
- **Coverage Instrumentation**: Builds Go with `GOEXPERIMENT=coverageredesign`
- **Version Support**: Works with any Go version (stable, beta, RC)
- **Checksum Verification**: Validates downloads using official SHA256 checksums
- **Simple API**: RESTful endpoints for easy integration
- **Shell & Go Wrappers**: Generates both shell scripts and Go source files

## Quick Start

### Running the Service

```bash
go run main.go
# Server starts on http://localhost:8080
```

### Using the Service

1. **Get a coverage wrapper for Go 1.23**:
```bash
curl -L http://localhost:8080/go1.23-cov > go1.23-cov
chmod +x go1.23-cov
./go1.23-cov download
```

2. **Build your application with coverage**:
```bash
./go1.23-cov build -cover -o myapp main.go
```

3. **Collect and analyze coverage data**:
```bash
GOCOVERDIR=/tmp/coverage ./myapp
./go1.23-cov tool covdata percent -i=/tmp/coverage
```

## API Endpoints

### `GET /`
Web interface showing available versions and documentation.

### `GET /api/versions`
Returns JSON with all available Go versions and their metadata.

### `GET /go{version}-cov`
Returns a shell script wrapper for the specified version.

Example: `curl http://localhost:8080/go1.23.4-cov`

### `GET /go{version}-cov.go`
Returns Go source code for building the wrapper.

Example: `curl http://localhost:8080/go1.23.4-cov.go`

### `GET /refresh`
Refreshes the version cache from go.dev/dl.

### `GET /health`
Health check endpoint returning service status.

## How It Works

1. **Version Discovery**: The service fetches version information from go.dev/dl
2. **Wrapper Generation**: Creates custom wrappers for each version using templates
3. **Coverage Build**: Wrappers download Go source and build with coverage enabled
4. **Local Installation**: Built versions are installed to `~/.go-coverage-builds/`

## Generated Wrapper Features

Each generated wrapper:
- Downloads the official Go source release
- Verifies SHA256 checksums
- Builds Go with coverage instrumentation
- Provides all standard Go commands
- Manages separate installation directories
- Supports additional GOEXPERIMENT values

## Example Workflow

```bash
# 1. Get wrapper for latest stable Go
curl -O http://localhost:8080/go1.23-cov
chmod +x go1.23-cov

# 2. Download and build Go with coverage
./go1.23-cov download

# 3. Build your app with coverage
./go1.23-cov build -cover -o webapp ./cmd/webapp

# 4. Run and collect coverage
mkdir coverage-data
GOCOVERDIR=coverage-data ./webapp

# 5. Analyze coverage including stdlib
./go1.23-cov tool covdata pkglist -i=coverage-data
./go1.23-cov tool covdata percent -i=coverage-data

# 6. See which stdlib packages were used
./go1.23-cov tool covdata pkglist -i=coverage-data | grep -E "^(net/|crypto/)"
```

## Environment Variables

For the service:
- `PORT`: Server port (default: 8080)

For generated wrappers:
- `GOEXPERIMENT`: Additional experiments to enable
- `SKIP_CHECKSUM`: Skip checksum verification if set to "1"
- `GOCOVERDIR`: Directory for coverage data collection

## Development

```bash
# Build the service
go build -o coverage-builder

# Run tests
go test ./...

# Build Docker image
docker build -t go-coverage-builder .
```

## Docker Usage

```dockerfile
FROM golang:1.21
WORKDIR /app
COPY . .
RUN go build -o coverage-builder
EXPOSE 8080
CMD ["./coverage-builder"]
```

```bash
docker run -p 8080:8080 go-coverage-builder
```

## Architecture

```
coverage-builder/
├── main.go                 # Main service implementation
├── templates/
│   └── wrapper.go.tmpl    # Template for generating wrappers
├── go.mod
└── README.md
```

## Limitations

- Building Go from source takes 5-10 minutes
- Coverage-instrumented binaries are larger and slower
- Requires significant disk space for each version
- Source downloads require internet access

## Security Considerations

- Always verify checksums when available
- Use HTTPS for production deployments
- Be cautious with executable downloads
- Consider rate limiting for public deployments

## Future Enhancements

- Caching of built versions
- Progress indicators for builds
- Multiple architecture support
- Webhook notifications for new versions
- Docker registry for pre-built images

## License

MIT License - See LICENSE file for details

## Contributing

Contributions welcome! Please:
1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Submit a pull request

## Support

For issues and questions:
- Create an issue on GitHub
- Check existing issues first
- Include version information and logs