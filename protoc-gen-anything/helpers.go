package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"text/template"

	"go.starlark.net/starlark"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
	"gopkg.in/yaml.v3"
	k8syaml "sigs.k8s.io/yaml"
)

func (g *Generator) funcMap() template.FuncMap {
	return template.FuncMap{
		"string": func(i interface {
			String() string
		}) string {
			return i.String()
		},
		"json": func(v interface{}) string {
			a, err := json.Marshal(v)
			if err != nil {
				return err.Error()
			}
			return string(a)
		},
		"prettyjson": func(v interface{}) string {
			a, err := json.MarshalIndent(v, "", "  ")
			if err != nil {
				return err.Error()
			}
			return string(a)
		},
		"splitArray": func(sep string, s string) []interface{} {
			var r []interface{}
			t := strings.Split(s, sep)
			for i := range t {
				if t[i] != "" {
					r = append(r, t[i])
				}
			}
			return r
		},
		"upperFirst": func(s string) string {
			if len(s) == 0 {
				return s
			}
			return strings.ToUpper(s[:1]) + s[1:]
		},
		"lowerFirst": func(s string) string {
			if len(s) == 0 {
				return s
			}
			return strings.ToLower(s[:1]) + s[1:]
		},
		"cleanComment": func(c interface{}) string {
			s := fmt.Sprintf("%v", c)
			var lines []string
			for _, line := range strings.Split(s, "\n") {
				line = strings.TrimSpace(line)
				line = strings.TrimPrefix(line, "//")
				line = strings.TrimSpace(line)
				if line != "" {
					lines = append(lines, line)
				}
			}
			return strings.Join(lines, " ")
		},
		"methodExtension":         g.helperMethodExtension,
		"messageExtension":        g.helperMessageExtension,
		"fieldExtension":          g.helperFieldExtension,
		"protoField":              g.helperFieldByName,
		"isNotEmpty":              g.helperIsNotEmpty,
		"isValidGoType":           g.helperIsValidGoType,
		"hasJSONFields":           g.helperHasJSONFields,
		"hasField":                g.helperHasField,
		"loadPrototxt":            g.helperLoadPrototxt,
		"loadYaml":                g.helperLoadYaml,
		"loadStarlark":            g.helperLoadStarlark,
		"mapGet":                  g.helperMapGet,
		"field":                   helperField, // unified field access for any type
		"summaryFromComments":     g.helperSummaryFromComments,
		"descriptionFromComments": g.helperDescriptionFromComments,
		// RPC streaming type checks
		"isServerStream": helperIsServerStream,
		"isClientStream": helperIsClientStream,
		"isBidi":         helperIsBidi,
		"isStreaming":    helperIsStreaming,
	}
}

// helperIsServerStream returns true if the method is server-streaming (server sends multiple responses).
func helperIsServerStream(method *protogen.Method) bool {
	return method.Desc.IsStreamingServer() && !method.Desc.IsStreamingClient()
}

// helperIsClientStream returns true if the method is client-streaming (client sends multiple requests).
func helperIsClientStream(method *protogen.Method) bool {
	return method.Desc.IsStreamingClient() && !method.Desc.IsStreamingServer()
}

// helperIsBidi returns true if the method is bidirectional streaming.
func helperIsBidi(method *protogen.Method) bool {
	return method.Desc.IsStreamingClient() && method.Desc.IsStreamingServer()
}

// helperIsStreaming returns true if the method has any streaming (client, server, or bidi).
func helperIsStreaming(method *protogen.Method) bool {
	return method.Desc.IsStreamingClient() || method.Desc.IsStreamingServer()
}

func (g *Generator) helperMethodExtension(method *protogen.Method, path string) any {
	options := method.Desc.Options().(*descriptorpb.MethodOptions)
	if options == nil {
		return nil
	}
	b, err := proto.Marshal(options)
	if err != nil {
		log.Fatalf("Error marshalling options: %v", err)
	}
	options.Reset()
	err = proto.UnmarshalOptions{Resolver: g.types}.Unmarshal(b, options)
	if err != nil {
		log.Fatalf("Error unmarshalling options: %v", err)
	}
	var extensions = make(map[string]any)
	options.ProtoReflect().Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		if fd.IsExtension() {
			extensions[string(fd.FullName())] = v.Interface()
		}
		return true
	})
	return extensions[path]
}

