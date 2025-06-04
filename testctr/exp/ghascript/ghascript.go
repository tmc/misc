package ghascript

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
	"rsc.io/script"
)

// workflowManager manages GitHub Actions workflow executions using environment variables for state sharing
type workflowManager struct {
	t              *testing.T
	workflowStates map[string]*workflowState // local cache
	mu             sync.RWMutex
}

// workflowState tracks the state of a workflow execution
type workflowState struct {
	name       string
	lastRun    time.Time
	lastResult string // "success", "failure", "running"
	lastOutput string
}

// workflowEnvKey returns the environment variable name for a workflow state
func workflowEnvKey(name, field string) string {
	return fmt.Sprintf("GHASCRIPT_WORKFLOW_%s_%s",
		strings.ToUpper(strings.ReplaceAll(name, "-", "_")),
		strings.ToUpper(field))
}

// WorkflowCmd creates a GitHub Actions workflow command for use in script tests.
// The returned command manages GitHub Actions workflows using a built-in runner.
func WorkflowCmd(t *testing.T) script.Cmd {
	mgr := &workflowManager{
		t:              t,
		workflowStates: make(map[string]*workflowState),
	}

	return script.Command(
		script.CmdUsage{
			Summary: "manage GitHub Actions workflows",
			Args:    "run|list|events|jobs|doctor workflow-name [options...]",
			Detail: []string{
				"The workflow command runs GitHub Actions workflows locally.",
				"",
				"Subcommands:",
				"  run workflow-name [opts...]  - Run a workflow",
				"  list                        - List available workflows",
				"  events workflow-name        - List events for a workflow",
				"  jobs workflow-name event    - List jobs for a workflow and event",
				"  doctor                      - Check Docker and buildx capabilities",
			},
		},
		func(s *script.State, args ...string) (script.WaitFunc, error) {
			if len(args) < 1 {
				return nil, script.ErrUsage
			}

			switch args[0] {
			case "run":
				return mgr.runWorkflow(s, args[1:])
			case "list":
				return mgr.listWorkflows(s, args[1:])
			case "events":
				return mgr.listEvents(s, args[1:])
			case "jobs":
				return mgr.listJobs(s, args[1:])
			case "doctor":
				return mgr.checkDoctor(s, args[1:])
			default:
				return nil, fmt.Errorf("unknown subcommand: %s", args[0])
			}
		},
	)
}

func (m *workflowManager) runWorkflow(s *script.State, args []string) (script.WaitFunc, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("run requires workflow name")
	}

	workflowName := args[0]

	// Check if workflow file exists
	workflowPath := filepath.Join(s.Getwd(), ".github", "workflows", workflowName+".yml")
	if _, err := os.Stat(workflowPath); os.IsNotExist(err) {
		// Also try .yaml extension
		workflowPath = filepath.Join(s.Getwd(), ".github", "workflows", workflowName+".yaml")
		if _, err := os.Stat(workflowPath); os.IsNotExist(err) {
			return nil, fmt.Errorf("workflow file not found: %s (.yml or .yaml)", workflowName)
		}
	}

	return func(s *script.State) (stdout, stderr string, err error) {
		// Mark workflow as running
		s.Setenv(workflowEnvKey(workflowName, "status"), "running")
		s.Setenv(workflowEnvKey(workflowName, "start_time"), time.Now().Format(time.RFC3339))

		// Parse additional options
		event := "push" // default event
		runner := NewWorkflowRunner(s.Getwd(), event)

		for i := 1; i < len(args); i++ {
			switch args[i] {
			case "-e", "--event":
				if i+1 < len(args) {
					event = args[i+1]
					runner.event = event
					i++
				}
			case "--env":
				if i+1 < len(args) {
					parts := strings.SplitN(args[i+1], "=", 2)
					if len(parts) == 2 {
						runner.SetEnv(parts[0], parts[1])
					}
					i++
				}
			case "--secret":
				if i+1 < len(args) {
					parts := strings.SplitN(args[i+1], "=", 2)
					if len(parts) == 2 {
						runner.SetSecret(parts[0], parts[1])
					}
					i++
				}
			}
		}

		// Execute workflow using our native runner
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		err = runner.RunWorkflow(ctx, workflowPath, nil)

		// Update workflow state
		if err != nil {
			s.Setenv(workflowEnvKey(workflowName, "status"), "failure")
			s.Setenv(workflowEnvKey(workflowName, "output"), err.Error())
			s.Setenv(workflowEnvKey(workflowName, "end_time"), time.Now().Format(time.RFC3339))

			// Store in local cache
			m.mu.Lock()
			m.workflowStates[workflowName] = &workflowState{
				name:       workflowName,
				lastRun:    time.Now(),
				lastResult: "failure",
				lastOutput: err.Error(),
			}
			m.mu.Unlock()

			return "", err.Error(), fmt.Errorf("workflow failed: %v", err)
		}

		outputStr := fmt.Sprintf("Workflow %s completed successfully", workflowName)
		s.Setenv(workflowEnvKey(workflowName, "status"), "success")
		s.Setenv(workflowEnvKey(workflowName, "output"), outputStr)
		s.Setenv(workflowEnvKey(workflowName, "end_time"), time.Now().Format(time.RFC3339))

		// Store in local cache
		m.mu.Lock()
		m.workflowStates[workflowName] = &workflowState{
			name:       workflowName,
			lastRun:    time.Now(),
			lastResult: "success",
			lastOutput: outputStr,
		}
		m.mu.Unlock()

		s.Logf("workflow %s completed successfully\n", workflowName)
		return outputStr, "", nil
	}, nil
}

