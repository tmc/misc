// Package plugins provides extensibility for ts2go
package plugins

import (
	"fmt"
	"strings"
)

// TypeTransformer is a plugin interface for transforming TypeScript types to Go types
type TypeTransformer interface {
	// Name returns the name of the transformer
	Name() string

	// CanTransform returns true if this transformer can handle the given TypeScript type
	CanTransform(typeName string) bool

	// Transform converts a TypeScript type to a Go type
	Transform(typeName string, isOptional bool) (string, bool)

	// GenerateCustomCode returns any custom code that should be added to the Go file
	// for this type, such as methods, constructors, etc.
	GenerateCustomCode(typeName string) string

	// AdditionalImports returns any additional imports needed for this type
	AdditionalImports(typeName string) []string
}

// SpecialType represents a special type that needs custom handling
type SpecialType struct {
	// TypeName is the TypeScript type name to transform
	TypeName string

	// GoTypeName is the Go type name to use
	GoTypeName string

	// FieldDefinitions defines the fields in the Go struct
	FieldDefinitions []FieldDefinition

	// CustomCode is any additional code to add for this type
	CustomCode string

	// Imports are any additional imports needed
	Imports []string

	// InterfaceMethods are methods that implement interfaces
	InterfaceMethods []InterfaceMethod
}

// FieldDefinition defines a field in a Go struct
type FieldDefinition struct {
	Name        string
	Type        string
	JSONName    string
	Optional    bool
	Description string
}

// InterfaceMethod defines a method that implements an interface
type InterfaceMethod struct {
	Name string
	Code string
}

// DefaultTransformer is a default implementation of TypeTransformer
type DefaultTransformer struct {
	// SpecialTypes is a map of TypeScript type names to SpecialType definitions
	SpecialTypes map[string]SpecialType
}

// NewDefaultTransformer creates a new DefaultTransformer with common special types
func NewDefaultTransformer() *DefaultTransformer {
	return &DefaultTransformer{
		SpecialTypes: map[string]SpecialType{
			"RequestId": {
				TypeName:   "RequestId",
				GoTypeName: "RequestID",
				FieldDefinitions: []FieldDefinition{
					{
						Name: "value",
						Type: "interface{}",
					},
				},
				CustomCode: requestIDCustomCode,
				Imports:    []string{"encoding/json", "fmt"},
			},
		},
	}
}

// Name returns the name of the transformer
func (d *DefaultTransformer) Name() string {
	return "DefaultTransformer"
}

// CanTransform returns true if this transformer can handle the given TypeScript type
func (d *DefaultTransformer) CanTransform(typeName string) bool {
	_, ok := d.SpecialTypes[typeName]
	return ok
}

// Transform converts a TypeScript type to a Go type
func (d *DefaultTransformer) Transform(typeName string, isOptional bool) (string, bool) {
	if specialType, ok := d.SpecialTypes[typeName]; ok {
		return specialType.GoTypeName, true
	}
	return "", false
}

// GenerateCustomCode returns any custom code that should be added to the Go file
func (d *DefaultTransformer) GenerateCustomCode(typeName string) string {
	if specialType, ok := d.SpecialTypes[typeName]; ok {
		return specialType.CustomCode
	}
	return ""
}

// AdditionalImports returns any additional imports needed for this type
func (d *DefaultTransformer) AdditionalImports(typeName string) []string {
	if specialType, ok := d.SpecialTypes[typeName]; ok {
		return specialType.Imports
	}
	return nil
}

// RegisterSpecialType registers a special type with the transformer
func (d *DefaultTransformer) RegisterSpecialType(specialType SpecialType) {
	d.SpecialTypes[specialType.TypeName] = specialType
}

// MCPTransformer is a transformer specifically for Model Context Protocol types
type MCPTransformer struct {
	*DefaultTransformer
}

// NewMCPTransformer creates a new MCPTransformer with MCP-specific types
func NewMCPTransformer() *MCPTransformer {
	transformer := &MCPTransformer{
		DefaultTransformer: NewDefaultTransformer(),
	}

	// Register additional MCP-specific types
	transformer.RegisterSpecialType(SpecialType{
		TypeName:   "JSONRPCMessage",
		GoTypeName: "JSONRPCMessage",
		CustomCode: "// JSONRPCMessage represents any valid JSON-RPC object that can be decoded off the wire\n" +
			"type JSONRPCMessage interface {\n" +
			"\tisJSONRPCMessage()\n" +
			"}",
	})

	// Register Content interface
	transformer.RegisterSpecialType(SpecialType{
		TypeName:   "Content",
		GoTypeName: "Content",
		CustomCode: "// Content is an interface for different content types\n" +
			"type Content interface {\n" +
			"\tcontentType() string\n" +
			"}",
	})

	return transformer
}

// Name returns the name of the transformer
func (m *MCPTransformer) Name() string {
	return "MCPTransformer"
}

// transformers is a list of all registered transformers
var transformers []TypeTransformer

// RegisterTransformer registers a transformer
func RegisterTransformer(transformer TypeTransformer) {
	transformers = append(transformers, transformer)
}

// GetTransformers returns all registered transformers
func GetTransformers() []TypeTransformer {
	if len(transformers) == 0 {
		// Register default transformers
		RegisterTransformer(NewDefaultTransformer())
	}
	return transformers
}

// TransformType attempts to transform a TypeScript type using registered transformers
func TransformType(typeName string, isOptional bool) (string, bool) {
	for _, transformer := range GetTransformers() {
		if transformer.CanTransform(typeName) {
			return transformer.Transform(typeName, isOptional)
		}
	}
	return "", false
}

