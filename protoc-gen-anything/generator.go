package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/pluginpb"
)

// Generator holds configuration for an invocation of this tool
type Generator struct {
	TemplateDir     string
	Verbose         bool
	ContinueOnError bool
	SourceRelative  bool

	types *protoregistry.Types

	files    map[string]*protogen.File
	services map[string]*protogen.Service
	methods  map[string]*protogen.Method
	messages map[string]*protogen.Message
	enums    map[string]*protogen.Enum
	oneofs   map[string]*protogen.Oneof
	fields   map[string]*protogen.Field
}

type Options struct {
	TemplateDir     string
	Verbose         bool
	ContinueOnError bool
	SourceRelative  bool
}

// NewGenerator creates a new protoc-gen-anything generator.
func NewGenerator(o Options) *Generator {
	return &Generator{
		TemplateDir:     o.TemplateDir,
		Verbose:         o.Verbose,
		ContinueOnError: o.ContinueOnError,
		SourceRelative:  o.SourceRelative,

		types:    new(protoregistry.Types),
		files:    make(map[string]*protogen.File),
		services: make(map[string]*protogen.Service),
		methods:  make(map[string]*protogen.Method),
		messages: make(map[string]*protogen.Message),
		enums:    make(map[string]*protogen.Enum),
		oneofs:   make(map[string]*protogen.Oneof),
		fields:   make(map[string]*protogen.Field),
	}
}

func (g *Generator) Generate(gen *protogen.Plugin) error {
	gen.SupportedFeatures = uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)
	if err := g.walkSchemas(gen); err != nil {
		return err
	}
	return g.generate(gen)
}

