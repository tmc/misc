package main

import (
	"flag"
	"text/template"

	"github.com/tmc/misc/protoc-gen-anything/examples/jsonschema/jsonschemagen"
	"google.golang.org/protobuf/compiler/protogen"
)

func main() {
	var flags flag.FlagSet
	templateDir := flags.String("templates", "templates", "path to custom templates")
	includeNullable := flags.Bool("nullable", false, "set whether fields should be nullable by default")
	embedDefinitions := flags.Bool("embed_defs", false, "whether to embed definitions in the schema or use references")
	
	opts := protogen.Options{
		ParamFunc: flags.Set,
	}
	
	opts.Run(func(p *protogen.Plugin) error {
		g := NewGenerator(*templateDir, *includeNullable, *embedDefinitions)
		return g.Generate(p)
	})
}

// Generator extends the protoc-gen-anything generator with JSONSchema capabilities
type Generator struct {
	*protogen.Plugin
	TemplateDir      string
	IncludeNullable  bool
	EmbedDefinitions bool
}

// NewGenerator creates a new generator with JSONSchema support
func NewGenerator(templateDir string, includeNullable, embedDefinitions bool) *Generator {
	return &Generator{
		TemplateDir:      templateDir,
		IncludeNullable:  includeNullable,
		EmbedDefinitions: embedDefinitions,
	}
}

// Generate processes proto files and generates JSONSchema
func (g *Generator) Generate(p *protogen.Plugin) error {
	p.SupportedFeatures = uint64(1) // FEATURE_PROTO3_OPTIONAL
	
	// For each file, iterate through messages and generate schemas
	for _, file := range p.Files {
		if !file.Generate {
			continue
		}
		
		// Process all messages
		for _, msg := range file.Messages {
			outputName := msg.GoIdent.GoName + ".schema.json"
			generatedFile := p.NewGeneratedFile(outputName, "")
			
			// Create a custom template for this message
			tmpl := template.New(outputName)
			
			// Add our custom JSONSchema funcs
			funcMap := template.FuncMap{}
			funcMap = jsonschemagen.AddTemplateFuncs(funcMap, g.IncludeNullable, g.EmbedDefinitions)
			tmpl.Funcs(funcMap)
			
			// Parse the schema template
			_, err := tmpl.Parse(`{{- $jsonSchema := generateJSONSchema .Message -}}
{{ $jsonSchema }}`)
			if err != nil {
				return err
			}
			
			// Execute the template with the message as context
			context := struct {
				Message *protogen.Message
			}{
				Message: msg,
			}
			
			if err := tmpl.Execute(generatedFile, context); err != nil {
				return err
			}
		}
	}
	
	return nil
}