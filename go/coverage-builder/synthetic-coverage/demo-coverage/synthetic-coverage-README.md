# Synthetic Coverage in Go: Comprehensive Guide

This repository contains a comprehensive set of guides and examples for implementing synthetic coverage in Go projects.

## Introduction

Synthetic coverage is a technique that allows you to add coverage information for code that isn't executed during tests, such as:

- Generated code that shouldn't be tested directly
- Third-party libraries included in coverage reports
- Mock implementations used in testing
- Files that don't exist in the repository
- Code that can't be instrumented

This technique helps you maintain high coverage metrics while focusing your testing efforts on code that matters.

## Guides & Examples

This repository includes:

1. **[Text Format Example](example-text-format.md)** - Simple approach using Go's text coverage format
2. **[Binary Format Example](example-binary-format.md)** - Working with the newer binary (GOCOVERDIR) format
3. **[Advanced Techniques](example-advanced-techniques.md)** - Automation, configuration, and best practices
4. **[Custom File Types](example-custom-file-types.md)** - Including non-standard file types and extensions

## Getting Started

To get started with synthetic coverage, follow these steps:

1. Generate real coverage by running your tests:
   ```bash
   go test -coverprofile=coverage.txt ./...
   ```

2. Create a synthetic coverage file:
   ```bash
   cat > synthetic.txt << EOF
   example.com/myproject/generated/models.go:1.1,100.1 50 1
   example.com/myproject/vendor/library.go:1.1,200.1 100 1
   EOF
   ```

3. Merge the real and synthetic coverage:
   ```bash
   # Using the provided merge tool
   go run merge-coverage.go -i=coverage.txt -s=synthetic.txt -o=merged.txt
   ```

4. Use the merged coverage file for reports:
   ```bash
   go tool cover -func=merged.txt
   go tool cover -html=merged.txt -o=coverage.html
   ```

## Use Cases

Synthetic coverage is particularly useful in these scenarios:

- **Generated Code**: Protobuf, gRPC, ORM models
- **Third-Party Dependencies**: Vendored libraries, frameworks
- **Mock Implementations**: Test doubles, fake implementations
- **Legacy Code**: Hard-to-test or deprecated code
- **Configuration Files**: Templates, schemas, manifests

## Tools Included

This repository provides several tools to help with synthetic coverage:

- **merge-coverage.go**: Merges real and synthetic coverage files
- **synthetic-from-comments.go**: Generates synthetic coverage from code comments
- **synthetic-from-patterns.go**: Generates synthetic coverage based on file patterns
- **coverage-config-processor.go**: Processes configuration-based coverage requirements
- **hehe-coverage.go**: Specialized tool for custom file extensions

## Best Practices

1. **Document your approach**: Add a `COVERAGE.md` file explaining your synthetic coverage strategy
2. **Version control configuration**: Commit configuration files but not generated synthetic files
3. **Automate the process**: Integrate synthetic coverage generation into your build pipeline
4. **Be transparent**: Clearly indicate which parts of coverage are synthetic
5. **Review regularly**: Periodically review your synthetic coverage as your codebase evolves

## Integration with CI/CD

Add to your GitHub Actions workflow:

```yaml
- name: Run tests with coverage
  run: go test -coverprofile=coverage.txt ./...

- name: Generate synthetic coverage
  run: go run tools/synthetic-from-patterns.go -output=synthetic.txt

- name: Merge coverage
  run: go run tools/merge-coverage.go -i=coverage.txt -s=synthetic.txt -o=merged.txt

- name: Upload coverage
  uses: codecov/codecov-action@v3
  with:
    file: merged.txt
```

## Further Reading

For a deep dive into Go's coverage system, check out:

- [Go's Coverage Implementation](https://go.dev/blog/cover)
- [Go 1.20 Coverage Improvements](https://go.dev/doc/go1.20#coverage)
- [Go Coverage Tools](https://pkg.go.dev/cmd/cover)

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Contributing

Contributions are welcome! Feel free to submit issues or pull requests for:

- Additional examples
- Improvements to existing tools
- Documentation enhancements
- New techniques for synthetic coverage

## Acknowledgments

Special thanks to the Go team for creating a flexible coverage system that allows for these synthetic coverage techniques.