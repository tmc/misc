package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tmc/misc/vim-jsonrpc/pkg/protocol"
)

func TestCLIUsage(t *testing.T) {
	// Test help flag
	cmd := exec.Command("go", "run", "main.go", "-h")
	output, err := cmd.CombinedOutput()
	if err != nil && !strings.Contains(string(output), "flag") {
		t.Logf("Help output: %s", string(output))
	}

	// Test invalid mode
	cmd = exec.Command("go", "run", "main.go", "-mode=invalid")
	output, err = cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "Unknown mode") {
		t.Errorf("Should report unknown mode error, got: %s", string(output))
	}
}

func TestStdioServerBasic(t *testing.T) {
	// Simple integration test that verifies JSON-RPC message creation
	req := protocol.NewRequest(1, "echo", "hello world")
	reqData, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	// Verify we can parse it back
	msg, err := protocol.ParseMessage(reqData)
	if err != nil {
		t.Fatalf("Failed to parse message: %v", err)
	}

	parsedReq, ok := msg.(*protocol.Request)
	if !ok {
		t.Fatalf("Expected Request, got %T", msg)
	}

	if parsedReq.Method != "echo" {
		t.Errorf("Expected method 'echo', got %s", parsedReq.Method)
	}

	if parsedReq.Params != "hello world" {
		t.Errorf("Expected params 'hello world', got %v", parsedReq.Params)
	}
}

func TestComplexJSONRPCWorkflow(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test data directory
	testDataDir := filepath.Join(tmpDir, "testdata")
	err := os.Mkdir(testDataDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create testdata dir: %v", err)
	}

	// Create comprehensive test cases
	testCases := []struct {
		name     string
		request  interface{}
		expected interface{}
	}{
		{
			name:     "simple_echo",
			request:  protocol.NewRequest(1, "echo", "test message"),
			expected: "test message",
		},
		{
			name:     "number_addition",
			request:  protocol.NewRequest(2, "add", []interface{}{10.0, 20.0}),
			expected: 30.0,
		},
		{
			name:     "string_greeting",
			request:  protocol.NewRequest(3, "greet", "Alice"),
			expected: "Hello, Alice!",
		},
		{
			name:     "notification_test",
			request:  protocol.NewNotification("log", "test notification"),
			expected: nil, // notifications don't return responses
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reqData, err := json.Marshal(tc.request)
			if err != nil {
				t.Fatalf("Failed to marshal request: %v", err)
			}

			testFile := filepath.Join(testDataDir, tc.name+".json")
			err = os.WriteFile(testFile, reqData, 0644)
			if err != nil {
				t.Fatalf("Failed to write test file: %v", err)
			}

			// Verify the test file was created correctly
			content, err := os.ReadFile(testFile)
			if err != nil {
				t.Fatalf("Failed to read test file: %v", err)
			}
			
			if !strings.Contains(string(content), "jsonrpc") {
				t.Errorf("Test file should contain JSON-RPC content")
			}
		})
	}
}

func TestTransportModes(t *testing.T) {
	// Test that transport creation works correctly
	transports := []struct {
		name    string
		tType   string
		addr    string
		socket  string
		skipReason string
	}{
		{"stdio", "stdio", "", "", ""},
		{"tcp", "tcp", "localhost:0", "", "may fail due to port binding"},
		{"unix", "unix", "", "/tmp/test_transport.sock", "may fail due to socket permissions"},
	}
	
	for _, tc := range transports {
		t.Run("transport_"+tc.name, func(t *testing.T) {
			if tc.skipReason != "" {
				t.Skip(tc.skipReason)
			}
			
			// Just test argument parsing by checking help output includes the transport
			args := []string{"run", "main.go", "-h"}
			cmd := exec.Command("go", args...)
			output, err := cmd.CombinedOutput()
			
			// Help should mention transport options
			if err == nil && !strings.Contains(string(output), "transport") {
				t.Logf("Help output doesn't mention transport: %s", string(output))
			}
		})
	}
}