func (m *workflowManager) listWorkflows(s *script.State, args []string) (script.WaitFunc, error) {
	return func(s *script.State) (stdout, stderr string, err error) {
		workflowsDir := filepath.Join(s.Getwd(), ".github", "workflows")
		s.Logf("Listing workflows in %s\n", workflowsDir)

		if _, err := os.Stat(workflowsDir); os.IsNotExist(err) {
			return "No workflows found (.github/workflows directory does not exist)\n", "", nil
		}

		files, err := os.ReadDir(workflowsDir)
		if err != nil {
			return "", "", fmt.Errorf("failed to read workflows directory: %v", err)
		}

		var workflows []string
		for _, file := range files {
			if !file.IsDir() && (strings.HasSuffix(file.Name(), ".yml") || strings.HasSuffix(file.Name(), ".yaml")) {
				workflows = append(workflows, filepath.Join(".github", "workflows", file.Name()))
			}
		}

		if len(workflows) == 0 {
			return "No workflow files found\n", "", nil
		}

		return strings.Join(workflows, "\n") + "\n", "", nil
	}, nil
}

func (m *workflowManager) listEvents(s *script.State, args []string) (script.WaitFunc, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("events requires workflow name")
	}

	_ = args[0] // workflowName (not used in current implementation)

	return func(s *script.State) (stdout, stderr string, err error) {
		// For now, return common GitHub Actions events
		// In a real implementation, we might parse the workflow file to extract events
		events := []string{
			"push",
			"pull_request",
			"workflow_dispatch",
			"schedule",
			"release",
			"issues",
			"pull_request_review",
		}

		return strings.Join(events, "\n") + "\n", "", nil
	}, nil
}

func (m *workflowManager) listJobs(s *script.State, args []string) (script.WaitFunc, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("jobs requires workflow name and event")
	}

	workflowName := args[0]
	_ = args[1] // event (not used in current implementation)

	return func(s *script.State) (stdout, stderr string, err error) {
		// Parse workflow to list jobs
		workflowPath := filepath.Join(s.Getwd(), ".github", "workflows", workflowName+".yml")
		if _, err := os.Stat(workflowPath); os.IsNotExist(err) {
			workflowPath = filepath.Join(s.Getwd(), ".github", "workflows", workflowName+".yaml")
			if _, err := os.Stat(workflowPath); os.IsNotExist(err) {
				return "", "", fmt.Errorf("workflow file not found: %s", workflowName)
			}
		}

		// Parse workflow file
		data, err := os.ReadFile(workflowPath)
		if err != nil {
			return "", "", fmt.Errorf("failed to read workflow file: %v", err)
		}

		var workflow Workflow
		if err := yaml.Unmarshal(data, &workflow); err != nil {
			return "", "", fmt.Errorf("failed to parse workflow YAML: %v", err)
		}

		// List jobs
		var jobs []string
		for jobName := range workflow.Jobs {
			jobs = append(jobs, jobName)
		}

		if len(jobs) == 0 {
			return "No jobs found\n", "", nil
		}

		return strings.Join(jobs, "\n") + "\n", "", nil
	}, nil
}

