#!/bin/bash

# test-text-injection.sh - Complete test showing text-based coverage injection

set -e

echo "=== Text-Based Coverage Injection Test ==="

WORKDIR="/tmp/text-injection-test"
rm -rf "$WORKDIR"
mkdir -p "$WORKDIR"

# Create test app
APPDIR="$WORKDIR/testapp"
mkdir -p "$APPDIR"

# Create go.mod
cat > "$APPDIR/go.mod" <<EOF
module myapp
go 1.20
EOF

# Create source files
cat > "$APPDIR/main.go" <<'EOF'
package main

import "fmt"

func main() {
    fmt.Println(Calculate(10, 5))
}

func Calculate(a, b int) int {
    if a > b {
        return a - b
    }
    return b - a
}

func UnusedCode() {
    fmt.Println("Never called")
}
EOF

cat > "$APPDIR/calc_test.go" <<'EOF'
package main

import "testing"

func TestCalculate(t *testing.T) {
    result := Calculate(10, 5)
    if result != 5 {
        t.Errorf("expected 5, got %d", result)
    }
}
EOF

echo -e "\n1. Running tests with text coverage..."
cd "$APPDIR"
go test -coverprofile="$WORKDIR/coverage.txt"

echo -e "\n2. Original coverage:"
cat "$WORKDIR/coverage.txt"
echo
go tool cover -func="$WORKDIR/coverage.txt"

echo -e "\n3. Creating synthetic coverage data..."
cat > "$WORKDIR/synthetic.txt" <<'EOF'
myapp/generated.go:1.1,100.1 50 1
myapp/mocks/database.go:1.1,200.1 100 1
myapp/mocks/database.go:201.1,300.1 50 0
github.com/vendor/somelib/utils.go:1.1,150.1 75 1
EOF

echo "Synthetic coverage lines:"
cat "$WORKDIR/synthetic.txt"

echo -e "\n4. Building text merger tool..."
cd /Volumes/tmc/go/src/github.com/tmc/misc/go/coverage-builder/synthetic-coverage
go build -o "$WORKDIR/merger" ./text-format/main.go

echo -e "\n5. Merging coverage..."
"$WORKDIR/merger" \
    -i="$WORKDIR/coverage.txt" \
    -s="$WORKDIR/synthetic.txt" \
    -o="$WORKDIR/merged.txt"

echo -e "\n6. Merged coverage:"
cat "$WORKDIR/merged.txt"

echo -e "\n7. Coverage report with synthetic data:"
cd "$APPDIR"
go tool cover -func="$WORKDIR/merged.txt" 2>/dev/null || true

echo -e "\n8. Creating HTML report..."
go tool cover -html="$WORKDIR/merged.txt" -o="$WORKDIR/coverage.html" 2>/dev/null || true

echo -e "\n9. Summary:"
echo "Original coverage files:"
ls -la "$WORKDIR/coverage.txt"
echo
echo "Synthetic coverage files:"
ls -la "$WORKDIR/synthetic.txt"
echo
echo "Merged coverage files:"
ls -la "$WORKDIR/merged.txt"
ls -la "$WORKDIR/coverage.html" 2>/dev/null || true

echo -e "\nNOTE: The HTML report may show errors for non-existent files,"
echo "but the coverage data is successfully merged and can be used by tools"
echo "that don't validate file existence."

echo -e "\nTest complete! All files in: $WORKDIR"