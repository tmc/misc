package ghascript

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/tools/txtar"
	"gopkg.in/yaml.v3"
)

// TestOption configures the Test function behavior
type TestOption func(*TestConfig)

// TestConfig holds configuration for the Test function
type TestConfig struct {
	WorkflowsDir string        // Directory containing .github/workflows (defaults to module root)
	Events       []string      // Events to test (defaults to push, pull_request, workflow_dispatch)
	Timeout      time.Duration // Timeout per workflow (defaults to 10 minutes)
	Parallel     bool          // Run workflows in parallel (defaults to true)
}

// WithWorkflowsDir specifies a custom directory containing .github/workflows
func WithWorkflowsDir(dir string) TestOption {
	return func(c *TestConfig) {
		c.WorkflowsDir = dir
	}
}

// WithEvents specifies which events to test
func WithEvents(events ...string) TestOption {
	return func(c *TestConfig) {
		c.Events = events
	}
}

// WithTimeout sets the timeout per workflow
func WithTimeout(timeout time.Duration) TestOption {
	return func(c *TestConfig) {
		c.Timeout = timeout
	}
}

// WithSequential disables parallel execution
func WithSequential() TestOption {
	return func(c *TestConfig) {
		c.Parallel = false
	}
}

// Test runs all GitHub Actions workflows in parallel.
// Each workflow is run as a separate subtest, and matrix jobs are run as individual subtests.
// By default, it finds workflows in the Go module root's .github/workflows directory.
// BUG: this doesn't work with txtarFiles..
// TODO: expose txtar and more direct/literal execution of workflow files.
func Test(t *testing.T, opts ...TestOption) {
	t.Helper()

	// Configure defaults
	config := &TestConfig{
		Events:   []string{"push", "pull_request", "workflow_dispatch"},
		Timeout:  10 * time.Minute,
		Parallel: true,
	}

	// Apply options
	for _, opt := range opts {
		opt(config)
	}

	// Find module root if no custom workflows directory specified
	if config.WorkflowsDir == "" {
		moduleRoot, err := findModuleRoot()
		if err != nil {
			t.Fatalf("Failed to find Go module root: %v", err)
		}
		config.WorkflowsDir = moduleRoot
	}

	// Find all workflow files
	workflows, err := FindWorkflows(config.WorkflowsDir)
	if err != nil {
		t.Fatalf("Failed to find workflows: %v", err)
	}

	if len(workflows) == 0 {
		t.Skipf("No GitHub Actions workflows found in %s/.github/workflows/", config.WorkflowsDir)
		return
	}

	t.Logf("Found %d workflow(s) in %s/.github/workflows/", len(workflows), config.WorkflowsDir)

	// Run each workflow
	for _, workflow := range workflows {
		workflow := workflow // capture loop variable
		workflowName := strings.TrimSuffix(filepath.Base(workflow), filepath.Ext(workflow))

		testFunc := func(t *testing.T) {
			if config.Parallel {
				t.Parallel()
			}
			// Extract txtar files to tmp directory:
			_, err := txtar.ParseFile(workflow)
			if err != nil {
				t.Fatalf("Failed to parse test file %s: %v", workflow, err)
			}

			runWorkflowTestWithConfig(t, workflow, config)
		}

		t.Run(workflowName, testFunc)
	}
}

// findModuleRoot finds the root directory of the Go module
func findModuleRoot() (string, error) {
	// Try using go env GOMOD first
	cmd := exec.Command("go", "env", "GOMOD")
	output, err := cmd.Output()
	if err == nil && len(output) > 0 {
		gomod := strings.TrimSpace(string(output))
		if gomod != "" && gomod != "/dev/null" {
			return filepath.Dir(gomod), nil
		}
	}

	// Fallback: search upward for go.mod
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break // reached root
		}
		dir = parent
	}

	// If no go.mod found, use current directory
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %v", err)
	}
	return wd, nil
}

// runWorkflowTestWithConfig runs a single workflow with the given configuration
func runWorkflowTestWithConfig(t *testing.T, workflowPath string, config *TestConfig) {
	t.Helper()

	// Parse workflow to extract matrix configurations
	workflow, err := parseWorkflow(workflowPath)
	if err != nil {
		t.Fatalf("Failed to parse workflow %s: %v", workflowPath, err)
	}

	// Use configured events or workflow events
	events := config.Events
	if len(events) == 0 {
		events = getWorkflowEvents(workflow)
		if len(events) == 0 {
			events = []string{"push"} // default event
		}
	}

	for _, event := range events {
		event := event

		// Extract matrix jobs for this workflow
		matrixJobs := extractMatrixJobs(workflow)

		if len(matrixJobs) == 0 {
			// No matrix - run workflow directly
			testFunc := func(t *testing.T) {
				if config.Parallel {
					t.Parallel()
				}
				runSingleWorkflowWithConfig(t, workflowPath, event, nil, config)
			}
			t.Run(event, testFunc)
		} else {
			// Run each matrix combination
			for jobName, matrices := range matrixJobs {
				for i, matrix := range matrices {
					jobName := jobName
					matrix := matrix
					matrixName := fmt.Sprintf("%s-matrix-%d", jobName, i)

					testFunc := func(t *testing.T) {
						if config.Parallel {
							t.Parallel()
						}
						runSingleWorkflowWithConfig(t, workflowPath, event, matrix, config)
					}
					t.Run(fmt.Sprintf("%s-%s", event, matrixName), testFunc)
				}
			}
		}
	}
}

