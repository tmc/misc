#!/bin/bash

# Absolutely minimal test script for the LSP server
# This removes all complexity to focus on reliable testing

# Create log directory
mkdir -p /tmp/lsp-logs
echo "=== Minimal LSP Test Started $(date) ===" > /tmp/lsp-logs/minimal-test.log

# Make sure server is executable
chmod +x ../server/minimal-lsp-server.sh

# Start server in a way we can send data to it
../server/minimal-lsp-server.sh > /tmp/lsp-logs/server-output.log 2>&1 << 'EOL'
Content-Length: 207

{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"processId":123,"rootUri":"file:///test","capabilities":{}}}
Content-Length: 147

{"jsonrpc":"2.0","id":2,"method":"textDocument/completion","params":{"textDocument":{"uri":"file:///test.txt"},"position":{"line":0,"character":0}}}
EOL

# Check if we got output
echo "Server output:"
cat /tmp/lsp-logs/server-output.log

# Analyze the output
if grep -q "items" /tmp/lsp-logs/server-output.log; then
  echo "✅ SUCCESS: Server responded with completion items"
else
  echo "❌ FAILURE: Server did not provide completion items"
fi

echo "Test completed. See /tmp/lsp-logs/ for details."