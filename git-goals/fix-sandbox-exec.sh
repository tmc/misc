#!/bin/bash
set -euo pipefail

# Check if sandbox-exec is in the PATH
if ! command -v sandbox-exec &> /dev/null; then
    echo "sandbox-exec not found in PATH. Attempting to fix..."
    
    # Try to find sandbox-exec
    sandbox_exec_path=$(find /usr/local/bin /usr/bin /bin -name sandbox-exec 2>/dev/null | head -n 1)
    
    if [ -n "$sandbox_exec_path" ]; then
        echo "Found sandbox-exec at $sandbox_exec_path"
        echo "Adding its directory to PATH..."
        export PATH="$(dirname "$sandbox_exec_path"):$PATH"
        echo "PATH updated. Please run source
