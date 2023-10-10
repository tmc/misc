package main

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig"
	"github.com/tmc/misc/protoc-gen-graphql/gqltypes"
	metadata "github.com/tmc/misc/protoc-gen-graphql/proto/graphql/v1"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/pluginpb"
)

// Generator holds configuration for an invocation of this tool
type Generator struct {
	TemplateDir string

	services       map[string]*protogen.Service
	exposedMethods map[string]*protogen.Method

	schemas   map[string]*gqltypes.Schema
	mutations map[string]*gqltypes.Mutation
}

// newGenerator creates a new generator.
func newGenerator(templateDir string) *Generator {
	return &Generator{
		TemplateDir:    templateDir,
		services:       make(map[string]*protogen.Service),
		exposedMethods: make(map[string]*protogen.Method),
		schemas:        make(map[string]*gqltypes.Schema),
		mutations:      make(map[string]*gqltypes.Mutation),
	}
}

func (g *Generator) Generate(gen *protogen.Plugin) error {
	gen.SupportedFeatures = uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)
	if err := g.walkSchemas(gen); err != nil {
		return err
	}
	return g.printSchemas(gen)
}

//go:embed templates/*
var defaultTemplates embed.FS

func (o *Generator) getTemplateFS() (fs.FS, error) {
	if o.TemplateDir == "" {
		return fs.Sub(defaultTemplates, "templates")
	}
	tFS := os.DirFS(o.TemplateDir)
	return fs.Sub(tFS, o.TemplateDir)
}

func (o *Generator) renderTemplate(templateName string, service *protogen.Service, g *protogen.GeneratedFile, funcMap template.FuncMap) error {
	tFS, err := o.getTemplateFS()
	if err != nil {
		return err
	}
	t := template.New(templateName).Funcs(funcMap).Funcs(sprig.TxtFuncMap())
	t, err = t.ParseFS(tFS, templateName)
	if err != nil {
		return err
	}
	return t.Execute(g, service)
}

func (g *Generator) walkSchemas(gen *protogen.Plugin) error {
	for _, f := range gen.Files {
		if !f.Generate {
			continue
		}
		for _, svc := range f.Services {
			for _, m := range svc.Methods {
				g.walkMethod(m)
			}
		}
	}
	return nil
}

func (g *Generator) walkMethod(m *protogen.Method) (*gqltypes.Input, bool) {
	if !methodExposed(m) {
		return nil, false
	}

	if _, ok := g.services[string(m.Parent.Desc.FullName())]; !ok {
		g.services[string(m.Parent.Desc.FullName())] = m.Parent
		g.schemas[string(m.Parent.Desc.FullName())] = newSchema()
	}
	schema := g.schemas[string(m.Parent.Desc.FullName())]

	// determine if this is a query or mutation
	gql, ok := getMethodGraphQLOpts(m)

	t := &gqltypes.Field{
		Name:    gql.GetQueryName(),
		Comment: helperComment("", m.Comments.Leading),
		Inputs:  []*gqltypes.Input{g.getInputType(schema, m.Input)},
		Type:    g.getOutputTypeName(m.Output),
	}
	if t.Name == "" {
		t.Name = g.getMethodName(m)
	}
	/*
		e := &gqltypes.Extension{
			Name:    g.getMethodName(m),
			Input:   g.getInputType(schema, m.Input),
			Output:  g.getOutputType(schema, m.Output),
			Comment: helperComment("", m.Comments.Leading),
		}
	*/
	isQuery := ok && gql.GetQueryName() != ""
	if isQuery && gql.GetMutationName() != "" {
		panic(fmt.Sprintf("method %s.%s cannot be both a query and a mutation", m.Parent.Desc.FullName(), m.Desc.Name()))
	}

	isQuery, err := regexp.MatchString(`(?i)(get|list|search)`, t.Name)
	if err != nil {
		panic(err)
	}

	if isQuery {
		schema.RootQuery.Fields = append(schema.RootQuery.Fields, t)
	} else {

		if gql.GetMutationName() != "" {
			t.Name = gql.GetMutationName()
		}
		schema.RootMutation.Fields = append(schema.RootMutation.Fields, t)
	}

	// TODO: handle multiple inputs
	return t.Inputs[0], true
}

