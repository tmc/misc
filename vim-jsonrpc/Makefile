.PHONY: build test clean install examples lint fmt vet

# Build the main binary
build:
	go build -o bin/vim-jsonrpc ./main.go

# Run all tests
test:
	go test -v ./...

# Run tests with coverage
test-cover:
	go test -v -cover ./...

# Run tests with race detection
test-race:
	go test -v -race ./...

# Clean build artifacts
clean:
	rm -rf bin/
	go clean

# Install the binary
install:
	go install ./...

# Build examples
examples:
	go build -o bin/simple-server ./examples/simple_server.go
	go build -o bin/simple-client ./examples/simple_client.go

# Run linter
lint:
	golangci-lint run

# Format code
fmt:
	go fmt ./...

# Run vet
vet:
	go vet ./...

# Run all checks
check: fmt vet lint test

# Development setup
dev-setup:
	go mod tidy
	go mod download

# Generate documentation
docs:
	godoc -http=:6060

# Benchmark tests
bench:
	go test -bench=. ./...

# Create directories
dirs:
	mkdir -p bin/

# Full build with all checks
all: dirs check build examples

# Demo - run server and client
demo-server:
	go run examples/simple_server.go

demo-client:
	go run examples/simple_client.go