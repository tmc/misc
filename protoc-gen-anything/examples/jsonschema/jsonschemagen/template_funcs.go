package jsonschemagen

import (
	"text/template"

	"google.golang.org/protobuf/compiler/protogen"
)

// AddTemplateFuncs adds the JSONSchema generation functions to the template funcs map
func AddTemplateFuncs(funcMap template.FuncMap, includeNullable, embedDefinitions bool) template.FuncMap {
	generator := NewJSONSchemaGenerator(includeNullable, embedDefinitions)
	
	funcMap["generateJSONSchema"] = func(msg *protogen.Message) string {
		jsonStr, err := generator.GenerateJSONString(msg, true)
		if err != nil {
			return "Error generating JSONSchema: " + err.Error()
		}
		return jsonStr
	}
	
	funcMap["getMessageSchema"] = func(msg *protogen.Message) map[string]interface{} {
		schema, err := generator.GenerateSchema(msg)
		if err != nil {
			return map[string]interface{}{
				"error": "Error generating schema: " + err.Error(),
			}
		}
		return schema
	}
	
	return funcMap
}