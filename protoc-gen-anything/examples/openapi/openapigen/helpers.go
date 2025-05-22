package openapigen

import (
	"text/template"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// AddHelperFunctions adds OpenAPI-specific helper functions to the template function map
func AddHelperFunctions(funcMap template.FuncMap) template.FuncMap {
	// Add OpenAPI-specific helper functions
	funcMap["getOpenAPIType"] = getOpenAPIType
	funcMap["httpPath"] = getHTTPPath
	funcMap["isStreaming"] = isStreaming
	
	return funcMap
}

// getOpenAPIType maps protobuf field types to OpenAPI schema types
func getOpenAPIType(field *protogen.Field) string {
	if field.Desc.IsList() {
		return "array"
	}

	if field.Desc.IsMap() {
		return "object"
	}

	switch field.Desc.Kind() {
	case protoreflect.BoolKind:
		return "boolean"
	case protoreflect.StringKind:
		return "string"
	case protoreflect.BytesKind:
		return "string" // format: byte
	case protoreflect.EnumKind:
		return "string" // could also be array of possible values
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind,
		protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind,
		protoreflect.Uint32Kind, protoreflect.Fixed32Kind,
		protoreflect.Uint64Kind, protoreflect.Fixed64Kind,
		protoreflect.FloatKind, protoreflect.DoubleKind:
		return "number"
	case protoreflect.GroupKind, protoreflect.MessageKind:
		return "object"
	default:
		return "string"
	}
}

// getHTTPPath extracts the HTTP path from the method options
func getHTTPPath(method *protogen.Method) string {
	// This would parse google.api.http annotation to get the path
	// For now, return a placeholder
	return "/api/" + string(method.Desc.Name())
}

// isStreaming checks if the method is a streaming method
func isStreaming(method *protogen.Method) bool {
	return method.Desc.IsStreamingClient() || method.Desc.IsStreamingServer()
}