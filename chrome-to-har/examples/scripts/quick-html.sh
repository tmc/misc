#!/bin/bash
# Quick HTML fetch script using churl
# Usage: ./quick-html.sh <URL> [output-file]

set -e

if [ $# -lt 1 ]; then
    echo "Usage: $0 <URL> [output-file]"
    echo "Example: $0 https://example.com my-site.html"
    exit 1
fi

URL="$1"
OUTPUT="${2:-output.html}"

echo "Fetching HTML from: $URL"
echo "Output file: $OUTPUT"

# Check if churl is available
if ! command -v churl &> /dev/null; then
    echo "Error: churl not found. Please install it first."
    echo "Run: go install github.com/tmc/misc/chrome-to-har/cmd/churl@latest"
    exit 1
fi

# Fetch HTML
churl "$URL" > "$OUTPUT"

echo "HTML file saved to: $OUTPUT"
echo "File size: $(wc -c < "$OUTPUT") bytes"