func newSchema() *gqltypes.Schema {
	s := &gqltypes.Schema{}
	s.RootQuery = &gqltypes.Type{
		Name: "Query",
	}
	s.RootMutation = &gqltypes.Type{
		Name: "Mutation",
	}
	return s
}

func (g *Generator) getMethodName(m *protogen.Method) string {
	gql, ok := getMethodGraphQLOpts(m)
	if ok && gql.GetQueryName() != "" {
		return gql.GetQueryName()
	}
	if ok && gql.GetMutationName() != "" {
		return gql.GetMutationName()
	}
	n := string(m.Desc.Name())
	n = strings.ToLower(string(n[0])) + string(n[1:])
	return n
}

func (g *Generator) getInputType(s *gqltypes.Schema, m *protogen.Message) *gqltypes.Input {
	o := &gqltypes.Input{
		Name:    g.getInputTypeName(m),
		Fields:  []*gqltypes.Field{},
		Comment: helperComment("", m.Comments.Leading),
	}
	for _, f := range m.Fields {
		o.Fields = append(o.Fields, g.getOutputField(s, f))
	}
	s.Inputs = append(s.Inputs, o)
	return o
}

func (g *Generator) getOutputType(s *gqltypes.Schema, m *protogen.Message) *gqltypes.Type {
	o := &gqltypes.Type{
		Name:       g.getOutputTypeName(m),
		IsRequired: true,
		Fields:     []*gqltypes.Field{},
		Comment:    g.getMessageComment(m),
	}
	for _, f := range m.Fields {
		o.Fields = append(o.Fields, g.getOutputField(s, f))
	}
	found := false
	for _, t := range s.Types {
		if t.Name == o.Name {
			found = true
			break
		}
	}
	if !found {
		s.Types = append(s.Types, o)
	}
	return o
}

func (g *Generator) getMessageComment(m *protogen.Message) string {
	if strings.Contains(string(m.Desc.ParentFile().Package().Name()), "protobuf") {
		return ""
	}
	return string(m.Comments.Leading)

}

func (g *Generator) getOutputField(s *gqltypes.Schema, f *protogen.Field) *gqltypes.Field {
	ts := f.Desc.Kind().String()
	t, ok := typesToGQLType[ts]
	if !ok {
		t = ts
	}
	if f.Message != nil {
		t = string(g.getOutputType(s, f.Message).Name)
	}
	if f.Enum != nil {
		t = g.getEnumType(s, f.Enum)
	}
	// Handle repeated fields
	if isRepeated(f) {
		// TODO(tmc): perhaps this should be a field on the type
		t = fmt.Sprintf("[%s]", t)
	}
	// Handle required fields
	if hasRequiredFieldOption(f) {
		t = fmt.Sprintf("%s!", t)
	}
	o := &gqltypes.Field{
		Name:    string(f.Desc.JSONName()),
		Type:    t,
		Comment: helperComment("", f.Comments.Leading),
	}
	return o
}

func (g *Generator) getEnumType(s *gqltypes.Schema, e *protogen.Enum) string {
	o := &gqltypes.Enum{
		Name:    string(e.Desc.Name()),
		Comment: helperComment("", e.Comments.Leading),
		Options: []*gqltypes.EnumOption{},
	}
	for _, v := range e.Values {
		o.Options = append(o.Options, &gqltypes.EnumOption{
			Name:    string(v.Desc.Name()),
			Comment: helperComment("", v.Comments.Leading),
		})
	}
	found := false
	for _, t := range s.Enums {
		if t.Name == o.Name {
			found = true
			break
		}
	}
	if !found {
		s.Enums = append(s.Enums, o)
	}
	return o.Name
}

func (g *Generator) getInputTypeName(m *protogen.Message) string {
	n := string(m.Desc.Name())
	n = strings.TrimSuffix(n, "Request")
	n = n + "Input"
	g.walkInput(m, n)
	return n
}

func (g *Generator) getOutputTypeName(m *protogen.Message) string {
	g.walkType(m)
	return string(m.Desc.Name())
}

func (g *Generator) walkType(m *protogen.Message) {
	for _, f := range m.Fields {
		g.walkField(f)
	}
}

func (g *Generator) walkInput(m *protogen.Message, name string) {
	for _, f := range m.Fields {
		g.walkField(f)
	}
}

func (g *Generator) walkField(f *protogen.Field) {
	if f.Message != nil {
		g.walkType(f.Message)
	}
}