func (g *Generator) helperMessageExtension(message protogen.Message, path string) any {
	options := message.Desc.Options().(*descriptorpb.MessageOptions)
	if options == nil {
		return nil
	}
	b, err := proto.Marshal(options)
	if err != nil {
		log.Fatalf("Error marshalling options: %v", err)
	}
	// options.Reset()
	err = proto.UnmarshalOptions{Resolver: g.types}.Unmarshal(b, options)
	if err != nil {
		log.Fatalf("Error unmarshalling options: %v", err)
	}
	var extensions = make(map[string]any)
	options.ProtoReflect().Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		if fd.IsExtension() {
			extensions[string(fd.FullName())] = v.Interface()
		}
		return true
	})
	return extensions[path]
}

func (g *Generator) helperFieldExtension(field *protogen.Field, path string) any {
	options := field.Desc.Options().(*descriptorpb.FieldOptions)
	if options == nil {
		return nil
	}
	b, err := proto.Marshal(options)
	if err != nil {
		log.Fatalf("Error marshalling options: %v", err)
	}
	options.Reset()
	err = proto.UnmarshalOptions{Resolver: g.types}.Unmarshal(b, options)
	if err != nil {
		log.Fatalf("Error unmarshalling options: %v", err)
	}
	var extensions = make(map[string]any)
	options.ProtoReflect().Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		if fd.IsExtension() {
			extensions[string(fd.FullName())] = v.Interface()
		}
		return true
	})
	return extensions[path]
}

// gets a value of a field by name, returns nil if the field is not found or empty
// gets a value of a field by name, returns nil if the field is not found or empty
// gets a value of a field by name, returns nil if the field is not found or empty
func (g *Generator) helperFieldByName(name string, msg interface{}) interface{} {
	if msg == nil {
		return nil
	}

	var m protoreflect.Message
	switch v := msg.(type) {
	case protoreflect.Message:
		m = v
	case *dynamicpb.Message:
		m = v
	default:
		return nil
	}

	fd := m.Descriptor().Fields().ByName(protoreflect.Name(name))
	if fd == nil {
		return nil
	}
	val := m.Get(fd)
	if !val.IsValid() {
		return nil
	}

	// Check if it's a message before trying to access it as one
	if (fd.Kind() == protoreflect.MessageKind || fd.Kind() == protoreflect.GroupKind) && !fd.IsMap() && !fd.IsList() {
		if val.Message().IsValid() {
			return val.Message()
		}
	}

	if fd.IsList() {
		list := val.List()
		var s []interface{}
		for i := 0; i < list.Len(); i++ {
			s = append(s, list.Get(i).Interface())
		}
		return s
	}

	return val.Interface()
}

func (g *Generator) helperMapGet(key string, mapVal interface{}) interface{} {
	if mapVal == nil {
		return nil
	}
	mv, ok := mapVal.(protoreflect.Map)
	if !ok {
		return nil
	}

	val := mv.Get(protoreflect.ValueOfString(key).MapKey())
	if !val.IsValid() {
		return nil
	}

	return val.Interface()
}

func (g *Generator) helperIsNotEmpty(value interface{}) bool {
	if value == nil {
		return false
	}
	switch v := value.(type) {
	case string:
		return v != ""
	case bool:
		return v
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return v != 0
	case protoreflect.Message:
		return v != nil && v.IsValid()
	default:
		return true
	}
}

func (g *Generator) helperIsValidGoType(value interface{}) bool {
	if value == nil {
		return false
	}
	switch v := value.(type) {
	case string:
		return v != ""
	case protoreflect.Message:
		return v != nil && v.IsValid()
	default:
		return false
	}
}

func (g *Generator) helperHasJSONFields(message *protogen.Message) bool {
	for _, field := range message.Fields {
		entField := g.helperFieldExtension(field, "metadata.v1.ent_field")
		if entField != nil {
			ef, ok := entField.(*dynamicpb.Message)
			if !ok {
				continue
			}
			fieldType := g.helperFieldByName("type", ef)
			if s, ok := fieldType.(string); ok && s == "JSON" {
				return true
			}
		}
	}
	return false
}

