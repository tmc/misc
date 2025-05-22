package sqlgen

import (
	"strings"
	"text/template"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Relationship represents a database relationship
type Relationship struct {
	Name       string
	Type       string
	JSONName   string
	ForeignKey string
}

// AddHelperFunctions adds SQL-specific helper functions to the template function map
func AddHelperFunctions(funcMap template.FuncMap) template.FuncMap {
	funcMap["sqlType"] = getSQLType
	funcMap["goType"] = getGoType
	funcMap["primaryKeys"] = getPrimaryKeys
	funcMap["hasForeignKeys"] = hasForeignKeys
	funcMap["isLastForeignKey"] = isLastForeignKey
	funcMap["getRelationships"] = getRelationships
	funcMap["isNil"] = isNil
	funcMap["contains"] = strings.Contains
	funcMap["trimPrefix"] = strings.TrimPrefix
	
	return funcMap
}

// isNil checks if a value is nil
func isNil(i interface{}) bool {
	return i == nil
}

// getSQLType maps protobuf field types to SQL column types
func getSQLType(field *protogen.Field) string {
	// For repeated fields, use a JSON column
	if field.Desc.IsList() {
		return "JSON"
	}

	// For map fields, use a JSON column
	if field.Desc.IsMap() {
		return "JSON"
	}

	// For message fields - handle special cases like Timestamp
	if field.Message != nil {
		msgName := string(field.Message.Desc.FullName())
		if msgName == "google.protobuf.Timestamp" {
			return "TIMESTAMP"
		}
		return "JSON" // Default to JSON for complex types
	}

	// For enums
	if field.Enum != nil {
		return "VARCHAR(50)"
	}

	// For scalar types
	switch field.Desc.Kind() {
	case protoreflect.BoolKind:
		return "BOOLEAN"
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return "INT"
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return "INT UNSIGNED"
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return "BIGINT"
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return "BIGINT UNSIGNED"
	case protoreflect.FloatKind:
		return "FLOAT"
	case protoreflect.DoubleKind:
		return "DOUBLE"
	case protoreflect.StringKind:
		return "VARCHAR(255)"
	case protoreflect.BytesKind:
		return "BLOB"
	default:
		return "VARCHAR(255)"
	}
}

// getGoType maps protobuf field types to Go types for the model
func getGoType(field *protogen.Field) string {
	// For repeated fields
	if field.Desc.IsList() {
		if field.Enum != nil {
			return "[]" + field.Enum.GoIdent.GoName
		}
		
		if field.Message != nil {
			return "[]" + field.Message.GoIdent.GoName
		}
		
		switch field.Desc.Kind() {
		case protoreflect.BoolKind:
			return "[]bool"
		case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
			return "[]int32"
		case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
			return "[]uint32"
		case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
			return "[]int64"
		case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
			return "[]uint64"
		case protoreflect.FloatKind:
			return "[]float32"
		case protoreflect.DoubleKind:
			return "[]float64"
		case protoreflect.StringKind:
			return "[]string"
		case protoreflect.BytesKind:
			return "[][]byte"
		default:
			return "[]interface{}"
		}
	}

	// For map fields
	if field.Desc.IsMap() {
		return "map[string]interface{}"
	}

	// For message fields - handle special cases like Timestamp
	if field.Message != nil {
		msgName := string(field.Message.Desc.FullName())
		if msgName == "google.protobuf.Timestamp" {
			return "time.Time"
		}
		return field.Message.GoIdent.GoName
	}

	// For enums
	if field.Enum != nil {
		return field.Enum.GoIdent.GoName
	}

	// For scalar types
	switch field.Desc.Kind() {
	case protoreflect.BoolKind:
		return "bool"
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return "int32"
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return "uint32"
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return "int64"
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return "uint64"
	case protoreflect.FloatKind:
		return "float32"
	case protoreflect.DoubleKind:
		return "float64"
	case protoreflect.StringKind:
		return "string"
	case protoreflect.BytesKind:
		return "[]byte"
	default:
		return "interface{}"
	}
}

// getPrimaryKeys returns the primary key columns for a message
func getPrimaryKeys(message *protogen.Message) string {
	var primaryKeys []string
	
	for _, field := range message.Fields {
		isPrimary := fieldExtension(field, "sql.options.primary_key")
		if isPrimary != nil && isPrimary.(bool) {
			columnName := fieldExtension(field, "sql.options.column_name")
			if columnName != nil {
				primaryKeys = append(primaryKeys, columnName.(string))
			} else {
				primaryKeys = append(primaryKeys, toSnakeCase(field.GoName))
			}
		}
	}
	
	// Default to "id" if no primary keys were found
	if len(primaryKeys) == 0 {
		for _, field := range message.Fields {
			if field.GoName == "Id" || field.GoName == "ID" {
				columnName := fieldExtension(field, "sql.options.column_name")
				if columnName != nil {
					primaryKeys = append(primaryKeys, columnName.(string))
				} else {
					primaryKeys = append(primaryKeys, toSnakeCase(field.GoName))
				}
				break
			}
		}
	}
	
	if len(primaryKeys) == 0 {
		return "id"
	}
	
	return strings.Join(primaryKeys, ", ")
}

// hasForeignKeys checks if a message has any foreign key fields
func hasForeignKeys(message *protogen.Message) bool {
	for _, field := range message.Fields {
		foreignKey := fieldExtension(field, "sql.options.foreign_key")
		if foreignKey != nil {
			return true
		}
	}
	return false
}

// isLastForeignKey checks if a field is the last foreign key in a message
func isLastForeignKey(field *protogen.Field, message *protogen.Message) bool {
	isLast := true
	found := false
	
	for i := len(message.Fields) - 1; i >= 0; i-- {
		f := message.Fields[i]
		foreignKey := fieldExtension(f, "sql.options.foreign_key")
		
		if foreignKey != nil {
			if !found {
				found = (f.GoName == field.GoName)
				isLast = found
			} else if f.GoName == field.GoName {
				isLast = false
			}
		}
	}
	
	return isLast
}

// getRelationships extracts database relationships from a message
func getRelationships(message *protogen.Message) []Relationship {
	var relationships []Relationship
	
	// Find all fields that reference other messages
	for _, field := range message.Fields {
		if field.Message != nil && !isSpecialType(field.Message) {
			// Skip fields marked to be skipped
			skip := fieldExtension(field, "sql.options.skip")
			if skip != nil && skip.(bool) {
				continue
			}
			
			// For non-repeated fields, add a has-one relationship
			if !field.Desc.IsList() {
				columnName := fieldExtension(field, "sql.options.column_name")
				foreignKey := field.GoName + "ID"
				if columnName != nil {
					foreignKey = columnName.(string)
				}
				
				relationships = append(relationships, Relationship{
					Name:       field.GoName,
					Type:       "*" + field.Message.GoIdent.GoName,
					JSONName:   field.GoName,
					ForeignKey: foreignKey,
				})
			} else {
				// For repeated fields, add a has-many relationship
				relationships = append(relationships, Relationship{
					Name:       field.GoName,
					Type:       "[]" + field.Message.GoIdent.GoName,
					JSONName:   field.GoName,
					ForeignKey: toSnakeCase(message.GoIdent.GoName) + "_id",
				})
			}
		}
	}
	
	return relationships
}

// isSpecialType checks if a message is a special well-known type
func isSpecialType(message *protogen.Message) bool {
	fullName := string(message.Desc.FullName())
	return strings.HasPrefix(fullName, "google.protobuf.")
}

// toSnakeCase converts a string from CamelCase to snake_case
func toSnakeCase(s string) string {
	var result string
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result += "_"
		}
		result += strings.ToLower(string(r))
	}
	return result
}

// fieldExtension is a helper to get field extension values
func fieldExtension(field *protogen.Field, path string) interface{} {
	// In a real implementation, this would extract the extension value
	// For this example, return nil to indicate no extensions
	return nil
}