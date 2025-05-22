# Practical Example: Binary Format Synthetic Coverage

This guide demonstrates how to inject synthetic coverage into Go's binary coverage format (GOCOVERDIR).

## Introduction to Binary Coverage

Go 1.20+ uses a binary format for coverage when the `GOCOVERDIR` environment variable is set. This format consists of:

- `covmeta.*` files containing metadata about packages, functions, and code blocks
- `covcounters.*` files containing execution counts

Working with this format is more complex than the text format, but sometimes necessary for newer Go projects.

## Approach 1: Convert-Modify-Convert (Recommended)

The simplest approach is to:
1. Generate binary coverage
2. Convert to text format
3. Add synthetic coverage in text format
4. Convert back to binary format (if needed)

Let's implement this approach:

## Step 1: Set Up a Sample Project

```bash
mkdir -p binary-demo/calc
cd binary-demo

# Initialize module
cat > go.mod << EOF
module example.com/binary-demo
go 1.21
EOF

# Create calculator implementation
cat > calc/calc.go << EOF
package calc

// Add returns the sum of two integers
func Add(a, b int) int {
	return a + b
}

// Subtract returns the difference between two integers
func Subtract(a, b int) int {
	return a - b
}

// Multiply returns the product of two integers
func Multiply(a, b int) int {
	return a * b
}

// Divide returns the quotient of a divided by b
// Returns 0 if b is 0
func Divide(a, b int) int {
	if b == 0 {
		return 0
	}
	return a / b
}
EOF

# Create a test file that only tests Add
cat > calc/calc_test.go << EOF
package calc

import "testing"

func TestAdd(t *testing.T) {
	if got := Add(2, 3); got != 5 {
		t.Errorf("Add(2, 3) = %d, want 5", got)
	}
}
EOF
```

## Step 2: Generate Binary Coverage

Run tests with `GOCOVERDIR` to generate binary coverage:

```bash
mkdir -p coverage-bin
GOCOVERDIR=$(pwd)/coverage-bin go test ./calc
```

Verify what was created:

```bash
ls -la coverage-bin
```

You should see binary coverage files:
```
covmeta.XXXXXXXX
covcounters.XXXXXXXX
```

## Step 3: Convert Binary to Text Format

Use Go's built-in tool to convert binary to text format:

```bash
go tool covdata textfmt -i=$(pwd)/coverage-bin -o=coverage.txt
```

Examine the text format:

```bash
cat coverage.txt
```

Check the coverage:

```bash
go tool cover -func=coverage.txt
```

Output should show partial coverage:
```
example.com/binary-demo/calc/calc.go:4:  Add         100.0%
example.com/binary-demo/calc/calc.go:9:  Subtract    0.0%
example.com/binary-demo/calc/calc.go:14: Multiply    0.0%
example.com/binary-demo/calc/calc.go:20: Divide      0.0%
total:                                  (statements) 25.0%
```

## Step 4: Create Synthetic Coverage File

Create a synthetic coverage file for the untested functions:

```bash
cat > synthetic.txt << EOF
example.com/binary-demo/calc/calc.go:9.29,11.2 1 1
example.com/binary-demo/calc/calc.go:14.29,16.2 1 1
example.com/binary-demo/calc/calc.go:20.27,21.12 1 1
example.com/binary-demo/calc/calc.go:21.12,23.3 1 1
example.com/binary-demo/calc/calc.go:24.2,24.14 1 1
EOF
```

## Step 5: Merge the Coverage Files

Use the merger tool from the text format example:

```bash
go run merge-coverage.go -i=coverage.txt -s=synthetic.txt -o=merged.txt
```

Check the merged coverage:

```bash
go tool cover -func=merged.txt
```

Output should show 100% coverage:
```
example.com/binary-demo/calc/calc.go:4:  Add         100.0%
example.com/binary-demo/calc/calc.go:9:  Subtract    100.0%
example.com/binary-demo/calc/calc.go:14: Multiply    100.0%
example.com/binary-demo/calc/calc.go:20: Divide      100.0%
total:                                  (statements) 100.0%
```

## Step 6: Convert Back to Binary Format (Optional)

If you need to use tools that require binary format, convert back:

```bash
mkdir -p coverage-merged
go tool covdata convert -i=merged.txt -o=$(pwd)/coverage-merged -covermode=set
```

Verify the binary data:

```bash
go tool covdata func -i=$(pwd)/coverage-merged
```

## Step 7: Add Coverage for Non-Existent Files

You can also add coverage for files that don't exist:

```bash
cat > extra.txt << EOF
example.com/binary-demo/generated/models.go:1.1,100.1 50 1
example.com/binary-demo/vendor/external.go:1.1,50.1 25 1
example.com/binary-demo/custom/foobar.hehe:1.1,42.1 25 1
EOF
```

Merge with our existing coverage:

```bash
go run merge-coverage.go -i=merged.txt -s=extra.txt -o=final.txt
```

Convert to binary if needed:

```bash
mkdir -p coverage-final
go tool covdata convert -i=final.txt -o=$(pwd)/coverage-final -covermode=set
```

## Step 8: Automate the Process

Create a shell script to automate the steps:

```bash
cat > synthetic-coverage.sh << 'EOF'
#!/bin/bash
set -e

# Directory setup
COVERAGE_DIR="$(pwd)/coverage-bin"
mkdir -p "$COVERAGE_DIR"

# Run tests with binary coverage
echo "Generating binary coverage..."
GOCOVERDIR="$COVERAGE_DIR" go test ./...

# Convert to text format
echo "Converting to text format..."
go tool covdata textfmt -i="$COVERAGE_DIR" -o=coverage.txt

# Merge with synthetic coverage
echo "Merging with synthetic coverage..."
go run merge-coverage.go -i=coverage.txt -s=synthetic.txt -o=merged.txt

# (Optional) Convert back to binary
if [ "$1" == "--binary" ]; then
  echo "Converting back to binary format..."
  mkdir -p coverage-merged
  go tool covdata convert -i=merged.txt -o="$(pwd)/coverage-merged" -covermode=set
  echo "Final binary coverage in: $(pwd)/coverage-merged"
fi

# Show coverage report
echo "Coverage report:"
go tool cover -func=merged.txt

# Generate HTML report
echo "Generating HTML report..."
go tool cover -html=merged.txt -o=coverage.html
echo "HTML report: $(pwd)/coverage.html"
EOF

chmod +x synthetic-coverage.sh
```

## Step 9: Integration with CI/CD

Add to your GitHub Actions workflow:

```yaml
- name: Run tests with binary coverage
  run: |
    mkdir -p $GOCOVERDIR
    go test ./...
  env:
    GOCOVERDIR: ./coverage-bin

- name: Process coverage
  run: |
    go tool covdata textfmt -i=./coverage-bin -o=coverage.txt
    go run merge-coverage.go -i=coverage.txt -s=synthetic.txt -o=merged.txt
    
- name: Upload coverage report
  uses: codecov/codecov-action@v3
  with:
    file: ./merged.txt
```

## Approach 2: Direct Binary Manipulation (Advanced)

For more control, you can directly manipulate the binary format, but this is complex and version-dependent:

1. Create a simplified version of the tool (this is a sketch - full implementation would be hundreds of lines):

```go
// synthetic-binary.go - WARNING: Uses internal packages that may change
package main

import (
	"flag"
	"fmt"
	"internal/coverage"
	"internal/coverage/decodecounter"
	"internal/coverage/decodemeta"
	"internal/coverage/encodecounter"
	"internal/coverage/encodemeta"
	"internal/coverage/pods"
	"log"
	"os"
	"path/filepath"
)

var (
	inputDir    = flag.String("i", "", "Input coverage directory")
	outputDir   = flag.String("o", "", "Output coverage directory")
	packagePath = flag.String("pkg", "", "Package path to inject")
	funcName    = flag.String("func", "", "Function name to inject")
	fileName    = flag.String("file", "", "File name to inject")
	lineStart   = flag.Int("line-start", 1, "Start line number")
	lineEnd     = flag.Int("line-end", 10, "End line number")
	statements  = flag.Int("statements", 1, "Number of statements")
	executed    = flag.Int("executed", 1, "Execution count (1=covered, 0=not covered)")
)

func main() {
	flag.Parse()
	
	// Validate inputs
	if *inputDir == "" || *outputDir == "" {
		log.Fatal("Must specify both -i (input) and -o (output) directories")
	}
	
	// Create output directory
	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		log.Fatal(err)
	}
	
	// Collect pods (coverage data sets)
	pods, err := pods.CollectPods([]string{*inputDir}, true) 
	if err != nil {
		log.Fatalf("Failed to collect pods: %v", err)
	}
	
	if len(pods) == 0 {
		log.Fatal("No coverage data found")
	}
	
	// Process each pod
	for _, pod := range pods {
		if err := processPod(pod); err != nil {
			log.Fatalf("Failed to process pod: %v", err)
		}
	}
	
	log.Printf("Successfully created synthetic coverage in %s", *outputDir)
}

// processPod handles a single coverage pod
func processPod(pod pods.Pod) error {
	// Implementation details omitted for brevity
	// This would:
	// 1. Read the meta file
	// 2. Add synthetic package/function entries
	// 3. Write new meta file
	// 4. Process counter files
	// 5. Add synthetic counters
	return nil
}
```

2. Use the tool:

```bash
go run synthetic-binary.go \
  -i=$(pwd)/coverage-bin \
  -o=$(pwd)/coverage-synth \
  -pkg=example.com/binary-demo/calc \
  -file=calc.go \
  -func=Subtract \
  -line-start=9 \
  -line-end=11 \
  -statements=1 \
  -executed=1
```

This approach is not recommended for most users because:
1. It relies on Go's internal packages that can change without notice
2. It's complex and error-prone
3. The convert-modify-convert approach is simpler and more stable

## Key Takeaways

1. The convert-modify-convert approach is recommended for most users
2. Binary format is more complex but offers no real advantages for synthetic coverage
3. Go's tools make it easy to convert between formats
4. Use direct binary manipulation only if you have specific requirements

When working with binary coverage, sticking to the text format for synthetic coverage injection is usually the best approach.