func (g *Generator) helperHasField(message *protogen.Message, fieldName string) bool {
	for _, field := range message.Fields {
		if field.GoName == fieldName {
			return true
		}
	}
	return false
}

func (g *Generator) helperLoadPrototxt(filename, typeName string) (interface{}, error) {
	desc, err := g.types.FindMessageByName(protoreflect.FullName(typeName))
	if err != nil {
		return nil, fmt.Errorf("failed to find message type %s: %w", typeName, err)
	}

	msg := dynamicpb.NewMessage(desc.Descriptor())
	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filename, err)
	}

	if err := prototext.Unmarshal(content, msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal prototxt: %w", err)
	}

	return msg, nil
}

func (g *Generator) helperSummaryFromComments(comments protogen.Comments) string {
	s := string(comments)
	if s == "" {
		return ""
	}
	parts := strings.SplitN(s, "\n", 2)
	return strings.TrimSpace(parts[0])
}

func (g *Generator) helperDescriptionFromComments(comments protogen.Comments) string {
	s := string(comments)
	if s == "" {
		return ""
	}
	parts := strings.SplitN(s, "\n", 2)
	if len(parts) < 2 {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

// helperLoadYaml loads a YAML file and optionally validates against a proto type.
// Usage: {{ $config := loadYaml "config.yaml" }}
// Usage: {{ $config := loadYaml "config.yaml" "cli.v1.CliConfig" }}
//
// If the YAML file contains a comment like "# proto-type: cli.v1.CliConfig",
// it will be validated against that proto type automatically.
func (g *Generator) helperLoadYaml(filename string, typeName ...string) (any, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read yaml file %s: %w", filename, err)
	}

	// Determine proto type: explicit arg > comment in file > none
	var protoType string
	if len(typeName) > 0 && typeName[0] != "" {
		protoType = typeName[0]
	} else {
		protoType = extractProtoType(content)
	}

	// If we have a proto type, validate against proto schema
	if protoType != "" {
		msg, err := g.loadYamlWithValidation(content, protoType, filename)
		if err != nil {
			return nil, err
		}
		return msg, nil
	}

	// No validation, just parse as map
	var result map[string]any
	if err := yaml.Unmarshal(content, &result); err != nil {
		return nil, fmt.Errorf("failed to parse yaml: %w", err)
	}
	return result, nil
}

// extractProtoType looks for "# proto-type: some.Type" in the first few lines
func extractProtoType(content []byte) string {
	lines := strings.SplitN(string(content), "\n", 5)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# proto-type:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# proto-type:"))
		}
	}
	return ""
}

// loadYamlWithValidation parses YAML and validates against a proto type
func (g *Generator) loadYamlWithValidation(content []byte, typeName, filename string) (any, error) {
	desc, err := g.types.FindMessageByName(protoreflect.FullName(typeName))
	if err != nil {
		// Type not found - fall back to unvalidated parsing but log warning
		g.Logger.Warn("proto type not found for yaml validation, parsing without validation", "type", typeName, "file", filename)
		var result map[string]any
		if err := yaml.Unmarshal(content, &result); err != nil {
			return nil, fmt.Errorf("failed to parse yaml: %w", err)
		}
		return result, nil
	}

	// Convert YAML to JSON (strict mode catches duplicate keys)
	jsonBytes, err := k8syaml.YAMLToJSONStrict(content)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filename, err)
	}

	// Parse JSON into proto message with strict validation (rejects unknown fields)
	msg := dynamicpb.NewMessage(desc.Descriptor())
	opts := protojson.UnmarshalOptions{DiscardUnknown: false}
	if err := opts.Unmarshal(jsonBytes, msg); err != nil {
		// Try to find YAML line number for unknown field errors
		errMsg := err.Error()
		if strings.Contains(errMsg, "unknown field") {
			// Collect valid field names from the proto descriptor for suggestions
			validFields := collectFieldNames(desc.Descriptor())
			return nil, formatYamlValidationError(content, filename, errMsg, validFields)
		}
		return nil, fmt.Errorf("%s: %w", filename, err)
	}

	// Return the validated proto message
	return msg, nil
}

// yamlValidationError holds structured info for YAML validation failures
type yamlValidationError struct {
	File       string
	Line       int
	Col        int
	FieldName  string
	Context    string // the actual YAML line
	Caret      string // caret pointing to error position
	Suggestion string // "did you mean?" suggestion
}

