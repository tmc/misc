# Sandboxing Coverage Strategies for Go

This guide explores techniques for generating accurate code coverage when using sandbox technologies to isolate and secure code execution in Go applications.

## Sandboxing Challenges for Coverage

Sandbox environments impose several unique challenges for code coverage:

1. **Permission Restrictions**: Sandboxes often restrict file access, making it difficult to write coverage data
2. **Syscall Filtering**: Some operations needed for coverage may be blocked by syscall filters
3. **Process Isolation**: Coverage tools may not be able to track across process boundaries
4. **Resource Limitations**: Sandboxes may have resource constraints affecting coverage collection
5. **Signal Handling**: Coverage may rely on signals that are blocked or handled differently in sandboxes

## Go Sandbox Coverage Techniques

### 1. Pre-instrumentation with Relaxed File Permissions

This approach instruments the code before it enters the sandbox and relaxes specific file permissions just for coverage files.

```go
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	// Create coverage directory with permissive permissions
	coverDir := "./coverage-sandbox"
	if err := os.MkdirAll(coverDir, 0777); err != nil {
		fmt.Printf("Error creating coverage dir: %v\n", err)
		os.Exit(1)
	}

	// Set environment variable for coverage
	os.Setenv("GOCOVERDIR", coverDir)

	// Build with coverage instrumentation before sandboxing
	buildCmd := exec.Command("go", "build", "-cover", "-o", "covered-binary", "./cmd/app")
	if err := buildCmd.Run(); err != nil {
		fmt.Printf("Error building with coverage: %v\n", err)
		os.Exit(1)
	}

	// Run in sandbox with coverage directory mapped
	cmd := exec.Command("sandbox-exec", "-f", "sandbox.sb", "./covered-binary")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	if err := cmd.Run(); err != nil {
		fmt.Printf("Error running in sandbox: %v\n", err)
		os.Exit(1)
	}
}
```

#### Example Sandbox Profile (for macOS):

```
(version 1)
(allow default)
(deny file-write* (subpath "/"))
(allow file-write* (subpath "coverage-sandbox"))
(allow file-write* (literal "/dev/null"))
(allow file-write* (literal "/dev/urandom"))
```

### 2. Coverage Server Approach

This technique runs a coverage server outside the sandbox that receives coverage data from the sandboxed application.

```go
// coverage_server.go - runs outside the sandbox
package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	// Create coverage directory
	coverDir := "./coverage-data"
	if err := os.MkdirAll(coverDir, 0755); err != nil {
		log.Fatalf("Error creating coverage directory: %v", err)
	}

	// Handle coverage data submissions
	http.HandleFunc("/coverage", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Generate a unique filename
		filename := filepath.Join(coverDir, fmt.Sprintf("cov-%d.out", time.Now().UnixNano()))
		
		// Save the coverage data
		file, err := os.Create(filename)
		if err != nil {
			http.Error(w, "Error creating file", http.StatusInternalServerError)
			return
		}
		defer file.Close()
		
		// Copy data from request to file
		_, err = io.Copy(file, r.Body)
		if err != nil {
			http.Error(w, "Error writing data", http.StatusInternalServerError)
			return
		}
		
		fmt.Fprintf(w, "Coverage data saved to %s\n", filename)
	})

	// Start server
	addr := "localhost:8099"
	log.Printf("Starting coverage server on %s\n", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
```

```go
// coverage_client.go - runs inside the sandbox
package coverage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"runtime"
	"sync"
)

var (
	coverageData     = make(map[string][]byte)
	coverageMutex    = &sync.Mutex{}
	coverageEndpoint = "http://localhost:8099/coverage"
)

// RegisterCoverageData adds coverage data to be sent
func RegisterCoverageData(name string, data []byte) {
	coverageMutex.Lock()
	defer coverageMutex.Unlock()
	coverageData[name] = data
}

// SendCoverageData sends all registered coverage data to the server
func SendCoverageData() error {
	coverageMutex.Lock()
	defer coverageMutex.Unlock()
	
	// Create a JSON payload with all coverage data
	payload, err := json.Marshal(coverageData)
	if err != nil {
		return fmt.Errorf("error marshaling coverage data: %v", err)
	}
	
	// Send to coverage server
	resp, err := http.Post(coverageEndpoint, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("error sending coverage data: %v", err)
	}
	defer resp.Body.Close()
	
	// Check response
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("server error (status %d): %s", resp.StatusCode, body)
	}
	
	// Clear sent data
	coverageData = make(map[string][]byte)
	
	return nil
}

// Install hooks to send coverage on program exit
func init() {
	runtime.SetFinalizer(&coverageMutex, func(_ *sync.Mutex) {
		SendCoverageData()
	})
}
```

