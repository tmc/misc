package graphqlgen

import (
	"strings"
	"text/template"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// AddHelperFunctions adds GraphQL-specific helper functions to the template function map
func AddHelperFunctions(funcMap template.FuncMap) template.FuncMap {
	funcMap["graphqlFieldType"] = getGraphQLFieldType
	funcMap["graphqlInputFieldType"] = getGraphQLInputFieldType
	funcMap["graphqlArgs"] = getGraphQLArgs
	funcMap["graphqlReturn"] = getGraphQLReturn
	
	return funcMap
}

// Map of protobuf scalar types to GraphQL scalar types
var scalarMapping = map[protoreflect.Kind]string{
	protoreflect.BoolKind:    "Boolean",
	protoreflect.Int32Kind:   "Int",
	protoreflect.Sint32Kind:  "Int",
	protoreflect.Uint32Kind:  "Int",
	protoreflect.Int64Kind:   "Int",
	protoreflect.Sint64Kind:  "Int",
	protoreflect.Uint64Kind:  "Int",
	protoreflect.FloatKind:   "Float",
	protoreflect.DoubleKind:  "Float",
	protoreflect.StringKind:  "String",
	protoreflect.BytesKind:   "String",
}

// getGraphQLFieldType converts a protobuf field to a GraphQL field type
func getGraphQLFieldType(field *protogen.Field) string {
	if isSpecialWellKnownType(field) {
		return getWellKnownTypeMapping(field)
	}

	typeName := getFieldTypeName(field)
	
	// Add list wrapper if it's a repeated field
	if field.Desc.IsList() {
		typeName = "[" + typeName + "]"
	}
	
	// Make it non-nullable if it's a required field in proto3
	if !field.Desc.HasOptionalKeyword() && !field.Desc.HasPresence() {
		typeName = typeName + "!"
	}
	
	return typeName
}

// getGraphQLInputFieldType converts a protobuf field to a GraphQL input field type
func getGraphQLInputFieldType(field *protogen.Field) string {
	if isSpecialWellKnownType(field) {
		return getWellKnownTypeMapping(field)
	}

	typeName := getFieldTypeName(field)
	
	// For messages, use the Input type variant
	if field.Message != nil && !isSpecialWellKnownType(field) {
		typeName = typeName + "Input"
	}
	
	// Add list wrapper if it's a repeated field
	if field.Desc.IsList() {
		typeName = "[" + typeName + "]"
	}
	
	// Make it non-nullable if it's a required field in proto3
	if !field.Desc.HasOptionalKeyword() && !field.Desc.HasPresence() {
		typeName = typeName + "!"
	}
	
	return typeName
}

// getFieldTypeName returns the base GraphQL type name for a field
func getFieldTypeName(field *protogen.Field) string {
	// For enums
	if field.Enum != nil {
		return field.Enum.GoIdent.GoName
	}
	
	// For messages (except special well-known types)
	if field.Message != nil && !isSpecialWellKnownType(field) {
		// Check if there's a name override from options
		// Note: in a real implementation, you'd extract this from the options
		return field.Message.GoIdent.GoName
	}
	
	// For scalar types
	if scalar, ok := scalarMapping[field.Desc.Kind()]; ok {
		return scalar
	}
	
	// Default
	return "String"
}

// isSpecialWellKnownType checks if a field is a well-known type with special GraphQL mapping
func isSpecialWellKnownType(field *protogen.Field) bool {
	if field.Message == nil {
		return false
	}
	
	wellKnownType := string(field.Message.Desc.FullName())
	return wellKnownType == "google.protobuf.Timestamp" ||
		wellKnownType == "google.protobuf.Duration" ||
		wellKnownType == "google.protobuf.Struct" ||
		wellKnownType == "google.protobuf.Any"
}

// getWellKnownTypeMapping returns the GraphQL type for well-known protobuf types
func getWellKnownTypeMapping(field *protogen.Field) string {
	if field.Message == nil {
		return "String"
	}
	
	wellKnownType := string(field.Message.Desc.FullName())
	switch wellKnownType {
	case "google.protobuf.Timestamp":
		return "DateTime"
	case "google.protobuf.Duration":
		return "String"
	case "google.protobuf.Struct", "google.protobuf.Any":
		return "JSON"
	default:
		return "String"
	}
}

// getGraphQLArgs generates the GraphQL arguments string for an input message
func getGraphQLArgs(message *protogen.Message) string {
	if message == nil {
		return ""
	}
	
	var args []string
	for _, field := range message.Fields {
		// Skip fields that are meant to be skipped
		fieldSkip := fieldExtension(field, "graphql.options.skip")
		if fieldSkip != nil && fieldSkip.(bool) {
			continue
		}
		
		fieldName := lowerFirst(field.GoName)
		fieldType := getGraphQLInputFieldType(field)
		
		args = append(args, fieldName+": "+fieldType)
	}
	
	return strings.Join(args, ", ")
}

// getGraphQLReturn determines the GraphQL return type for an output message
func getGraphQLReturn(message *protogen.Message) string {
	if message == nil {
		return "Boolean"
	}
	
	// Check if this is a list response
	for _, field := range message.Fields {
		if strings.HasSuffix(field.GoName, "s") && field.Desc.IsList() {
			// This is likely a list response (e.g., ListUsersResponse with users field)
			if field.Message != nil {
				return "[" + field.Message.GoIdent.GoName + "]"
			}
		}
	}
	
	// Otherwise return the message type directly
	return message.GoIdent.GoName
}

// lowerFirst makes the first character of a string lowercase
func lowerFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

// fieldExtension is a helper to get field extension values
func fieldExtension(field *protogen.Field, path string) interface{} {
	// In a real implementation, this would extract the extension value
	// For this example, return nil to indicate no extensions
	return nil
}