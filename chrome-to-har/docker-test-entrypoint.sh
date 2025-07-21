#!/bin/bash
# Entry point for Docker test container

set -e

# Start Xvfb
echo "Starting Xvfb..."
Xvfb :99 -screen 0 1024x768x24 -nolisten tcp -nolisten unix &
XVFB_PID=$!

# Wait for Xvfb to start
sleep 2

# Function to cleanup on exit
cleanup() {
    echo "Cleaning up..."
    if [ -n "$XVFB_PID" ]; then
        kill $XVFB_PID 2>/dev/null || true
    fi
}
trap cleanup EXIT

# Default to running all tests
TEST_COMMAND="${1:-all}"

case "$TEST_COMMAND" in
    unit)
        echo "Running unit tests..."
        go test -v -short -timeout 10m ./...
        ;;
    integration)
        echo "Running integration tests..."
        go test -v -tags=integration -timeout 10m ./...
        ;;
    all)
        echo "Running all tests..."
        go test -v -short -timeout 10m ./...
        go test -v -tags=integration -timeout 10m ./...
        ;;
    coverage)
        echo "Running tests with coverage..."
        go test -v -coverprofile=coverage.out -timeout 10m ./...
        go tool cover -html=coverage.out -o coverage.html
        echo "Coverage report generated: coverage.html"
        ;;
    *)
        # Pass through any other commands
        exec "$@"
        ;;
esac