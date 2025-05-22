#!/bin/bash

# Go Language Server implementation using GLSP
# Demonstrates building custom LSP servers with Go

# Configuration
SERVER_NAME="go-lsp-server"
LOG_LEVEL=${LOG_LEVEL:-1}

# Build the server if needed
if [ ! -f "server/go-lsp-server/go-lsp-server" ] || [ "$1" = "--build" ]; then
    echo "Building Go LSP server..." >&2
    cd server/go-lsp-server
    go build -o go-lsp-server .
    cd ../..
fi

# Run the server
exec server/go-lsp-server/go-lsp-server