#!/bin/bash

respond() {
  local body="$1"
  local length=${#body}
  local response="Content-Length: $length\r\n\r\n$body"

  echo "$response" >>/tmp/out.log

  echo -e "$response"
}

completions=$(head </usr/share/dict/words -n 1000 | jq --raw-input --slurp 'split("\n")[:-1] | map({ label: . })')

while IFS= read -r line; do
  # Capture the content-length header value
  [[ "$line" =~ ^Content-Length:\ ([0-9]+) ]]
  length="${BASH_REMATCH[1]}"

  # account for \r at end of header
  length=$((length + 2))

  # Read the message based on the Content-Length value
  json_payload=$(head -c "$length")

  # We need -E here because jq fails on newline chars -- https://github.com/jqlang/jq/issues/1049
  id=$(echo -E "$json_payload" | jq -r '.id')
  method=$(echo -E "$json_payload" | jq -r '.method')

  case "$method" in
  'initialize')
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
    respond '{
          "jsonrpc": "2.0",
          "id": '"$id"',
          "result": {
            "isIncomplete": false,
            "items": '"$completions"'
          }
        }'
    ;;

  *) ;;
  esac
done
