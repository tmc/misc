# Integrating Scripttest, Docker, and Synthetic Coverage

This guide shows how to integrate `rsc.io/script/scripttest` with Docker to test CLI applications while maintaining accurate code coverage through synthetic coverage techniques.

## The Challenge

Scripttest tests run commands in separate processes, and when combined with Docker, we have two layers of isolation:

1. **Process isolation**: The script commands run in separate processes
2. **Container isolation**: Those processes run inside containers

This makes traditional code coverage ineffective as it can't cross these boundaries.

## Solution Architecture

We'll address this by:

1. Using scripttest to define tests that execute inside Docker
2. Capturing command-to-code mappings for synthetic coverage
3. Implementing a custom test runner that maps Docker executions to code paths
4. Generating synthetic coverage based on executed commands

## Implementation Example

### Step 1: Define a Scripttest Test That Uses Docker

```go
package main_test

import (
	"testing"

	"rsc.io/script/scripttest"
)

func TestDockerizedCommands(t *testing.T) {
	// Create a new test script
	p := scripttest.NewParagraphs("docker-test")
	defer p.Cleanup()

	// Define test that runs our CLI in a Docker container
	p.Add("docker-basic", `
# Run the basic command in Docker
>docker run --rm my-cli-image:latest command1 arg1 arg2
stdout contains 'success'
! stderr .
>docker run --rm my-cli-image:latest command2 arg1
stdout contains 'processed'
! stderr .
	`)

	// Run the scripttest tests
	if err := p.Run(t, ""); err != nil {
		t.Fatal(err)
	}
}
```

### Step 2: Create a Command Mapper That Understands Docker Executions

```go
// DockerCommandMapper analyzes scripttest tests to map Docker commands
// to actual code paths in your application
type DockerCommandMapper struct {
    DockerImageTagMap map[string]string  // Maps image tags to code repos
    CommandMap        map[string][]string // Maps commands to code paths
}

// NewDockerCommandMapper creates a new mapper
func NewDockerCommandMapper() *DockerCommandMapper {
    return &DockerCommandMapper{
        DockerImageTagMap: make(map[string]string),
        CommandMap:        make(map[string][]string),
    }
}

// AnalyzeScripttestFile processes a scripttest file to extract Docker commands
func (m *DockerCommandMapper) AnalyzeScripttestFile(filePath string) error {
    // Read the scripttest file
    content, err := os.ReadFile(filePath)
    if err != nil {
        return err
    }

    // Parse commands using regex
    dockerCmdRegex := regexp.MustCompile(`(?m)^>docker run --rm ([a-zA-Z0-9:.-]+) (.+)$`)
    matches := dockerCmdRegex.FindAllStringSubmatch(string(content), -1)

    // Process each command
    for _, match := range matches {
        if len(match) < 3 {
            continue
        }
        
        imageTag := match[1]
        command := match[2]
        
        // Get repo path for this image
        repoPath, ok := m.DockerImageTagMap[imageTag]
        if !ok {
            // If not defined, we can attempt to infer it or skip
            continue
        }
        
        // Use the command mapper to find code paths for this command
        codePathMapper := NewCommandPathMapper(repoPath)
        paths, err := codePathMapper.GetPathsForCommand(command)
        if err != nil {
            // Log warning but continue
            fmt.Printf("Warning: Could not map command %s: %v\n", command, err)
            continue
        }
        
        // Store the mapping
        m.CommandMap[command] = paths
    }
    
    return nil
}

// GenerateSyntheticCoverage creates synthetic coverage from the command map
func (m *DockerCommandMapper) GenerateSyntheticCoverage(outputPath string) error {
    var syntheticCoverage strings.Builder
    
    // Add synthetic coverage header
    syntheticCoverage.WriteString("mode: set\n")
    
    // Add an entry for each mapped code path
    for _, paths := range m.CommandMap {
        for _, path := range paths {
            // Format: file:line.column,line.column numstmt count
            syntheticCoverage.WriteString(fmt.Sprintf("%s 1 1\n", path))
        }
    }
    
    // Write to file
    return os.WriteFile(outputPath, []byte(syntheticCoverage.String()), 0644)
}
```

