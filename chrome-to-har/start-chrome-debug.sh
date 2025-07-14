#!/bin/bash
# Script to start Chrome with remote debugging enabled

echo "Starting Chrome with remote debugging on port 9222..."
echo

# Detect OS and Chrome path
if [[ "$OSTYPE" == "darwin"* ]]; then
    # macOS
    CHROME_PATHS=(
        "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
        "/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary"
        "/Applications/Brave Browser.app/Contents/MacOS/Brave Browser"
        "/Applications/Chromium.app/Contents/MacOS/Chromium"
    )
    
    for chrome in "${CHROME_PATHS[@]}"; do
        if [ -f "$chrome" ]; then
            echo "Found: $chrome"
            "$chrome" --remote-debugging-port=9222 --new-window about:blank &
            break
        fi
    done
    
elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
    # Linux
    if command -v google-chrome &> /dev/null; then
        google-chrome --remote-debugging-port=9222 --new-window about:blank &
    elif command -v chromium &> /dev/null; then
        chromium --remote-debugging-port=9222 --new-window about:blank &
    elif command -v brave-browser &> /dev/null; then
        brave-browser --remote-debugging-port=9222 --new-window about:blank &
    else
        echo "No Chrome/Chromium browser found in PATH"
        exit 1
    fi
    
elif [[ "$OSTYPE" == "msys" || "$OSTYPE" == "cygwin" ]]; then
    # Windows
    "/c/Program Files/Google/Chrome/Application/chrome.exe" --remote-debugging-port=9222 --new-window about:blank &
fi

echo
echo "Waiting for Chrome to start..."
sleep 2

# Check if Chrome is accessible
if curl -s http://localhost:9222/json/version > /dev/null 2>&1; then
    echo "✓ Chrome is now running with remote debugging on port 9222"
    echo
    echo "You can now use:"
    echo "  ./cdp --remote-host localhost"
    echo "  ./churl --remote-host localhost https://example.com"
else
    echo "✗ Failed to connect to Chrome on port 9222"
    echo "Please check if Chrome started correctly"
fi