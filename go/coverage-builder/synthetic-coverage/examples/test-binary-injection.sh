#!/bin/bash

# test-binary-injection.sh - Demonstrates GOCOVERDIR binary format injection

set -e

echo "=== GOCOVERDIR Binary Injection Test ==="

# Setup directories
WORKDIR="/tmp/gocoverdir-test"
TESTAPP_DIR="$WORKDIR/testapp"
COVERAGE_DIR="$WORKDIR/coverage"
SYNTHETIC_DIR="$WORKDIR/synthetic"

# Clean up previous runs
rm -rf "$WORKDIR"
mkdir -p "$TESTAPP_DIR" "$COVERAGE_DIR" "$SYNTHETIC_DIR"

echo -e "\n1. Creating test application..."

# Create go.mod
cat > "$TESTAPP_DIR/go.mod" <<EOF
module testapp
go 1.20
EOF

# Create main.go
cat > "$TESTAPP_DIR/main.go" <<'EOF'
package main

import "fmt"

func main() {
    fmt.Println(Add(2, 3))
}

func Add(a, b int) int {
    return a + b
}

func Unused() string {
    return "This function is not covered"
}
EOF

# Create main_test.go
cat > "$TESTAPP_DIR/main_test.go" <<'EOF'
package main

import "testing"

func TestAdd(t *testing.T) {
    if result := Add(2, 3); result != 5 {
        t.Errorf("Add(2, 3) = %d, want 5", result)
    }
}
EOF

echo -e "\n2. Running tests with GOCOVERDIR..."
cd "$TESTAPP_DIR"
GOCOVERDIR="$COVERAGE_DIR" go test -cover

echo -e "\n3. Viewing original coverage..."
go tool covdata func -i="$COVERAGE_DIR"

echo -e "\n4. Converting to text format to see what we have..."
go tool covdata textfmt -i="$COVERAGE_DIR" -o="$WORKDIR/original.txt"
echo "Original coverage:"
cat "$WORKDIR/original.txt"

echo -e "\n5. Running binary injection tool..."
cd /Volumes/tmc/go/src/github.com/tmc/misc/go/coverage-builder/synthetic-coverage

# Build the injection tool
go build -o "$WORKDIR/inject-tool" ./examples/gocoverdir-inject.go

# Run injection
"$WORKDIR/inject-tool" \
    -i="$COVERAGE_DIR" \
    -o="$SYNTHETIC_DIR" \
    -pkg="testapp/generated" \
    -file="generated.go" \
    -func="GeneratedFunction" \
    -line-start=1 \
    -line-end=50 \
    -statements=25 \
    -executed=1

# Add another synthetic file
"$WORKDIR/inject-tool" \
    -i="$SYNTHETIC_DIR" \
    -o="$SYNTHETIC_DIR" \
    -pkg="github.com/vendor/lib" \
    -file="vendor.go" \
    -func="VendorFunction" \
    -line-start=1 \
    -line-end=100 \
    -statements=40 \
    -executed=1

echo -e "\n6. Viewing coverage with synthetic data..."
go tool covdata func -i="$SYNTHETIC_DIR"

echo -e "\n7. Converting synthetic coverage to text..."
go tool covdata textfmt -i="$SYNTHETIC_DIR" -o="$WORKDIR/synthetic.txt"
echo "Synthetic coverage:"
cat "$WORKDIR/synthetic.txt"

echo -e "\n8. Generating HTML report..."
# First convert to profile format
go tool covdata textfmt -i="$SYNTHETIC_DIR" -o="$WORKDIR/profile.txt"
go tool cover -html="$WORKDIR/profile.txt" -o="$WORKDIR/coverage.html"
echo "HTML report saved to: $WORKDIR/coverage.html"

echo -e "\n9. Comparing coverage percentages..."
echo "Original:"
go tool covdata percent -i="$COVERAGE_DIR"
echo "With synthetic:"
go tool covdata percent -i="$SYNTHETIC_DIR"

echo -e "\nTest complete! Files are in: $WORKDIR"