package jsonschema

import (
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Generator contains settings for JSONSchema generation
type Generator struct {
	// Configuration options
	IncludeNullable            bool
	EmbedDefinitions           bool
	AllFieldsRequired          bool
	DisallowAdditionalProps    bool
	JSONFieldnames             bool
	EnumsAsConstants           bool
	FileExtension              string
	PrefixWithPackage          bool
	BigIntsAsStrings           bool
	IncludeRequiredOneOfFields bool

	// Internal tracking
	Definitions    map[string]interface{}
	generatedTypes map[string]bool
}

// NewGenerator creates a new Generator with default options
func NewGenerator(includeNullable, embedDefinitions bool) *Generator {
	return &Generator{
		IncludeNullable:            includeNullable,
		EmbedDefinitions:           embedDefinitions,
		AllFieldsRequired:          false,
		DisallowAdditionalProps:    false,
		JSONFieldnames:             true,
		EnumsAsConstants:           false,
		FileExtension:              "json",
		PrefixWithPackage:          false,
		BigIntsAsStrings:           true,
		IncludeRequiredOneOfFields: false,
		Definitions:                make(map[string]interface{}),
		generatedTypes:             make(map[string]bool),
	}
}

// GenerateSchema generates a JSONSchema for the given message
func (g *Generator) GenerateSchema(msg *protogen.Message) (map[string]interface{}, error) {
	schema := make(map[string]interface{})
	schema["$schema"] = "http://json-schema.org/draft-07/schema#"
	
	// Extract title and description from leading comments
	comment := strings.TrimSpace(string(msg.Comments.Leading))
	if comment != "" {
		parts := strings.Split(comment, "\n\n")
		if len(parts) > 1 {
			// First part is title, rest is description
			schema["title"] = strings.TrimSpace(parts[0])
			schema["description"] = strings.TrimSpace(strings.Join(parts[1:], "\n\n"))
		} else {
			schema["description"] = comment
		}
	}
	
	// If no title was found in the comments, use the message name
	if _, hasTitle := schema["title"]; !hasTitle {
		schema["title"] = msg.GoIdent.GoName
	}
	
	schema["type"] = "object"
	
	// Optionally disallow additional properties
	if g.DisallowAdditionalProps {
		schema["additionalProperties"] = false
	}

	properties := make(map[string]interface{})
	required := []string{}

	// Process oneofs first to track them
	oneofs := make(map[string][]string)
	for _, oneof := range msg.Oneofs {
		if oneof.Desc.IsSynthetic() {
			continue // Skip synthetic oneofs (created for proto3 optional fields)
		}
		
		oneofFieldNames := []string{}
		for _, field := range oneof.Fields {
			fieldName := g.getFieldName(field)
			oneofFieldNames = append(oneofFieldNames, fieldName)
		}
		oneofs[oneof.GoName] = oneofFieldNames
	}

	// Process all fields
	for _, field := range msg.Fields {
		fieldName := g.getFieldName(field)
		fieldSchema, err := g.generateFieldSchema(field)
		if err != nil {
			return nil, err
		}

		properties[fieldName] = fieldSchema

		// Determine if field should be required
		isRequired := g.AllFieldsRequired || (!field.Desc.HasOptionalKeyword() && !field.Desc.IsMap() && !field.Desc.IsList())
		
		// For oneof fields, they're typically not all required
		isInOneof := false
		for _, oneofFields := range oneofs {
			for _, name := range oneofFields {
				if name == fieldName {
					isInOneof = true
					break
				}
			}
			if isInOneof {
				break
			}
		}
		
		// Only mark as required if not in a oneof (unless configured to include oneof fields)
		if isRequired && (!isInOneof || g.IncludeRequiredOneOfFields) {
			required = append(required, fieldName)
		}
	}

	schema["properties"] = properties
	if len(required) > 0 {
		schema["required"] = required
	}

	// Handle oneofs by adding them as "oneOf" constraints
	if len(oneofs) > 0 && !g.IncludeRequiredOneOfFields {
		oneOfSchemas := []map[string]interface{}{}

		for _, fieldNames := range oneofs {
			for _, fieldName := range fieldNames {
				// Create a schema that requires just this field from the oneof
				oneofSchema := map[string]interface{}{
					"required": []string{fieldName},
				}
				oneOfSchemas = append(oneOfSchemas, oneofSchema)
			}
		}
		
		if len(oneOfSchemas) > 0 {
			schema["oneOf"] = oneOfSchemas
		}
	}

	// Add definitions if not embedding
	if !g.EmbedDefinitions && len(g.Definitions) > 0 {
		schema["definitions"] = g.Definitions
	}

	return schema, nil
}

// getFieldName returns the appropriate field name based on settings
func (g *Generator) getFieldName(field *protogen.Field) string {
	if g.JSONFieldnames {
		return string(field.Desc.JSONName())
	}
	return string(field.Desc.Name())
}

// GenerateFieldSchema creates the schema for an individual field
// Exported for testing purposes
func (g *Generator) GenerateFieldSchema(field *protogen.Field) (map[string]interface{}, error) {
	return g.generateFieldSchema(field)
}

// generateFieldSchema creates the schema for an individual field
func (g *Generator) generateFieldSchema(field *protogen.Field) (map[string]interface{}, error) {
	schema := make(map[string]interface{})

	// Add description if available
	comment := strings.TrimSpace(string(field.Comments.Leading))
	if comment != "" {
		parts := strings.Split(comment, "\n\n")
		if len(parts) > 1 {
			// First part is title, rest is description
			schema["title"] = strings.TrimSpace(parts[0])
			schema["description"] = strings.TrimSpace(strings.Join(parts[1:], "\n\n"))
		} else {
			schema["description"] = comment
		}
	}

	// Handle nullable fields
	if g.IncludeNullable && field.Desc.HasOptionalKeyword() {
		schema["nullable"] = true
	}

	// Handle repeated fields (arrays)
	if field.Desc.IsList() {
		schema["type"] = "array"
		items, err := g.generateScalarFieldType(field)
		if err != nil {
			return nil, err
		}
		schema["items"] = items
		return schema, nil
	}

	// Handle maps
	if field.Desc.IsMap() {
		schema["type"] = "object"
		
		// Map values 
		valueField := field.Message.Fields[1] // Maps have key=0, value=1
		valueSchema, err := g.generateScalarFieldType(valueField)
		if err != nil {
			return nil, err
		}
		
		schema["additionalProperties"] = valueSchema
		return schema, nil
	}

	// Handle message fields
	if field.Message != nil && !field.Desc.IsMap() {
		// Handle well-known types
		if schema, handled := g.handleWellKnownType(field); handled {
			return schema, nil
		}
		
		// For embedded messages - either reference them or embed them
		if g.EmbedDefinitions {
			// Generate schema directly
			messageSchema, err := g.GenerateSchema(field.Message)
			if err != nil {
				return nil, err
			}
			delete(messageSchema, "$schema") // Remove top-level schema attribute
			return messageSchema, nil
		} else {
			// Create a reference
			refName := g.getRefName(field.Message)
			reference := make(map[string]interface{})
			reference["$ref"] = fmt.Sprintf("#/definitions/%s", refName)
			
			// If this is a new definition we haven't seen before, add it
			if !g.generatedTypes[refName] {
				g.generatedTypes[refName] = true
				messageSchema, err := g.GenerateSchema(field.Message)
				if err != nil {
					return nil, err
				}
				delete(messageSchema, "$schema") // Remove top-level schema attribute
				delete(messageSchema, "definitions") // Remove definitions to avoid duplication
				g.Definitions[refName] = messageSchema
			}
			
			return reference, nil
		}
	}

	// Handle enum fields
	if field.Enum != nil {
		if g.EnumsAsConstants {
			return g.generateEnumAsConstants(field)
		}
		return g.generateEnumAsStrings(field)
	}

	// Handle scalar fields
	return g.generateScalarFieldType(field)
}

// getRefName returns a reference name for a message
func (g *Generator) getRefName(msg *protogen.Message) string {
	if g.PrefixWithPackage {
		return string(msg.Desc.ParentFile().Package()) + "." + msg.GoIdent.GoName
	}
	return msg.GoIdent.GoName
}

// generateEnumAsConstants generates an enum as a "const" type with enumerated values
func (g *Generator) generateEnumAsConstants(field *protogen.Field) (map[string]interface{}, error) {
	schema := make(map[string]interface{})
	
	// Add description
	enumDesc := strings.TrimSpace(string(field.Enum.Comments.Leading))
	if enumDesc != "" {
		schema["description"] = enumDesc
	}
	
	// Create enum values (both as strings and integers)
	enumValues := []interface{}{}
	for _, value := range field.Enum.Values {
		// Skip the unspecified value (typically the 0 value)
		if !strings.Contains(strings.ToLower(string(value.Desc.Name())), "unspecified") {
			enumValues = append(enumValues, string(value.Desc.Name()))
			enumValues = append(enumValues, int(value.Desc.Number()))
		}
	}
	
	if len(enumValues) > 0 {
		schema["enum"] = enumValues
	}
	
	return schema, nil
}

// generateEnumAsStrings generates an enum as a string type with enumerated values
func (g *Generator) generateEnumAsStrings(field *protogen.Field) (map[string]interface{}, error) {
	schema := make(map[string]interface{})
	schema["type"] = "string"
	
	// Add description
	enumDesc := strings.TrimSpace(string(field.Enum.Comments.Leading))
	if enumDesc != "" {
		schema["description"] = enumDesc
	}
	
	// Create enum values
	enumValues := []string{}
	for _, value := range field.Enum.Values {
		// Skip the unspecified value (typically the 0 value)
		if !strings.Contains(strings.ToLower(string(value.Desc.Name())), "unspecified") {
			enumValues = append(enumValues, string(value.Desc.Name()))
		}
	}
	
	if len(enumValues) > 0 {
		schema["enum"] = enumValues
	}
	
	return schema, nil
}

// handleWellKnownType handles well-known protobuf types
func (g *Generator) handleWellKnownType(field *protogen.Field) (map[string]interface{}, bool) {
	schema := make(map[string]interface{})
	
	messageName := string(field.Message.Desc.FullName())
	
	switch messageName {
	case "google.protobuf.Timestamp":
		schema["type"] = "string"
		schema["format"] = "date-time"
		return schema, true
		
	case "google.protobuf.Duration":
		schema["type"] = "string"
		schema["pattern"] = "^(-?[0-9]+(\\.[0-9]+)?)(s|ms|us|ns)$"
		return schema, true
		
	case "google.protobuf.Any":
		schema["type"] = "object"
		schema["additionalProperties"] = true
		return schema, true
	
	case "google.protobuf.Struct":
		schema["type"] = "object"
		schema["additionalProperties"] = true
		return schema, true
		
	case "google.protobuf.Value":
		// Value can be anything
		return map[string]interface{}{}, true
		
	case "google.protobuf.ListValue":
		schema["type"] = "array"
		schema["items"] = map[string]interface{}{}
		return schema, true
		
	case "google.protobuf.DoubleValue", "google.protobuf.FloatValue":
		schema["type"] = "number"
		return schema, true
		
	case "google.protobuf.Int32Value", "google.protobuf.Int64Value", 
	     "google.protobuf.UInt32Value", "google.protobuf.UInt64Value":
		schema["type"] = "integer"
		if g.BigIntsAsStrings && (messageName == "google.protobuf.Int64Value" || 
		                          messageName == "google.protobuf.UInt64Value") {
			schema["type"] = "string"
			schema["pattern"] = "^[0-9]+$"
		}
		return schema, true
		
	case "google.protobuf.BoolValue":
		schema["type"] = "boolean"
		return schema, true
		
	case "google.protobuf.StringValue":
		schema["type"] = "string"
		return schema, true
		
	case "google.protobuf.BytesValue":
		schema["type"] = "string"
		schema["contentEncoding"] = "base64"
		return schema, true
	}
	
	return nil, false
}

// generateScalarFieldType converts protobuf scalar types to JSONSchema types
func (g *Generator) generateScalarFieldType(field *protogen.Field) (map[string]interface{}, error) {
	schema := make(map[string]interface{})

	switch field.Desc.Kind() {
	case protoreflect.StringKind:
		schema["type"] = "string"
	case protoreflect.BytesKind:
		schema["type"] = "string"
		schema["contentEncoding"] = "base64"
	case protoreflect.BoolKind:
		schema["type"] = "boolean"
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		schema["type"] = "integer"
		schema["format"] = "int32"
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		schema["type"] = "integer"
		schema["format"] = "uint32"
		schema["minimum"] = 0
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		if g.BigIntsAsStrings {
			schema["type"] = "string"
			schema["pattern"] = "^-?[0-9]+$"
		} else {
			schema["type"] = "integer"
			schema["format"] = "int64"
		}
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		if g.BigIntsAsStrings {
			schema["type"] = "string"
			schema["pattern"] = "^[0-9]+$"
		} else {
			schema["type"] = "integer"
			schema["format"] = "uint64"
			schema["minimum"] = 0
		}
	case protoreflect.FloatKind:
		schema["type"] = "number"
		schema["format"] = "float"
	case protoreflect.DoubleKind:
		schema["type"] = "number"
		schema["format"] = "double"
	case protoreflect.MessageKind:
		if field.Message != nil {
			msgSchema, err := g.GenerateSchema(field.Message)
			if err != nil {
				return nil, err
			}
			delete(msgSchema, "$schema") // Remove top-level schema attribute
			return msgSchema, nil
		}
		fallthrough
	default:
		// For unsupported types
		schema["type"] = "object"
	}

	return schema, nil
}

// GenerateJSON generates a JSON representation of the schema
func (g *Generator) GenerateJSON(msg *protogen.Message, indent bool) ([]byte, error) {
	schema, err := g.GenerateSchema(msg)
	if err != nil {
		return nil, err
	}

	if indent {
		return json.MarshalIndent(schema, "", "  ")
	}
	return json.Marshal(schema)
}

// GenerateJSONString generates a JSON string from the schema
func (g *Generator) GenerateJSONString(msg *protogen.Message, indent bool) (string, error) {
	jsonBytes, err := g.GenerateJSON(msg, indent)
	if err != nil {
		return "", err
	}
	return string(jsonBytes), nil
}