func (m *workflowManager) checkDoctor(s *script.State, args []string) (script.WaitFunc, error) {
	return func(s *script.State) (stdout, stderr string, err error) {
		var output strings.Builder

		output.WriteString("Docker and buildx capability check:\n\n")

		// Check Docker version
		version, err := checkDockerVersion()
		if err != nil {
			output.WriteString(fmt.Sprintf("❌ Docker: %v\n", err))
		} else {
			output.WriteString(fmt.Sprintf("✅ Docker: %s\n", version))
		}

		// Check buildx
		buildxInfo := getBuildxInfo()
		if buildxInfo["available"].(bool) {
			output.WriteString("✅ Docker buildx: Available")
			if version, ok := buildxInfo["version"]; ok {
				output.WriteString(fmt.Sprintf(" (%v)", version))
			}
			output.WriteString("\n")

			if builders, ok := buildxInfo["builders"]; ok {
				output.WriteString(fmt.Sprintf("   Builders: %v\n", builders))
			}
		} else {
			output.WriteString("⚠️  Docker buildx: Not available\n")
			output.WriteString("   Install with: docker buildx install\n")
			output.WriteString("   Or update to Docker Desktop 2.4.0+ / Docker CE 19.03+\n")
		}

		// Check general Docker functionality
		output.WriteString("\nDocker functionality check:\n")
		if err := ensureDockerCapabilities(); err != nil {
			output.WriteString(fmt.Sprintf("❌ Docker capabilities: %v\n", err))
		} else {
			output.WriteString("✅ Docker capabilities: All checks passed\n")
		}

		output.WriteString("\nNote: Some GitHub Actions may require buildx for advanced features\n")
		output.WriteString("like multi-platform builds or advanced caching.\n")

		return output.String(), "", nil
	}, nil
}

// WorkflowCond returns a condition that checks if a workflow file exists
func WorkflowCond() script.Cond {
	return script.PrefixCondition("check workflow existence", func(s *script.State, suffix string) (bool, error) {
		if suffix == "" {
			return false, fmt.Errorf("workflow condition requires a workflow name")
		}

		workflowName := strings.TrimSpace(suffix)
		workflowPath := filepath.Join(s.Getwd(), ".github", "workflows", workflowName+".yml")

		if _, err := os.Stat(workflowPath); os.IsNotExist(err) {
			// Try .yaml extension
			workflowPath = filepath.Join(s.Getwd(), ".github", "workflows", workflowName+".yaml")
			if _, err := os.Stat(workflowPath); os.IsNotExist(err) {
				return false, nil
			}
		}

		return true, nil
	})
}

// WorkflowSuccessCond returns a condition that checks if a workflow's last run was successful
func WorkflowSuccessCond() script.Cond {
	return script.PrefixCondition("check workflow success", func(s *script.State, suffix string) (bool, error) {
		if suffix == "" {
			return false, fmt.Errorf("workflow-success condition requires a workflow name")
		}

		workflowName := strings.TrimSpace(suffix)
		status, _ := s.LookupEnv(workflowEnvKey(workflowName, "status"))
		return status == "success", nil
	})
}

// WorkflowFailedCond returns a condition that checks if a workflow's last run failed
func WorkflowFailedCond() script.Cond {
	return script.PrefixCondition("check workflow failure", func(s *script.State, suffix string) (bool, error) {
		if suffix == "" {
			return false, fmt.Errorf("workflow-failed condition requires a workflow name")
		}

		workflowName := strings.TrimSpace(suffix)
		status, _ := s.LookupEnv(workflowEnvKey(workflowName, "status"))
		return status == "failure", nil
	})
}

// DefaultCmds returns the default set of commands including workflow integration
func DefaultCmds(t *testing.T) map[string]script.Cmd {
	cmds := script.DefaultCmds()
	cmds["workflow"] = WorkflowCmd(t)
	// Also provide "act" alias for compatibility
	cmds["act"] = WorkflowCmd(t)
	return cmds
}

// DefaultConds returns the default set of conditions including act conditions
func DefaultConds() map[string]script.Cond {
	conds := script.DefaultConds()
	conds["workflow"] = WorkflowCond()
	conds["workflow-success"] = WorkflowSuccessCond()
	conds["workflow-failed"] = WorkflowFailedCond()
	return conds
}

// FindWorkflows discovers all workflow files in .github/workflows/ (exported for CLI use)
func FindWorkflows(dir string) ([]string, error) {
	workflowsDir := filepath.Join(dir, ".github", "workflows")

	if _, err := os.Stat(workflowsDir); os.IsNotExist(err) {
		return nil, nil // No workflows directory
	}

	files, err := os.ReadDir(workflowsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read workflows directory: %v", err)
	}

	var workflows []string
	for _, file := range files {
		if !file.IsDir() && (strings.HasSuffix(file.Name(), ".yml") || strings.HasSuffix(file.Name(), ".yaml")) {
			workflows = append(workflows, filepath.Join(workflowsDir, file.Name()))
		}
	}

	return workflows, nil
}