### Step 3: Create a Test Execution Tracker to Record Executed Docker Commands

```go
// DockerTestExecutionTracker tracks Docker commands executed during tests
type DockerTestExecutionTracker struct {
    ExecutedCommands map[string]bool
    OutputFile       string
}

// NewDockerTestExecutionTracker creates a new tracker
func NewDockerTestExecutionTracker(outputFile string) *DockerTestExecutionTracker {
    return &DockerTestExecutionTracker{
        ExecutedCommands: make(map[string]bool),
        OutputFile:       outputFile,
    }
}

// StartTracking begins tracking Docker commands
func (t *DockerTestExecutionTracker) StartTracking() error {
    // Create a temporary directory to store the executed commands
    tmpDir, err := os.MkdirTemp("", "docker-tracker-*")
    if err != nil {
        return err
    }
    
    // Set environment variable to tell our hooks where to store command info
    os.Setenv("DOCKER_COMMAND_TRACKER_DIR", tmpDir)
    
    return nil
}

// StopTracking stops tracking and processes the results
func (t *DockerTestExecutionTracker) StopTracking() error {
    // Get tracker directory from environment
    trackerDir := os.Getenv("DOCKER_COMMAND_TRACKER_DIR")
    if trackerDir == "" {
        return fmt.Errorf("tracker directory not set")
    }
    
    // Clean up the environment variable
    os.Unsetenv("DOCKER_COMMAND_TRACKER_DIR")
    
    // Read all command files from the tracker directory
    files, err := os.ReadDir(trackerDir)
    if err != nil {
        return err
    }
    
    // Process each command file
    for _, file := range files {
        cmdFile := filepath.Join(trackerDir, file.Name())
        
        // Read the command
        cmd, err := os.ReadFile(cmdFile)
        if err != nil {
            continue
        }
        
        // Record the command as executed
        t.ExecutedCommands[string(cmd)] = true
    }
    
    // Clean up the temporary directory
    os.RemoveAll(trackerDir)
    
    return nil
}

// GenerateSyntheticCoverage generates synthetic coverage for executed commands
func (t *DockerTestExecutionTracker) GenerateSyntheticCoverage(mapper *DockerCommandMapper) error {
    var syntheticCoverage strings.Builder
    
    // Add synthetic coverage header
    syntheticCoverage.WriteString("mode: set\n")
    
    // Add an entry for each executed command's code path
    for cmd := range t.ExecutedCommands {
        paths, ok := mapper.CommandMap[cmd]
        if !ok {
            // Command not mapped, skip
            continue
        }
        
        for _, path := range paths {
            // Format: file:line.column,line.column numstmt count
            syntheticCoverage.WriteString(fmt.Sprintf("%s 1 1\n", path))
        }
    }
    
    // Write to file
    return os.WriteFile(t.OutputFile, []byte(syntheticCoverage.String()), 0644)
}
```

### Step 4: Create Docker Hooks for Command Tracking

To track commands executed inside Docker containers, we need a mechanism to report what commands were executed. This can be achieved with a wrapper script inside the Docker image.

#### 1. Create a wrapper script for your CLI (inside the Docker image):

```bash
#!/bin/bash
# /usr/local/bin/cli-wrapper.sh

# Store original command
ORIGINAL_CMD="$@"

# Get tracker directory from environment
TRACKER_DIR="${DOCKER_COMMAND_TRACKER_DIR:-/tmp/docker-tracker}"

# If tracking is enabled, record the command
if [ -n "$DOCKER_COMMAND_TRACKER_DIR" ]; then
    # Ensure directory exists in the container
    mkdir -p "$TRACKER_DIR"
    
    # Generate a unique ID for this command
    CMD_ID=$(date +%s.%N)-$(echo "$ORIGINAL_CMD" | md5sum | cut -d' ' -f1)
    
    # Save the command to a file
    echo "$ORIGINAL_CMD" > "$TRACKER_DIR/$CMD_ID.cmd"
    
    # Make sure the file is accessible from outside
    chmod 666 "$TRACKER_DIR/$CMD_ID.cmd"
fi

# Execute the original command
exec /usr/local/bin/real-cli "$@"
```

