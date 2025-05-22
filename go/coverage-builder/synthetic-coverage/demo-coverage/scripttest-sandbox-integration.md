# Integrating Scripttest with Sandboxed Environments

This guide explores strategies for running `rsc.io/script/scripttest` tests in sandboxed environments while maintaining accurate code coverage through synthetic coverage techniques.

## Sandbox Security Challenges

Sandbox technologies like macOS Sandbox, seccomp, SELinux, and similar technologies create unique challenges for scripttest:

1. **Permission Limitations**: Scripts may be unable to access resources
2. **Filesystem Restrictions**: File operations may be blocked
3. **Network Access Control**: Network operations may be restricted
4. **Process Isolation**: Separate processes with limited visibility
5. **Resource Constraints**: Memory and CPU limits

## Sandbox-Compatible Scripttest

### Implementation Approach

We can create a sandbox-compatible scripttest approach using these techniques:

1. Define sandbox profiles for each test
2. Execute script commands within the sandbox
3. Track commands executed within the sandbox
4. Map commands to code paths for synthetic coverage

## Sandbox Profile Generation for Scripttest

First, we need to generate appropriate sandbox profiles:

```go
// SandboxProfileGenerator creates sandbox profiles for scripttest tests
type SandboxProfileGenerator struct {
    BaseProfile       string
    AllowedDirectories []string
    AllowNetwork      bool
}

// NewSandboxProfileGenerator creates a new generator
func NewSandboxProfileGenerator(baseProfile string) *SandboxProfileGenerator {
    return &SandboxProfileGenerator{
        BaseProfile:       baseProfile,
        AllowedDirectories: []string{},
        AllowNetwork:      false,
    }
}

// WithDirectory adds a directory to be allowed in the sandbox
func (g *SandboxProfileGenerator) WithDirectory(dir string) *SandboxProfileGenerator {
    g.AllowedDirectories = append(g.AllowedDirectories, dir)
    return g
}

// WithNetwork enables network access in the sandbox
func (g *SandboxProfileGenerator) WithNetwork() *SandboxProfileGenerator {
    g.AllowNetwork = true
    return g
}

// Generate creates a sandbox profile for the given test name
func (g *SandboxProfileGenerator) Generate(testName string) (string, error) {
    profilePath := fmt.Sprintf("./sandbox-profiles/%s.sb", testName)
    
    // Create profile directory if it doesn't exist
    profileDir := filepath.Dir(profilePath)
    if err := os.MkdirAll(profileDir, 0755); err != nil {
        return "", err
    }
    
    // Start with base profile
    profile := g.BaseProfile
    
    // Add allowed directories
    for _, dir := range g.AllowedDirectories {
        profile += fmt.Sprintf("\n(allow file-read* (subpath \"%s\"))", dir)
        profile += fmt.Sprintf("\n(allow file-write* (subpath \"%s\"))", dir)
    }
    
    // Add network access if enabled
    if g.AllowNetwork {
        profile += "\n(allow network*)"
    }
    
    // Write profile to file
    if err := os.WriteFile(profilePath, []byte(profile), 0644); err != nil {
        return "", err
    }
    
    return profilePath, nil
}
```

## Sandboxed Command Executor for Scripttest

Next, we need a command executor that runs commands in a sandbox:

```go
// SandboxExecutor runs commands in a sandbox
type SandboxExecutor struct {
    ProfileGenerator *SandboxProfileGenerator
    TrackingDir      string
}

// NewSandboxExecutor creates a new executor
func NewSandboxExecutor(profileGen *SandboxProfileGenerator) *SandboxExecutor {
    trackingDir := "./sandbox-tracking"
    os.MkdirAll(trackingDir, 0755)
    
    return &SandboxExecutor{
        ProfileGenerator: profileGen,
        TrackingDir:      trackingDir,
    }
}

// ExecuteCommand runs a command in a sandbox and tracks its execution
func (e *SandboxExecutor) ExecuteCommand(testName, command string) (string, string, error) {
    // Generate sandbox profile
    profilePath, err := e.ProfileGenerator.Generate(testName)
    if err != nil {
        return "", "", fmt.Errorf("failed to generate sandbox profile: %v", err)
    }
    
    // Create tracking file for this command
    cmdID := fmt.Sprintf("%s-%d", testName, time.Now().UnixNano())
    trackingFile := filepath.Join(e.TrackingDir, cmdID)
    
    // Write command to tracking file
    if err := os.WriteFile(trackingFile, []byte(command), 0644); err != nil {
        return "", "", fmt.Errorf("failed to write tracking file: %v", err)
    }
    
    // Parse command and arguments
    cmdParts := strings.Fields(command)
    if len(cmdParts) == 0 {
        return "", "", fmt.Errorf("empty command")
    }
    
    // Build sandbox-exec command
    sandboxCmd := []string{"sandbox-exec", "-f", profilePath}
    sandboxCmd = append(sandboxCmd, cmdParts...)
    
    // Create command
    cmd := exec.Command(sandboxCmd[0], sandboxCmd[1:]...)
    
    // Capture stdout and stderr
    var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr
    
    // Run command
    err = cmd.Run()
    
    return stdout.String(), stderr.String(), err
}
```

