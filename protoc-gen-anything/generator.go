package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/tmc/tmpl/sprig"
	"golang.org/x/tools/imports"
	"golang.org/x/tools/txtar"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/pluginpb"
)

type GeneratedFileBuffer struct {
	Content    bytes.Buffer
	IsFirstGen bool
}

// Generator holds configuration for an invocation of this tool
type Generator struct {
	TemplateDir     string
	Verbose         bool
	ContinueOnError bool
	RunGoImports    bool
	TxtarMode       bool // parse template output as txtar archives

	Logger *slog.Logger

	types        *protoregistry.Types
	starlarkFuncs template.FuncMap // user-defined functions from funcs.star

	files          map[string]*protogen.File
	services       map[string]*protogen.Service
	methods        map[string]*protogen.Method
	messages       map[string]*protogen.Message
	enums          map[string]*protogen.Enum
	oneofs         map[string]*protogen.Oneof
	fields         map[string]*protogen.Field
	generatedFiles map[string]*GeneratedFileBuffer
	seenStaticOutputs map[string]bool // tracks static template outputs to prevent duplicates
}

type Options struct {
	TemplateDir     string
	Verbose         bool
	ContinueOnError bool
	RunGoImports    bool
	TxtarMode       bool
	Logger          *slog.Logger
}

// OutputDirective controls how template output is processed
type OutputDirective struct {
	Format   string // "single" or "txtar"
	Behavior string // "append", "overwrite", "error"
}

// NewGenerator creates a new protoc-gen-anything generator.
func NewGenerator(o Options) *Generator {
	g := &Generator{
		TemplateDir:     o.TemplateDir,
		Verbose:         o.Verbose,
		ContinueOnError: o.ContinueOnError,
		RunGoImports:    o.RunGoImports,
		TxtarMode:       o.TxtarMode,
		Logger:          o.Logger,

		types:          new(protoregistry.Types),
		starlarkFuncs:  make(template.FuncMap),
		files:          make(map[string]*protogen.File),
		services:       make(map[string]*protogen.Service),
		methods:        make(map[string]*protogen.Method),
		messages:       make(map[string]*protogen.Message),
		enums:          make(map[string]*protogen.Enum),
		oneofs:         make(map[string]*protogen.Oneof),
		fields:            make(map[string]*protogen.Field),
		generatedFiles:    make(map[string]*GeneratedFileBuffer),
		seenStaticOutputs: make(map[string]bool),
	}
	// Note: Starlark functions are loaded in walkSchemas after proto types are registered
	return g
}

func (g *Generator) Generate(gen *protogen.Plugin) error {
	gen.SupportedFeatures = uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)
	if err := g.walkSchemas(gen); err != nil {
		return err
	}
	if err := g.generate(gen); err != nil {
		return err
	}
	return g.finalizeGeneration(gen)
}

// generate generates the code for plugin.
func (g *Generator) generate(gen *protogen.Plugin) error {
	tFS, err := g.getTemplateFS()
	if err != nil {
		return fmt.Errorf("failed to get template FS: %w", err)
	}

	// Iterate over each file to generate corresponding files
	for _, f := range gen.Files {
		if !f.Generate {
			continue
		}
		if err := g.generateForFile(f, tFS, gen); err != nil {
			return err
		}
	}
	return nil
}

func (g *Generator) generateForFile(f *protogen.File, tFS fs.FS, gen *protogen.Plugin) error {
	if err := g.applyTemplates("file", f, nil, nil, nil, nil, nil, nil, tFS, gen); err != nil {
		return err
	}

	for _, s := range f.Services {
		if err := g.generateForService(f, s, tFS, gen); err != nil {
			return err
		}
	}

	for _, msg := range f.Messages {
		if err := g.generateForMessage(f, msg, tFS, gen); err != nil {
			return err
		}
	}

	for _, enum := range f.Enums {
		if err := g.generateForEnum(f, enum, tFS, gen); err != nil {
			return err
		}
	}

	return nil
}

func (g *Generator) generateForService(f *protogen.File, s *protogen.Service, tFS fs.FS, gen *protogen.Plugin) error {
	if err := g.applyTemplates("service", f, s, nil, nil, nil, nil, nil, tFS, gen); err != nil {
		return err
	}

	for _, m := range s.Methods {
		if err := g.generateForMethod(f, s, m, tFS, gen); err != nil {
			return err
		}
	}

	return nil
}

func (g *Generator) generateForMethod(f *protogen.File, s *protogen.Service, m *protogen.Method, tFS fs.FS, gen *protogen.Plugin) error {
	return g.applyTemplates("method", f, s, m, nil, nil, nil, nil, tFS, gen)
}

