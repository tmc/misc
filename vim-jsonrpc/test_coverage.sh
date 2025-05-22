#!/bin/bash

set -e

echo "Running comprehensive test suite for vim-jsonrpc..."

# Clean any previous coverage files
rm -f coverage.out coverage.html

echo "=== Running unit tests with coverage ==="
go test -v -race -coverprofile=coverage.out -covermode=atomic ./...

echo ""
echo "=== Generating coverage report ==="
go tool cover -html=coverage.out -o coverage.html

echo ""
echo "=== Coverage summary ==="
go tool cover -func=coverage.out | tail -1

echo ""
echo "=== Running integration tests ==="
go test -v -tags=integration ./...

echo ""
echo "=== Running script tests ==="
go test -v -run TestMain ./...

echo ""
echo "=== Testing examples compilation ==="
echo "Testing client example..."
go build -o /tmp/test_client examples/client/simple_client.go
echo "✓ Client example compiles successfully"

echo "Testing server example..."
go build -o /tmp/test_server examples/server/simple_server.go
echo "✓ Server example compiles successfully"

echo ""
echo "=== Checking code quality ==="
echo "Running go vet..."
go vet ./...
echo "✓ go vet passed"

echo ""
echo "Running go fmt check..."
if [ "$(gofmt -l . | wc -l)" -ne 0 ]; then
    echo "❌ Code is not formatted. Run 'go fmt ./...'"
    gofmt -l .
    exit 1
else
    echo "✓ Code is properly formatted"
fi

echo ""
echo "=== Dependency check ==="
echo "Running go mod verify..."
go mod verify
echo "✓ Module verification passed"

echo "Running go mod tidy check..."
cp go.mod go.mod.bak
cp go.sum go.sum.bak
go mod tidy
if ! diff -q go.mod go.mod.bak >/dev/null || ! diff -q go.sum go.sum.bak >/dev/null; then
    echo "❌ go.mod/go.sum not tidy. Run 'go mod tidy'"
    mv go.mod.bak go.mod
    mv go.sum.bak go.sum
    exit 1
else
    echo "✓ go.mod and go.sum are tidy"
    rm go.mod.bak go.sum.bak
fi

echo ""
echo "=== Build verification ==="
echo "Building main binary..."
go build -o /tmp/vim-jsonrpc .
echo "✓ Main binary builds successfully"

echo ""
echo "=== Test results summary ==="
echo "✓ All unit tests passed"
echo "✓ All integration tests passed"
echo "✓ Code coverage report generated (coverage.html)"
echo "✓ Examples compile successfully"
echo "✓ Code quality checks passed"
echo "✓ Dependencies verified"
echo "✓ Build verification passed"

echo ""
echo "Coverage report available at: coverage.html"
echo "Test suite completed successfully! 🎉"