package api

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
)

// GenerateHandler handles code generation requests
type GenerateHandler struct {
	TempDir string
	Logger  *zap.Logger
}

// NewGenerateHandler creates a new GenerateHandler
func NewGenerateHandler(tempDir string) *GenerateHandler {
	logger, _ := zap.NewProduction()
	return &GenerateHandler{
		TempDir: tempDir,
		Logger:  logger,
	}
}

// HandleGenerate processes code generation requests
func (h *GenerateHandler) HandleGenerate(w http.ResponseWriter, r *http.Request) {
	// Parse request
	var req GenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	// Validate request
	if len(req.Proto.Files) == 0 {
		http.Error(w, "No proto files provided", http.StatusBadRequest)
		return
	}
	if len(req.Templates) == 0 {
		http.Error(w, "No templates provided", http.StatusBadRequest)
		return
	}

	// Create session directory
	sessionDir, err := ioutil.TempDir(h.TempDir, "protogen-")
	if err != nil {
		h.Logger.Error("Failed to create temporary directory", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(sessionDir)

	// Create directories for proto files, templates, and output
	protoDir := filepath.Join(sessionDir, "proto")
	templateDir := filepath.Join(sessionDir, "templates")
	outputDir := filepath.Join(sessionDir, "output")
	
	for _, dir := range []string{protoDir, templateDir, outputDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			h.Logger.Error("Failed to create directory", zap.String("dir", dir), zap.Error(err))
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}

	// Write proto files
	for _, file := range req.Proto.Files {
		path := filepath.Join(protoDir, file.Name)
		if err := ioutil.WriteFile(path, []byte(file.Content), 0644); err != nil {
			h.Logger.Error("Failed to write proto file", zap.String("file", file.Name), zap.Error(err))
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}

	// Write template files
	for _, tmpl := range req.Templates {
		path := filepath.Join(templateDir, tmpl.Name)
		// Ensure parent directory exists (for nested templates)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			h.Logger.Error("Failed to create template directory", zap.String("dir", filepath.Dir(path)), zap.Error(err))
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if err := ioutil.WriteFile(path, []byte(tmpl.Content), 0644); err != nil {
			h.Logger.Error("Failed to write template file", zap.String("file", tmpl.Name), zap.Error(err))
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}

	// Run code generation
	resp, err := h.runGeneration(protoDir, templateDir, outputDir, req.Options)
	if err != nil {
		h.Logger.Error("Generation failed", zap.Error(err))
		http.Error(w, fmt.Sprintf("Generation failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.Logger.Error("Failed to encode response", zap.Error(err))
	}
}

// runGeneration executes protoc with protoc-gen-anything to generate code
func (h *GenerateHandler) runGeneration(protoDir, templateDir, outputDir string, options Options) (*GenerateResponse, error) {
	// Find all proto files
	var protoFiles []string
	err := filepath.Walk(protoDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".proto") {
			relPath, err := filepath.Rel(protoDir, path)
			if err != nil {
				return err
			}
			protoFiles = append(protoFiles, relPath)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to find proto files: %w", err)
	}

	// Prepare protoc command
	args := []string{
		"--anything_out=templates=" + templateDir + ":" + outputDir,
	}
	
	// Add options
	if options.ContinueOnError {
		args[0] += ",continue_on_error"
	}
	if options.Verbose {
		args[0] += ",verbose"
	}
	
	// Add proto files
	args = append(args, protoFiles...)

	cmd := exec.Command("protoc", args...)
	cmd.Dir = protoDir

	// Capture stdout and stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	// Start command
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start protoc: %w", err)
	}

	// Read output
	stdoutBytes, err := io.ReadAll(stdout)
	if err != nil {
		return nil, fmt.Errorf("failed to read stdout: %w", err)
	}
	stderrBytes, err := io.ReadAll(stderr)
	if err != nil {
		return nil, fmt.Errorf("failed to read stderr: %w", err)
	}

	// Wait for command to finish
	err = cmd.Wait()

	// Create response
	resp := &GenerateResponse{
		Success: err == nil,
		Files:   []OutputFile{},
		Logs:    []LogMessage{},
		Errors:  []string{},
	}

	// Add logs
	for _, line := range strings.Split(string(stdoutBytes), "\n") {
		if line != "" {
			resp.Logs = append(resp.Logs, LogMessage{Level: "info", Message: line})
		}
	}

	// Add errors
	if err != nil {
		resp.Errors = append(resp.Errors, err.Error())
	}
	for _, line := range strings.Split(string(stderrBytes), "\n") {
		if line != "" {
			resp.Logs = append(resp.Logs, LogMessage{Level: "error", Message: line})
			resp.Errors = append(resp.Errors, line)
		}
	}

	// Read generated files
	err = filepath.Walk(outputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			relPath, err := filepath.Rel(outputDir, path)
			if err != nil {
				return err
			}
			content, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}
			resp.Files = append(resp.Files, OutputFile{
				Name:    relPath,
				Content: string(content),
			})
		}
		return nil
	})
	if err != nil {
		resp.Errors = append(resp.Errors, fmt.Sprintf("failed to read generated files: %v", err))
	}

	return resp, nil
}