#!/bin/bash
# Example script showing POST/PUT usage with churl

# Build the churl command
echo "Building churl..."
go build -o churl ./cmd/churl || exit 1

echo "=== churl POST/PUT Examples ==="
echo

# Note: These examples require a running HTTP server to test against
# For demonstration, we'll show the command syntax

echo "1. Basic JSON POST request:"
echo "./churl -X POST -d '{\"name\": \"test\", \"value\": 42}' -H 'Content-Type: application/json' https://httpbin.org/post"
echo

echo "2. Form data POST request:"
echo "./churl -X POST -d 'name=test&value=42' https://httpbin.org/post"
echo

echo "3. PUT request with auto-detected content type:"
echo "./churl -X PUT -d '{\"id\": 123, \"name\": \"updated\"}' https://httpbin.org/put"
echo

echo "4. POST request with custom headers:"
echo "./churl -X POST -d 'key=value' -H 'Authorization: Bearer token123' -H 'Custom-Header: value' https://httpbin.org/post"
echo

echo "5. Plain text POST:"
echo "./churl -X POST -d 'Simple text data' https://httpbin.org/post"
echo

echo "6. GET request with churl (for comparison):"
echo "./churl https://httpbin.org/get"
echo

echo "=== Usage Notes ==="
echo "- churl automatically detects content type based on data format"
echo "- JSON objects/arrays -> application/json"
echo "- key=value&key2=value2 -> application/x-www-form-urlencoded" 
echo "- Other text -> text/plain"
echo "- Custom Content-Type headers override auto-detection"
echo "- Use -v flag for verbose output to see request details"

echo
echo "For testing, you can use httpbin.org endpoints:"
echo "- POST: https://httpbin.org/post"
echo "- PUT: https://httpbin.org/put"
echo "- GET: https://httpbin.org/get"