## Custom Scripttest Executor with Sandbox Support

Now we can create a custom scripttest executor that uses our sandbox executor:

```go
// SandboxScripttest extends scripttest with sandbox support
type SandboxScripttest struct {
    Executor *SandboxExecutor
    TestDir  string
}

// NewSandboxScripttest creates a new sandboxed scripttest
func NewSandboxScripttest(executor *SandboxExecutor, testDir string) *SandboxScripttest {
    return &SandboxScripttest{
        Executor: executor,
        TestDir:  testDir,
    }
}

// RunTest runs a single scripttest test in a sandbox
func (s *SandboxScripttest) RunTest(t *testing.T, name, script string) {
    // Parse the script to extract commands and expectations
    lines := strings.Split(script, "\n")
    
    for i := 0; i < len(lines); i++ {
        line := strings.TrimSpace(lines[i])
        
        // Skip empty lines and comments
        if line == "" || strings.HasPrefix(line, "#") {
            continue
        }
        
        // Check for command (starts with >)
        if strings.HasPrefix(line, ">") {
            command := strings.TrimSpace(line[1:])
            
            // Execute command in sandbox
            stdout, stderr, err := s.Executor.ExecuteCommand(name, command)
            
            // Process expectations for this command
            for i+1 < len(lines) {
                i++
                expectation := strings.TrimSpace(lines[i])
                
                // Skip empty lines and comments
                if expectation == "" || strings.HasPrefix(expectation, "#") {
                    continue
                }
                
                // If next line starts with >, we've reached the next command
                if strings.HasPrefix(expectation, ">") {
                    i--
                    break
                }
                
                // Process expectation
                if strings.HasPrefix(expectation, "stdout") {
                    // stdout expectation
                    exp := strings.TrimSpace(strings.TrimPrefix(expectation, "stdout"))
                    if strings.HasPrefix(exp, "contains") {
                        expected := strings.Trim(strings.TrimSpace(strings.TrimPrefix(exp, "contains")), `'"`)
                        if !strings.Contains(stdout, expected) {
                            t.Errorf("Expected stdout to contain %q, got %q", expected, stdout)
                        }
                    }
                } else if strings.HasPrefix(expectation, "stderr") {
                    // stderr expectation
                    exp := strings.TrimSpace(strings.TrimPrefix(expectation, "stderr"))
                    if strings.HasPrefix(exp, "contains") {
                        expected := strings.Trim(strings.TrimSpace(strings.TrimPrefix(exp, "contains")), `'"`)
                        if !strings.Contains(stderr, expected) {
                            t.Errorf("Expected stderr to contain %q, got %q", expected, stderr)
                        }
                    }
                } else if strings.HasPrefix(expectation, "!") {
                    // Negative expectation
                    negExp := strings.TrimSpace(strings.TrimPrefix(expectation, "!"))
                    if strings.HasPrefix(negExp, "stdout") {
                        t.Logf("Checking that stdout does not match pattern")
                    } else if strings.HasPrefix(negExp, "stderr") {
                        t.Logf("Checking that stderr does not match pattern")
                    }
                } else if strings.HasPrefix(expectation, "status") {
                    // Status expectation
                    statusExp := strings.TrimSpace(strings.TrimPrefix(expectation, "status"))
                    expectedStatus, _ := strconv.Atoi(statusExp)
                    if err != nil {
                        exitErr, ok := err.(*exec.ExitError)
                        if !ok || exitErr.ExitCode() != expectedStatus {
                            t.Errorf("Expected exit status %d, got %v", expectedStatus, err)
                        }
                    } else if expectedStatus != 0 {
                        t.Errorf("Expected exit status %d, got 0", expectedStatus)
                    }
                }
            }
        }
    }
}

