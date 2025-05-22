package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall/js"
	"time"

	"go.uber.org/zap"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// InMemoryFS implements a simple in-memory filesystem
type InMemoryFS struct {
	files map[string][]byte
}

func NewInMemoryFS() *InMemoryFS {
	return &InMemoryFS{
		files: make(map[string][]byte),
	}
}

// Open implements fs.FS
func (m *InMemoryFS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}

	// Check if it's a directory request
	if name == "." {
		return &InMemoryDir{fs: m, name: name, pos: 0}, nil
	}

	if filepath.Dir(name) != "." {
		dirName := filepath.Dir(name)
		return &InMemoryDir{fs: m, name: dirName, pos: 0}, nil
	}

	data, ok := m.files[name]
	if !ok {
		return nil, &fs.PathError{Op: "open", Path: name, Err: os.ErrNotExist}
	}

	return &InMemoryFile{name: name, content: data, pos: 0}, nil
}

// WriteFile adds or updates a file in the in-memory filesystem
func (m *InMemoryFS) WriteFile(name string, data []byte) error {
	m.files[name] = data
	return nil
}

// ReadFile reads a file from the in-memory filesystem
func (m *InMemoryFS) ReadFile(name string) ([]byte, error) {
	data, ok := m.files[name]
	if !ok {
		return nil, os.ErrNotExist
	}
	return data, nil
}

// InMemoryFile implements a file in the in-memory filesystem
type InMemoryFile struct {
	name    string
	content []byte
	pos     int
}

func (f *InMemoryFile) Stat() (fs.FileInfo, error) {
	return &InMemoryFileInfo{name: f.name, size: int64(len(f.content))}, nil
}

func (f *InMemoryFile) Read(p []byte) (int, error) {
	if f.pos >= len(f.content) {
		return 0, os.ErrInvalid
	}
	n := copy(p, f.content[f.pos:])
	f.pos += n
	return n, nil
}

func (f *InMemoryFile) Close() error {
	return nil
}

// InMemoryDir implements a directory in the in-memory filesystem
type InMemoryDir struct {
	fs   *InMemoryFS
	name string
	pos  int
	list []string
}

func (d *InMemoryDir) Stat() (fs.FileInfo, error) {
	return &InMemoryFileInfo{name: d.name, isDir: true}, nil
}

func (d *InMemoryDir) Read(p []byte) (int, error) {
	return 0, fmt.Errorf("cannot read from directory")
}

func (d *InMemoryDir) Close() error {
	return nil
}

func (d *InMemoryDir) ReadDir(count int) ([]fs.DirEntry, error) {
	if d.list == nil {
		d.list = make([]string, 0)
		prefix := ""
		if d.name != "." {
			prefix = d.name + "/"
		}
		
		for name := range d.fs.files {
			if strings.HasPrefix(name, prefix) {
				// Only include direct children
				remainder := strings.TrimPrefix(name, prefix)
				if !strings.Contains(remainder, "/") {
					d.list = append(d.list, name)
				}
			}
		}
	}

	n := len(d.list) - d.pos
	if count > 0 && n > count {
		n = count
	}

	entries := make([]fs.DirEntry, n)
	for i := 0; i < n; i++ {
		fileName := d.list[d.pos+i]
		isDir := false
		entries[i] = &InMemoryDirEntry{name: filepath.Base(fileName), isDir: isDir}
	}
	
	d.pos += n
	return entries, nil
}

// InMemoryFileInfo implements fs.FileInfo for in-memory files
type InMemoryFileInfo struct {
	name  string
	size  int64
	isDir bool
}

func (i *InMemoryFileInfo) Name() string       { return i.name }
func (i *InMemoryFileInfo) Size() int64        { return i.size }
func (i *InMemoryFileInfo) Mode() fs.FileMode  { return 0644 }
func (i *InMemoryFileInfo) ModTime() time.Time { return time.Now() }
func (i *InMemoryFileInfo) IsDir() bool        { return i.isDir }
func (i *InMemoryFileInfo) Sys() interface{}   { return nil }

// InMemoryDirEntry implements fs.DirEntry for in-memory directories
type InMemoryDirEntry struct {
	name  string
	isDir bool
}

