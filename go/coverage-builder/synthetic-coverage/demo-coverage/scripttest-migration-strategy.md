# Migrating to ScriptTest with Synthetic Coverage: A Practical Guide

## Introduction

Many Go projects start with traditional unit and integration tests but could benefit from the comprehensive testing that `rsc.io/script/scripttest` provides, especially for CLI applications. This guide presents a strategy for adding scripttest tests to an existing codebase while maintaining accurate coverage metrics through synthetic coverage.

## Why Migrate to ScriptTest?

ScriptTest offers several advantages that complement traditional Go tests:

1. **End-to-end validation**: Tests the actual binary as users would use it
2. **Real-world scenarios**: Emulates user interactions with the CLI
3. **Simplified testing**: Clear, declarative test scripts that are easy to maintain
4. **Comprehensive coverage**: Tests code paths that might be missed by unit tests
5. **Documentation**: Test scripts serve as usage examples

However, one challenge is that scripttest executes your code in a separate process, which means standard coverage instrumentation doesn't capture this execution. This is where synthetic coverage becomes essential.

## Step 1: Analyze Your Current Test Coverage

Before adding scripttest tests, understand your current test coverage:

```bash
# Generate coverage profile
go test -coverprofile=coverage.txt ./...

# See coverage by package
go tool cover -func=coverage.txt

# Generate HTML report to identify gaps
go tool cover -html=coverage.txt -o=coverage.html
```

Look for:
- Untested or under-tested commands
- Integration points between components
- Error handling paths
- User workflow scenarios

## Step 2: Identify Candidate Scenarios for ScriptTest

The best candidates for scripttest tests are:

1. **Complete user workflows**: End-to-end scenarios that use multiple commands
2. **Complex command sequences**: Commands that need to be executed in a specific order
3. **Edge cases**: Error handling, invalid inputs, boundary conditions
4. **Hard-to-test manually**: Scenarios with complex setup or validation
5. **Under-tested code paths**: Areas with low coverage in traditional tests

Create a list of test scenarios, prioritizing those that would provide the most value:

```
# Example Scenario List
1. User registration and login flow
2. Project creation and configuration
3. Data import/export cycle
4. Error handling for invalid inputs
5. Configuration file parsing and validation
6. Help documentation and version information
```

## Step 3: Set Up ScriptTest Infrastructure

Add the necessary infrastructure to run scripttest tests:

```go
// test/scripttest/script_test.go
package scripttest

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"rsc.io/script/scripttest"
)

func TestScript(t *testing.T) {
	// Build the tool
	binPath := buildTool(t)
	
	// Set up scripttest
	ts := scripttest.New()
	ts.Cmds["mytool"] = binPath
	
	// Run scripts in the testdata directory
	ts.Run(t, "testdata", "*.txt")
}

func buildTool(t *testing.T) string {
	t.Helper()
	
	// Create temp directory for binary
	tmpDir, err := os.MkdirTemp("", "scripttest-bin")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	
	// Build the binary
	binPath := filepath.Join(tmpDir, "mytool")
	cmd := exec.Command("go", "build", "-o", binPath, "../cmd/mytool")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build tool: %v", err)
	}
	
	return binPath
}
```

Create a directory for test scripts:

```bash
mkdir -p test/scripttest/testdata
```

## Step 4: Create Initial ScriptTest Files

Start with a simple test script to verify the setup:

```
# test/scripttest/testdata/basic.txt
# Basic functionality test

# Test version command
exec mytool version
stdout 'v[0-9]+\.[0-9]+\.[0-9]+'
! stderr .

# Test help command
exec mytool help
stdout 'Usage:'
stdout 'Commands:'
! stderr .
```

Run the test to make sure everything is set up correctly:

```bash
go test ./test/scripttest
```

## Step 5: Add ScriptTest for Identified Scenarios

Implement test scripts for the scenarios identified in Step 2. For each scenario:

1. Create a new script file with a descriptive name
2. Include comments explaining the scenario
3. Break the test into logical steps
4. Verify command output and behavior
5. Test both success and failure paths

Example for a user workflow:

```
# test/scripttest/testdata/user_workflow.txt
# User registration and login workflow

# Create a test user
exec mytool user create --name=testuser --email=test@example.com --password=password123
stdout 'User created successfully'
! stderr .

# Attempt to create a duplicate user (should fail)
! exec mytool user create --name=testuser --email=test@example.com --password=password123
stderr 'User already exists'

# Login with the created user
exec mytool login --user=testuser --password=password123
stdout 'Login successful'
! stderr .

# Try invalid password
! exec mytool login --user=testuser --password=wrongpassword
stderr 'Invalid credentials'

# Test logout
exec mytool logout
stdout 'Logged out successfully'
! stderr .
```

