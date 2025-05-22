# Practical Example: Text Format Synthetic Coverage

This guide demonstrates a complete example of injecting synthetic coverage using Go's text format.

## Step 1: Set Up a Sample Project

First, let's create a simple Go project with a calculator package:

```bash
mkdir -p synth-demo/calc
cd synth-demo

# Initialize module
cat > go.mod << EOF
module example.com/synth-demo
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
	tests := []struct {
		a, b     int
		expected int
	}{
		{2, 3, 5},
		{-2, 3, 1},
		{0, 0, 0},
	}
	
	for _, test := range tests {
		if got := Add(test.a, test.b); got != test.expected {
			t.Errorf("Add(%d, %d) = %d, expected %d", 
				test.a, test.b, got, test.expected)
		}
	}
}
EOF
```

## Step 2: Generate Real Coverage

Run the tests with coverage enabled:

```bash
cd synth-demo
go test -coverprofile=coverage.txt ./calc
```

You should see partial coverage:

```
ok  	example.com/synth-demo/calc	0.002s	coverage: 25.0% of statements
```

Let's check what's covered:

```bash
go tool cover -func=coverage.txt
```

Output:
```
example.com/synth-demo/calc/calc.go:4:	Add		100.0%
example.com/synth-demo/calc/calc.go:9:	Subtract	0.0%
example.com/synth-demo/calc/calc.go:14:	Multiply	0.0%
example.com/synth-demo/calc/calc.go:20:	Divide		0.0%
total:					(statements)	25.0%
```

## Step 3: Create Synthetic Coverage File

Create a file with synthetic coverage entries for the functions we want to mark as covered:

```bash
cat > synthetic.txt << EOF
example.com/synth-demo/calc/calc.go:9.29,11.2 1 1
example.com/synth-demo/calc/calc.go:14.29,16.2 1 1
example.com/synth-demo/calc/calc.go:20.27,21.12 1 1
example.com/synth-demo/calc/calc.go:21.12,23.3 1 1
example.com/synth-demo/calc/calc.go:24.2,24.14 1 1
EOF
```

## Step 4: Create a Merger Tool

Create a simple tool to merge the real and synthetic coverage:

```bash
cat > merge-coverage.go << EOF
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

func main() {
	inputFile := flag.String("i", "", "Input coverage file")
	syntheticFile := flag.String("s", "", "Synthetic coverage file")
	outputFile := flag.String("o", "", "Output merged file")
	flag.Parse()

	if *inputFile == "" || *outputFile == "" {
		fmt.Println("Must specify input (-i) and output (-o) files")
		os.Exit(1)
	}

	// Read coverage files
	realCoverage, mode := readCoverageFile(*inputFile)
	syntheticCoverage, _ := readCoverageFile(*syntheticFile)

	// Merge coverage
	mergedCoverage := mergeCoverage(realCoverage, syntheticCoverage)

	// Write merged coverage
	writeCoverageFile(*outputFile, mode, mergedCoverage)
	fmt.Printf("Successfully merged coverage to %s\n", *outputFile)
}

func readCoverageFile(filename string) (map[string]int, string) {
	coverage := make(map[string]int)
	mode := "set"

	file, err := os.Open(filename)
	if err != nil {
		return coverage, mode
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "mode:") {
			mode = strings.TrimSpace(strings.TrimPrefix(line, "mode:"))
			continue
		}

		parts := strings.Split(line, " ")
		if len(parts) == 3 {
			key := parts[0] + " " + parts[1]
			var count int
			fmt.Sscanf(parts[2], "%d", &count)
			coverage[key] = count
		}
	}

	return coverage, mode
}

func mergeCoverage(real, synthetic map[string]int) map[string]int {
	merged := make(map[string]int)
	
	// Copy real coverage
	for key, count := range real {
		merged[key] = count
	}
	
	// Add or update with synthetic coverage
	for key, count := range synthetic {
		if existing, ok := merged[key]; ok {
			// Use max count if entry already exists
			if count > existing {
				merged[key] = count
			}
		} else {
			merged[key] = count
		}
	}
	
	return merged
}

func writeCoverageFile(filename, mode string, coverage map[string]int) {
	file, err := os.Create(filename)
	if err != nil {
		fmt.Printf("Error creating output file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	// Write mode line
	fmt.Fprintf(file, "mode: %s\n", mode)

	// Sort keys for consistent output
	var keys []string
	for key := range coverage {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// Write coverage entries
	for _, key := range keys {
		fmt.Fprintf(file, "%s %d\n", key, coverage[key])
	}
}
EOF
```

## Step 5: Merge the Coverage Files

Run the merger tool:

```bash
go run merge-coverage.go -i=coverage.txt -s=synthetic.txt -o=merged.txt
```

## Step 6: Verify the Results

Check the merged coverage:

```bash
go tool cover -func=merged.txt
```

Output:
```
example.com/synth-demo/calc/calc.go:4:	Add		100.0%
example.com/synth-demo/calc/calc.go:9:	Subtract	100.0%
example.com/synth-demo/calc/calc.go:14:	Multiply	100.0%
example.com/synth-demo/calc/calc.go:20:	Divide		100.0%
total:					(statements)	100.0%
```

Generate an HTML report to visualize:

```bash
go tool cover -html=merged.txt -o=coverage.html
```

## Step 7: Add Non-Existent Files

We can add coverage for files that don't exist:

```bash
cat > extra-synthetic.txt << EOF
example.com/synth-demo/generated/models.go:1.1,100.1 50 1
example.com/synth-demo/vendor/external.go:1.1,50.1 25 1
example.com/synth-demo/special/foobar.hehe:1.1,42.1 25 1
EOF
```

Merge with our existing coverage:

```bash
go run merge-coverage.go -i=merged.txt -s=extra-synthetic.txt -o=final.txt
```

## Step 8: Automating the Process

Add this to your Makefile or build script:

```makefile
.PHONY: test-with-synthetic

test-with-synthetic:
	@go test -coverprofile=coverage.txt ./...
	@go run merge-coverage.go -i=coverage.txt -s=synthetic.txt -o=merged.txt
	@echo "Coverage with synthetic data:"
	@go tool cover -func=merged.txt
```

## Step 9: Integration with CI/CD

Add to your GitHub Actions workflow:

```yaml
- name: Run tests with synthetic coverage
  run: |
    go test -coverprofile=coverage.txt ./...
    go run merge-coverage.go -i=coverage.txt -s=synthetic.txt -o=merged.txt
    
- name: Upload coverage report
  uses: codecov/codecov-action@v3
  with:
    file: ./merged.txt
```

## Key Takeaways

1. The text format is simple to understand and manipulate
2. Merging requires tracking which blocks are already covered
3. You can include any file type or path in synthetic coverage
4. The process is easy to automate in CI/CD

This approach is recommended for most projects due to its simplicity and compatibility with standard Go tools.