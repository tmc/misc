#!/bin/bash

# Setup script to ensure everything is ready to use

echo "Setting up Bash LSP Server Demo..."

# Create log directory
mkdir -p /tmp/lsp-logs
echo "Created log directory at /tmp/lsp-logs"

# Make all scripts executable
chmod +x server/*.sh scripts/*.sh
echo "Made all scripts executable"

# Check for key dependencies
if ! command -v jq &> /dev/null; then
    echo "⚠️  WARNING: jq is not installed, which is required by the LSP server"
    echo "Please install jq using your package manager:"
    echo "  - macOS: brew install jq"
    echo "  - Ubuntu/Debian: apt install jq"
    echo "  - RHEL/CentOS: yum install jq"
fi

# Check for vim-lsp plugin
if [ ! -d "./vim-plugins/vim-lsp" ]; then
    echo "⚠️  WARNING: Required Vim plugins are missing"
    echo "Please ensure the vim-plugins directory contains:"
    echo "  - vim-lsp"
    echo "  - asyncomplete.vim"
    echo "  - asyncomplete-lsp.vim"
fi

echo
echo "Setup complete. Try running:"
echo "  ./scripts/run-minimal.sh examples/test.txt"
echo
echo "Or test the server directly with:"
echo "  cd scripts && ./test-minimal.sh"