func (e *yamlValidationError) Error() string {
	// Format like go build errors:
	// file.yaml:3:1: unknown field "short_descriptin" (did you mean "short_description"?)
	//     short_descriptin: A generated GitHub CLI
	//     ^
	var b strings.Builder
	fmt.Fprintf(&b, "%s:%d:%d: unknown field %q", e.File, e.Line, e.Col, e.FieldName)
	if e.Suggestion != "" {
		fmt.Fprintf(&b, " (did you mean %q?)", e.Suggestion)
	}
	if e.Context != "" {
		b.WriteString("\n")
		b.WriteString("    ")
		b.WriteString(e.Context)
		b.WriteString("\n")
		b.WriteString("    ")
		b.WriteString(e.Caret)
	}
	return b.String()
}

// formatYamlValidationError creates a user-friendly error in go build format:
// file:line:col: message
// with context showing the offending line
func formatYamlValidationError(content []byte, filename, errMsg string, validFields []string) error {
	fieldName := extractFieldName(errMsg)
	if fieldName == "" {
		return fmt.Errorf("%s: %s", filename, errMsg)
	}

	// Parse YAML to find the field location
	var node yaml.Node
	if err := yaml.Unmarshal(content, &node); err != nil {
		return fmt.Errorf("%s: %s", filename, errMsg)
	}

	line, col := findFieldLineCol(&node, fieldName)
	if line == 0 {
		return fmt.Errorf("%s: unknown field %q", filename, fieldName)
	}

	// Extract the context line
	lines := strings.Split(string(content), "\n")
	var context, caret string
	if line > 0 && line <= len(lines) {
		context = lines[line-1]
		// Create caret pointing to the column
		if col > 0 {
			caret = strings.Repeat(" ", col-1) + "^"
		}
	}

	// Find closest match for "did you mean?"
	suggestion := findClosestField(fieldName, validFields)

	return &yamlValidationError{
		File:       filename,
		Line:       line,
		Col:        col,
		FieldName:  fieldName,
		Context:    context,
		Caret:      caret,
		Suggestion: suggestion,
	}
}

// findClosestField finds the closest matching field name using Levenshtein distance
func findClosestField(typo string, validFields []string) string {
	if len(validFields) == 0 {
		return ""
	}

	var closest string
	minDist := len(typo) + 1 // max possible distance

	for _, field := range validFields {
		dist := levenshtein(typo, field)
		// Only suggest if edit distance is reasonable (< 40% of longer string)
		maxLen := len(typo)
		if len(field) > maxLen {
			maxLen = len(field)
		}
		if dist < minDist && dist <= maxLen*2/5 {
			minDist = dist
			closest = field
		}
	}
	return closest
}

// collectFieldNames recursively collects all field names from a message descriptor
func collectFieldNames(md protoreflect.MessageDescriptor) []string {
	var names []string
	fields := md.Fields()
	for i := 0; i < fields.Len(); i++ {
		f := fields.Get(i)
		// Use proto name (snake_case) since that's what YAML uses via k8syaml
		names = append(names, string(f.Name()))
		// For nested messages, also collect their field names
		if f.Kind() == protoreflect.MessageKind && !f.IsMap() {
			nested := collectFieldNames(f.Message())
			names = append(names, nested...)
		}
		// For map values that are messages
		if f.IsMap() && f.MapValue().Kind() == protoreflect.MessageKind {
			nested := collectFieldNames(f.MapValue().Message())
			names = append(names, nested...)
		}
	}
	return names
}

// levenshtein computes the Levenshtein distance between two strings
func levenshtein(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	// Create matrix
	d := make([][]int, len(a)+1)
	for i := range d {
		d[i] = make([]int, len(b)+1)
		d[i][0] = i
	}
	for j := range d[0] {
		d[0][j] = j
	}

	// Fill matrix
	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			d[i][j] = min(
				d[i-1][j]+1,      // deletion
				d[i][j-1]+1,      // insertion
				d[i-1][j-1]+cost, // substitution
			)
		}
	}
	return d[len(a)][len(b)]
}

