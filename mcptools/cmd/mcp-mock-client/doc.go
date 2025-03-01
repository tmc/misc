/*
Mcp-mock-client replays MCP requests from a recording file.

It reads recorded requests and writes them to stdout, one per line.
Each request is a JSON object from the original recording.

Usage:
    mcp-mock-client [flags] recording

Example:
    # Replay requests to a mock server
    mcp-mock-client requests.mcp | mcp-mock-server responses.mcp

    # Just see the requests
    mcp-mock-client -n requests.mcp
*/
package main

