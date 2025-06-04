# ghascript

Package ghascript provides rsc.io/script integration for running GitHub Actions workflows locally using a built-in workflow runner.

## Features

- **Script Integration**: Use GitHub Actions workflows in script tests
- **Matrix Support**: Automatically runs all matrix combinations in parallel
- **Module Root Discovery**: Automatically finds workflows in your Go module
- **Event Testing**: Test specific GitHub events (push, pull_request, etc.)
- **Parallel Execution**: Run workflows in parallel for faster testing
- **CLI Tool**: Standalone command for testing all workflows

## Installation

```bash
# No external dependencies required!
go get github.com/tmc/misc/testctr/testctrscript/ghascript
```

## Usage

### Simple Test Function

The easiest way to test all your workflows:

```go
func TestAllWorkflows(t *testing.T) {
    ghascript.Test(t) // Automatically finds and tests all workflows
}
```

### With Options

```go
func TestWorkflowsCustom(t *testing.T) {
    ghascript.Test(t,
        ghascript.WithEvents("push", "pull_request"),
        ghascript.WithTimeout(5*time.Minute),
        ghascript.WithWorkflowsDir("/custom/path"),
        ghascript.WithSequential(), // Run sequentially instead of parallel
    )
}
```

### Script Tests

Use in rsc.io/script tests:

```go
func TestGitHubActionsScripts(t *testing.T) {
    engine := &script.Engine{
        Cmds:  ghascript.DefaultCmds(t),
        Conds: ghascript.DefaultConds(),
    }
    scripttest.Run(t, scripttest.Params{
        Dir:    "testdata",
        Engine: engine,
    })
}
```

### Example Script

```txt
# Test CI workflow
workflow list
stdout '.github/workflows/ci.yml'

# Check workflow exists
[workflow ci]

# Run workflow  
workflow run ci
stdout 'Workflow ci completed successfully'

# Verify success
[workflow-success ci]

-- .github/workflows/ci.yml --
name: CI
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - name: Run tests
        run: echo "Tests passed"
```

## CLI Tool

Install and use the standalone CLI:

```bash
# Build the CLI
go build ./exp/cmd/ghascript

# Test all workflows in current module
./ghascript

# Test with options
./ghascript -events push -timeout 5m -verbose

# Dry run to see what would be executed
./ghascript -dry-run
```

## Commands

### workflow run

Run a GitHub Actions workflow:

```
workflow run workflow-name [options]
```

Options:
- `-e event` - Specify event type
- `--env KEY=value` - Set environment variables
- `--secret KEY=value` - Set secrets

### workflow list

List available workflows:

```
workflow list
```

### workflow events

List events for a workflow:

```
workflow events workflow-name
```

### workflow jobs

List jobs for a workflow and event:

```
workflow jobs workflow-name event
```

### workflow doctor

Check Docker and buildx capabilities:

```
workflow doctor
```

This command checks:
- Docker installation and version
- Docker buildx availability
- General Docker functionality
- Issues warnings for missing capabilities

## Conditions

Check workflow state:

```
[workflow ci]                # Workflow file exists
[workflow-success ci]        # Last run was successful  
[workflow-failed ci]         # Last run failed
```

## Integration with testctr

ghascript works seamlessly with testctr for service dependencies:

```txt
# Start database service
testctr start postgres:15 db -p 5432 -e POSTGRES_PASSWORD=test
testctr wait db

# Run workflow that uses the database
workflow run integration-test
stdout 'Workflow integration-test completed successfully'

# Cleanup
testctr stop db
```

## Matrix Support

Workflows with matrix strategies are automatically expanded:

```yaml
# .github/workflows/matrix.yml
jobs:
  test:
    strategy:
      matrix:
        os: [ubuntu-latest, macos-latest]
        node: [16, 18, 20]
    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/setup-node@v3
        with:
          node-version: ${{ matrix.node }}
```

This creates 6 separate test runs (3 node versions × 2 operating systems).

## Configuration

Workflows run in Ubuntu containers by default. The runner automatically:

- Maps GitHub runner labels to appropriate container images
- Sets up GitHub Actions context variables
- Installs common tools (git, curl, wget)
- Handles basic setup actions (setup-node, setup-go, setup-python)

## Examples

See the [testdata](testdata/) directory for complete examples.

## Requirements

- Docker must be running (for testctr container execution)
- Go module with .github/workflows/ directory

## Architecture

ghascript uses a built-in GitHub Actions workflow runner that:

- **Parses workflow YAML** using native Go libraries
- **Executes steps in testctr containers** for full isolation
- **Supports common GitHub Actions features** like matrix jobs, expressions, and built-in actions
- **Integrates with testctr** for service dependencies and container management
- **Detects Docker capabilities** and warns about missing buildx for advanced features
- **No external dependencies** - everything runs in Go