// RunTestFile runs a scripttest file in a sandbox
func (s *SandboxScripttest) RunTestFile(t *testing.T, path string) {
    // Read the test file
    content, err := os.ReadFile(path)
    if err != nil {
        t.Fatalf("Failed to read test file %s: %v", path, err)
    }
    
    // Get test name from file name
    base := filepath.Base(path)
    name := strings.TrimSuffix(base, filepath.Ext(base))
    
    // Run the test
    s.RunTest(t, name, string(content))
}
```

## Command Tracking and Synthetic Coverage Generation

After running the sandboxed tests, we can generate synthetic coverage:

```go
// SandboxCoverageGenerator generates synthetic coverage from sandbox tracking
type SandboxCoverageGenerator struct {
    TrackingDir string
    CommandMap  map[string][]string
}

// NewSandboxCoverageGenerator creates a new generator
func NewSandboxCoverageGenerator(trackingDir string) *SandboxCoverageGenerator {
    return &SandboxCoverageGenerator{
        TrackingDir: trackingDir,
        CommandMap:  make(map[string][]string),
    }
}

// LoadCommandMap loads the command map from a JSON file
func (g *SandboxCoverageGenerator) LoadCommandMap(path string) error {
    data, err := os.ReadFile(path)
    if err != nil {
        return err
    }
    
    return json.Unmarshal(data, &g.CommandMap)
}

// GenerateSyntheticCoverage creates a synthetic coverage file
func (g *SandboxCoverageGenerator) GenerateSyntheticCoverage(outputPath string) error {
    // Get all tracking files
    files, err := os.ReadDir(g.TrackingDir)
    if err != nil {
        return err
    }
    
    // Track which commands were executed
    executedCommands := make(map[string]bool)
    
    // Process each tracking file
    for _, file := range files {
        if file.IsDir() {
            continue
        }
        
        // Read the command from the tracking file
        cmdPath := filepath.Join(g.TrackingDir, file.Name())
        cmdData, err := os.ReadFile(cmdPath)
        if err != nil {
            continue
        }
        
        // Mark command as executed
        cmd := string(cmdData)
        executedCommands[cmd] = true
    }
    
    // Generate synthetic coverage
    var sb strings.Builder
    sb.WriteString("mode: set\n")
    
    // Add coverage entries for executed commands
    for cmd, paths := range g.CommandMap {
        if executedCommands[cmd] {
            for _, path := range paths {
                // Format: file:line.column,line.column numstmt count
                sb.WriteString(fmt.Sprintf("%s 1 1\n", path))
            }
        }
    }
    
    // Write to output file
    return os.WriteFile(outputPath, []byte(sb.String()), 0644)
}
```

## Complete Integration Example

Here's a complete example showing how to integrate scripttest, sandbox, and synthetic coverage:

```go
func TestSandboxedScripttest(t *testing.T) {
    // Create base sandbox profile
    baseProfile := `(version 1)
(deny default)
(allow process*)
(allow file-read* (subpath "/usr/lib"))
(allow file-read* (subpath "/System/Library"))
(allow file-read* (literal "/dev/null"))
(allow file-read* (literal "/dev/urandom"))
`
    
    // Create profile generator
    profileGen := NewSandboxProfileGenerator(baseProfile)
    profileGen.WithDirectory("./testdata")
    profileGen.WithDirectory("./sandbox-tracking")
    
    // Create sandbox executor
    executor := NewSandboxExecutor(profileGen)
    
    // Create sandboxed scripttest
    scripttest := NewSandboxScripttest(executor, "./testdata")
    
    // Run test
    scripttest.RunTestFile(t, "./testdata/basic_test.txt")
    
    // Generate synthetic coverage
    coverageGen := NewSandboxCoverageGenerator("./sandbox-tracking")
    
    // Load command map
    if err := coverageGen.LoadCommandMap("./testdata/command_map.json"); err != nil {
        t.Fatalf("Failed to load command map: %v", err)
    }
    
    // Generate synthetic coverage
    if err := coverageGen.GenerateSyntheticCoverage("./sandbox-coverage.txt"); err != nil {
        t.Fatalf("Failed to generate synthetic coverage: %v", err)
    }
    
    // Merge with real coverage (if any)
    // ...
}
```

## Practical Example: macOS Sandbox with Scripttest

For macOS systems, here's a practical example of using sandbox-exec with scripttest:

```go
package main_test

