# go-test-cover-all

A tool that finds all Go modules under the current directory and runs tests with coverage for all of them, aggregating the results.

## Usage

```bash
# From the directory you want to test
go run github.com/tmc/misc/testctr/exp/cmd/go-test-cover-all

# With options
go run github.com/tmc/misc/testctr/exp/cmd/go-test-cover-all -v -timeout=30m

# From the tool directory (tests the tool itself)
cd exp/cmd/go-test-cover-all
go run . -v
```

## Options

- `-v` - Verbose output (show test output)
 
- `-timeout <duration>` - Test timeout per package (default: 10m)
- `-coverdir <dir>` - Coverage data directory (default: .coverage)
- `-clean` - Remove coverage directory before running (default: true)

## How it works

1. Finds all `go.mod` files under the current directory
2. Runs `go test -cover ./... -args -test.gocoverdir=<dir>` for each module
3. Collects all coverage data files (covcounters.* and covmeta.*) in module-specific directories
4. Copies all coverage files to the root `.coverage` directory
5. Reports the total coverage percentage using `go tool covdata percent`

## Coverage Output

The coverage data is stored in the current directory's `.coverage` directory:

```
.coverage/
├── modules/
│   ├── root/         # Coverage data for root module
│   ├── module1/      # Coverage data for first submodule
│   └── module2/      # Coverage data for second submodule
├── covmeta.*         # Merged coverage metadata
└── covcounters.*     # Merged coverage counters
```

The tool automatically copies all module coverage data files to the root `.coverage` directory for unified reporting with `go tool covdata`.

## Requirements

- Go 1.20+ (for the new coverage format with GOCOVERDIR)