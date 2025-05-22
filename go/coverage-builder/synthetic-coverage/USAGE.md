# Synthetic Coverage Usage Guide

This guide explains how to add synthetic (fake) coverage data to Go coverage reports for files that don't exist or weren't instrumented.

## Why Synthetic Coverage?

Sometimes you need to include coverage data for:
- Generated code that shouldn't be tested directly
- Third-party libraries included in coverage reports
- Mock implementations
- Code that can't be instrumented

## Two Approaches

### 1. Text Format Manipulation (Recommended)

The simplest approach is to work with the text format:

```bash
# Convert binary coverage to text
go tool covdata textfmt -i=$GOCOVERDIR -o=coverage.txt

# Add synthetic coverage
go run text-format/main.go \
  -i=coverage.txt \
  -o=coverage-with-synthetic.txt \
  -s=synthetic.txt

# View the result
cat coverage-with-synthetic.txt
```

Synthetic file format:
```
github.com/example/fake/file.go:1.1,10.1 5 1
github.com/example/fake/file.go:12.1,20.1 3 0
```

Format: `file:startLine.startCol,endLine.endCol statements count`

### 2. Binary Format Manipulation (Advanced)

Direct GOCOVERDIR manipulation for tools that require binary format:

```bash
# Generate some real coverage first
GOCOVERDIR=/tmp/real-coverage go test ./...

# Inject synthetic coverage
go run main.go \
  -i=/tmp/real-coverage \
  -o=/tmp/with-synthetic \
  -pkg=github.com/example/fake \
  -file=synthetic.go \
  -func=FakeFunction \
  -line-start=1 \
  -line-end=20 \
  -statements=10 \
  -executed=1
```

## Examples

### Example 1: Add Coverage for a Mock

```go
// Create synthetic coverage for a mock file
synthetic := []string{
    "github.com/myapp/mocks/database.go:1.1,50.1 20 1",
    "github.com/myapp/mocks/database.go:52.1,100.1 15 1",
}
```

### Example 2: Generated Code

```go
// Mark generated code as fully covered
synthetic := fmt.Sprintf("%s:1.1,%d.1 %d %d",
    "github.com/myapp/generated/api.go",
    fileLines,      // End line
    fileStatements, // Number of statements
    1,             // Execution count
)
```

### Example 3: Third-Party Library

```go
// Include vendored library in coverage
synthetic := []string{
    "github.com/vendor/lib/core.go:1.1,100.1 50 1",
    "github.com/vendor/lib/util.go:1.1,200.1 75 1",
}
```

## Full Example Program

```go
package main

import (
    "log"
    "os"
    "os/exec"
    "path/filepath"
)

func main() {
    // 1. Generate real coverage
    coverDir := "/tmp/coverage"
    os.Setenv("GOCOVERDIR", coverDir)
    
    cmd := exec.Command("go", "test", "./...")
    if err := cmd.Run(); err != nil {
        log.Fatal(err)
    }
    
    // 2. Convert to text
    textFile := "/tmp/coverage.txt"
    cmd = exec.Command("go", "tool", "covdata", "textfmt",
        "-i="+coverDir, "-o="+textFile)
    if err := cmd.Run(); err != nil {
        log.Fatal(err)
    }
    
    // 3. Add synthetic coverage
    synthetic := `github.com/example/fake/mock.go:1.1,100.1 50 1
github.com/example/fake/generated.go:1.1,200.1 100 1`
    
    syntheticFile := "/tmp/synthetic.txt"
    os.WriteFile(syntheticFile, []byte(synthetic), 0644)
    
    // 4. Merge
    mergedFile := "/tmp/merged.txt"
    cmd = exec.Command("go", "run", "text-format/main.go",
        "-i", textFile,
        "-s", syntheticFile,
        "-o", mergedFile)
    if err := cmd.Run(); err != nil {
        log.Fatal(err)
    }
    
    // 5. Use merged coverage
    log.Println("Coverage with synthetic data:", mergedFile)
}
```

## Limitations

1. **Binary Format**: Direct manipulation is complex due to hash dependencies
2. **Tool Compatibility**: Some tools may validate file existence
3. **Accuracy**: Synthetic data should approximate real coverage patterns

## Best Practices

1. Use text format when possible - it's simpler and more portable
2. Keep synthetic coverage realistic (avoid 100% coverage everywhere)
3. Document why synthetic coverage is needed
4. Use consistent naming for fake packages/files
5. Validate output with `go tool cover -html`

## Integration with CI/CD

```yaml
# Example GitHub Actions workflow
- name: Run tests with coverage
  env:
    GOCOVERDIR: /tmp/coverage
  run: go test -cover ./...

- name: Add synthetic coverage
  run: |
    go tool covdata textfmt -i=/tmp/coverage -o=coverage.txt
    go run synthetic-coverage/text-format/main.go \
      -i=coverage.txt \
      -s=synthetic-files.txt \
      -o=final-coverage.txt

- name: Upload coverage
  uses: codecov/codecov-action@v3
  with:
    file: final-coverage.txt
```

## Troubleshooting

### "Invalid coverage file format"
- Ensure correct format: `file:start.col,end.col statements count`
- Check for trailing spaces or invalid characters

### "Package not found"
- Binary format requires exact package path matching
- Use full import paths, not relative paths

### "Hash mismatch"
- Binary format issue - use text format instead
- Or ensure all files are processed together

## Advanced Usage

### Custom Coverage Profiles

Create different synthetic profiles for different scenarios:

```go
// Development profile - mark all generated code as covered
devProfile := []string{
    "*/generated/*.go:1.1,9999.1 1000 1",
    "*/mocks/*.go:1.1,9999.1 500 1",
}

// Production profile - more realistic coverage
prodProfile := []string{
    "*/generated/*.go:1.1,9999.1 1000 0",  // Not executed
    "*/mocks/*.go:1.1,9999.1 500 0",      // Not executed
}
```

### Conditional Synthetic Coverage

Add synthetic coverage based on build tags or environment:

```go
if os.Getenv("INCLUDE_GENERATED") == "true" {
    addSyntheticCoverage(generatedFiles)
}
```