### 3. Synthetic Coverage with Static Analysis

When direct coverage collection is impossible, use static analysis to generate synthetic coverage based on known execution paths.

```go
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
)

// SandboxAnalyzer maps sandbox calls to code paths
type SandboxAnalyzer struct {
	SourcePath string
	CallMap    map[string][]string // Maps function calls to file:line locations
}

// NewSandboxAnalyzer creates a new analyzer
func NewSandboxAnalyzer(sourcePath string) *SandboxAnalyzer {
	return &SandboxAnalyzer{
		SourcePath: sourcePath,
		CallMap:    make(map[string][]string),
	}
}

// AnalyzeSource parses and analyzes source code
func (a *SandboxAnalyzer) AnalyzeSource() error {
	fset := token.NewFileSet()
	
	// Parse all .go files in the directory
	pkgs, err := parser.ParseDir(fset, a.SourcePath, nil, parser.ParseComments)
	if err != nil {
		return err
	}
	
	// Process each package
	for _, pkg := range pkgs {
		for filePath, file := range pkg.Files {
			// Visit all functions in the file
			ast.Inspect(file, func(n ast.Node) bool {
				// Check for function declarations
				if fn, ok := n.(*ast.FuncDecl); ok {
					// Get function name
					fnName := fn.Name.Name
					if fn.Recv != nil {
						// If it's a method, include the receiver type
						if len(fn.Recv.List) > 0 {
							if t, ok := fn.Recv.List[0].Type.(*ast.StarExpr); ok {
								if ident, ok := t.X.(*ast.Ident); ok {
									fnName = ident.Name + "." + fnName
								}
							} else if ident, ok := fn.Recv.List[0].Type.(*ast.Ident); ok {
								fnName = ident.Name + "." + fnName
							}
						}
					}
					
					// Get the position in source code
					pos := fset.Position(fn.Pos())
					loc := fmt.Sprintf("%s:%d", filePath, pos.Line)
					
					// Add to call map
					a.CallMap[fnName] = append(a.CallMap[fnName], loc)
				}
				return true
			})
		}
	}
	
	return nil
}

// GenerateSyntheticCoverage creates coverage for executed functions
func (a *SandboxAnalyzer) GenerateSyntheticCoverage(executedFns []string, outputPath string) error {
	var sb strings.Builder
	sb.WriteString("mode: set\n")
	
	// Add coverage entries for executed functions
	for _, fn := range executedFns {
		locations, ok := a.CallMap[fn]
		if !ok {
			continue
		}
		
		for _, loc := range locations {
			// Format: file:line.column,line.column numstmt count
			sb.WriteString(fmt.Sprintf("%s.0,%s.1 1 1\n", loc, loc))
		}
	}
	
	// Write to output file
	return os.WriteFile(outputPath, []byte(sb.String()), 0644)
}
```

## Sandbox Execution Record and Replay

This approach records sandbox executions and replays them in a non-sandboxed environment for coverage.

### 1. Record sandbox executions:

```go
// record.go
package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"
)

// Request records an HTTP request
type Request struct {
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers"`
	Body    []byte            `json:"body"`
}

// RecordingServer records sandbox requests for replay
type RecordingServer struct {
	Requests []Request
	OutputFile string
}

// NewRecordingServer creates a new server
func NewRecordingServer(outputFile string) *RecordingServer {
	return &RecordingServer{
		Requests: []Request{},
		OutputFile: outputFile,
	}
}