func (d *InMemoryDirEntry) Name() string               { return d.name }
func (d *InMemoryDirEntry) IsDir() bool                { return d.isDir }
func (d *InMemoryDirEntry) Type() fs.FileMode          { return 0644 }
func (d *InMemoryDirEntry) Info() (fs.FileInfo, error) { return &InMemoryFileInfo{name: d.name, isDir: d.isDir}, nil }

// WebAsmGenerator adapts protoc-gen-anything for WebAssembly
type WebAsmGenerator struct {
	TemplateFS *InMemoryFS
	OutputFS   *InMemoryFS
	Logger     *zap.Logger
	
	types          *protoregistry.Types
	files          map[string]*protogen.File
	services       map[string]*protogen.Service
	methods        map[string]*protogen.Method
	messages       map[string]*protogen.Message
	enums          map[string]*protogen.Enum
	oneofs         map[string]*protogen.Oneof
	fields         map[string]*protogen.Field
	generatedFiles map[string]*bytes.Buffer
}

// NewWebAsmGenerator creates a new WebAsmGenerator instance
func NewWebAsmGenerator() *WebAsmGenerator {
	logger, _ := zap.NewProduction()
	return &WebAsmGenerator{
		TemplateFS:     NewInMemoryFS(),
		OutputFS:       NewInMemoryFS(),
		Logger:         logger,
		types:          new(protoregistry.Types),
		files:          make(map[string]*protogen.File),
		services:       make(map[string]*protogen.Service),
		methods:        make(map[string]*protogen.Method),
		messages:       make(map[string]*protogen.Message),
		enums:          make(map[string]*protogen.Enum),
		oneofs:         make(map[string]*protogen.Oneof),
		fields:         make(map[string]*protogen.Field),
		generatedFiles: make(map[string]*bytes.Buffer),
	}
}

// Generate processes proto files and templates to generate output
func (g *WebAsmGenerator) Generate(protoFiles map[string]string, templates map[string]string, options map[string]interface{}) (map[string]string, error) {
	// Reset state
	g.types = new(protoregistry.Types)
	g.files = make(map[string]*protogen.File)
	g.services = make(map[string]*protogen.Service)
	g.methods = make(map[string]*protogen.Method)
	g.messages = make(map[string]*protogen.Message)
	g.enums = make(map[string]*protogen.Enum)
	g.oneofs = make(map[string]*protogen.Oneof)
	g.fields = make(map[string]*protogen.Field)
	g.generatedFiles = make(map[string]*bytes.Buffer)
	
	// Write proto files to in-memory filesystem
	for name, content := range protoFiles {
		if err := g.TemplateFS.WriteFile(name, []byte(content)); err != nil {
			return nil, fmt.Errorf("failed to write proto file: %w", err)
		}
	}
	
	// Write template files to in-memory filesystem
	for name, content := range templates {
		if err := g.TemplateFS.WriteFile(name, []byte(content)); err != nil {
			return nil, fmt.Errorf("failed to write template file: %w", err)
		}
	}
	
	// TODO: Implement the actual generation logic
	// This would involve:
	// 1. Parsing proto files using protogen
	// 2. Processing templates
	// 3. Generating output files
	
	// For now, we'll just return a mock implementation
	output := make(map[string]string)
	for name, content := range templates {
		outputName := strings.TrimSuffix(name, ".tmpl")
		output[outputName] = "// Generated from " + name + "\n" + content
	}
	
	return output, nil
}

// Required JS functions
func main() {
	// Set up the generator
	generator := NewWebAsmGenerator()
	
	// Register the generate function in JavaScript
	js.Global().Set("generateFromProto", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		if len(args) != 1 {
			return "Invalid number of arguments"
		}
		
		// Parse the input JSON
		inputJSON := args[0].String()
		var input struct {
			ProtoFiles map[string]string      `json:"protoFiles"`
			Templates  map[string]string      `json:"templates"`
			Options    map[string]interface{} `json:"options"`
		}
		
		if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
			return fmt.Sprintf("Failed to parse input: %v", err)
		}
		
		// Generate the output
		output, err := generator.Generate(input.ProtoFiles, input.Templates, input.Options)
		if err != nil {
			return fmt.Sprintf("Generation error: %v", err)
		}
		
		// Return the output as JSON
		outputJSON, err := json.Marshal(output)
		if err != nil {
			return fmt.Sprintf("Failed to stringify output: %v", err)
		}
		
		return string(outputJSON)
	}))
	
	// Keep the program running
	select {}
}