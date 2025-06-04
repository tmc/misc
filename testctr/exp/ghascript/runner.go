package ghascript

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/tmc/misc/testctr"
	"gopkg.in/yaml.v3"
)

// WorkflowRunner executes GitHub Actions workflows using testctr containers
type WorkflowRunner struct {
	workDir     string
	event       string
	environment map[string]string
	secrets     map[string]string
	containers  map[string]*testctr.Container
}

// NewWorkflowRunner creates a new workflow runner
func NewWorkflowRunner(workDir, event string) *WorkflowRunner {
	return &WorkflowRunner{
		workDir:     workDir,
		event:       event,
		environment: make(map[string]string),
		secrets:     make(map[string]string),
		containers:  make(map[string]*testctr.Container),
	}
}

// SetEnv sets an environment variable
func (r *WorkflowRunner) SetEnv(key, value string) {
	r.environment[key] = value
}

// SetSecret sets a secret
func (r *WorkflowRunner) SetSecret(key, value string) {
	r.secrets[key] = value
}

// RunWorkflow executes a GitHub Actions workflow
func (r *WorkflowRunner) RunWorkflow(ctx context.Context, workflowPath string, matrix map[string]interface{}) error {
	// Check Docker capabilities before starting
	if err := ensureDockerCapabilities(); err != nil {
		return fmt.Errorf("Docker capabilities check failed: %v", err)
	}

	// Parse workflow
	workflow, err := r.parseWorkflow(workflowPath)
	if err != nil {
		return fmt.Errorf("failed to parse workflow: %v", err)
	}

	// Check if workflow should run for this event
	if !r.shouldRunForEvent(workflow, r.event) {
		return fmt.Errorf("workflow does not run for event: %s", r.event)
	}

	// Create GitHub context
	githubContext := r.createGitHubContext(workflow)

	// Run jobs
	for jobName, job := range workflow.Jobs {
		if err := r.runJob(ctx, jobName, job, githubContext, matrix); err != nil {
			return fmt.Errorf("job %s failed: %v", jobName, err)
		}
	}

	return nil
}

