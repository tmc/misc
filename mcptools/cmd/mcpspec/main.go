package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/tmc/misc/mcptools/internal/mcp"
)

var (
	extract = flag.String("extract", "", "extract spec from server command")
	verify  = flag.String("verify", "", "verify spec against server command")
)

// ToolSpec represents a tool specification
type ToolSpec struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Examples    []ExampleSpec `json:"examples,omitempty"`
	Tests       []string      `json:"tests,omitempty"` // paths to test files
}

// ExampleSpec represents an example usage of a tool
type ExampleSpec struct {
	Name   string          `json:"name"`
	Input  json.RawMessage `json:"input"`
	Output json.RawMessage `json:"output"`
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: mcpspec [flags] spec.json\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *extract != "" {
		if err := extractSpec(*extract, flag.Arg(0)); err != nil {
			log.Fatal(err)
		}
		return
	}

	if *verify != "" {
		if err := verifySpec(*verify, flag.Arg(0)); err != nil {
			log.Fatal(err)
		}
		return
	}

	flag.Usage()
	os.Exit(2)
}

func extractSpec(serverCmd, outFile string) error {
	// Start the server
	cmd := exec.Command("sh", "-c", serverCmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	defer cmd.Process.Kill()

	// Initialize connection
	id1 := json.Number("1")
	initReq := mcp.Message{
		JSONRPC: "2.0",
		ID:      &id1,
		Method:  "initialize",
		Params: json.RawMessage(`{
			"name": "mcpspec",
			"version": "1.0.0"
		}`),
	}
	if err := sendMessage(stdin, &initReq); err != nil {
		return err
	}
	if _, err := readMessage(stdout); err != nil {
		return err
	}

	// List tools
	id2 := json.Number("2")
	listReq := mcp.Message{
		JSONRPC: "2.0",
		ID:      &id2,
		Method:  "listTools",
	}
	if err := sendMessage(stdin, &listReq); err != nil {
		return err
	}
	resp, err := readMessage(stdout)
	if err != nil {
		return err
	}

	// Parse tools response
	var result struct {
		Tools []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return err
	}

	// Create specs for each tool
	var specs []ToolSpec
	for _, tool := range result.Tools {
		spec := ToolSpec{
			Name:        tool.Name,
			Description: tool.Description,
			Tests:       findTests(tool.Name),
		}
		specs = append(specs, spec)
	}

	// Write spec file
	f, err := os.Create(outFile)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(specs)
}

func verifySpec(serverCmd, specFile string) error {
	// Read spec file
	f, err := os.Open(specFile)
	if err != nil {
		return err
	}
	defer f.Close()

	var specs []ToolSpec
	if err := json.NewDecoder(f).Decode(&specs); err != nil {
		return err
	}

	// Run each test file
	for _, spec := range specs {
		for _, testFile := range spec.Tests {
			if err := runTest(serverCmd, testFile); err != nil {
				return fmt.Errorf("test %s failed: %v", testFile, err)
			}
		}
	}

	return nil
}

func runTest(serverCmd, testFile string) error {
	// Read test file
	f, err := os.Open(testFile)
	if err != nil {
		return err
	}
	defer f.Close()

	entries, err := mcp.LoadRecording(f)
	if err != nil {
		return err
	}

	// Start server
	cmd := exec.Command("sh", "-c", serverCmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	defer cmd.Process.Kill()

	// Run through test entries
	for i, e := range entries {
		switch e.Dir {
		case "in":
			if err := sendRawMessage(stdin, e.Data); err != nil {
				return fmt.Errorf("entry %d: %v", i, err)
			}
		case "out":
			got, err := readRawMessage(stdout)
			if err != nil {
				return fmt.Errorf("entry %d: %v", i, err)
			}
			if !jsonEqual(got, e.Data) {
				return fmt.Errorf("entry %d: got %s, want %s", i, got, e.Data)
			}
		}
	}

	return cmd.Wait()
}

func findTests(toolName string) []string {
	pattern := filepath.Join("testdata", "scripts", toolName+"*.txt")
	matches, _ := filepath.Glob(pattern)
	return matches
}

func sendMessage(w io.Writer, msg *mcp.Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return sendRawMessage(w, data)
}

func sendRawMessage(w io.Writer, data []byte) error {
	_, err := fmt.Fprintf(w, "%s\n", data)
	return err
}

func readMessage(r io.Reader) (*mcp.Message, error) {
	data, err := readRawMessage(r)
	if err != nil {
		return nil, err
	}
	var msg mcp.Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

func readRawMessage(r io.Reader) ([]byte, error) {
	scanner := bufio.NewScanner(r)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return nil, err
		}
		return nil, io.EOF
	}
	return scanner.Bytes(), nil
}

func jsonEqual(a, b []byte) bool {
	var va, vb interface{}
	if err := json.Unmarshal(a, &va); err != nil {
		return false
	}
	if err := json.Unmarshal(b, &vb); err != nil {
		return false
	}
	return strings.TrimSpace(mustMarshal(va)) == strings.TrimSpace(mustMarshal(vb))
}

func mustMarshal(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(data)
}
