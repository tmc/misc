#!/bin/bash
# Script to check test failures across all modules

cd /Volumes/tmc/go/src/github.com/tmc/misc/testctr

echo "=== Finding and testing all Go modules ==="
echo

# Find all go.mod files
find . -name "go.mod" -type f | while read -r modfile; do
    moddir=$(dirname "$modfile")
    relpath=${moddir#./}
    
    echo "Testing $relpath..."
    
    # Run tests and capture output
    if output=$(cd "$moddir" && go test -cover ./... 2>&1); then
        echo "✓ PASS: $relpath"
    else
        echo "✗ FAIL: $relpath"
        echo "$output" | tail -10
        echo "---"
    fi
    echo
done