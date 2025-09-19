.PHONY: all build test clean chrome-to-har churl chrome-shell install docs lint
.PHONY: test-unit test-integration test-all test-chrome test-docker test-coverage

BINARY_NAME=chrome-to-har
CHURL_NAME=churl
SHELL_NAME=chrome-shell
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GO=$(shell which go)
GOFMT=$(shell which gofmt)
GOVET=$(GO) vet

all: build

# Build all binaries
build: chrome-to-har churl chrome-shell

chrome-to-har:
	$(GO) build -o $(BINARY_NAME) -ldflags="-X main.Version=$(VERSION)" .

churl:
	$(GO) build -o $(CHURL_NAME) -ldflags="-X main.Version=$(VERSION)" ./cmd/churl

chrome-shell:
	$(GO) build -o $(SHELL_NAME) -ldflags="-X main.Version=$(VERSION)" ./cmd/chrome-shell

# Run all tests
test: test-unit

# Run unit tests only
test-unit:
	$(GO) test -v -short ./...

# Run integration tests (requires Chrome)
test-integration:
	$(GO) test -v -tags=integration -timeout 10m ./...

# Run all tests including integration
test-all: test-unit test-integration

# Run tests with Chrome setup script
test-chrome:
	./test-chrome.sh --all

# Run tests in Docker
test-docker:
	docker-compose -f docker-compose.test.yml run --rm test

# Run tests with coverage
test-coverage:
	$(GO) test -v -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Run tests in CI mode
test-ci:
	@echo "Running tests in CI mode..."
	$(GO) test -v -short -coverprofile=coverage.out ./...

# Install binaries
install:
	$(GO) install .
	$(GO) install ./cmd/churl
	$(GO) install ./cmd/chrome-shell

# Clean built binaries and test artifacts
clean:
	rm -f $(BINARY_NAME) $(CHURL_NAME) $(SHELL_NAME)
	rm -f coverage.out coverage.html
	rm -rf coverage/

# Format code
fmt:
	$(GOFMT) -s -w .

# Run linters
lint:
	$(GOVET) ./...

# Generate docs
docs:
	@echo "No doc generation configured."

# Show help
help:
	@echo "make                 Build all binaries"
	@echo "make chrome-to-har   Build chrome-to-har binary"
	@echo "make churl           Build churl binary"
	@echo "make test            Run unit tests"
	@echo "make test-unit       Run unit tests only"
	@echo "make test-integration Run integration tests (requires Chrome)"
	@echo "make test-all        Run all tests"
	@echo "make test-chrome     Run tests with Chrome setup script"
	@echo "make test-docker     Run tests in Docker container"
	@echo "make test-coverage   Run tests with coverage report"
	@echo "make test-ci         Run tests in CI mode"
	@echo "make install         Install binaries"
	@echo "make clean           Remove built binaries and test artifacts"
	@echo "make fmt             Format code"
	@echo "make lint            Run linters"