// GetCustomCode returns any custom code for a type
func GetCustomCode(typeName string) string {
	for _, transformer := range GetTransformers() {
		if transformer.CanTransform(typeName) {
			return transformer.GenerateCustomCode(typeName)
		}
	}
	return ""
}

// GetAdditionalImports returns any additional imports for a type
func GetAdditionalImports(typeName string) []string {
	var imports []string
	for _, transformer := range GetTransformers() {
		if transformer.CanTransform(typeName) {
			imports = append(imports, transformer.AdditionalImports(typeName)...)
		}
	}
	return imports
}

// requestIDCustomCode contains the custom code for RequestID
const requestIDCustomCode = `// StringID creates a new string request identifier.
func StringID(s string) RequestID { return RequestID{value: s} }

// Int64ID creates a new integer request identifier.
func Int64ID(i int64) RequestID { return RequestID{value: i} }

// Float64ID creates a new float64 request identifier.
func Float64ID(f float64) RequestID { return RequestID{value: f} }

// IsValid returns true if the ID is a valid identifier.
// The default value for RequestID will return false.
func (id RequestID) IsValid() bool { return id.value != nil }

// String returns a string representation of the ID.
func (id RequestID) String() string {
	if !id.IsValid() {
		return "<invalid>"
	}
	return fmt.Sprintf("%v", id.value)
}

// MarshalJSON implements the json.Marshaler interface.
func (id RequestID) MarshalJSON() ([]byte, error) {
	if !id.IsValid() {
		return []byte("null"), nil
	}
	return json.Marshal(id.value)
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (id *RequestID) UnmarshalJSON(data []byte) error {
	// Try string first
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		id.value = s
		return nil
	}

	// Then try number
	var n float64
	if err := json.Unmarshal(data, &n); err == nil {
		// Check if it's an integer
		if n == float64(int64(n)) {
			id.value = int64(n)
		} else {
			id.value = n
		}
		return nil
	}

	// Finally try null
	var null interface{}
	if err := json.Unmarshal(data, &null); err == nil && null == nil {
		id.value = nil
		return nil
	}

	return fmt.Errorf("invalid request ID format: %s", string(data))
}

// AsInt64 attempts to convert the ID to an int64.
// Returns the value and a boolean indicating if the conversion was successful.
func (id RequestID) AsInt64() (int64, bool) {
	switch v := id.value.(type) {
	case int64:
		return v, true
	case float64:
		if v == float64(int64(v)) {
			return int64(v), true
		}
	}
	return 0, false
}

// AsFloat64 attempts to convert the ID to a float64.
// Returns the value and a boolean indicating if the conversion was successful.
func (id RequestID) AsFloat64() (float64, bool) {
	switch v := id.value.(type) {
	case float64:
		return v, true
	case int64:
		return float64(v), true
	}
	return 0, false
}

// AsString attempts to convert the ID to a string.
// Returns the value and a boolean indicating if the conversion was successful.
func (id RequestID) AsString() (string, bool) {
	if s, ok := id.value.(string); ok {
		return s, true
	}
	return "", false
}`

// CodeGen helps generate Go code
type CodeGen struct {
	// Buffer for code generation
	buffer strings.Builder
}

// NewCodeGen creates a new CodeGen
func NewCodeGen() *CodeGen {
	return &CodeGen{}
}

// AddInterface adds an interface definition
func (g *CodeGen) AddInterface(name string, methods []string, description string) *CodeGen {
	if description != "" {
		g.buffer.WriteString(fmt.Sprintf("// %s\n", description))
	}
	g.buffer.WriteString(fmt.Sprintf("type %s interface {\n", name))
	for _, method := range methods {
		g.buffer.WriteString(fmt.Sprintf("\t%s\n", method))
	}
	g.buffer.WriteString("}\n\n")
	return g
}

// AddStruct adds a struct definition
func (g *CodeGen) AddStruct(name string, fields []FieldDefinition, description string) *CodeGen {
	if description != "" {
		g.buffer.WriteString(fmt.Sprintf("// %s\n", description))
	}
	g.buffer.WriteString(fmt.Sprintf("type %s struct {\n", name))
	for _, field := range fields {
		if field.Description != "" {
			g.buffer.WriteString(fmt.Sprintf("\t// %s\n", field.Description))
		}
		jsonTag := ""
		if field.JSONName != "" {
			if field.Optional {
				jsonTag = fmt.Sprintf(" `json:\"%s,omitempty\"`", field.JSONName)
			} else {
				jsonTag = fmt.Sprintf(" `json:\"%s\"`", field.JSONName)
			}
		}
		g.buffer.WriteString(fmt.Sprintf("\t%s %s%s\n", field.Name, field.Type, jsonTag))
	}
	g.buffer.WriteString("}\n\n")
	return g
}

// AddMethod adds a method
func (g *CodeGen) AddMethod(receiver string, name string, params string, returnType string, body string) *CodeGen {
	g.buffer.WriteString(fmt.Sprintf("func (%s) %s(%s)", receiver, name, params))
	if returnType != "" {
		g.buffer.WriteString(fmt.Sprintf(" %s", returnType))
	}
	g.buffer.WriteString(" {\n")
	g.buffer.WriteString(fmt.Sprintf("\t%s\n", body))
	g.buffer.WriteString("}\n\n")
	return g
}

// AddRawCode adds raw code
func (g *CodeGen) AddRawCode(code string) *CodeGen {
	g.buffer.WriteString(code)
	g.buffer.WriteString("\n\n")
	return g
}

// String returns the generated code
func (g *CodeGen) String() string {
	return g.buffer.String()
}
