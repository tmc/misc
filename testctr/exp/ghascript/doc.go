// Package ghascript provides rsc.io/script integration for running GitHub Actions workflows locally.
//
// This package extends testctrscript with the ability to run GitHub Actions workflows
// in containerized environments for testing CI/CD pipelines locally using a built-in workflow runner.
//
// # Basic Usage
//
// The package provides script commands for managing GitHub Actions workflows:
//
//	workflow run workflow-name [options] # Run a GitHub Actions workflow  
//	workflow list                       # List available workflows
//	workflow events workflow-name       # List events for a workflow
//	workflow jobs workflow-name event   # List jobs for a workflow and event
//
//	# Also available with "act" alias for compatibility:
//	act run workflow-name [options]     # Same as "workflow run"
//
// # Example Script
//
//	# List available workflows
//	workflow list
//	stdout '.github/workflows/ci.yml'
//
//	# Run the CI workflow
//	workflow run ci
//	stdout 'Workflow ci completed successfully'
//
//	# Run with specific event
//	workflow run ci -e push
//	stdout 'Workflow ci completed successfully'
//
// # Integration with testctrscript
//
// ghascript integrates seamlessly with testctrscript commands and conditions:
//
//	# Start supporting services
//	testctr start postgres:15 db -p 5432 -e POSTGRES_PASSWORD=test
//	testctr wait db
//
//	# Run workflow that uses the database
//	workflow run integration-test
//	stdout 'Workflow integration-test completed successfully'
//
//	# Cleanup
//	testctr stop db
//
// # Workflow File Requirements
//
// Scripts can include GitHub Actions workflow files using txtar format:
//
//	# Test CI workflow
//	workflow run ci
//	stdout 'Workflow ci completed successfully'
//
//	-- .github/workflows/ci.yml --
//	name: CI
//	on: [push, pull_request]
//	jobs:
//	  test:
//	    runs-on: ubuntu-latest
//	    steps:
//	      - uses: actions/checkout@v4
//	      - name: Run tests
//	        run: echo "All tests passed"
//
// # Configuration Options
//
// The package supports workflow configuration through command options:
//
//	workflow run ci --env KEY=value    # Set environment variables
//	workflow run ci --secret KEY=value # Set secrets
//	workflow run ci -e push            # Specify event type
//
// # Conditions
//
// The package provides conditions for checking workflow state:
//
//	[workflow ci]                      # Workflow file exists
//	[workflow-success ci]              # Last run was successful
//	[workflow-failed ci]               # Last run failed
//
// # Advanced Features
//
// Built-in support for GitHub Actions features:
//
//	# Set environment variables
//	workflow run ci --env NODE_VERSION=18
//
//	# Set secrets for workflows
//	workflow run deploy --secret GITHUB_TOKEN=ghp_xxx
//
//	# Run specific events
//	workflow run ci -e pull_request
//
//	# Matrix jobs are automatically expanded and run in parallel
//
// # Simple Test Function
//
// The Test function automatically runs all workflows in the module:
//
//	func TestWorkflows(t *testing.T) {
//	    ghascript.Test(t) // Runs all workflows in parallel with matrix support
//	}
//
//	// With options:
//	func TestWorkflowsCustom(t *testing.T) {
//	    ghascript.Test(t,
//	        ghascript.WithEvents("push", "pull_request"),
//	        ghascript.WithTimeout(5*time.Minute),
//	        ghascript.WithSequential(),
//	    )
//	}
package ghascript