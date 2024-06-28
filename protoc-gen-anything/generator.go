package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig"
	"go.uber.org/zap"
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

	Logger *zap.Logger

	types *protoregistry.Types

	files          map[string]*protogen.File
	services       map[string]*protogen.Service
	methods        map[string]*protogen.Method
	messages       map[string]*protogen.Message
	enums          map[string]*protogen.Enum
	oneofs         map[string]*protogen.Oneof
	fields         map[string]*protogen.Field
	generatedFiles map[string]*GeneratedFileBuffer
}

type Options struct {
	TemplateDir     string
	Verbose         bool
	ContinueOnError bool
	Logger          *zap.Logger
}

// NewGenerator creates a new protoc-gen-anything generator.
func NewGenerator(o Options) *Generator {
	return &Generator{
		TemplateDir:     o.TemplateDir,
		Verbose:         o.Verbose,
		ContinueOnError: o.ContinueOnError,
		Logger:          o.Logger,

		types:          new(protoregistry.Types),
		files:          make(map[string]*protogen.File),
		services:       make(map[string]*protogen.Service),
		methods:        make(map[string]*protogen.Method),
		messages:       make(map[string]*protogen.Message),
		enums:          make(map[string]*protogen.Enum),
		oneofs:         make(map[string]*protogen.Oneof),
		fields:         make(map[string]*protogen.Field),
		generatedFiles: make(map[string]*GeneratedFileBuffer),
	}
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

func (g *Generator) logVerbose(args ...interface{}) {
	if g.Verbose {
		g.Logger.Sugar().Info(args...)
	}
}

func (g *Generator) logVerbosef(format string, args ...interface{}) {
	if g.Verbose {
		g.Logger.Sugar().Infof(format, args...)
	}
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
	// g.Logger.Debug("Applying templates for entity type", zap.String("entityType", entityType))

	err := fs.WalkDir(tFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("failed to walk template dir: %w", err)
		}
		if d.IsDir() {
			return nil
		}

		metadata := extractMetadataFromPath(path)
		if metadata["type"] != entityType {
			return nil
		}

		return g.processTemplate(path, file, service, method, message, enum, oneof, field, tFS, gen)
	})

	if err != nil {
		g.Logger.Error("Failed to apply templates", zap.Error(err), zap.Bool("continueOnError", g.ContinueOnError))
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
		g.Logger.Error("Failed to expand path", zap.Error(err), zap.String("path", path))
		return fmt.Errorf("failed to expand path: %w", err)
	}

	// Skip if the outputFileName is just a directory
	if outputFileName == "" || outputFileName[len(outputFileName)-1] == '/' {
		//g.Logger.Debug("Skipping empty or directory-only output", zap.String("outputFileName", outputFileName))
		return nil
	}

	g.Logger.Info("Generating file", zap.String("outputFileName", outputFileName))

	fileBuffer, exists := g.generatedFiles[outputFileName]
	if !exists {
		fileBuffer = &GeneratedFileBuffer{IsFirstGen: true}
		g.generatedFiles[outputFileName] = fileBuffer
	}

	context := g.determineContext(file, service, method, message, enum, oneof, field)
	context.IsFirstGeneration = fileBuffer.IsFirstGen
	context.OutputFileName = outputFileName
	context.TemplateFileName = path

	// use protojson:
	g.Logger.Debug("processTemplate",
		zap.String("path", path),
	)

	templateContent, err := g.readTemplateContent(tFS, path)
	if err != nil {
		return err
	}

	tmpl, err := g.parseTemplate(path, templateContent)
	if err != nil {
		return err
	}

	var tempBuffer bytes.Buffer
	err = tmpl.Execute(&tempBuffer, context)
	if err != nil {
		g.Logger.Error("Failed to execute template",
			zap.Error(err),
			zap.String("path", path),
			zap.String("outputFileName", outputFileName))
		return err
	}

	behavior := g.detectTemplateBehavior(tempBuffer.String())

	if err := g.applyTemplateBehavior(behavior, fileBuffer, &tempBuffer); err != nil {
		return err
	}

	fileBuffer.IsFirstGen = false

	g.Logger.Info("Successfully processed template for file", zap.String("outputFileName", outputFileName))

	return nil
}

