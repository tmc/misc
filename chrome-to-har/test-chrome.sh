#!/bin/bash
# Script to run Chrome-based tests with proper setup

set -e

echo "Chrome Test Runner for chrome-to-har"
echo "===================================="
echo

# Detect Chrome installation
find_chrome() {
    if [[ "$OSTYPE" == "darwin"* ]]; then
        # macOS
        CHROME_PATHS=(
            "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
            "/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary"
            "/Applications/Chromium.app/Contents/MacOS/Chromium"
            "/Applications/Brave Browser.app/Contents/MacOS/Brave Browser"
        )
        
        for chrome in "${CHROME_PATHS[@]}"; do
            if [ -f "$chrome" ]; then
                echo "$chrome"
                return 0
            fi
        done
    elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
        # Linux
        if command -v google-chrome-stable &> /dev/null; then
            echo "google-chrome-stable"
            return 0
        elif command -v google-chrome &> /dev/null; then
            echo "google-chrome"
            return 0
        elif command -v chromium-browser &> /dev/null; then
            echo "chromium-browser"
            return 0
        elif command -v chromium &> /dev/null; then
            echo "chromium"
            return 0
        fi
    fi
    
    return 1
}

# Check for Chrome
CHROME_PATH=$(find_chrome)
if [ -z "$CHROME_PATH" ]; then
    echo "Error: Chrome/Chromium not found!"
    echo "Please install Chrome or set CHROME_PATH environment variable"
    exit 1
fi

echo "Found Chrome: $CHROME_PATH"
export CHROME_PATH

# Get Chrome version
if [[ "$CHROME_PATH" == *".app"* ]]; then
    CHROME_VERSION=$("$CHROME_PATH" --version 2>/dev/null || echo "Unknown")
else
    CHROME_VERSION=$($CHROME_PATH --version 2>/dev/null || echo "Unknown")
fi
echo "Chrome version: $CHROME_VERSION"
echo

# Parse command line options
RUN_UNIT=true
RUN_INTEGRATION=false
HEADLESS=true
VERBOSE=false
TIMEOUT="10m"
SPECIFIC_TEST=""

while [[ $# -gt 0 ]]; do
    case $1 in
        --integration)
            RUN_INTEGRATION=true
            RUN_UNIT=false
            shift
            ;;
        --all)
            RUN_INTEGRATION=true
            RUN_UNIT=true
            shift
            ;;
        --no-headless)
            HEADLESS=false
            shift
            ;;
        --verbose|-v)
            VERBOSE=true
            shift
            ;;
        --timeout)
            TIMEOUT="$2"
            shift 2
            ;;
        --test)
            SPECIFIC_TEST="$2"
            shift 2
            ;;
        -h|--help)
            echo "Usage: $0 [options]"
            echo
            echo "Options:"
            echo "  --integration    Run only integration tests"
            echo "  --all           Run both unit and integration tests"
            echo "  --no-headless   Run Chrome in visible mode"
            echo "  --verbose, -v   Enable verbose output"
            echo "  --timeout TIME  Set test timeout (default: 10m)"
            echo "  --test NAME     Run specific test by name"
            echo "  -h, --help      Show this help message"
            echo
            echo "Environment variables:"
            echo "  CHROME_PATH     Path to Chrome executable"
            echo "  HEADLESS        Set to 'false' to disable headless mode"
            echo
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Set environment variables
export HEADLESS=$HEADLESS
if [ "$HEADLESS" = "true" ]; then
    echo "Running in headless mode"
else
    echo "Running in visible mode (not headless)"
fi

# For Linux, we might need Xvfb for headless testing
if [[ "$OSTYPE" == "linux-gnu"* ]] && [ "$HEADLESS" = "true" ]; then
    if command -v xvfb-run &> /dev/null; then
        echo "Using Xvfb for headless testing on Linux"
        XVFB_PREFIX="xvfb-run -a"
    else
        echo "Warning: xvfb-run not found. Tests might fail in headless mode."
        echo "Install with: sudo apt-get install xvfb"
    fi
else
    XVFB_PREFIX=""
fi

echo

# Build the project first
echo "Building project..."
go build -o chrome-to-har .
go build -o churl ./cmd/churl
go build -o cdp ./cmd/cdp
echo "Build complete"
echo

# Run tests
FAILED=0

if [ "$RUN_UNIT" = "true" ]; then
    echo "Running unit tests..."
    echo "===================="
    
    TEST_CMD="go test -v -short -timeout $TIMEOUT"
    if [ -n "$SPECIFIC_TEST" ]; then
        TEST_CMD="$TEST_CMD -run $SPECIFIC_TEST"
    fi
    TEST_CMD="$TEST_CMD ./..."
    
    if [ "$VERBOSE" = "true" ]; then
        echo "Command: $XVFB_PREFIX $TEST_CMD"
    fi
    
    if $XVFB_PREFIX $TEST_CMD; then
        echo "✓ Unit tests passed"
    else
        echo "✗ Unit tests failed"
        FAILED=1
    fi
    echo
fi

if [ "$RUN_INTEGRATION" = "true" ]; then
    echo "Running integration tests..."
    echo "==========================="
    
    TEST_CMD="go test -v -tags=integration -timeout $TIMEOUT"
    if [ -n "$SPECIFIC_TEST" ]; then
        TEST_CMD="$TEST_CMD -run $SPECIFIC_TEST"
    fi
    TEST_CMD="$TEST_CMD ./..."
    
    if [ "$VERBOSE" = "true" ]; then
        echo "Command: $XVFB_PREFIX $TEST_CMD"
    fi
    
    if $XVFB_PREFIX $TEST_CMD; then
        echo "✓ Integration tests passed"
    else
        echo "✗ Integration tests failed"
        FAILED=1
    fi
    echo
fi

# Summary
echo "Test Summary"
echo "============"
if [ $FAILED -eq 0 ]; then
    echo "✓ All tests passed!"
else
    echo "✗ Some tests failed"
    exit 1
fi

# Cleanup
echo
echo "Cleaning up test binaries..."
rm -f chrome-to-har churl cdp

echo "Done!"