#!/bin/bash
# Quick HAR capture script
# Usage: ./quick-har.sh <URL> [output-file]

set -e

if [ $# -lt 1 ]; then
    echo "Usage: $0 <URL> [output-file]"
    echo "Example: $0 https://example.com my-site.har"
    exit 1
fi

URL="$1"
OUTPUT="${2:-output.har}"

echo "Capturing HAR from: $URL"
echo "Output file: $OUTPUT"

# Check if chrome-to-har is available
if ! command -v chrome-to-har &> /dev/null; then
    echo "Error: chrome-to-har not found. Please install it first."
    echo "Run: go install github.com/tmc/misc/chrome-to-har@latest"
    exit 1
fi

# Capture HAR
chrome-to-har --output "$OUTPUT" "$URL"

echo "HAR file saved to: $OUTPUT"
echo "File size: $(wc -c < "$OUTPUT") bytes"