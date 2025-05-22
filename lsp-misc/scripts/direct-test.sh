#!/bin/bash
# Test the bash-lsp-server directly without Vim

# Create named pipes for communication
PIPE_DIR=$(mktemp -d)
IN_PIPE="$PIPE_DIR/in"
OUT_PIPE="$PIPE_DIR/out"
mkfifo "$IN_PIPE"
mkfifo "$OUT_PIPE"

# Start the server with redirected I/O
../server/bash-lsp-server.sh < "$IN_PIPE" > "$OUT_PIPE" 2>/tmp/lsp-server.log &
SERVER_PID=$!

# Function to send a request
send_request() {
  local request="$1"
  local length=${#request}
  echo -e "Content-Length: $length\r\n\r\n$request" > "$IN_PIPE"
}

# Send initialize request
echo "Sending initialize request..."
INIT_REQUEST='{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "initialize",
  "params": {
    "processId": '"$$"',
    "rootUri": "file:///'"$(pwd)"'",
    "capabilities": {}
  }
}'
send_request "$INIT_REQUEST"

# Read and display response (non-blocking)
cat "$OUT_PIPE" > /tmp/lsp-response.txt &
CAT_PID=$!

# Wait a bit to allow response
sleep 1

# Send completion request
echo "Sending completion request..."
COMPLETION_REQUEST='{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "textDocument/completion",
  "params": {
    "textDocument": {
      "uri": "file:///'"$(pwd)"'/../examples/test.txt"
    },
    "position": {
      "line": 0,
      "character": 3
    }
  }
}'
send_request "$COMPLETION_REQUEST"

# Wait for responses
sleep 2

# Display response
echo "Server response (saved to /tmp/lsp-response.txt):"
cat /tmp/lsp-response.txt

# Cleanup
kill $SERVER_PID $CAT_PID 2>/dev/null
rm -rf "$PIPE_DIR"

echo
echo "The LSP server is working correctly if you see initialization and completion responses above."
echo "To use it in Vim, you need to install the vim-lsp plugin."