#!/bin/bash
cd /Volumes/tmc/go/src/github.com/tmc/misc/testctr

echo "=== Debugging empty coverage directories ==="
echo

# List of modules that have empty coverage dirs
EMPTY_MODULES=(
    "backends/dockerclient"
    "backends/testcontainers-go" 
    "backends/testing/test-all-testctr-backends"
    "exp/cmd/parse-tc-module"
)

for module in "${EMPTY_MODULES[@]}"; do
    echo "=== Module: $module ==="
    
    if [ ! -d "$module" ]; then
        echo "❌ Directory does not exist"
        continue
    fi
    
    cd "$module"
    
    echo "📁 Files in directory:"
    ls -la
    echo
    
    echo "🧪 Test files:"
    find . -name "*_test.go" -type f | head -5
    echo
    
    echo "📦 Go mod status:"
    if [ -f "go.mod" ]; then
        echo "✓ go.mod exists"
        head -3 go.mod
    else
        echo "❌ No go.mod file"
    fi
    echo
    
    echo "🔍 Testing with detailed output:"
    echo "Command: go test -cover ./... -args -test.gocoverdir=./debug_coverage"
    mkdir -p debug_coverage
    
    if output=$(go test -cover ./... -args -test.gocoverdir=./debug_coverage 2>&1); then
        echo "✓ Tests passed"
        echo "Output: $output"
    else
        echo "❌ Tests failed"
        echo "Error output:"
        echo "$output"
    fi
    
    echo "📊 Coverage files generated:"
    ls -la debug_coverage/ 2>/dev/null || echo "No coverage files"
    
    # Clean up
    rm -rf debug_coverage
    
    cd - > /dev/null
    echo "----------------------------------------"
    echo
done