## Step 6: Set Up Synthetic Coverage Generation

Create a tool to generate synthetic coverage from script files:

```go
// tools/scripttest-coverage/main.go
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	scriptDir    = flag.String("scripts", "", "Directory containing testscript files")
	outputFile   = flag.String("output", "scripttest-coverage.txt", "Output coverage file")
	packageRoot  = flag.String("package", "", "Package root (e.g., github.com/myorg/myproject)")
	commandMap   = flag.String("command-map", "", "YAML file mapping commands to code paths")
	verbose      = flag.Bool("v", false, "Verbose output")
)

func main() {
	flag.Parse()
	
	if *scriptDir == "" {
		fmt.Fprintf(os.Stderr, "Error: -scripts flag is required\n")
		os.Exit(1)
	}
	
	if *packageRoot == "" {
		fmt.Fprintf(os.Stderr, "Error: -package flag is required\n")
		os.Exit(1)
	}
	
	// Load command mapping
	mapping, err := loadCommandMapping(*commandMap)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading command mapping: %v\n", err)
		os.Exit(1)
	}
	
	// Find all script files
	scriptFiles, err := findScriptFiles(*scriptDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error finding script files: %v\n", err)
		os.Exit(1)
	}
	
	if *verbose {
		fmt.Printf("Found %d script files\n", len(scriptFiles))
	}
	
	// Extract commands from script files
	commands, err := extractCommands(scriptFiles)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error extracting commands: %v\n", err)
		os.Exit(1)
	}
	
	if *verbose {
		fmt.Printf("Extracted %d commands\n", len(commands))
	}
	
	// Generate synthetic coverage
	coverage, err := generateCoverage(commands, mapping, *packageRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating coverage: %v\n", err)
		os.Exit(1)
	}
	
	// Write coverage file
	if err := os.WriteFile(*outputFile, []byte(coverage), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing coverage file: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("Generated synthetic coverage in %s\n", *outputFile)
}

// Implementation of helper functions...
```

Create a command mapping file:

```yaml
# command-map.yaml
# Maps commands to code paths
commands:
  - name: "version"
    functions:
      - file: "cmd/mytool/main.go"
        function: "showVersion"
        start_line: 50
        end_line: 55
  
  - name: "help"
    functions:
      - file: "cmd/mytool/main.go"
        function: "showHelp"
        start_line: 60
        end_line: 75
  
  - name: "user create"
    functions:
      - file: "cmd/mytool/user.go"
        function: "createUser"
        start_line: 100
        end_line: 150
        
  # Add more mappings as needed
```

## Step 7: Incremental Migration Strategy

Follow this strategy to incrementally migrate to scripttest:

1. **Start with simple commands**: Begin with basic commands like `version`, `help`, etc.
2. **Focus on high-value scenarios**: Prioritize complex workflows and under-tested code
3. **Balance duplication and coverage**: It's okay to test some functionality both ways
4. **Update mappings as you go**: Refine command mappings based on code changes
5. **Integrate with CI/CD**: Add synthetic coverage generation to your pipeline

Example CI/CD workflow:

```yaml
# .github/workflows/test.yml
name: Test

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      # Run standard tests
      - name: Run tests
        run: go test -coverprofile=standard-coverage.txt ./...
      
      # Run scripttest tests
      - name: Run scripttest tests
        run: go test ./test/scripttest
      
      # Generate synthetic coverage
      - name: Generate synthetic coverage
        run: |
          go run ./tools/scripttest-coverage/main.go \
            -scripts=./test/scripttest/testdata \
            -output=scripttest-coverage.txt \
            -package=github.com/myorg/myproject \
            -command-map=./command-map.yaml
      
      # Merge coverage files
      - name: Merge coverage
        run: |
          go run ./tools/merge-coverage/main.go \
            -standard=standard-coverage.txt \
            -synthetic=scripttest-coverage.txt \
            -output=coverage.txt
      
      # Upload coverage report
      - name: Upload coverage to Codecov
        uses: codecov/codecov-action@v3
        with:
          file: ./coverage.txt
```

## Step 8: Track Your Progress

Monitor coverage metrics to ensure your migration is effective:

```bash
# Track coverage before and after adding scripttest
go tool cover -func=standard-coverage.txt > before.txt
go tool cover -func=coverage.txt > after.txt
diff -u before.txt after.txt
```

Create a dashboard or report to visualize progress:

```
# Coverage Report
| Area             | Before | After | Change |
|------------------|--------|-------|--------|
| cmd/user.go      | 65.2%  | 92.1% | +26.9% |
| cmd/project.go   | 78.4%  | 85.3% | +6.9%  |
| internal/auth.go | 81.7%  | 94.5% | +12.8% |
| Total            | 72.3%  | 87.6% | +15.3% |
```

## Step 9: Advanced Integration Techniques

### Automatic Command Mapping

Instead of maintaining a manual mapping file, you can use callgraph analysis:

```go
// tools/generate-command-map/main.go
package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"gopkg.in/yaml.v2"
)

// CommandMapping structure
type CommandMapping struct {
	Name      string     `yaml:"name"`
	Functions []Function `yaml:"functions"`
}

// Function structure
type Function struct {
	File      string `yaml:"file"`
	Function  string `yaml:"function"`
	StartLine int    `yaml:"start_line"`
	EndLine   int    `yaml:"end_line"`
}

// Implementation of command mapping generator
// ...
```

### Dynamic Test Script Generation

Create tools to automatically generate test scripts based on code structure:

```go
// tools/generate-scripts/main.go
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

var scriptTemplate = `# {{.Description}}
# Generated on {{.Date}}

{{range .Commands}}
# Test {{.Name}}
exec mytool {{.CommandLine}}
{{if .ExpectSuccess}}
stdout '{{.ExpectedOutput}}'
{{if .CheckStderr}}! stderr .{{end}}
{{else}}
! exec mytool {{.CommandLine}}
stderr '{{.ExpectedError}}'
{{end}}

{{end}}
`

// TestCommand structure
type TestCommand struct {
	Name           string
	CommandLine    string
	ExpectSuccess  bool
	ExpectedOutput string
	ExpectedError  string
	CheckStderr    bool
}

// Template data structure
type TemplateData struct {
	Description string
	Date        string
	Commands    []TestCommand
}

// Implementation of script generator
// ...
```

### Coverage Evolution Tracking

Track how scripttest coverage evolves over time:

```go
// tools/coverage-tracker/main.go
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"time"
	
	_ "github.com/mattn/go-sqlite3"
)

var (
	coverageFile = flag.String("coverage", "coverage.txt", "Coverage file to track")
	dbFile       = flag.String("db", "coverage.db", "Database file for tracking")
)

func main() {
	flag.Parse()
	
	// Open database
	db, err := sql.Open("sqlite3", *dbFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()
	
	// Create tables if they don't exist
	if err := createTables(db); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating tables: %v\n", err)
		os.Exit(1)
	}
	
	// Parse coverage file
	coverage, err := parseCoverageFile(*coverageFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing coverage file: %v\n", err)
		os.Exit(1)
	}
	
	// Record coverage
	if err := recordCoverage(db, coverage); err != nil {
		fmt.Fprintf(os.Stderr, "Error recording coverage: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Println("Coverage recorded successfully")
}

// Implementation of coverage tracking
// ...
```

## Step 10: Long-term Maintenance

### Documentation

Document your scripttest approach for the team:

```markdown
# ScriptTest Best Practices

## When to Use ScriptTest

- End-to-end workflows
- CLI command testing
- User journey validation
- Edge case validation

## Writing Effective Test Scripts

1. Use clear, descriptive comments
2. Test both success and failure paths
3. Verify all relevant output (stdout and stderr)
4. Use wildcards and regexes for variable output
5. Keep scripts focused on specific features

## Synthetic Coverage Management

- Update command mappings when adding new commands
- Run mapping generator when code structure changes
- Review coverage reports regularly
```

### Coverage Validation

Create a tool to validate that synthetic coverage is accurate:

```go
// tools/validate-coverage/main.go
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

var (
	syntheticFile = flag.String("synthetic", "", "Synthetic coverage file")
	actualFile    = flag.String("actual", "", "Actual coverage file from instrumented run")
	tolerance     = flag.Float64("tolerance", 5.0, "Tolerance percentage for discrepancies")
)

func main() {
	flag.Parse()
	
	if *syntheticFile == "" || *actualFile == "" {
		fmt.Fprintf(os.Stderr, "Both -synthetic and -actual flags are required\n")
		os.Exit(1)
	}
	
	// Parse coverage files
	syntheticCov, err := parseCoverageFile(*syntheticFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing synthetic coverage: %v\n", err)
		os.Exit(1)
	}
	
	actualCov, err := parseCoverageFile(*actualFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing actual coverage: %v\n", err)
		os.Exit(1)
	}
	
	// Compare coverage
	discrepancies := compareCoverage(syntheticCov, actualCov, *tolerance)
	
	// Report results
	if len(discrepancies) > 0 {
		fmt.Printf("Found %d discrepancies between synthetic and actual coverage:\n", len(discrepancies))
		for _, d := range discrepancies {
			fmt.Printf("  %s: synthetic=%f%%, actual=%f%%, diff=%f%%\n", 
				d.File, d.SyntheticCoverage, d.ActualCoverage, d.Difference)
		}
		os.Exit(1)
	} else {
		fmt.Println("Synthetic coverage validated successfully")
	}
}

// Implementation of coverage validation
// ...
```

