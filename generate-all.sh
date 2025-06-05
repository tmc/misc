#!/bin/bash
# generate-all.sh - Run go generate in all directories with go.mod files

set -e

# Find all directories containing go.mod files and run go generate
find . -name "go.mod" -type f | while read -r modfile; do
    dir=$(dirname "$modfile")
    echo "Running go generate in $dir..."
    (cd "$dir" && go generate ./... 2>&1) || echo "Failed in $dir"
done

echo "Done!"