func methodExposed(m *protogen.Method) bool {
	mops, ok := getMethodGraphQLOpts(m)
	if !ok {
		sops, ok := getServiceGraphQLOpts(m.Parent)
		return ok && sops.GetExposed()
	}
	return ok && mops.GetExposed()
}

func (g *Generator) printSchemas(plugin *protogen.Plugin) error {
	for _, f := range plugin.Files {
		if !f.Generate {
			continue
		}
		for _, svc := range f.Services {
			if err := printServiceSchema(svc, g, plugin); err != nil {
				return err
			}
		}
	}
	return nil
}

func fullNameToService(name string) string {
	return name
}

func printServiceSchema(svc *protogen.Service, opts *Generator, gen *protogen.Plugin) error {
	serviceName := fullNameToService(string(svc.Desc.FullName()))

	gf := gen.NewGeneratedFile(fmt.Sprintf("%s.graphql", serviceName), protogen.GoImportPath(serviceName))

	return opts.renderTemplate("graphql-service-schema.tmpl", svc, gf, template.FuncMap{
		"schema":         opts.helperSchema,
		"exposed":        opts.helperExposed,
		"method_opts":    opts.helperMethodOpts,
		"graphql_name":   opts.helperGraphQLName,
		"graphql_input":  opts.helperGraphQLInput,
		"graphql_output": opts.helperGraphQLOutput,
		// "input_types":    opts.helperInputTypes,
		// "output_types":   opts.helperOutputTypes,
		"graphql_field": opts.helperGraphqlField,
		"comment":       helperComment,
		"fix_comment":   helperFixComment,
	})
}

func (o *Generator) helperExposed(m *protogen.Method) bool {
	mops, ok := getMethodGraphQLOpts(m)
	return ok && mops.GetExposed()
}

func (o *Generator) helperMethodOpts(m *protogen.Method) *metadata.GraphQLOperation {
	// TOOD(tmc): change this work across other types.
	mops, _ := getMethodGraphQLOpts(m)
	return mops
}

func (o *Generator) helperGraphQLName(m *protogen.Method) string {
	// TODO(tmc): encode rules
	n := m.GoName
	// lowercase the first letter:
	return strings.ToLower(string(n[:1])) + n[1:]

}

func (o *Generator) helperGraphQLInput(m *protogen.Message) string {
	return m.GoIdent.GoName
}

func (o *Generator) helperGraphQLOutput(m *protogen.Message) string {
	return m.GoIdent.GoName
}

// func (o *Generator) helperInputTypes() map[string]*protogen.Message {
// }

// func (o *Generator) helperOutputTypes() map[string]*protogen.Message {
// }

func (o *Generator) helperGraphqlField(f *protogen.Field) string {
	return ""
}

func (o *Generator) helperSchema(svc *protogen.Service) *gqltypes.Schema {
	s, ok := o.schemas[string(svc.Desc.FullName())]
	if ok {
		return s
	}
	return newSchema()
}

var typesToGQLType = map[string]string{
	"int32":  "Int",
	"int64":  "Int",
	"bool":   "Boolean",
	"string": "String",
}

var commentRe = regexp.MustCompile("\n// ?")

// regexp to capture api-linter comments:
var apiLinterRe = regexp.MustCompile(`(?s)(\(-- api-linter:.*)`)

func helperComment(name string, s interface{}) string {
	val := strings.TrimLeft(fmt.Sprint(s), "*/\n ")
	if strings.HasPrefix(val, "@exclude") {
		return ""
	}
	c := commentRe.ReplaceAllString(val, "\n")

	// strip out api-linter comments:
	c = apiLinterRe.ReplaceAllString(c, "")
	return helperFixComment(name, c)
}

// helperFixComment fixes the comment to be reflect the lowerCamelCased name.
func helperFixComment(name string, comment string) string {
	if name == "" {
		return comment
	}
	words := strings.Split(strings.TrimSpace(comment), " ")
	firstWord := words[0]
	// convert lowerCamelCase from snake_case
	firstWord = strings.Replace(name, "_", " ", -1)
	firstWord = strings.Title(name)
	firstWord = strings.Replace(name, " ", "", -1)
	if firstWord == name {
		words[0] = name
	}
	return strings.Join(words, " ")
}