func (g *Generator) logVerbose(args ...interface{}) {
	if g.Verbose {
		fmt.Fprintln(os.Stderr, args...)
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
		// Generate code for file level
		err = g.generateForFile(f, tFS, gen)
		if err != nil {
			return err
		}
		for _, s := range f.Services {
			// Generate code for service level
			err = g.generateForService(f, s, tFS, gen)
			if err != nil {
				return err
			}
			for _, m := range s.Methods {
				// Generate code for method level
				err = g.generateForMethod(f, s, m, tFS, gen)
				if err != nil {
					return err
				}
			}
		}
		for _, msg := range f.Messages {
			// Generate code for message level
			err = g.generateForMessage(f, msg, tFS, gen)
			if err != nil {
				return err
			}
			for _, oneof := range msg.Oneofs {
				// Generate code for oneof level
				err = g.generateForOneof(f, msg, oneof, tFS, gen)
				if err != nil {
					return err
				}
			}
			for _, field := range msg.Fields {
				// Generate code for field level
				err = g.generateForField(f, msg, field, tFS, gen)
				if err != nil {
					return err
				}
			}
			for _, enum := range msg.Enums {
				// Generate code for enum level
				err = g.generateForEnum(f, enum, tFS, gen)
				if err != nil {
					return err
				}
			}
			// Generate code for nested message level:
			for _, nestedMsg := range msg.Messages {
				err = g.generateForNestedMessage(f, msg, nestedMsg, tFS, gen)
				if err != nil {
					return err
				}
			}
		}
		for _, enum := range f.Enums {
			// Generate code for enum level
			err = g.generateForEnum(f, enum, tFS, gen)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (g *Generator) generateForFile(f *protogen.File, tFS fs.FS, gen *protogen.Plugin) error {
	g.logVerbose("generating for file:", f.Desc.Path())
	context := determineContext("file", f, nil, nil, nil, nil, nil, nil)
	return g.applyTemplates("file", f, nil, nil, nil, nil, nil, nil, context, tFS, gen)
}

func (g *Generator) generateForService(f *protogen.File, s *protogen.Service, tFS fs.FS, gen *protogen.Plugin) error {
	g.logVerbose("generating for service:", s.GoName)
	context := determineContext("service", f, s, nil, nil, nil, nil, nil)
	return g.applyTemplates("service", f, s, nil, nil, nil, nil, nil, context, tFS, gen)
}

func (g *Generator) generateForMethod(f *protogen.File, s *protogen.Service, m *protogen.Method, tFS fs.FS, gen *protogen.Plugin) error {
	g.logVerbose("generating for method:", m.GoName)
	context := determineContext("method", f, s, m, nil, nil, nil, nil)
	return g.applyTemplates("method", f, s, m, nil, nil, nil, nil, context, tFS, gen)
}

func (g *Generator) generateForMessage(f *protogen.File, msg *protogen.Message, tFS fs.FS, gen *protogen.Plugin) error {
	g.logVerbose("generating for message:", msg.GoIdent.GoName)
	context := determineContext("message", f, nil, nil, msg, nil, nil, nil)
	return g.applyTemplates("message", f, nil, nil, msg, nil, nil, nil, context, tFS, gen)
}

func (g *Generator) generateForEnum(f *protogen.File, enum *protogen.Enum, tFS fs.FS, gen *protogen.Plugin) error {
	g.logVerbose("generating for enum:", enum.GoIdent.GoName)
	context := determineContext("enum", f, nil, nil, nil, enum, nil, nil)
	return g.applyTemplates("enum", f, nil, nil, nil, enum, nil, nil, context, tFS, gen)
}

func (g *Generator) generateForOneof(f *protogen.File, msg *protogen.Message, oneof *protogen.Oneof, tFS fs.FS, gen *protogen.Plugin) error {
	g.logVerbose("generating for oneof:", oneof.GoName)
	context := determineContext("oneof", f, nil, nil, msg, nil, oneof, nil)
	return g.applyTemplates("oneof", f, nil, nil, msg, nil, oneof, nil, context, tFS, gen)
}

func (g *Generator) generateForField(f *protogen.File, msg *protogen.Message, field *protogen.Field, tFS fs.FS, gen *protogen.Plugin) error {
	g.logVerbose("generating for field:", field.GoName)
	context := determineContext("field", f, nil, nil, msg, nil, nil, field)
	return g.applyTemplates("field", f, nil, nil, msg, nil, nil, field, context, tFS, gen)
}

func (g *Generator) generateForNestedMessage(f *protogen.File, parentMsg *protogen.Message, nestedMsg *protogen.Message, tFS fs.FS, gen *protogen.Plugin) error {
	g.logVerbose("generating for nested message:", nestedMsg.GoIdent.GoName)
	context := determineContext("nestedMessage", f, nil, nil, nestedMsg, nil, nil, nil)
	return g.applyTemplates("nestedMessage", f, nil, nil, nestedMsg, nil, nil, nil, context, tFS, gen)
}

func (g *Generator) applyTemplates(entityType string, file *protogen.File, service *protogen.Service, method *protogen.Method, message *protogen.Message, enum *protogen.Enum, oneof *protogen.Oneof, field *protogen.Field, context any, tFS fs.FS, gen *protogen.Plugin) error {
	var err error

	// Walk through the template directory for each specific entity type
	err = fs.WalkDir(tFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("failed to walk template dir: %w", err)
		}
		if d.IsDir() {
			return nil
		}

		// Parse the template path to identify placeholders
		metadata := extractMetadataFromPath(path)
		if metadata["type"] != entityType {
			return nil
		}

		// Generate the output file path by replacing placeholders with actual values
		outputFileName, err := expandPath(path, file, service, method, message, enum, oneof, field)
		if err != nil {
			return fmt.Errorf("failed to expand path: %w", err)
		}
		g.logVerbose("generating file:", outputFileName)
		g.logVerbose("numext:", g.types.NumExtensions())
		generatedFile := gen.NewGeneratedFile(outputFileName, "")

		// Parse the template
		tmpl, err := template.New(path).
			Funcs(sprig.TxtFuncMap()).
			Funcs(g.funcMap()).
			ParseFS(tFS, path)
		if err != nil {
			return fmt.Errorf("failed to parse template: %w", err)
		}

		// Apply the template with the context
		//g.logVerbose("applying template:", path, "for", metadata["type"], outputFileName)
		err = tmpl.Execute(generatedFile, context)
		if err != nil {
			err = fmt.Errorf("failed to execute template: %w", err)
			if g.ContinueOnError {
				fmt.Fprintln(os.Stderr, err)
				return nil
			}
			return err
		}
		return nil
	})
	if err != nil {
		err = fmt.Errorf("failed to apply specific templates: %w", err)
		if g.ContinueOnError {
			fmt.Fprintln(os.Stderr, err)
			return nil
		}
		return err
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

func expandPath(pathTemplate string, file *protogen.File, service *protogen.Service, method *protogen.Method, message *protogen.Message, enum *protogen.Enum, oneof *protogen.Oneof, field *protogen.Field) (string, error) {
	tmpl, err := template.New("path").Parse(pathTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	// Create a context with all possible entities
	context := map[string]interface{}{
		"File":    file,
		"Service": service,
		"Method":  method,
		"Message": message,
		"Enum":    enum,
		"Oneof":   oneof,
		"Field":   field,
	}

	// Execute the template with the provided context
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, context)
	if err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return cleanPath(buf.String()), nil
}

// cleanPath removes .tmpl extension and replaces double slashes with single slash
func cleanPath(path string) string {
	return strings.TrimSuffix(path, ".tmpl")
}

func determineContext(entityType string, file *protogen.File, service *protogen.Service, method *protogen.Method, message *protogen.Message, enum *protogen.Enum, oneof *protogen.Oneof, field *protogen.Field) map[string]interface{} {
	context := map[string]interface{}{
		"File":    file,
		"Service": service,
		"Method":  method,
		"Message": message,
		"Enum":    enum,
		"Oneof":   oneof,
		"Field":   field,
	}
	return context
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
		fmt.Fprintln(os.Stderr, "Failed to register extensions:", err)
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

func (o *Generator) getTemplateFS() (fs.FS, error) {
	tFS := os.DirFS(".")
	return fs.Sub(tFS, o.TemplateDir)
}

func (o *Generator) renderTemplate(templateName string, service *protogen.Service, g *protogen.GeneratedFile, funcMap template.FuncMap) error {
	tFS, err := o.getTemplateFS()
	if err != nil {
		return err
	}
	t := template.New(templateName).
		Funcs(sprig.TxtFuncMap()).
		Funcs(o.funcMap()).
		Funcs(funcMap)
	t, err = t.ParseFS(tFS, templateName)
	if err != nil {
		return err
	}
	return t.Execute(g, service)
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
		fmt.Fprintln(os.Stderr, "Registered extension:", xds.Get(i).FullName(), "current:", extTypes.NumExtensions())
	}
	return nil
}
