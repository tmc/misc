# Go MCP Servers Makefile

.PHONY: all build clean test install help run-filesystem run-git run-http run-system run-time run-database

# Default target
all: build

# Build all servers
build:
	@echo "Building all MCP servers..."
	@mkdir -p bin
	go build -o bin/filesystem-mcp-server ./servers/filesystem
	go build -o bin/git-mcp-server ./servers/git
	go build -o bin/http-mcp-server ./servers/http
	go build -o bin/system-mcp-server ./servers/system
	go build -o bin/time-mcp-server ./servers/time
	go build -o bin/database-mcp-server ./servers/database
	@echo "All servers built successfully!"

# Build individual servers
build-filesystem:
	@echo "Building filesystem MCP server..."
	@mkdir -p bin
	go build -o bin/filesystem-mcp-server ./servers/filesystem

build-git:
	@echo "Building git MCP server..."
	@mkdir -p bin
	go build -o bin/git-mcp-server ./servers/git

build-http:
	@echo "Building HTTP MCP server..."
	@mkdir -p bin
	go build -o bin/http-mcp-server ./servers/http

build-system:
	@echo "Building system MCP server..."
	@mkdir -p bin
	go build -o bin/system-mcp-server ./servers/system

build-time:
	@echo "Building time MCP server..."
	@mkdir -p bin
	go build -o bin/time-mcp-server ./servers/time

build-database:
	@echo "Building database MCP server..."
	@mkdir -p bin
	go build -o bin/database-mcp-server ./servers/database

# Test all modules
test:
	@echo "Running tests..."
	go test ./...

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/

# Install dependencies
deps:
	@echo "Installing dependencies..."
	go mod tidy
	go mod download

# Run individual servers (for testing)
run-filesystem: build-filesystem
	@echo "Running filesystem MCP server..."
	./bin/filesystem-mcp-server

run-git: build-git
	@echo "Running git MCP server..."
	./bin/git-mcp-server

run-http: build-http
	@echo "Running HTTP MCP server..."
	./bin/http-mcp-server

run-system: build-system
	@echo "Running system MCP server..."
	./bin/system-mcp-server

run-time: build-time
	@echo "Running time MCP server..."
	./bin/time-mcp-server

run-database: build-database
	@echo "Running database MCP server..."
	./bin/database-mcp-server

# Install servers to system PATH
install: build
	@echo "Installing MCP servers to /usr/local/bin..."
	sudo cp bin/* /usr/local/bin/

# Create distribution archive
dist: build
	@echo "Creating distribution archive..."
	@mkdir -p dist
	tar -czf dist/go-mcp-servers.tar.gz bin/ README.md examples/

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Lint code (requires golangci-lint)
lint:
	@echo "Linting code..."
	golangci-lint run

# Help
help:
	@echo "Available targets:"
	@echo "  all          - Build all servers (default)"
	@echo "  build        - Build all servers"
	@echo "  build-*      - Build specific server (filesystem, git, http, system, time, database)"
	@echo "  test         - Run tests"
	@echo "  clean        - Clean build artifacts"
	@echo "  deps         - Install dependencies"
	@echo "  run-*        - Run specific server (filesystem, git, http, system, time, database)"
	@echo "  install      - Install servers to system PATH"
	@echo "  dist         - Create distribution archive"
	@echo "  fmt          - Format code"
	@echo "  lint         - Lint code"
	@echo "  help         - Show this help"