## Real-World Example: Migrating a CLI Tool

Let's look at a concrete example of migrating an existing CLI tool to scripttest.

### Original CLI Structure

```
mycli/
├── cmd/
│   ├── root.go
│   ├── version.go
│   ├── config.go
│   └── process.go
├── internal/
│   ├── processor/
│   │   ├── processor.go
│   │   └── utils.go
│   └── config/
│       ├── config.go
│       └── validator.go
├── pkg/
│   └── utils/
│       └── utils.go
└── main.go
```

### Identify Test Scenarios

1. Basic commands: version, help
2. Configuration: create, update, validate
3. Processing workflow: init, run, export
4. Error handling: invalid inputs, missing files

### Create ScriptTest Files

```
# test/scripttest/testdata/basic.txt
# Basic command tests

# Test version
exec mycli version
stdout 'v[0-9]+\.[0-9]+\.[0-9]+'
! stderr .

# Test help
exec mycli help
stdout 'Usage:'
stdout 'Commands:'
! stderr .
```

```
# test/scripttest/testdata/config.txt
# Configuration command tests

# Create config
exec mycli config create --name=test
stdout 'Config created'
! stderr .

# Update config
exec mycli config update --name=test --value=100
stdout 'Config updated'
! stderr .

# Validate config
exec mycli config validate --name=test
stdout 'Config is valid'
! stderr .

# Try invalid config
! exec mycli config validate --name=nonexistent
stderr 'Config not found'
```

```
# test/scripttest/testdata/process.txt
# Processing workflow tests

# Initialize processing
exec mycli process init --target=data.txt
stdout 'Processing initialized'
! stderr .

# Run processing
exec mycli process run --target=data.txt
stdout 'Processing completed'
! stderr .

# Export results
exec mycli process export --target=data.txt --format=json
stdout '{"status":"success"'
! stderr .
```

### Command Mapping

```yaml
# command-map.yaml
commands:
  - name: "version"
    functions:
      - file: "cmd/version.go"
        function: "runVersion"
        start_line: 10
        end_line: 15
  
  - name: "config create"
    functions:
      - file: "cmd/config.go"
        function: "createConfig"
        start_line: 20
        end_line: 35
      - file: "internal/config/config.go"
        function: "Create"
        start_line: 15
        end_line: 40
  
  - name: "config update"
    functions:
      - file: "cmd/config.go"
        function: "updateConfig"
        start_line: 40
        end_line: 55
      - file: "internal/config/config.go"
        function: "Update"
        start_line: 45
        end_line: 70
  
  # Add remaining commands...
```

### Monitor Progress

```
# Coverage Report After Migration
| File                        | Before | After | Gain  |
|-----------------------------|--------|-------|-------|
| cmd/root.go                 | 85.7%  | 100%  | +14.3%|
| cmd/version.go              | 66.7%  | 100%  | +33.3%|
| cmd/config.go               | 72.4%  | 95.2% | +22.8%|
| cmd/process.go              | 68.9%  | 91.5% | +22.6%|
| internal/processor/processor.go | 78.3%  | 92.1% | +13.8%|
| internal/config/config.go   | 81.2%  | 97.8% | +16.6%|
| internal/config/validator.go| 72.5%  | 88.3% | +15.8%|
| pkg/utils/utils.go          | 90.1%  | 95.5% | +5.4% |
| Total                       | 77.2%  | 94.1% | +16.9%|
```

## Conclusion

Migrating to scripttest with synthetic coverage is an incremental process that provides significant benefits:

1. More comprehensive testing of your application
2. Better documentation of usage patterns
3. More accurate coverage metrics
4. Testing of real-world user workflows
5. Improved confidence in the robustness of your code

The key to success is to start small, focus on high-value scenarios, and continuously refine your approach. With the right tools and processes in place, you can gradually build a comprehensive scripttest suite that complements your existing tests and provides more thorough validation of your application's behavior.

Remember that the goal is not to replace traditional Go tests but to complement them with end-to-end tests that validate real-world usage scenarios. By combining both approaches and using synthetic coverage to maintain accurate metrics, you can achieve a more robust and reliable testing strategy for your Go applications.