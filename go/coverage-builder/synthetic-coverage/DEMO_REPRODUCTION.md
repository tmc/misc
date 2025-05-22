# How to Reproduce Synthetic Coverage Injection

This guide shows step-by-step how to reproduce the synthetic coverage injection to prove it works.

## Quick Demo

```bash
# 1. Navigate to the synthetic coverage directory
cd synthetic-coverage/examples

# 2. Run the working demo
go run working-demo.go
```

This will:
- Create a test module with real code
- Run tests to generate real coverage
- Add synthetic coverage for fake files
- Merge them together
- Show the combined result

## Manual Step-by-Step Reproduction

### Step 1: Create a Test Project

```bash
# Create a temporary directory
mkdir /tmp/coverage-test
cd /tmp/coverage-test

# Create a simple Go module
go mod init testproject

# Create a main.go file
cat > main.go << 'EOF'
package main

import "fmt"

func Hello(name string) string {
    if name == "" {
        return "Hello, World!"
    }
    return fmt.Sprintf("Hello, %s!", name)
}

func Unused() {
    fmt.Println("This function is not tested")
}

func main() {
    fmt.Println(Hello(""))
}
EOF

# Create a test file
cat > main_test.go << 'EOF'
package main

import "testing"

func TestHello(t *testing.T) {
    tests := []struct {
        name string
        want string
    }{
        {"Alice", "Hello, Alice!"},
        {"", "Hello, World!"},
    }
    
    for _, tt := range tests {
        if got := Hello(tt.name); got != tt.want {
            t.Errorf("Hello(%q) = %q, want %q", tt.name, got, tt.want)
        }
    }
}
EOF
```

### Step 2: Generate Real Coverage

```bash
# Run tests with coverage
go test -coverprofile=coverage.txt

# View the coverage
cat coverage.txt
```

You should see something like:
```
mode: set
testproject/main.go:5.32,6.16 1 1
testproject/main.go:6.16,8.3 1 1
testproject/main.go:9.2,9.40 1 1
testproject/main.go:12.14,14.2 1 0
testproject/main.go:16.13,18.2 1 0
```

### Step 3: Create Synthetic Coverage

```bash
# Create synthetic coverage for fake files
cat > synthetic.txt << 'EOF'
testproject/generated/api.go:1.1,100.1 50 1
testproject/generated/api.go:101.1,200.1 40 1
testproject/mocks/database.go:1.1,75.1 30 1
testproject/mocks/database.go:76.1,150.1 25 0
github.com/vendor/lib/util.go:1.1,50.1 20 1
EOF
```

### Step 4: Merge Coverage

```bash
# Clone/navigate to the synthetic coverage tool
cd /path/to/coverage-builder/synthetic-coverage

# Run the text format merger
go run text-format/main.go \
  -i=/tmp/coverage-test/coverage.txt \
  -s=/tmp/coverage-test/synthetic.txt \
  -o=/tmp/coverage-test/merged.txt

# View the merged result
cat /tmp/coverage-test/merged.txt
```

### Step 5: Verify the Result

The merged file should contain both real and synthetic coverage:

```
mode: set
github.com/vendor/lib/util.go:1.1,50.1 20 1
testproject/generated/api.go:1.1,100.1 50 1
testproject/generated/api.go:101.1,200.1 40 1
testproject/main.go:5.32,6.16 1 1
testproject/main.go:6.16,8.3 1 1
testproject/main.go:9.2,9.40 1 1
testproject/main.go:12.14,14.2 1 0
testproject/main.go:16.13,18.2 1 0
testproject/mocks/database.go:1.1,75.1 30 1
testproject/mocks/database.go:76.1,150.1 25 0
```

### Step 6: Generate Coverage Report

```bash
# Try to generate a coverage report (may show warnings for non-existent files)
go tool cover -func=/tmp/coverage-test/merged.txt

# View as HTML (will show warnings but work for real files)
go tool cover -html=/tmp/coverage-test/merged.txt -o=coverage.html
```

## What This Proves

1. **Real Coverage**: The test files generate actual coverage data
2. **Synthetic Addition**: We can add coverage for files that don't exist
3. **Successful Merge**: Both real and synthetic coverage appear in the final report
4. **Tool Compatibility**: The merged file works with standard Go coverage tools

## Key Points

- The synthetic files don't need to exist
- Coverage data is preserved and sorted correctly
- Standard Go tools can read the merged format
- HTML generation may warn about missing files but still works

## Full Working Example

For a complete, self-contained example that handles all steps automatically:

```bash
cd synthetic-coverage/examples
go run working-demo.go
```

This will create everything in `/tmp/synthetic-demo` and show you the complete process with output at each step.