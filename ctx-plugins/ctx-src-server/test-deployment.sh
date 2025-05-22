#!/bin/bash
# Test the deployed ctx-src-server
set -e

# Default values
SERVICE_URL=""
OWNER="tmc"
REPO="misc"
PATHS=("ctx-plugins/**/*.go")
EXCLUDES=()
REF="main"
NO_XML=false
OUTPUT_FILE=""

# Parse command-line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --url)
            SERVICE_URL="$2"
            shift 2
            ;;
        --owner)
            OWNER="$2"
            shift 2
            ;;
        --repo)
            REPO="$2"
            shift 2
            ;;
        --path)
            PATHS+=("$2")
            shift 2
            ;;
        --exclude)
            EXCLUDES+=("$2")
            shift 2
            ;;
        --ref)
            REF="$2"
            shift 2
            ;;
        --no-xml)
            NO_XML=true
            shift
            ;;
        --output)
            OUTPUT_FILE="$2"
            shift 2
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Check if service URL is provided
if [ -z "$SERVICE_URL" ]; then
    echo "Service URL is required. Please specify with --url flag."
    exit 1
fi

# Remove http:// or https:// prefix if present
SERVICE_URL="${SERVICE_URL#http://}"
SERVICE_URL="${SERVICE_URL#https://}"

# Create JSON request
JSON_REQUEST=$(cat <<EOF
{
  "owner": "$OWNER",
  "repo": "$REPO",
  "ref": "$REF",
  "paths": $(printf '%s\n' "${PATHS[@]}" | jq -R . | jq -s .),
  "excludes": $(printf '%s\n' "${EXCLUDES[@]}" | jq -R . | jq -s .),
  "no_xml": $NO_XML
}
EOF
)

echo "Sending request to https://$SERVICE_URL/src"
echo "Request body:"
echo "$JSON_REQUEST" | jq .

# Send request and save response
if [ -n "$OUTPUT_FILE" ]; then
    echo "Saving response to $OUTPUT_FILE"
    curl -X POST "https://$SERVICE_URL/src" \
        -H "Content-Type: application/json" \
        -d "$JSON_REQUEST" \
        -o "$OUTPUT_FILE"
    
    # Display file size
    SIZE=$(wc -c < "$OUTPUT_FILE")
    LINES=$(wc -l < "$OUTPUT_FILE")
    echo "Response saved to $OUTPUT_FILE ($SIZE bytes, $LINES lines)"
else
    # Stream response to stdout
    curl -X POST "https://$SERVICE_URL/src" \
        -H "Content-Type: application/json" \
        -d "$JSON_REQUEST"
fi