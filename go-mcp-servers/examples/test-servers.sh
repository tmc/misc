#!/bin/bash

# Test script for MCP servers
# This script tests basic functionality of each server

set -e

echo "Building all MCP servers..."
make build

echo ""
echo "Testing MCP Servers"
echo "==================="

# Test filesystem server
echo ""
echo "Testing Filesystem Server:"
echo "--------------------------"
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}' | ./bin/filesystem-mcp-server | jq .

echo ""
echo "Testing list directory:"
echo '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"list_directory","arguments":{"path":".","recursive":false}}}' | ./bin/filesystem-mcp-server | jq .

# Test git server
echo ""
echo "Testing Git Server:"
echo "-------------------"
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}' | ./bin/git-mcp-server | jq .

echo ""
echo "Testing git status:"
echo '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"git_status","arguments":{}}}' | ./bin/git-mcp-server | jq .

# Test HTTP server
echo ""
echo "Testing HTTP Server:"
echo "--------------------"
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}' | ./bin/http-mcp-server | jq .

echo ""
echo "Testing HTTP GET:"
echo '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"http_get","arguments":{"url":"https://httpbin.org/get","timeout":10}}}' | ./bin/http-mcp-server | jq .

# Test system server
echo ""
echo "Testing System Server:"
echo "----------------------"
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}' | ./bin/system-mcp-server | jq .

echo ""
echo "Testing system info:"
echo '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"system_info","arguments":{"detailed":false}}}' | ./bin/system-mcp-server | jq .

# Test time server
echo ""
echo "Testing Time Server:"
echo "--------------------"
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}' | ./bin/time-mcp-server | jq .

echo ""
echo "Testing current time:"
echo '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"current_time","arguments":{"timezone":"UTC","format":"RFC3339"}}}' | ./bin/time-mcp-server | jq .

echo ""
echo "All tests completed!"