#!/bin/bash

# Setup script for Go LSP server

echo "Setting up Go LSP server..."

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "Error: Go is not installed"
    exit 1
fi

# Navigate to server directory
cd server/go-lsp-server

# Install dependencies
echo "Installing dependencies..."
go mod download

# Build the server
echo "Building server..."
go build -o go-lsp-server .

# Make launch script executable
cd ../..
chmod +x server/go-lsp-server.sh

echo "Go LSP server setup complete!"
echo ""
echo "To use with vim, add the following to your .vimrc:"
echo "  source /path/to/lsp-misc/configs/go-lsp-vim-config"
echo ""
echo "Then update the path in the config file to point to your installation."