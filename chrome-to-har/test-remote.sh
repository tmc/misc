#!/bin/bash
# Test script for remote Chrome connection functionality

echo "=== Chrome-to-HAR Remote Connection Test ==="
echo

# Check if Chrome is running with remote debugging
echo "Checking if Chrome is accessible on localhost:9222..."
if curl -s http://localhost:9222/json/version > /dev/null 2>&1; then
    echo "✓ Chrome is running with remote debugging enabled"
    echo
    
    # Get Chrome version
    echo "Chrome version info:"
    curl -s http://localhost:9222/json/version | jq -r '"\(.Browser)\nProtocol: \(."Protocol-Version")"' 2>/dev/null || curl -s http://localhost:9222/json/version
    echo
else
    echo "✗ Chrome is not accessible on localhost:9222"
    echo
    echo "Please start Chrome with remote debugging:"
    echo "  macOS: \"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome\" --remote-debugging-port=9222"
    echo "  Linux: google-chrome --remote-debugging-port=9222"
    echo "  Windows: chrome.exe --remote-debugging-port=9222"
    exit 1
fi

# Test listing tabs
echo "=== Testing tab listing ==="
./cdp --remote-host localhost --list-tabs
echo

# Test churl with remote Chrome
echo "=== Testing churl with remote Chrome ==="
echo "Fetching https://example.com..."
./churl --remote-host localhost --output-format json https://example.com | jq -r '.title, .url' 2>/dev/null || echo "Failed to fetch page"
echo

# Test cdp interactive mode (non-interactive test)
echo "=== Testing cdp commands ==="
echo -e "title\nurl\nexit" | ./cdp --remote-host localhost --url https://example.com 2>&1 | grep -E "(Result:|Connected|Error)"
echo

echo "=== Test complete ==="