func (g *Generator) generateForMessage(f *protogen.File, msg *protogen.Message, tFS fs.FS, gen *protogen.Plugin) error {
	if err := g.applyTemplates("message", f, nil, nil, msg, nil, nil, nil, tFS, gen); err != nil {
		return err
	}

	for _, oneof := range msg.Oneofs {
		if err := g.generateForOneof(f, msg, oneof, tFS, gen); err != nil {
			return err
		}
	}

	for _, field := range msg.Fields {
		if err := g.generateForField(f, msg, field, tFS, gen); err != nil {
			return err
		}
	}

	for _, enum := range msg.Enums {
		if err := g.generateForEnum(f, enum, tFS, gen); err != nil {
			return err
		}
	}

	for _, nestedMsg := range msg.Messages {
		if err := g.generateForNestedMessage(f, msg, nestedMsg, tFS, gen); err != nil {
			return err
		}
	}

	return nil
}

func (g *Generator) generateForEnum(f *protogen.File, enum *protogen.Enum, tFS fs.FS, gen *protogen.Plugin) error {
	return g.applyTemplates("enum", f, nil, nil, nil, enum, nil, nil, tFS, gen)
}

func (g *Generator) generateForOneof(f *protogen.File, msg *protogen.Message, oneof *protogen.Oneof, tFS fs.FS, gen *protogen.Plugin) error {
	return g.applyTemplates("oneof", f, nil, nil, msg, nil, oneof, nil, tFS, gen)
}

func (g *Generator) generateForField(f *protogen.File, msg *protogen.Message, field *protogen.Field, tFS fs.FS, gen *protogen.Plugin) error {
	return g.applyTemplates("field", f, nil, nil, msg, nil, nil, field, tFS, gen)
}
func (g *Generator) generateForNestedMessage(f *protogen.File, parentMsg *protogen.Message, nestedMsg *protogen.Message, tFS fs.FS, gen *protogen.Plugin) error {
	return g.applyTemplates("nestedMessage", f, nil, nil, nestedMsg, nil, nil, nil, tFS, gen)
}

func (g *Generator) applyTemplates(entityType string, file *protogen.File, service *protogen.Service, method *protogen.Method, message *protogen.Message, enum *protogen.Enum, oneof *protogen.Oneof, field *protogen.Field, tFS fs.FS, gen *protogen.Plugin) error {
	var templatesFound int
	err := fs.WalkDir(tFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("failed to walk template dir: %w", err)
		}
		if d.IsDir() {
			return nil
		}
		// Skip non-template files
		if isNonTemplateFile(path) {
			return nil
		}

		metadata := extractMetadataFromPath(path)
		if metadata["type"] != entityType {
			return nil // Skip silently - type mismatches are expected and frequent
		}

		templatesFound++
		return g.processTemplate(path, file, service, method, message, enum, oneof, field, tFS, gen)
	})

	// Don't log "no templates found" - it's expected for most entity types

	if err != nil {
		// Error already logged in processTemplate, just propagate
		if g.ContinueOnError {
			return nil
		}
		return err
	}

	return nil
}

func (g *Generator) processTemplate(path string, file *protogen.File, service *protogen.Service, method *protogen.Method, message *protogen.Message, enum *protogen.Enum, oneof *protogen.Oneof, field *protogen.Field, tFS fs.FS, gen *protogen.Plugin) error {
	outputFileName, err := g.expandPath(path, file, service, method, message, enum, oneof, field)
	if err != nil {
		g.Logger.Error("failed to expand path", "error", err, "path", path)
		return fmt.Errorf("failed to expand path: %w", err)
	}

	// Skip if the outputFileName is just a directory or empty (path expansion returned skip signal)
	if outputFileName == "" || outputFileName[len(outputFileName)-1] == '/' {
		return nil
	}

	// For static templates (no template variables in path), only generate once
	// This prevents duplicate generation when multiple proto files are processed
	isStaticPath := !strings.Contains(path, "{{")
	if isStaticPath && g.seenStaticOutputs[outputFileName] {
		return nil
	}

	templateContent, err := g.readTemplateContent(tFS, path)
	if err != nil {
		return err
	}

	tmpl, err := g.parseTemplate(path, templateContent)
	if err != nil {
		return err
	}

	context := g.determineContext(file, service, method, message, enum, oneof, field)
	context.OutputFileName = outputFileName
	context.TemplateFileName = path

	var tempBuffer bytes.Buffer
	err = tmpl.Execute(&tempBuffer, context)
	if err != nil {
		g.Logger.Error("failed to execute template", "error", err, "path", path, "outputFileName", outputFileName)
		return err
	}

	// Parse output directive and get cleaned content
	directive, content := g.parseOutputDirective(tempBuffer.Bytes())

	// Skip if content is empty (template produced no output, e.g., conditional didn't match)
	if len(bytes.TrimSpace(content)) == 0 {
		return nil
	}

	// Mark static paths as seen only after we know template produced content
	if isStaticPath {
		g.seenStaticOutputs[outputFileName] = true
	}

	// Handle txtar format
	if directive.Format == "txtar" {
		return g.processTxtarOutput(content, directive.Behavior)
	}

	// Single file output
	g.Logger.Info("generating file", "outputFileName", outputFileName)
	return g.addToGeneratedFile(outputFileName, content, directive.Behavior)
}

