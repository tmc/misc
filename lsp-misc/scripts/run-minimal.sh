#!/bin/bash

# Run Vim with the minimal LSP configuration

# Ensure log directory exists
mkdir -p /tmp/lsp-logs

# Make sure server is executable
chmod +x ./server/minimal-lsp-server.sh

# Run Vim from the project root
cd "$(dirname "$0")/.."
vim -u configs/minimal-vimrc "$@"