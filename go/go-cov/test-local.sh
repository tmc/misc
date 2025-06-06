#!/bin/bash
set -e

echo "=== Local go-cov testing ==="

# Option 1: Start server locally and test with curl
echo "Starting local server..."
PORT=8080 go run cmd/server/main.go &
SERVER_PID=$!

# Wait for server to start
sleep 2

echo "Testing endpoints..."

# Test module discovery
echo "1. Testing module discovery:"
curl -s "http://localhost:8080/go-cov1.24.3?go-get=1" | grep -q "go-import" && echo "✅ Module discovery works"

# Test version info
echo "2. Testing version info:"
curl -s "http://localhost:8080/go-cov1.24.3/@v/latest.info" | grep -q "Version" && echo "✅ Version info works"

# Test go.mod
echo "3. Testing go.mod:"
curl -s "http://localhost:8080/go-cov1.24.3/@v/latest.mod" | grep -q "module go.tmc.dev/go-cov1.24.3" && echo "✅ Module file works"

# Test zip generation
echo "4. Testing zip generation:"
curl -s "http://localhost:8080/go-cov1.24.3/@v/latest.zip" -o /tmp/test-installer.zip
if [ -f /tmp/test-installer.zip ] && [ -s /tmp/test-installer.zip ]; then
    echo "✅ Zip generation works"
    
    # Extract and check content
    cd /tmp
    rm -rf test-extract
    mkdir test-extract
    cd test-extract
    unzip -q ../test-installer.zip
    
    if grep -q 'version.*=.*"go1.24.3"' main.go && grep -q 'downloadGoSource' main.go && grep -q 'GOEXPERIMENT=coverageredesign' main.go; then
        echo "✅ Version correctly embedded and source-building features present"
    else
        echo "❌ Version or source-building features not found in installer"
    fi
    
    # Test compilation with restricted PATH
    echo "5. Testing compilation with restricted PATH:"
    export PATH="$(go env GOROOT)/bin"
    if go build -o installer main.go; then
        echo "✅ Installer compiles with restricted PATH"
        
        # Test execution (should fail without network but show proper error)
        echo "6. Testing installer execution (should fail gracefully):"
        if ! ./installer 2>&1 | grep -q "Failed to download Go source"; then
            echo "⚠️  Installer didn't show expected error message"
        else
            echo "✅ Installer fails gracefully as expected"
        fi
    else
        echo "❌ Installer failed to compile"
    fi
else
    echo "❌ Zip generation failed"
fi

# Cleanup
kill $SERVER_PID 2>/dev/null || true
rm -f /tmp/test-installer.zip
rm -rf /tmp/test-extract

echo "=== Testing complete ==="