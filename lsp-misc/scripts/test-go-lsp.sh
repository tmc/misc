#!/bin/bash

# Test script for Go LSP server
echo "Testing Go LSP server..."

# Build the server
echo "Building server..."
cd server/go-lsp-server
go build -o go-lsp-server .
cd ../..

# Test basic functionality
echo "Testing basic LSP initialization..."

# Create a test file
cat > /tmp/test.go << 'EOF'
package main

import "fmt"

// TODO: Add more functionality
func main() {
    fmt.Println("Hello, World!")
}
EOF

# Run server with test file
echo "Starting Go LSP server..."
./server/go-lsp-server.sh &
SERVER_PID=$!

# Give server time to start
sleep 2

# Kill server
kill $SERVER_PID

echo "Go LSP server test completed"