#### 2. Update your Dockerfile to use the wrapper:

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o /usr/local/bin/real-cli ./cmd/cli

FROM alpine:latest
COPY --from=builder /usr/local/bin/real-cli /usr/local/bin/real-cli
COPY scripts/cli-wrapper.sh /usr/local/bin/cli-wrapper.sh
RUN chmod +x /usr/local/bin/cli-wrapper.sh
# Create a symlink so the wrapper is used instead of the real CLI
RUN ln -s /usr/local/bin/cli-wrapper.sh /usr/local/bin/my-cli
ENTRYPOINT ["/usr/local/bin/my-cli"]
```

### Step 5: Integration with Volume Mounting for Tracking

To ensure command tracking works across container boundaries, we'll mount a volume to share the tracking directory:

```go
func TestDockerCommandTrackingWithScripttest(t *testing.T) {
    // Set up command tracking
    tracker := NewDockerTestExecutionTracker("./synthetic-docker.txt")
    if err := tracker.StartTracking(); err != nil {
        t.Fatal(err)
    }
    defer tracker.StopTracking()
    
    // Get the tracker directory
    trackerDir := os.Getenv("DOCKER_COMMAND_TRACKER_DIR")
    
    // Set up a Docker volume mount flag for the tracker directory
    dockerRunFlag := fmt.Sprintf("-v %s:/tmp/docker-tracker", trackerDir)
    
    // Set up an environment variable to pass to Docker
    dockerEnvFlag := "-e DOCKER_COMMAND_TRACKER_DIR=/tmp/docker-tracker"
    
    // Create a new test script with the volume mount
    p := scripttest.NewParagraphs("docker-tracking-test")
    defer p.Cleanup()
    
    // Define test that runs our CLI in a Docker container with tracking
    p.Add("docker-with-tracking", fmt.Sprintf(`
# Run commands with tracking enabled
>docker run --rm %s %s my-cli-image:latest command1 arg1 arg2
stdout contains 'success'
! stderr .
    `, dockerRunFlag, dockerEnvFlag))
    
    // Run the tests
    if err := p.Run(t, ""); err != nil {
        t.Fatal(err)
    }
    
    // Process tracked commands and generate synthetic coverage
    mapper := NewDockerCommandMapper()
    mapper.DockerImageTagMap["my-cli-image:latest"] = "/path/to/my/repo"
    
    if err := mapper.AnalyzeScripttestFile("./testdata/docker-tracking-test"); err != nil {
        t.Fatal(err)
    }
    
    // Generate synthetic coverage
    if err := tracker.GenerateSyntheticCoverage(mapper); err != nil {
        t.Fatal(err)
    }
}
```

## Complete Integration Workflow

1. **Preparation**:
   - Build Docker images with command tracking capabilities
   - Map Docker image tags to code repositories
   - Create command mappers to link Docker commands to code paths

2. **Testing**:
   - Run scripttest tests that interact with Docker
   - Track commands executed inside Docker containers
   - Map commands to code paths

3. **Coverage Generation**:
   - Generate synthetic coverage based on tracked commands
   - Merge synthetic coverage with any real coverage
   - Generate coverage reports

## Best Practices

1. **Minimize Container Overhead**: Keep the command tracking mechanism lightweight
2. **Secure Volume Mounts**: Ensure volume mounts don't expose sensitive information
3. **Consistent Image Tags**: Use consistent tagging to ensure accurate command mapping
4. **Command Validation**: Validate that mapped commands match actual code paths
5. **CI/CD Integration**: Automate synthetic coverage generation in CI/CD pipelines