func (g *Generator) expandPath(pathTemplate string, file *protogen.File, service *protogen.Service, method *protogen.Method, message *protogen.Message, enum *protogen.Enum, oneof *protogen.Oneof, field *protogen.Field) (string, error) {
	// Skip templates that require entities we don't have.
	// A path like "{{.Service.GoName}}/foo.go" cannot be expanded without a service.
	// This is intentional: per-service templates should only generate for files that have services.
	// Note: This checks for direct field access patterns, not conditional checks like {{if .Service}}.
	if strings.Contains(pathTemplate, ".Service.") && service == nil {
		return "", nil // Skip - path requires service fields
	}
	if strings.Contains(pathTemplate, ".Method.") && method == nil {
		return "", nil // Skip - path requires method fields
	}
	if strings.Contains(pathTemplate, ".Message.") && message == nil {
		return "", nil // Skip - path requires message fields
	}
	if strings.Contains(pathTemplate, ".Enum.") && enum == nil {
		return "", nil // Skip - path requires enum fields
	}

	tmpl, err := template.New("path").
		Funcs(sprig.TxtFuncMap()).
		Funcs(g.funcMap()).
		Funcs(g.starlarkFuncs).
		Parse(pathTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	context := g.determineContext(file, service, method, message, enum, oneof, field)

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, context)
	if err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return g.cleanPath(buf.String()), nil
}

func (g *Generator) cleanPath(path string) string {
	return strings.TrimSuffix(path, ".tmpl")
}

type RenderContext struct {
	TemplateFileName  string // The name of the template file being rendered.
	OutputFileName    string // The name of the output file being generated.
	IsFirstGeneration bool   // Whether this is the first generation of the file path.

	File    *protogen.File
	Service *protogen.Service
	Method  *protogen.Method
	Message *protogen.Message
	Enum    *protogen.Enum
	Oneof   *protogen.Oneof
	Field   *protogen.Field
}

func (g *Generator) determineContext(file *protogen.File, service *protogen.Service, method *protogen.Method, message *protogen.Message, enum *protogen.Enum, oneof *protogen.Oneof, field *protogen.Field) RenderContext {
	return RenderContext{
		File:    file,
		Service: service,
		Method:  method,
		Message: message,
		Enum:    enum,
		Oneof:   oneof,
		Field:   field,
	}
}

func (g *Generator) readTemplateContent(tFS fs.FS, path string) (string, error) {
	templateContent, err := fs.ReadFile(tFS, path)
	if err != nil {
		g.Logger.Error("failed to read template file", "error", err, "path", path)
		return "", fmt.Errorf("failed to read template file: %w", err)
	}

	g.Logger.Debug("template content", "path", path, "content", string(templateContent))
	return string(templateContent), nil
}