// findFieldLineCol returns line and column (1-indexed) for a field
func findFieldLineCol(node *yaml.Node, fieldName string) (int, int) {
	if node == nil {
		return 0, 0
	}

	switch node.Kind {
	case yaml.DocumentNode:
		for _, child := range node.Content {
			if line, col := findFieldLineCol(child, fieldName); line > 0 {
				return line, col
			}
		}
	case yaml.MappingNode:
		for i := 0; i < len(node.Content); i += 2 {
			keyNode := node.Content[i]
			if keyNode.Value == fieldName {
				return keyNode.Line, keyNode.Column
			}
			if i+1 < len(node.Content) {
				if line, col := findFieldLineCol(node.Content[i+1], fieldName); line > 0 {
					return line, col
				}
			}
		}
	case yaml.SequenceNode:
		for _, child := range node.Content {
			if line, col := findFieldLineCol(child, fieldName); line > 0 {
				return line, col
			}
		}
	}
	return 0, 0
}

// extractFieldName extracts the field name from "unknown field \"xyz\"" error
func extractFieldName(errMsg string) string {
	const prefix = "unknown field \""
	idx := strings.Index(errMsg, prefix)
	if idx < 0 {
		return ""
	}
	start := idx + len(prefix)
	end := strings.Index(errMsg[start:], "\"")
	if end < 0 {
		return ""
	}
	return errMsg[start : start+end]
}

// helperLoadStarlark executes a Starlark file and returns a named variable.
// Usage: {{ $config := loadStarlark "config.star" "config" }}
func (g *Generator) helperLoadStarlark(filename, varName string) (any, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read starlark file %s: %w", filename, err)
	}

	thread := &starlark.Thread{Name: filename}
	globals, err := starlark.ExecFile(thread, filename, content, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to execute starlark file: %w", err)
	}

	val, ok := globals[varName]
	if !ok {
		return nil, fmt.Errorf("variable %q not found in %s", varName, filename)
	}

	return starlarkToGoDeep(val), nil
}

// helperField provides unified field access for maps, proto messages, and structs.
// Usage: {{ field $obj "fieldName" }} or {{ field $obj "fieldName" "default" }}
func helperField(obj any, fieldName string, defaultVal ...any) any {
	var def any
	if len(defaultVal) > 0 {
		def = defaultVal[0]
	}

	if obj == nil {
		return def
	}

	// Handle Go maps (from YAML/Starlark)
	if m, ok := obj.(map[string]any); ok {
		if v, exists := m[fieldName]; exists {
			return v
		}
		return def
	}

	// Handle proto messages
	if pm, ok := obj.(protoreflect.Message); ok {
		fd := pm.Descriptor().Fields().ByName(protoreflect.Name(fieldName))
		if fd != nil {
			val := pm.Get(fd)
			if val.IsValid() {
				if fd.IsMap() {
					return val.Map()
				}
				if fd.IsList() {
					list := val.List()
					result := make([]any, list.Len())
					for i := 0; i < list.Len(); i++ {
						result[i] = list.Get(i).Interface()
					}
					return result
				}
				if fd.Kind() == protoreflect.MessageKind {
					return val.Message()
				}
				return val.Interface()
			}
		}
		return def
	}

	return def
}

// starlarkToGoDeep recursively converts Starlark values to Go values.
// Unlike starlarkToGo, this fully converts dicts and lists to Go maps and slices.
func starlarkToGoDeep(v starlark.Value) any {
	switch val := v.(type) {
	case starlark.NoneType:
		return nil
	case starlark.Bool:
		return bool(val)
	case starlark.Int:
		i, _ := val.Int64()
		return i
	case starlark.Float:
		return float64(val)
	case starlark.String:
		return string(val)
	case *starlark.List:
		result := make([]any, val.Len())
		for i := 0; i < val.Len(); i++ {
			result[i] = starlarkToGoDeep(val.Index(i))
		}
		return result
	case *starlark.Dict:
		result := make(map[string]any)
		for _, item := range val.Items() {
			key := starlarkToGoDeep(item[0])
			if k, ok := key.(string); ok {
				result[k] = starlarkToGoDeep(item[1])
			}
		}
		return result
	case starlark.Tuple:
		result := make([]any, len(val))
		for i, elem := range val {
			result[i] = starlarkToGoDeep(elem)
		}
		return result
	default:
		return val.String()
	}
}