// runSingleWorkflowWithConfig executes a single workflow with optional matrix configuration
func runSingleWorkflowWithConfig(t *testing.T, workflowPath, event string, matrix map[string]interface{}, config *TestConfig) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	defer cancel()

	// Build act command
	args := []string{
		"--workflows", workflowPath,
		"--eventpath", "/dev/null",
	}

	// Add event
	if event != "" {
		args = append(args, event)
	}

	// Add matrix variables as environment variables
	if matrix != nil {
		for key, value := range matrix {
			args = append(args, "--env", fmt.Sprintf("matrix_%s=%v", key, value))
		}
	}

	// Execute workflow
	if err := runWorkflowCommand(ctx, workflowPath, event, matrix); err != nil {
		if matrix != nil {
			t.Errorf("Workflow failed for matrix %v: %v", matrix, err)
		} else {
			t.Errorf("Workflow failed: %v", err)
		}
		return
	}

	if matrix != nil {
		t.Logf("Workflow completed successfully for matrix %v", matrix)
	} else {
		t.Logf("Workflow completed successfully")
	}
}

// Note: Workflow, Job, Strategy, Step types are defined in workflow.go

// parseWorkflow parses a YAML workflow file
func parseWorkflow(workflowPath string) (*Workflow, error) {
	data, err := os.ReadFile(workflowPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read workflow file: %v", err)
	}

	var workflow Workflow
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		return nil, fmt.Errorf("failed to parse workflow YAML: %v", err)
	}

	return &workflow, nil
}

// getWorkflowEvents extracts the events that trigger a workflow
func getWorkflowEvents(workflow *Workflow) []string {
	var events []string

	switch on := workflow.On.(type) {
	case string:
		events = append(events, on)
	case []interface{}:
		for _, event := range on {
			if eventStr, ok := event.(string); ok {
				events = append(events, eventStr)
			}
		}
	case map[string]interface{}:
		for event := range on {
			events = append(events, event)
		}
	}

	// Limit to commonly testable events
	var testableEvents []string
	for _, event := range events {
		switch event {
		case "push", "pull_request", "workflow_dispatch":
			testableEvents = append(testableEvents, event)
		}
	}

	return testableEvents
}

// extractMatrixJobs extracts matrix configurations from workflow jobs
func extractMatrixJobs(workflow *Workflow) map[string][]map[string]interface{} {
	matrixJobs := make(map[string][]map[string]interface{})

	for jobName, job := range workflow.Jobs {
		if job.Strategy != nil && job.Strategy.Matrix != nil {
			matrices := generateMatrixCombinations(job.Strategy.Matrix)
			if len(matrices) > 0 {
				matrixJobs[jobName] = matrices
			}
		}
	}

	return matrixJobs
}

// generateMatrixCombinations generates all combinations of matrix variables
func generateMatrixCombinations(matrix map[string]interface{}) []map[string]interface{} {
	if len(matrix) == 0 {
		return nil
	}

	// Convert matrix values to string slices
	matrixVars := make(map[string][]string)
	for key, value := range matrix {
		switch v := value.(type) {
		case []interface{}:
			var strSlice []string
			for _, item := range v {
				strSlice = append(strSlice, fmt.Sprintf("%v", item))
			}
			matrixVars[key] = strSlice
		case string:
			matrixVars[key] = []string{v}
		default:
			matrixVars[key] = []string{fmt.Sprintf("%v", v)}
		}
	}

	// Generate cartesian product
	return cartesianProduct(matrixVars)
}

// cartesianProduct generates all combinations of the given variables
func cartesianProduct(vars map[string][]string) []map[string]interface{} {
	if len(vars) == 0 {
		return nil
	}

	// Get keys and values
	keys := make([]string, 0, len(vars))
	values := make([][]string, 0, len(vars))

	for key, vals := range vars {
		keys = append(keys, key)
		values = append(values, vals)
	}

	// Generate combinations
	var combinations []map[string]interface{}

	var generate func(int, map[string]interface{})
	generate = func(index int, current map[string]interface{}) {
		if index == len(keys) {
			// Make a copy of current combination
			combo := make(map[string]interface{})
			for k, v := range current {
				combo[k] = v
			}
			combinations = append(combinations, combo)
			return
		}

		key := keys[index]
		for _, value := range values[index] {
			current[key] = value
			generate(index+1, current)
		}
	}

	generate(0, make(map[string]interface{}))
	return combinations
}

// runWorkflowCommand executes a workflow using our native runner
func runWorkflowCommand(ctx context.Context, workflowPath string, event string, matrix map[string]interface{}) error {
	// Create workflow runner
	workDir := filepath.Dir(filepath.Dir(filepath.Dir(workflowPath))) // Go up from .github/workflows/file.yml
	runner := NewWorkflowRunner(workDir, event)

	// Run the workflow
	return runner.RunWorkflow(ctx, workflowPath, matrix)
}