// parseWorkflow parses a GitHub Actions workflow file
func (r *WorkflowRunner) parseWorkflow(workflowPath string) (*Workflow, error) {
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

// shouldRunForEvent checks if workflow should run for the given event
func (r *WorkflowRunner) shouldRunForEvent(workflow *Workflow, event string) bool {
	if workflow.On == nil {
		return false
	}

	switch on := workflow.On.(type) {
	case string:
		return on == event
	case []interface{}:
		for _, e := range on {
			if str, ok := e.(string); ok && str == event {
				return true
			}
		}
	case map[string]interface{}:
		_, exists := on[event]
		return exists
	}

	return false
}

// createGitHubContext creates the GitHub context for expressions
func (r *WorkflowRunner) createGitHubContext(workflow *Workflow) map[string]interface{} {
	return map[string]interface{}{
		"event_name":  r.event,
		"workflow":    workflow.Name,
		"repository":  "test/repo", // Mock values for testing
		"ref":         "refs/heads/main",
		"sha":         "abcd1234",
		"actor":       "test-user",
		"workspace":   "/github/workspace",
		"run_id":      "123456789",
		"run_number":  "1",
	}
}

// runJob executes a single job
func (r *WorkflowRunner) runJob(ctx context.Context, jobName string, job Job, githubContext map[string]interface{}, matrix map[string]interface{}) error {
	// Determine runner image
	runnerImage := r.getRunnerImage(job.RunsOn)

	// Create container for this job
	containerName := fmt.Sprintf("ghascript-job-%s", jobName)
	
	var opts []testctr.Option
	opts = append(opts, testctr.WithCommand("sleep", "3600")) // Keep container alive

	// Add environment variables
	for key, value := range r.environment {
		opts = append(opts, testctr.WithEnv(key, value))
	}

	// Add GitHub context as environment variables
	for key, value := range githubContext {
		envKey := fmt.Sprintf("GITHUB_%s", strings.ToUpper(key))
		opts = append(opts, testctr.WithEnv(envKey, fmt.Sprintf("%v", value)))
	}

	// Add matrix variables
	if matrix != nil {
		for key, value := range matrix {
			envKey := fmt.Sprintf("MATRIX_%s", strings.ToUpper(key))
			opts = append(opts, testctr.WithEnv(envKey, fmt.Sprintf("%v", value)))
		}
	}

	// Add job environment variables
	for key, value := range job.Env {
		expandedValue := r.expandExpressions(value, githubContext, matrix)
		opts = append(opts, testctr.WithEnv(key, expandedValue))
	}

	// Create container - use a minimal test instance
	testInstance := &testing.T{}
	container := testctr.New(testInstance, runnerImage, opts...)
	r.containers[containerName] = container

	// Set up workspace
	if err := r.setupWorkspace(ctx, container); err != nil {
		return fmt.Errorf("failed to setup workspace: %v", err)
	}

	// Run steps
	for i, step := range job.Steps {
		stepContext := map[string]interface{}{
			"name": step.Name,
		}
		
		if err := r.runStep(ctx, container, step, githubContext, matrix, stepContext, i); err != nil {
			return fmt.Errorf("step %d (%s) failed: %v", i+1, step.Name, err)
		}
	}

	return nil
}

// getRunnerImage returns the container image for the runner
func (r *WorkflowRunner) getRunnerImage(runsOn interface{}) string {
	var runnerOS string
	
	switch ro := runsOn.(type) {
	case string:
		runnerOS = ro
	case []interface{}:
		if len(ro) > 0 {
			if str, ok := ro[0].(string); ok {
				runnerOS = str
			}
		}
	}

	// Map GitHub runner labels to Docker images
	switch {
	case strings.Contains(runnerOS, "ubuntu"):
		return "ubuntu:22.04"
	case strings.Contains(runnerOS, "macos"):
		return "ubuntu:22.04" // Fallback - can't run actual macOS in containers
	case strings.Contains(runnerOS, "windows"):
		return "ubuntu:22.04" // Fallback - simplified for now
	default:
		return "ubuntu:22.04" // Default
	}
}

// setupWorkspace sets up the workspace in the container
func (r *WorkflowRunner) setupWorkspace(ctx context.Context, container *testctr.Container) error {
	commands := [][]string{
		{"mkdir", "-p", "/github/workspace"},
		{"mkdir", "-p", "/github/home"},
		{"apt-get", "update", "-qq"},
		{"apt-get", "install", "-y", "-qq", "git", "curl", "wget", "unzip"},
	}

	for _, cmd := range commands {
		if _, _, err := container.Exec(ctx, cmd); err != nil {
			// Don't fail on setup errors, just log them
			continue
		}
	}

	return nil
}

// runStep executes a single workflow step
func (r *WorkflowRunner) runStep(ctx context.Context, container *testctr.Container, step Step, githubContext, matrix, stepContext map[string]interface{}, stepNumber int) error {
	// Skip step if condition is false
	if step.If != "" {
		if shouldSkip, err := r.evaluateCondition(step.If, githubContext, matrix, stepContext); err != nil {
			return fmt.Errorf("failed to evaluate condition: %v", err)
		} else if shouldSkip {
			return nil // Skip this step
		}
	}

	// Set step environment variables
	stepEnv := make([]string, 0)
	for key, value := range step.Env {
		expandedValue := r.expandExpressions(value, githubContext, matrix)
		stepEnv = append(stepEnv, fmt.Sprintf("%s=%s", key, expandedValue))
	}

	if step.Uses != "" {
		return r.runActionStep(ctx, container, step, githubContext, matrix, stepEnv)
	} else if step.Run != "" {
		return r.runCommandStep(ctx, container, step, githubContext, matrix, stepEnv)
	}

	return nil // Empty step
}

// runCommandStep executes a run command step
func (r *WorkflowRunner) runCommandStep(ctx context.Context, container *testctr.Container, step Step, githubContext, matrix map[string]interface{}, stepEnv []string) error {
	// Expand expressions in the run command
	command := r.expandExpressions(step.Run, githubContext, matrix)

	// Determine shell
	shell := "bash"
	if step.Shell != "" {
		shell = step.Shell
	}

	// Create environment setup
	envSetup := strings.Join(stepEnv, "\n")
	if envSetup != "" {
		envSetup = "export " + strings.ReplaceAll(envSetup, "=", "=\"") + "\"\n"
	}

	// Create script
	script := fmt.Sprintf(`#!/bin/%s
set -e
cd /github/workspace
%s
%s`, shell, envSetup, command)

	// Write and execute script
	scriptPath := fmt.Sprintf("/tmp/step-%d.sh", time.Now().UnixNano())
	if _, _, err := container.Exec(ctx, []string{"sh", "-c", fmt.Sprintf("cat > %s << 'EOF'\n%s\nEOF", scriptPath, script)}); err != nil {
		return fmt.Errorf("failed to write script: %v", err)
	}

	if _, _, err := container.Exec(ctx, []string{"chmod", "+x", scriptPath}); err != nil {
		return fmt.Errorf("failed to make script executable: %v", err)
	}

	exitCode, output, err := container.Exec(ctx, []string{scriptPath})
	if err != nil {
		return fmt.Errorf("failed to execute script: %v", err)
	}

	if exitCode != 0 {
		return fmt.Errorf("step failed with exit code %d: %s", exitCode, output)
	}

	return nil
}

// runActionStep executes an action step (simplified implementation)
func (r *WorkflowRunner) runActionStep(ctx context.Context, container *testctr.Container, step Step, githubContext, matrix map[string]interface{}, stepEnv []string) error {
	// For now, implement common actions as shell commands
	action := step.Uses
	
	// Check if action might need buildx
	if r.actionMightNeedBuildx(action) {
		warnIfBuildxNeeded(action)
	}
	
	switch {
	case strings.HasPrefix(action, "actions/checkout"):
		return r.runCheckoutAction(ctx, container, step)
	case strings.HasPrefix(action, "actions/setup-"):
		return r.runSetupAction(ctx, container, step)
	case strings.HasPrefix(action, "docker/build-push-action"):
		return r.runDockerBuildAction(ctx, container, step)
	case strings.HasPrefix(action, "docker/setup-buildx-action"):
		return r.runSetupBuildxAction(ctx, container, step)
	default:
		// For unknown actions, just log that we're skipping them
		fmt.Printf("Skipping unsupported action: %s\n", action)
		return nil
	}
}

// runCheckoutAction implements a basic checkout action
func (r *WorkflowRunner) runCheckoutAction(ctx context.Context, container *testctr.Container, step Step) error {
	// Copy workspace content to container
	// For testing, we'll just create some dummy files
	commands := [][]string{
		{"mkdir", "-p", "/github/workspace"},
		{"sh", "-c", "echo 'Mock repository content' > /github/workspace/README.md"},
	}

	for _, cmd := range commands {
		if _, _, err := container.Exec(ctx, cmd); err != nil {
			return fmt.Errorf("checkout failed: %v", err)
		}
	}

	return nil
}

// runSetupAction implements basic setup actions
func (r *WorkflowRunner) runSetupAction(ctx context.Context, container *testctr.Container, step Step) error {
	action := step.Uses
	
	switch {
	case strings.Contains(action, "setup-node"):
		return r.setupNode(ctx, container, step)
	case strings.Contains(action, "setup-go"):
		return r.setupGo(ctx, container, step)
	case strings.Contains(action, "setup-python"):
		return r.setupPython(ctx, container, step)
	default:
		fmt.Printf("Skipping setup action: %s\n", action)
		return nil
	}
}

// setupNode installs Node.js
func (r *WorkflowRunner) setupNode(ctx context.Context, container *testctr.Container, step Step) error {
	version := "18" // default
	if step.With != nil {
		if v, ok := step.With["node-version"]; ok {
			version = fmt.Sprintf("%v", v)
		}
	}

	commands := [][]string{
		{"curl", "-fsSL", "https://deb.nodesource.com/setup_" + version + ".x", "-o", "/tmp/nodesource_setup.sh"},
		{"bash", "/tmp/nodesource_setup.sh"},
		{"apt-get", "install", "-y", "nodejs"},
	}

	for _, cmd := range commands {
		if _, _, err := container.Exec(ctx, cmd); err != nil {
			return fmt.Errorf("node setup failed: %v", err)
		}
	}

	return nil
}

// setupGo installs Go
func (r *WorkflowRunner) setupGo(ctx context.Context, container *testctr.Container, step Step) error {
	version := "1.21" // default
	if step.With != nil {
		if v, ok := step.With["go-version"]; ok {
			version = fmt.Sprintf("%v", v)
		}
	}

	goURL := fmt.Sprintf("https://golang.org/dl/go%s.linux-amd64.tar.gz", version)
	commands := [][]string{
		{"wget", "-q", goURL, "-O", "/tmp/go.tar.gz"},
		{"tar", "-C", "/usr/local", "-xzf", "/tmp/go.tar.gz"},
		{"sh", "-c", "echo 'export PATH=$PATH:/usr/local/go/bin' >> /etc/environment"},
	}

	for _, cmd := range commands {
		if _, _, err := container.Exec(ctx, cmd); err != nil {
			return fmt.Errorf("go setup failed: %v", err)
		}
	}

	return nil
}

// setupPython installs Python
func (r *WorkflowRunner) setupPython(ctx context.Context, container *testctr.Container, step Step) error {
	commands := [][]string{
		{"apt-get", "install", "-y", "python3", "python3-pip"},
		{"ln", "-sf", "/usr/bin/python3", "/usr/bin/python"},
	}

	for _, cmd := range commands {
		if _, _, err := container.Exec(ctx, cmd); err != nil {
			return fmt.Errorf("python setup failed: %v", err)
		}
	}

	return nil
}

// expandExpressions expands GitHub Actions expressions like ${{ github.ref }}
func (r *WorkflowRunner) expandExpressions(text string, githubContext, matrix map[string]interface{}) string {
	// Simple expression expansion - handles basic cases
	re := regexp.MustCompile(`\$\{\{\s*([^}]+)\s*\}\}`)
	
	return re.ReplaceAllStringFunc(text, func(match string) string {
		// Extract expression
		expr := re.FindStringSubmatch(match)[1]
		expr = strings.TrimSpace(expr)
		
		// Handle github.* expressions
		if strings.HasPrefix(expr, "github.") {
			key := strings.TrimPrefix(expr, "github.")
			if value, ok := githubContext[key]; ok {
				return fmt.Sprintf("%v", value)
			}
		}
		
		// Handle matrix.* expressions
		if strings.HasPrefix(expr, "matrix.") && matrix != nil {
			key := strings.TrimPrefix(expr, "matrix.")
			if value, ok := matrix[key]; ok {
				return fmt.Sprintf("%v", value)
			}
		}
		
		// Handle env.* expressions
		if strings.HasPrefix(expr, "env.") {
			key := strings.TrimPrefix(expr, "env.")
			if value := os.Getenv(key); value != "" {
				return value
			}
		}
		
		// Return original if can't expand
		return match
	})
}

// evaluateCondition evaluates a step condition
func (r *WorkflowRunner) evaluateCondition(condition string, githubContext, matrix, stepContext map[string]interface{}) (bool, error) {
	// Expand expressions first
	expanded := r.expandExpressions(condition, githubContext, matrix)
	
	// Simple condition evaluation
	switch strings.ToLower(strings.TrimSpace(expanded)) {
	case "true", "success()":
		return false, nil // Don't skip
	case "false", "failure()":
		return true, nil // Skip
	default:
		// For complex conditions, default to not skipping
		return false, nil
	}
}

// actionMightNeedBuildx checks if an action might require Docker buildx
func (r *WorkflowRunner) actionMightNeedBuildx(action string) bool {
	buildxActions := []string{
		"docker/build-push-action",
		"docker/setup-buildx-action",
		"docker/bake-action",
		// Add more buildx-dependent actions as needed
	}
	
	for _, buildxAction := range buildxActions {
		if strings.HasPrefix(action, buildxAction) {
			return true
		}
	}
	
	return false
}

// runDockerBuildAction implements docker/build-push-action
func (r *WorkflowRunner) runDockerBuildAction(ctx context.Context, container *testctr.Container, step Step) error {
	fmt.Printf("Running docker/build-push-action (simplified implementation)\n")
	
	// This is a simplified implementation
	// In a real scenario, we'd parse the 'with' parameters and execute docker build
	if step.With != nil {
		if context, ok := step.With["context"]; ok {
			fmt.Printf("  Build context: %v\n", context)
		}
		if dockerfile, ok := step.With["file"]; ok {
			fmt.Printf("  Dockerfile: %v\n", dockerfile)
		}
		if platforms, ok := step.With["platforms"]; ok {
			fmt.Printf("  Platforms: %v (requires buildx)\n", platforms)
		}
	}
	
	return nil
}

// runSetupBuildxAction implements docker/setup-buildx-action
func (r *WorkflowRunner) runSetupBuildxAction(ctx context.Context, container *testctr.Container, step Step) error {
	fmt.Printf("Running docker/setup-buildx-action\n")
	
	// Check if buildx is available on the host
	checkDockerBuildx()
	
	if !buildxAvailable {
		fmt.Printf("  WARNING: Buildx setup requested but buildx is not available on host\n")
		return fmt.Errorf("buildx setup failed: buildx not available")
	}
	
	fmt.Printf("  Buildx is available and configured\n")
	return nil
}