func (g *Generator) parseTemplate(path string, templateContent string) (*template.Template, error) {
	tmpl, err := template.New(path).
		Funcs(sprig.TxtFuncMap()).
		Funcs(g.funcMap()).
		Funcs(g.starlarkFuncs).
		Parse(templateContent)
	if err != nil {
		g.Logger.Error("failed to parse template", "error", err, "path", path)
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	g.Logger.Debug("template parsed successfully", "path", path, "definedTemplates", strings.Split(tmpl.DefinedTemplates(), "; "))

	return tmpl, nil
}

// pga directive regex: //pga: key=value key2=value2
var pgaDirectiveRe = regexp.MustCompile(`^//pga:\s*(.+)$`)

// parseOutputDirective parses //pga: directives from the beginning of content
// and returns the directive settings plus the content with directives stripped.
func (g *Generator) parseOutputDirective(content []byte) (OutputDirective, []byte) {
	directive := OutputDirective{
		Format:   "single",
		Behavior: "append",
	}

	// If --txtar flag is set, default to txtar format
	if g.TxtarMode {
		directive.Format = "txtar"
	}

	lines := bytes.Split(content, []byte("\n"))
	var stripped [][]byte
	inDirectives := true

	for _, line := range lines {
		trimmed := bytes.TrimSpace(line)
		if inDirectives && len(trimmed) > 0 {
			if match := pgaDirectiveRe.FindSubmatch(trimmed); match != nil {
				// Parse key=value pairs
				pairs := strings.Fields(string(match[1]))
				for _, pair := range pairs {
					kv := strings.SplitN(pair, "=", 2)
					if len(kv) == 2 {
						switch kv[0] {
						case "format":
							directive.Format = kv[1]
						case "behavior":
							directive.Behavior = kv[1]
						}
					}
				}
				continue // Skip directive line from output
			}
			inDirectives = false
		}
		stripped = append(stripped, line)
	}

	return directive, bytes.Join(stripped, []byte("\n"))
}

// processTxtarOutput parses txtar-formatted content and adds each file to generatedFiles
func (g *Generator) processTxtarOutput(content []byte, behavior string) error {
	ar := txtar.Parse(content)

	for _, file := range ar.Files {
		filename := g.normalizeTxtarPath(file.Name)
		g.Logger.Info("generating file from txtar", "filename", filename)
		if err := g.addToGeneratedFile(filename, file.Data, behavior); err != nil {
			return err
		}
	}
	return nil
}

// normalizeTxtarPath handles relative and absolute paths in txtar markers
func (g *Generator) normalizeTxtarPath(path string) string {
	// Remove leading ./ if present
	path = strings.TrimPrefix(path, "./")
	// Absolute paths are used as-is
	if filepath.IsAbs(path) {
		return path
	}
	// Clean the path
	return filepath.Clean(path)
}

// addToGeneratedFile adds content to a generated file buffer with the specified behavior
func (g *Generator) addToGeneratedFile(filename string, content []byte, behavior string) error {
	fileBuffer, exists := g.generatedFiles[filename]
	if !exists {
		fileBuffer = &GeneratedFileBuffer{IsFirstGen: true}
		g.generatedFiles[filename] = fileBuffer
	}

	switch behavior {
	case "append":
		fileBuffer.Content.Write(content)
	case "overwrite":
		fileBuffer.Content.Reset()
		fileBuffer.Content.Write(content)
	case "error":
		if !fileBuffer.IsFirstGen {
			return fmt.Errorf("conflict detected for file: %s", filename)
		}
		fileBuffer.Content.Write(content)
	default:
		fileBuffer.Content.Write(content)
	}

	fileBuffer.IsFirstGen = false
	return nil
}

func (g *Generator) finalizeGeneration(gen *protogen.Plugin) error {
	for outputFileName, buffer := range g.generatedFiles {
		content := buffer.Content.Bytes()

		// Run goimports if requested and this is a Go file
		if g.RunGoImports && strings.HasSuffix(outputFileName, ".go") {
			formatted, err := imports.Process(outputFileName, content, nil)
			if err != nil {
				g.Logger.Warn("failed to run goimports", "file", outputFileName, "error", err)
			} else {
				content = formatted
			}
		}

		generatedFile := gen.NewGeneratedFile(outputFileName, "")
		_, err := generatedFile.Write(content)
		if err != nil {
			return fmt.Errorf("failed to write generated file %s: %w", outputFileName, err)
		}
	}
	return nil
}

func (g *Generator) walkSchemas(gen *protogen.Plugin) error {
	// First pass: register types from all files (including dependencies)
	// This allows templates to use types from imported proto files
	for _, f := range gen.Files {
		if err := registerAllExtensions(g.types, f.Desc); err != nil {
			g.Logger.Error("failed to register extensions", "error", err)
		}
		if err := registerAllMessages(g.types, f.Desc); err != nil {
			g.Logger.Error("failed to register messages", "error", err)
		}
	}

	// Load starlark functions now that proto types are registered
	// This allows Starlark code to access proto types via proto.file()
	if g.TemplateDir != "" {
		if funcs, err := loadStarlarkFuncs(g.TemplateDir, g.types); err == nil {
			g.starlarkFuncs = funcs
		} else if !os.IsNotExist(err) {
			g.Logger.Warn("failed to load starlark functions", "error", err)
		}
	}

	// Second pass: walk files marked for generation to populate indexes
	for _, f := range gen.Files {
		if !f.Generate {
			continue
		}
		g.walkFileEntities(f)
	}
	return nil
}

// walkFileEntities populates internal indexes for services, messages, enums etc.
// Type registration is handled separately in walkSchemas.
func (g *Generator) walkFileEntities(f *protogen.File) {
	for _, s := range f.Services {
		g.walkService(s)
	}
	for _, m := range f.Messages {
		g.walkMessage(m)
	}
	for _, e := range f.Enums {
		g.walkEnum(e)
	}
}

func (g *Generator) walkService(s *protogen.Service) {
	for _, m := range s.Methods {
		g.walkMethod(m)
	}
	g.services[s.GoName] = s
}

func (g *Generator) walkMethod(m *protogen.Method) {
	g.methods[m.GoName] = m
}

func (g *Generator) walkMessage(m *protogen.Message) {
	for _, o := range m.Oneofs {
		g.walkOneof(o)
	}
	for _, e := range m.Enums {
		g.walkEnum(e)
	}
	for _, nested := range m.Messages {
		g.walkMessage(nested)
	}
	for _, f := range m.Fields {
		g.walkField(f)
	}
	g.messages[m.GoIdent.GoName] = m
}

func (g *Generator) walkEnum(e *protogen.Enum) {
	g.enums[e.GoIdent.GoName] = e
}

func (g *Generator) walkOneof(o *protogen.Oneof) {
	g.oneofs[o.GoName] = o
}

func (g *Generator) walkField(f *protogen.Field) {
	g.fields[f.GoName] = f
}

func (g *Generator) getTemplateFS() (fs.FS, error) {
	tFS := os.DirFS(".")
	return fs.Sub(tFS, g.TemplateDir)
}

// registerAllExtensions recursively registers all extensions in the given descriptors.
func registerAllExtensions(extTypes *protoregistry.Types, descs interface {
	Messages() protoreflect.MessageDescriptors
	Extensions() protoreflect.ExtensionDescriptors
}) error {
	mds := descs.Messages()
	for i := 0; i < mds.Len(); i++ {
		if err := registerAllExtensions(extTypes, mds.Get(i)); err != nil {
			return err
		}
	}
	xds := descs.Extensions()
	for i := 0; i < xds.Len(); i++ {
		if err := extTypes.RegisterExtension(dynamicpb.NewExtensionType(xds.Get(i))); err != nil {
			return err
		}
	}
	return nil
}

// isNonTemplateFile returns true if the file should be skipped during template processing.
// This includes helper files like funcs.star, prototxt data files, etc.
func isNonTemplateFile(path string) bool {
	ext := filepath.Ext(path)
	switch ext {
	case ".star", ".prototxt", ".textproto", ".json", ".yaml", ".yml":
		return true
	}
	// Also skip files that start with underscore (partial templates)
	base := filepath.Base(path)
	if strings.HasPrefix(base, "_") {
		return true
	}
	return false
}

func extractMetadataFromPath(path string) map[string]string {
	metadata := map[string]string{
		"type": "file", // Default to "file" if no metadata is found
	}

	// Dynamically determine the entity type based on the placeholders in the path
	if strings.Contains(path, "{{.Method.") {
		metadata["type"] = "method"
	} else if strings.Contains(path, "{{.Message.") {
		if strings.Count(path, "{{.Message.") > 1 {
			metadata["type"] = "nestedMessage"
		} else {
			metadata["type"] = "message"
		}
	} else if strings.Contains(path, "{{.Oneof.") {
		metadata["type"] = "oneof"
	} else if strings.Contains(path, "{{.Enum.") {
		metadata["type"] = "enum"
	} else if strings.Contains(path, "{{.Service.") {
		metadata["type"] = "service"
	} else if strings.Contains(path, "{{.File.") {
		metadata["type"] = "file"
	} else if strings.Contains(path, "{{.Field.") {
		metadata["type"] = "field"
	}

	return metadata
}

// registerAllMessages recursively registers all messages in the given descriptors.
func registerAllMessages(extTypes *protoregistry.Types, descs interface {
	Messages() protoreflect.MessageDescriptors
}) error {
	mds := descs.Messages()
	for i := 0; i < mds.Len(); i++ {
		md := mds.Get(i)
		if _, err := extTypes.FindMessageByName(md.FullName()); err != nil {
			if err := extTypes.RegisterMessage(dynamicpb.NewMessageType(md)); err != nil {
				return err
			}
		}
		if err := registerAllMessages(extTypes, md); err != nil {
			return err
		}
	}
	return nil
}
