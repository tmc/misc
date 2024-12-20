#!/usr/bin/env bash
set -euo pipefail

# Uninstallation script for pcie-topology-analyzer
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
COMPLETION_DIR="${COMPLETION_DIR:-/etc/bash_completion.d}"
ZSH_COMPLETION_DIR="${ZSH_COMPLETION_DIR:-/usr/local/share/zsh/site-functions}"

# Remove installed files
rm -f "$INSTALL_DIR/pcie-topology-analyzer"
rm -f "$COMPLETION_DIR/pcie-topology-analyzer"
rm -f "$ZSH_COMPLETION_DIR/_pcie-topology-analyzer"

echo "Uninstallation complete!"

