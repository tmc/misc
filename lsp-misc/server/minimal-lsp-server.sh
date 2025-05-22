#!/bin/bash

# Minimal LSP server with absolute reliability
# This version focuses on minimal features and maximum stability

# Log file for debugging
LOGFILE="/tmp/lsp-logs/minimal-lsp.log"
echo "Minimal LSP server started at $(date)" > "$LOGFILE"

# Extremely simplified completions
COMPLETIONS='[
  {"label": "apple"},
  {"label": "banana"},
  {"label": "cherry"}
]'

# Function to send a response
respond() {
  local body="$1"
  local length=${#body}
  local response="Content-Length: $length\r\n\r\n$body"
  
  echo "Sending: $body" >> "$LOGFILE"
  echo -e "$response"
}

# Process a single message
process_message() {
  local content="$1"
  
  # Extract id and method
  local id method
  id=$(echo -E "$content" | jq -r '.id // "null"')
  method=$(echo -E "$content" | jq -r '.method')
  
  echo "Processing method: $method with id: $id" >> "$LOGFILE"
  
  case "$method" in
    'initialize')
      # Simple initialization with minimal capabilities
      respond '{
        "jsonrpc": "2.0",
        "id": '"$id"',
        "result": {
          "capabilities": {
            "completionProvider": {}
          }
        }
      }'
      ;;
      
    'textDocument/completion')
      # Always return the same small set of completions
      respond '{
        "jsonrpc": "2.0",
        "id": '"$id"',
        "result": {
          "isIncomplete": false,
          "items": '"$COMPLETIONS"'
        }
      }'
      ;;
      
    'shutdown')
      # Respond to shutdown request
      respond '{
        "jsonrpc": "2.0",
        "id": '"$id"',
        "result": null
      }'
      ;;
      
    'exit')
      # Exit cleanly
      echo "Exit requested" >> "$LOGFILE"
      exit 0
      ;;
      
    *)
      # Ignore other methods, only log them
      echo "Ignoring method: $method" >> "$LOGFILE"
      ;;
  esac
}

# Main loop
echo "Starting main loop..." >> "$LOGFILE"
while true; do
  # Read the Content-Length header
  read -r header || {
    echo "Input stream closed" >> "$LOGFILE"
    exit 1
  }
  
  if [[ "$header" =~ ^Content-Length:\ ([0-9]+) ]]; then
    length="${BASH_REMATCH[1]}"
    echo "Received header with length: $length" >> "$LOGFILE"
    
    # Skip the blank line
    read -r blank_line
    
    # Read the message content
    content=$(head -c "$length")
    echo "Received content: $content" >> "$LOGFILE"
    
    # Process the message
    process_message "$content"
  else
    echo "Invalid header: $header" >> "$LOGFILE"
  fi
done