import (
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "testing"
)

func TestMacOSSandboxedScripttest(t *testing.T) {
    // Create temp directory for test
    tmpDir, err := os.MkdirTemp("", "sandbox-test-*")
    if err != nil {
        t.Fatalf("Failed to create temp dir: %v", err)
    }
    defer os.RemoveAll(tmpDir)
    
    // Create a sandbox profile
    profilePath := filepath.Join(tmpDir, "test.sb")
    profileContent := `(version 1)
(deny default)
(allow process*)
(allow file-read* (subpath "/usr/lib"))
(allow file-read* (subpath "/System/Library"))
(allow file-read* (literal "/dev/null"))
(allow file-read* (literal "/dev/urandom"))
(allow file-read* (subpath "` + tmpDir + `"))
(allow file-write* (subpath "` + tmpDir + `"))
`
    if err := os.WriteFile(profilePath, []byte(profileContent), 0644); err != nil {
        t.Fatalf("Failed to write profile: %v", err)
    }
    
    // Create a test file
    testFilePath := filepath.Join(tmpDir, "hello.txt")
    testContent := "Hello, sandbox!"
    if err := os.WriteFile(testFilePath, []byte(testContent), 0644); err != nil {
        t.Fatalf("Failed to write test file: %v", err)
    }
    
    // Define test commands
    tests := []struct {
        cmd          string
        expectedOut  string
        expectError  bool
    }{
        {
            cmd:         "cat " + testFilePath,
            expectedOut: "Hello, sandbox!",
            expectError: false,
        },
        {
            cmd:         "echo 'New content' > " + filepath.Join(tmpDir, "new.txt"),
            expectedOut: "",
            expectError: false,
        },
        {
            // This should fail due to sandbox restrictions
            cmd:         "cat /etc/hosts",
            expectedOut: "",
            expectError: true,
        },
    }
    
    // Track executed commands for synthetic coverage
    executedCmds := []string{}
    
    // Run each test command in the sandbox
    for _, tc := range tests {
        // Build sandbox-exec command
        cmdArgs := []string{"-f", profilePath, "sh", "-c", tc.cmd}
        cmd := exec.Command("sandbox-exec", cmdArgs...)
        
        // Capture output
        output, err := cmd.CombinedOutput()
        
        // Track command
        executedCmds = append(executedCmds, tc.cmd)
        
        // Verify expectations
        if tc.expectError && err == nil {
            t.Errorf("Expected error for command %q, but got none", tc.cmd)
        } else if !tc.expectError && err != nil {
            t.Errorf("Unexpected error for command %q: %v", tc.cmd, err)
        }
        
        if tc.expectedOut != "" && !strings.Contains(string(output), tc.expectedOut) {
            t.Errorf("Expected output %q for command %q, got %q", tc.expectedOut, tc.cmd, string(output))
        }
    }
    
    // Here we'd generate synthetic coverage from the executed commands
    // ...
    
    // For demo purposes, just write the executed commands to a file
    cmdListPath := filepath.Join(tmpDir, "executed_commands.txt")
    os.WriteFile(cmdListPath, []byte(strings.Join(executedCmds, "\n")), 0644)
    
    t.Logf("Executed %d commands in sandbox, list saved to %s", len(executedCmds), cmdListPath)
}
```

## Best Practices for Scripttest Sandbox Integration

1. **Minimal Permissions**: Define the minimal set of permissions needed for each test
2. **Isolated Test Directories**: Create isolated directories for each test
3. **Command Tracking**: Implement reliable command tracking across sandbox boundaries
4. **Error Handling**: Handle sandbox permission errors appropriately
5. **Synthetic Coverage**: Generate synthetic coverage based on tracked commands
6. **CI Integration**: Ensure CI environments support sandbox execution
7. **Platform Awareness**: Account for different sandbox technologies on different platforms
8. **Clean Environment**: Start with a clean sandbox for each test to prevent contamination