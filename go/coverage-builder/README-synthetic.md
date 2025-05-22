# Synthetic Coverage Feature

This project includes functionality to add synthetic (fake) coverage data to Go coverage reports, allowing you to include coverage for files that don't exist or weren't instrumented during testing.

## Quick Start

```bash
# 1. Generate real coverage
go test -coverprofile=coverage.txt ./...

# 2. Create synthetic coverage file
cat > synthetic.txt << EOF
github.com/fake/module/generated.go:1.1,100.1 50 1
github.com/fake/module/mock.go:1.1,50.1 25 1
EOF

# 3. Merge coverage
go run synthetic-coverage/text-format/main.go \
  -i=coverage.txt \
  -s=synthetic.txt \
  -o=final-coverage.txt
```

## Use Cases

- **Generated Code**: Include auto-generated files in coverage reports
- **Mock Objects**: Add coverage for test doubles and mocks
- **Vendor Libraries**: Account for third-party dependencies
- **Excluded Files**: Include files that can't be instrumented

## Tools Included

### 1. Text Format Tool (`synthetic-coverage/text-format/`)
Simple tool to merge synthetic coverage with existing text format coverage files.

```bash
go run text-format/main.go -i=input.txt -s=synthetic.txt -o=output.txt
```

### 2. Binary Format Tool (`synthetic-coverage/main.go`)
Advanced tool for direct GOCOVERDIR manipulation (experimental).

```bash
go run main.go -i=/tmp/coverdir -o=/tmp/output -pkg=fake/pkg -file=fake.go
```

### 3. Examples (`synthetic-coverage/examples/`)
- `working-demo.go`: Complete working example with real tests
- `add-fake-file.go`: GOCOVERDIR integration example
- `demo.go`: Step-by-step demonstration

## Coverage Format

Synthetic coverage uses the standard Go coverage text format:
```
package/file.go:startLine.startCol,endLine.endCol statements count
```

Example:
```
github.com/example/api/generated.go:1.1,200.1 100 1
```

This means:
- File: `github.com/example/api/generated.go`
- Lines: 1-200
- Statements: 100
- Executed: 1 time (covered)

## Integration with CI/CD

```yaml
# GitHub Actions example
- name: Run tests
  run: go test -coverprofile=coverage.txt ./...

- name: Add synthetic coverage
  run: |
    go run synthetic-coverage/text-format/main.go \
      -i=coverage.txt \
      -s=ci/synthetic-coverage.txt \
      -o=final-coverage.txt

- name: Upload coverage
  uses: codecov/codecov-action@v3
  with:
    file: final-coverage.txt
```

## Limitations

1. HTML reports may fail for non-existent files
2. Some tools validate file existence
3. Binary format manipulation is complex
4. Coverage percentages should be realistic

## Future Enhancements

- [ ] Automatic detection of generated files
- [ ] Configuration file support
- [ ] Integration with `go generate`
- [ ] Coverage profile validation
- [ ] Pattern-based synthetic coverage

See `synthetic-coverage/USAGE.md` for detailed documentation.
