#!/usr/bin/env bash
set -euo pipefail

# Installation script for pcie-topology-analyzer
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
COMPLETION_DIR="${COMPLETION_DIR:-/etc/bash_completion.d}"
ZSH_COMPLETION_DIR="${ZSH_COMPLETION_DIR:-/usr/local/share/zsh/site-functions}"

# Create directories if they don't exist
mkdir -p "$INSTALL_DIR"
mkdir -p "$COMPLETION_DIR"
mkdir -p "$ZSH_COMPLETION_DIR"

# Install main script
install -m 755 pcie-topology-analyzer.sh "$INSTALL_DIR/pcie-topology-analyzer"

# Install completions
install -m 644 completion.bash "$COMPLETION_DIR/pcie-topology-analyzer"
install -m 644 completion.zsh "$ZSH_COMPLETION_DIR/_pcie-topology-analyzer"

echo "Installation complete!"
echo "Main script installed to: $INSTALL_DIR"
echo "Bash completion installed to: $COMPLETION_DIR"
echo "Zsh completion installed to: $ZSH_COMPLETION_DIR"

