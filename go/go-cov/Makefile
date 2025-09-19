.PHONY: server build test clean deploy test-local test-manual test-restricted versions

# Run the web service locally
server:
	@echo "Starting go-cov server on http://localhost:8080"
	@echo "Test with: curl http://localhost:8080/go-cov1.24.3?go-get=1"
	PORT=8080 go run cmd/server/main.go

# Build the server
build:
	go build -o go-cov-server cmd/server/main.go

# Run automated tests
test:
	go test -v ./...

# Run local integration test
test-local:
	./test-local.sh

# Manual testing commands
test-manual:
	@echo "=== Manual Testing Commands ==="
	@echo ""
	@echo "1. Start server:"
	@echo "   make server"
	@echo ""
	@echo "2. Test endpoints:"
	@echo "   curl 'http://localhost:8080/go-cov1.24.3?go-get=1'"
	@echo "   curl 'http://localhost:8080/go-cov1.24.3/@v/latest.info'"
	@echo "   curl 'http://localhost:8080/go-cov1.24.3/@v/latest.mod'"
	@echo "   curl 'http://localhost:8080/go-cov1.24.3/@v/latest.zip' -o installer.zip"
	@echo ""
	@echo "3. Test installer:"
	@echo "   unzip installer.zip"
	@echo "   go build main.go"
	@echo "   ./main  # Should fail with network error"

# Test with restricted PATH (like the scripttest)
test-restricted:
	@echo "Testing with restricted PATH..."
	@export PATH="$$(go env GOROOT)/bin"; \
	export HOME=/tmp/go-cov-test; \
	mkdir -p $$HOME/sdk; \
	echo "PATH=$$PATH"; \
	echo "Testing installer compilation..."; \
	curl -s 'http://localhost:8080/go-cov1.24.3/@v/latest.zip' -o /tmp/installer.zip; \
	cd /tmp; \
	unzip -q installer.zip; \
	go build -o installer main.go && echo "✅ Compiles with restricted PATH" || echo "❌ Failed to compile"

# Clean build artifacts
clean:
	rm -f go-cov-server
	rm -f /tmp/installer.zip /tmp/main.go /tmp/go.mod /tmp/installer

# Deploy information
deploy:
	@echo "Deploy cmd/server to go.tmc.dev"
	@echo ""
	@echo "Service endpoints:"
	@echo "  GET /go-cov1.24.3?go-get=1       - Module discovery"
	@echo "  GET /go-cov1.24.3/@v/list        - List versions"  
	@echo "  GET /go-cov1.24.3/@v/latest.info - Version info"
	@echo "  GET /go-cov1.24.3/@v/latest.mod  - Module file"
	@echo "  GET /go-cov1.24.3/@v/latest.zip  - Generated installer"
	@echo ""
	@echo "Usage:"
	@echo "  go run go.tmc.dev/go-cov1.24.3@latest"

# Show available versions
versions:
	@echo "Supported Go versions (dynamically generated):"
	@echo "  Any version from https://go.dev/dl/"
	@echo ""
	@echo "Examples:"
	@echo "  go run go.tmc.dev/go-cov1.21.0@latest"
	@echo "  go run go.tmc.dev/go-cov1.22.5@latest"
	@echo "  go run go.tmc.dev/go-cov1.23.4@latest"
	@echo "  go run go.tmc.dev/go-cov1.24.3@latest"