func (g *Generator) expandPath(pathTemplate string, file *protogen.File, service *protogen.Service, method *protogen.Method, message *protogen.Message, enum *protogen.Enum, oneof *protogen.Oneof, field *protogen.Field) (string, error) {
	tmpl, err := template.New("path").
		Funcs(sprig.TxtFuncMap()).
		Funcs(g.funcMap()).
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

// deref returns the underlying type of a pointer type, or false
func deref[T any](p *T) any {
	if p == nil {
		return false
	}
	return *p
}

func (g *Generator) readTemplateContent(tFS fs.FS, path string) (string, error) {
	templateContent, err := fs.ReadFile(tFS, path)
	if err != nil {
		g.Logger.Error("Failed to read template file", zap.Error(err), zap.String("path", path))
		return "", fmt.Errorf("failed to read template file: %w", err)
	}

	g.Logger.Debug("Template content", zap.String("path", path), zap.String("content", string(templateContent)))
	return string(templateContent), nil
}

func (g *Generator) parseTemplate(path string, templateContent string) (*template.Template, error) {
	tmpl, err := template.New(path).
		Funcs(sprig.TxtFuncMap()).
		Funcs(g.funcMap()).
		Parse(templateContent)
	if err != nil {
		g.Logger.Error("Failed to parse template", zap.Error(err), zap.String("path", path))
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	g.Logger.Debug("Template parsed successfully",
		zap.String("path", path),
		zap.Strings("definedTemplates", strings.Split(tmpl.DefinedTemplates(), "; ")))

	return tmpl, nil
}

func (g *Generator) detectTemplateBehavior(content string) string {
	if strings.Contains(content, "// GENERATION_BEHAVIOR: append") {
		return "append"
	} else if strings.Contains(content, "// GENERATION_BEHAVIOR: overwrite") {
		return "overwrite"
	} else if strings.Contains(content, "// GENERATION_BEHAVIOR: error_on_conflict") {
		return "error_on_conflict"
	}
	return "append" // Default behavior
}
func (g *Generator) applyTemplateBehavior(behavior string, fileBuffer *GeneratedFileBuffer, tempBuffer *bytes.Buffer) error {
	switch behavior {
	case "append":
		fileBuffer.Content.Write(tempBuffer.Bytes())
	case "overwrite":
		fileBuffer.Content.Reset()
		fileBuffer.Content.Write(tempBuffer.Bytes())
	case "error_on_conflict":
		if !fileBuffer.IsFirstGen {
			return fmt.Errorf("conflict detected for file")
		}
		fileBuffer.Content.Write(tempBuffer.Bytes())
	default:
		// Default to append
		fileBuffer.Content.Write(tempBuffer.Bytes())
	}
	return nil
}

func (g *Generator) finalizeGeneration(gen *protogen.Plugin) error {
	for outputFileName, buffer := range g.generatedFiles {
		generatedFile := gen.NewGeneratedFile(outputFileName, "")
		_, err := generatedFile.Write(buffer.Content.Bytes())
		if err != nil {
			return fmt.Errorf("failed to write generated file %s: %w", outputFileName, err)
		}
	}
	return nil
}

func (g *Generator) walkSchemas(gen *protogen.Plugin) error {
	for _, f := range gen.Files {
		if !f.Generate {
			continue
		}
		g.walkFile(f)
	}
	return nil
}

func (g *Generator) walkFile(f *protogen.File) {
	if err := registerAllExtensions(g.types, f.Desc); err != nil {
		g.Logger.Error("Failed to register extensions", zap.Error(err))
	}
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

func (g *Generator) renderTemplate(templateName string, service *protogen.Service, generatedFile *protogen.GeneratedFile, funcMap template.FuncMap) error {
	tFS, err := g.getTemplateFS()
	if err != nil {
		return err
	}
	t := template.New(templateName).
		Funcs(sprig.TxtFuncMap()).
		Funcs(g.funcMap()).
		Funcs(funcMap)
	t, err = t.ParseFS(tFS, templateName)
	if err != nil {
		return err
	}
	return t.Execute(generatedFile, service)
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