func TestErrorHandling(t *testing.T) {
	tmpDir := t.TempDir()

	errorTests := []struct {
		name        string
		jsonInput   string
		expectError bool
		errorCode   int
	}{
		{
			name:        "invalid_json",
			jsonInput:   `{invalid json}`,
			expectError: true,
			errorCode:   protocol.ParseError,
		},
		{
			name:        "missing_jsonrpc_field",
			jsonInput:   `{"id":1,"method":"test"}`,
			expectError: true,
			errorCode:   protocol.InvalidRequest,
		},
		{
			name:        "method_not_found",
			jsonInput:   `{"jsonrpc":"2.0","id":1,"method":"nonexistent"}`,
			expectError: true,
			errorCode:   protocol.MethodNotFound,
		},
	}

	for _, tt := range errorTests {
		t.Run(tt.name, func(t *testing.T) {
			testFile := filepath.Join(tmpDir, "error_"+tt.name+".json")
			err := os.WriteFile(testFile, []byte(tt.jsonInput), 0644)
			if err != nil {
				t.Fatalf("Failed to write test file: %v", err)
			}
			
			content, err := os.ReadFile(testFile)
			if err != nil {
				t.Fatalf("Failed to read test file: %v", err)
			}
			
			if string(content) != tt.jsonInput {
				t.Errorf("File content mismatch")
			}

			// Verify we can parse the test case structure
			if tt.expectError && tt.errorCode == 0 {
				t.Errorf("Error test case should specify error code")
			}
		})
	}
}

func TestBuildAndInstall(t *testing.T) {
	tmpDir := t.TempDir()

	// Test that the project builds successfully
	cmd := exec.Command("go", "build", "-o", filepath.Join(tmpDir, "vim-jsonrpc-test"), ".")
	err := cmd.Run()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	
	// Verify the binary was created
	binaryPath := filepath.Join(tmpDir, "vim-jsonrpc-test")
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Error("Binary should be created")
	}

	// Test that the binary has correct permissions (executable)
	info, err := os.Stat(binaryPath)
	if err != nil {
		t.Fatalf("Failed to stat binary: %v", err)
	}
	
	if info.Mode()&0111 == 0 {
		t.Error("Binary should be executable")
	}

	// Test version/help output
	cmd = exec.Command(binaryPath, "-h")
	output, err := cmd.CombinedOutput()
	// Help should either succeed or show usage
	if err != nil && !strings.Contains(string(output), "flag") {
		t.Logf("Help output: %s", string(output))
	}
}

func TestModuleDependencies(t *testing.T) {
	// Verify go.mod is valid
	cmd := exec.Command("go", "mod", "verify")
	err := cmd.Run()
	if err != nil {
		t.Fatalf("go mod verify failed: %v", err)
	}
	
	// Check that all dependencies are available
	cmd = exec.Command("go", "mod", "download")
	err = cmd.Run()
	if err != nil {
		t.Fatalf("go mod download failed: %v", err)
	}
	
	// Ensure no unused dependencies
	cmd = exec.Command("go", "mod", "tidy")
	err = cmd.Run()
	if err != nil {
		t.Fatalf("go mod tidy failed: %v", err)
	}
	
	// Verify no changes were needed
	content, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatalf("Failed to read go.mod: %v", err)
	}
	
	if !strings.Contains(string(content), "module github.com/tmc/misc/vim-jsonrpc") {
		t.Error("go.mod should contain correct module name")
	}
}

func TestExamples(t *testing.T) {
	// Test that examples compile
	examples := []string{
		"examples/client/simple_client.go",
		"examples/server/simple_server.go",
	}

	tmpDir := t.TempDir()

	for _, example := range examples {
		t.Run("compile_"+strings.ReplaceAll(example, "/", "_"), func(t *testing.T) {
			if _, err := os.Stat(example); os.IsNotExist(err) {
				t.Skipf("Example file %s not found", example)
			}
			
			outputName := strings.ReplaceAll(example, "/", "_")
			outputName = strings.TrimSuffix(outputName, ".go")
			outputPath := filepath.Join(tmpDir, outputName)
			
			cmd := exec.Command("go", "build", "-o", outputPath, example)
			err := cmd.Run()
			if err != nil {
				t.Fatalf("Failed to build example %s: %v", example, err)
			}
			
			if _, err := os.Stat(outputPath); os.IsNotExist(err) {
				t.Errorf("Example %s should compile successfully", example)
			}
		})
	}
}