// ServeHTTP implements http.Handler
func (s *RecordingServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Read request body
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading body", http.StatusInternalServerError)
		return
	}
	
	// Convert headers to map
	headers := make(map[string]string)
	for name, values := range r.Header {
		if len(values) > 0 {
			headers[name] = values[0]
		}
	}
	
	// Record the request
	req := Request{
		Method:  r.Method,
		Path:    r.URL.Path,
		Headers: headers,
		Body:    body,
	}
	
	s.Requests = append(s.Requests, req)
	
	// Save to output file after each request
	s.SaveRequests()
	
	// Forward to actual handler (could be mocked for testing)
	fmt.Fprintf(w, "Request recorded: %s %s\n", r.Method, r.URL.Path)
}

// SaveRequests saves recorded requests to file
func (s *RecordingServer) SaveRequests() error {
	data, err := json.Marshal(s.Requests)
	if err != nil {
		return err
	}
	
	return os.WriteFile(s.OutputFile, data, 0644)
}

func main() {
	outputFile := "sandbox-requests.json"
	server := NewRecordingServer(outputFile)
	
	// Start recording server
	log.Println("Starting recording server on :8080")
	if err := http.ListenAndServe(":8080", server); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
```

### 2. Replay for coverage:

```go
// replay.go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
)

// Request from the recorded data
type Request struct {
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers"`
	Body    []byte            `json:"body"`
}

func main() {
	// Set up coverage
	coverageDir := "./coverage-replay"
	os.MkdirAll(coverageDir, 0755)
	os.Setenv("GOCOVERDIR", coverageDir)
	
	// Load recorded requests
	recordFile := "sandbox-requests.json"
	data, err := os.ReadFile(recordFile)
	if err != nil {
		fmt.Printf("Error reading record file: %v\n", err)
		os.Exit(1)
	}
	
	var requests []Request
	if err := json.Unmarshal(data, &requests); err != nil {
		fmt.Printf("Error parsing record file: %v\n", err)
		os.Exit(1)
	}
	
	// Create HTTP client for replay
	client := &http.Client{}
	
	// Replay each request to the non-sandboxed app
	replayURL := "http://localhost:8081" // Non-sandboxed app
	
	for i, req := range requests {
		// Create a new request
		httpReq, err := http.NewRequest(req.Method, replayURL+req.Path, bytes.NewBuffer(req.Body))
		if err != nil {
			fmt.Printf("Error creating request %d: %v\n", i, err)
			continue
		}
		
		// Add headers
		for name, value := range req.Headers {
			httpReq.Header.Set(name, value)
		}
		
		// Send request
		resp, err := client.Do(httpReq)
		if err != nil {
			fmt.Printf("Error sending request %d: %v\n", i, err)
			continue
		}
		
		// Read response (not strictly necessary)
		body, _ := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		
		fmt.Printf("Replayed request %d: %s %s -> %d\n", 
			i, req.Method, req.Path, resp.StatusCode)
	}
	
	fmt.Println("Coverage data collected in", coverageDir)
}
```

## Best Practices for Sandbox Coverage

1. **Minimal Sandbox Permissions**: Define the minimal set of permissions needed for coverage
2. **Host-Side Processing**: When possible, process coverage data outside the sandbox
3. **Deterministic Tests**: Ensure sandbox tests are deterministic for accurate synthetic coverage
4. **API Bridge**: Create an API bridge between sandboxed and non-sandboxed environments for coverage
5. **Command Tracking**: Track commands executed in the sandbox to generate accurate synthetic coverage
6. **Comprehensive Profiling**: Capture not just coverage but also performance metrics when possible
7. **Environment Consistency**: Keep sandbox and non-sandbox environments as similar as possible

## Integration with Synthetic Coverage Tools

To integrate sandbox execution with your existing synthetic coverage workflow:

1. Extract execution paths from sandbox operation logs
2. Map recorded execution paths to code locations using static analysis
3. Generate synthetic coverage entries for the executed paths
4. Merge the synthetic coverage with any real coverage from non-sandboxed tests
5. Analyze and report the combined coverage