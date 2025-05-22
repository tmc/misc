package tsclientgen

import (
	"strings"
	"text/template"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// AddHelperFunctions adds TypeScript-specific helper functions to the template function map
func AddHelperFunctions(funcMap template.FuncMap) template.FuncMap {
	funcMap["tsType"] = getTSType
	funcMap["extractPathParams"] = extractPathParams
	funcMap["hasPrefix"] = strings.HasPrefix
	funcMap["trimPrefix"] = strings.TrimPrefix
	funcMap["trimSuffix"] = strings.TrimSuffix
	funcMap["replace"] = strings.ReplaceAll
	
	return funcMap
}

// getTSType maps protobuf field types to TypeScript types
func getTSType(field *protogen.Field) string {
	if field.Desc.IsList() {
		if field.Message != nil {
			return field.Message.GoIdent.GoName + "[]"
		}
		
		switch field.Desc.Kind() {
		case protoreflect.BoolKind:
			return "boolean[]"
		case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind,
			protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind,
			protoreflect.Uint32Kind, protoreflect.Fixed32Kind,
			protoreflect.Uint64Kind, protoreflect.Fixed64Kind,
			protoreflect.FloatKind, protoreflect.DoubleKind:
			return "number[]"
		default:
			return "string[]"
		}
	}

	if field.Desc.IsMap() {
		valueField := field.Message.Fields[1] // Map value is the second field
		var valueType string
		
		if valueField.Message != nil {
			valueType = valueField.Message.GoIdent.GoName
		} else {
			switch valueField.Desc.Kind() {
			case protoreflect.BoolKind:
				valueType = "boolean"
			case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind,
				protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind,
				protoreflect.Uint32Kind, protoreflect.Fixed32Kind,
				protoreflect.Uint64Kind, protoreflect.Fixed64Kind,
				protoreflect.FloatKind, protoreflect.DoubleKind:
				valueType = "number"
			default:
				valueType = "string"
			}
		}
		
		return "Record<string, " + valueType + ">"
	}

	if field.Message != nil {
		return field.Message.GoIdent.GoName
	}

	switch field.Desc.Kind() {
	case protoreflect.BoolKind:
		return "boolean"
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind,
		protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind,
		protoreflect.Uint32Kind, protoreflect.Fixed32Kind,
		protoreflect.Uint64Kind, protoreflect.Fixed64Kind,
		protoreflect.FloatKind, protoreflect.DoubleKind:
		return "number"
	default:
		return "string"
	}
}

// extractPathParams extracts path parameters from a URL template
func extractPathParams(path string) []string {
	var params []string
	segments := strings.Split(path, "/")
	
	for _, segment := range segments {
		if strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}") {
			// Remove the curly braces
			param := segment[1 : len(segment)-1]
			params = append(params, param)
		}
	}
	
	return params
}