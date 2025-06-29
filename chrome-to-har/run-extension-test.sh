#!/bin/bash

echo "=== Chrome AI Extension Test ==="
echo ""
echo "This will test our browser extension approach for AI API access."
echo ""

# Step 1: Launch Chrome with extension loading capability
echo "Step 1: Launching Chrome Canary with extension support..."
echo ""

# Kill any existing Chrome processes
pkill -f "Google Chrome Canary" 2>/dev/null || true
sleep 2

# Get the extension path
EXTENSION_PATH="$(pwd)/extension"
echo "Extension path: $EXTENSION_PATH"

# Launch Chrome with our extension
"/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary" \
  --load-extension="$EXTENSION_PATH" \
  --enable-features=Gemini,AILanguageModelService,BuiltInAIAPI \
  --enable-ai-language-model-service \
  --optimization-guide-on-device-model=enabled \
  --prompt-api-for-gemini-nano=enabled \
  --prompt-api-for-gemini-nano-multimodal-input=enabled \
  --remote-debugging-port=9230 \
  --user-data-dir=/tmp/chrome-extension-test \
  --no-first-run \
  --no-default-browser-check \
  > /dev/null 2>&1 &

CHROME_PID=$!
echo "Chrome launched with PID: $CHROME_PID"
echo ""
echo "Waiting for Chrome to start..."
sleep 8

echo "Chrome should now be running with our extension loaded."
echo ""
echo "Next steps:"
echo "1. In Chrome, you should see our 'AI API Bridge' extension icon"
echo "2. Navigate to any website (e.g., https://example.com)"
echo "3. Click the extension icon to test AI API access"
echo "4. The extension popup will show if AI APIs are available"
echo ""
echo "Press Enter when you're ready to test the Go connection..."
read

echo ""
echo "Step 2: Testing Go ↔ Extension communication..."