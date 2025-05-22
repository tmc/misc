package jsonschemagen

import (
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// JSONSchemaGenerator contains settings for JSONSchema generation
type JSONSchemaGenerator struct {
	IncludeNullable  bool
	EmbedDefinitions bool
	Definitions      map[string]interface{}
}

// NewJSONSchemaGenerator creates a new JSONSchemaGenerator with default options
func NewJSONSchemaGenerator(includeNullable, embedDefinitions bool) *JSONSchemaGenerator {
	return &JSONSchemaGenerator{
		IncludeNullable:  includeNullable,
		EmbedDefinitions: embedDefinitions,
		Definitions:      make(map[string]interface{}),
	}
}

// GenerateSchema generates a JSONSchema for the given message
func (g *JSONSchemaGenerator) GenerateSchema(msg *protogen.Message) (map[string]interface{}, error) {
	schema := make(map[string]interface{})
	schema["$schema"] = "http://json-schema.org/draft-07/schema#"
	schema["title"] = msg.GoIdent.GoName
	description := strings.TrimSpace(string(msg.Comments.Leading))
	if description != "" {
		schema["description"] = description
	}
	schema["type"] = "object"

	properties := make(map[string]interface{})
	required := []string{}

	for _, field := range msg.Fields {
		fieldName := string(field.Desc.JSONName())
		fieldSchema, err := g.generateFieldSchema(field)
		if err != nil {
			return nil, err
		}

		properties[fieldName] = fieldSchema

		// If field is required (proto3 has all fields as optional by default)
		if !field.Desc.HasOptionalKeyword() && !field.Desc.IsMap() && !field.Desc.IsList() {
			required = append(required, fieldName)
		}
	}

	schema["properties"] = properties
	if len(required) > 0 {
		schema["required"] = required
	}

	// Add definitions if not embedding
	if !g.EmbedDefinitions && len(g.Definitions) > 0 {
		schema["definitions"] = g.Definitions
	}

	return schema, nil
}

// generateFieldSchema creates the schema for an individual field
func (g *JSONSchemaGenerator) generateFieldSchema(field *protogen.Field) (map[string]interface{}, error) {
	schema := make(map[string]interface{})

	// Add description if available
	description := strings.TrimSpace(string(field.Comments.Leading))
	if description != "" {
		schema["description"] = description
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
			refName := field.Message.GoIdent.GoName
			reference := make(map[string]interface{})
			reference["$ref"] = fmt.Sprintf("#/definitions/%s", refName)
			
			// If this is a new definition we haven't seen before, add it
			if _, exists := g.Definitions[refName]; !exists {
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
		enumSchema := make(map[string]interface{})
		enumSchema["type"] = "string"
		
		enumValues := []string{}
		for _, value := range field.Enum.Values {
			// Skip the unspecified value (typically the 0 value)
			if !strings.Contains(strings.ToLower(string(value.Desc.Name())), "unspecified") {
				enumValues = append(enumValues, string(value.Desc.Name()))
			}
		}
		enumSchema["enum"] = enumValues
		
		// Add description
		enumDesc := strings.TrimSpace(string(field.Enum.Comments.Leading))
		if enumDesc != "" {
			enumSchema["description"] = enumDesc
		}
		
		return enumSchema, nil
	}

	// Handle scalar fields
	return g.generateScalarFieldType(field)
}

// generateScalarFieldType converts protobuf scalar types to JSONSchema types
func (g *JSONSchemaGenerator) generateScalarFieldType(field *protogen.Field) (map[string]interface{}, error) {
	schema := make(map[string]interface{})

	switch field.Desc.Kind() {
	case protoreflect.StringKind:
		schema["type"] = "string"
	case protoreflect.BytesKind:
		schema["type"] = "string"
		schema["format"] = "byte"
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
		schema["type"] = "integer"
		schema["format"] = "int64"
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		schema["type"] = "integer"
		schema["format"] = "uint64"
		schema["minimum"] = 0
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

// GenerateJSONString generates a JSON string from the schema
func (g *JSONSchemaGenerator) GenerateJSONString(msg *protogen.Message, indent bool) (string, error) {
	schema, err := g.GenerateSchema(msg)
	if err != nil {
		return "", err
	}

	var bytes []byte
	if indent {
		bytes, err = json.MarshalIndent(schema, "", "  ")
	} else {
		bytes, err = json.Marshal(schema)
	}
	
	if err != nil {
		return "", err
	}
	
	return string(bytes), nil
}