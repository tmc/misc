# Synthetic Coverage Tool

This tool allows adding synthetic coverage data to existing Go coverage files. This is useful for:
- Adding coverage for manually tested code
- Including third-party libraries in coverage reports
- Creating coverage data for code that can't be instrumented

## Approach

The Go coverage format consists of two main file types:
1. **Meta-data files** (`covmeta.*`) - Contains package, function, and source location information
2. **Counter files** (`covcounters.*`) - Contains execution counts for each code unit

### Limitations

The current Go coverage format has some limitations for adding synthetic data:
1. The meta-data file hash is computed from all package data
2. Counter files reference the meta-data hash, creating a tight coupling
3. Package and function IDs must be consistent across files

### Solution Approaches

1. **Rewrite Entire Coverage Data** (Current approach)
   - Read existing meta and counter files
   - Add synthetic packages/functions
   - Recompute hashes and write new files
   - Pros: Works with existing tools
   - Cons: Complex, requires rewriting all data

2. **Coverage Proxy Tool**
   - Create a proxy that intercepts coverage data requests
   - Inject synthetic data on-the-fly
   - Pros: No file modification needed
   - Cons: Requires custom tooling

3. **Post-Processing Text Format**
   - Convert binary to text format
   - Add synthetic lines to text format
   - Use for reporting only
   - Pros: Simple to implement
   - Cons: Limited to text format tools

## Implementation Notes

The current implementation attempts to:
1. Parse existing coverage binary files
2. Add new package and function entries
3. Add corresponding counter data
4. Maintain consistency across all files

However, due to the tight coupling of the format, this is challenging. A simpler approach might be to work with the text format output instead.

## Alternative: Text Format Manipulation

```go
// Simple approach: Add lines to text format coverage
func addSyntheticTextCoverage(inputFile, outputFile string, synthetic []CoverageLine) error {
    // Read existing coverage
    data, err := os.ReadFile(inputFile)
    if err != nil {
        return err
    }
    
    lines := strings.Split(string(data), "\n")
    
    // Add synthetic lines after the mode line
    var output []string
    output = append(output, lines[0]) // mode: set
    
    // Add synthetic coverage lines
    for _, syn := range synthetic {
        line := fmt.Sprintf("%s:%d.%d,%d.%d %d %d", 
            syn.File, syn.StartLine, syn.StartCol,
            syn.EndLine, syn.EndCol, syn.Statements, syn.Count)
        output = append(output, line)
    }
    
    // Add rest of original lines
    output = append(output, lines[1:]...)
    
    return os.WriteFile(outputFile, []byte(strings.Join(output, "\n")), 0644)
}
```

This simpler approach works with the text format that many tools already support.