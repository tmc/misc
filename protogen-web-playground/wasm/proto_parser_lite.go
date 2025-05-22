package main

import (
	"fmt"
	"regexp"
	"strings"
)

// LightProtoFile represents a lightweight Protocol Buffer file definition
type LightProtoFile struct {
	Package  string
	Messages []*LightMessage
	Services []*LightService
	Enums    []*LightEnum
	Imports  []string
}

// LightMessage represents a lightweight Protocol Buffer message definition
type LightMessage struct {
	Name     string
	Fields   []*LightField
	Oneofs   []*LightOneof
	Messages []*LightMessage
	Enums    []*LightEnum
	Comment  string
	Options  map[string]string
}

// LightField represents a lightweight Protocol Buffer field definition
type LightField struct {
	Name     string
	Type     string
	Number   int
	Repeated bool
	Optional bool
	Comment  string
	Options  map[string]string
}

// LightOneof represents a lightweight Protocol Buffer oneof definition
type LightOneof struct {
	Name    string
	Fields  []*LightField
	Comment string
}

// LightService represents a lightweight Protocol Buffer service definition
type LightService struct {
	Name    string
	Methods []*LightMethod
	Comment string
	Options map[string]string
}

// LightMethod represents a lightweight Protocol Buffer method definition
type LightMethod struct {
	Name       string
	InputType  string
	OutputType string
	Comment    string
	Options    map[string]string
	Streaming  bool
}

// LightEnum represents a lightweight Protocol Buffer enum definition
type LightEnum struct {
	Name    string
	Values  map[string]int
	Comment string
	Options map[string]string
}

// ProtoParserLite is a lightweight Protocol Buffer parser
type ProtoParserLite struct {
	// Configuration options
	SkipUnknownOptions bool
}

// NewProtoParserLite creates a new lightweight Protocol Buffer parser
func NewProtoParserLite() *ProtoParserLite {
	return &ProtoParserLite{
		SkipUnknownOptions: true,
	}
}

// Parse parses a Protocol Buffer file and returns a lightweight representation
func (p *ProtoParserLite) Parse(content string) (*LightProtoFile, error) {
	result := &LightProtoFile{
		Messages: make([]*LightMessage, 0),
		Services: make([]*LightService, 0),
		Enums:    make([]*LightEnum, 0),
		Imports:  make([]string, 0),
	}

	// Remove comments (simplistic approach - a real parser would handle this better)
	// This is just for demonstration - a real parser would preserve comments
	lines := strings.Split(content, "\n")
	var cleanedLines []string
	var currentComment string

	for _, line := range lines {
		// Handle line comments
		if idx := strings.Index(line, "//"); idx >= 0 {
			comment := strings.TrimSpace(line[idx+2:])
			if comment != "" {
				currentComment += comment + "\n"
			}
			line = line[:idx]
		}

		if strings.TrimSpace(line) != "" {
			cleanedLines = append(cleanedLines, line)
		}
	}

	cleanedContent := strings.Join(cleanedLines, "\n")

	// Extract package
	packageRegex := regexp.MustCompile(`package\s+([a-zA-Z0-9_.]+)\s*;`)
	packageMatches := packageRegex.FindStringSubmatch(cleanedContent)
	if len(packageMatches) > 1 {
		result.Package = packageMatches[1]
	}

	// Extract imports
	importRegex := regexp.MustCompile(`import\s+(?:"([^"]+)"|'([^']+)')\s*;`)
	importMatches := importRegex.FindAllStringSubmatch(cleanedContent, -1)
	for _, match := range importMatches {
		if match[1] != "" {
			result.Imports = append(result.Imports, match[1])
		} else if match[2] != "" {
			result.Imports = append(result.Imports, match[2])
		}
	}

	// Extract messages (simplified - doesn't handle nested messages correctly)
	messageRegex := regexp.MustCompile(`message\s+([a-zA-Z0-9_]+)\s*{([^}]*)}`)
	messageMatches := messageRegex.FindAllStringSubmatch(cleanedContent, -1)
	for _, match := range messageMatches {
		messageName := match[1]
		messageContent := match[2]

		message := &LightMessage{
			Name:     messageName,
			Fields:   make([]*LightField, 0),
			Oneofs:   make([]*LightOneof, 0),
			Messages: make([]*LightMessage, 0),
			Enums:    make([]*LightEnum, 0),
			Options:  make(map[string]string),
		}

		// Extract fields
		fieldRegex := regexp.MustCompile(`(repeated|optional)?\s*([a-zA-Z0-9_.]+)\s+([a-zA-Z0-9_]+)\s*=\s*(\d+)`)
		fieldMatches := fieldRegex.FindAllStringSubmatch(messageContent, -1)
		for _, fieldMatch := range fieldMatches {
			repeated := fieldMatch[1] == "repeated"
			optional := fieldMatch[1] == "optional"
			fieldType := fieldMatch[2]
			fieldName := fieldMatch[3]
			fieldNumber := 0
			fmt.Sscanf(fieldMatch[4], "%d", &fieldNumber)

			field := &LightField{
				Name:     fieldName,
				Type:     fieldType,
				Number:   fieldNumber,
				Repeated: repeated,
				Optional: optional,
				Options:  make(map[string]string),
			}

			message.Fields = append(message.Fields, field)
		}

		result.Messages = append(result.Messages, message)
	}

	// Extract services (simplified)
	serviceRegex := regexp.MustCompile(`service\s+([a-zA-Z0-9_]+)\s*{([^}]*)}`)
	serviceMatches := serviceRegex.FindAllStringSubmatch(cleanedContent, -1)
	for _, match := range serviceMatches {
		serviceName := match[1]
		serviceContent := match[2]

		service := &LightService{
			Name:    serviceName,
			Methods: make([]*LightMethod, 0),
			Options: make(map[string]string),
		}

		// Extract methods
		methodRegex := regexp.MustCompile(`rpc\s+([a-zA-Z0-9_]+)\s*\(\s*([a-zA-Z0-9_.]+)\s*\)\s*returns\s*\(\s*([a-zA-Z0-9_.]+)\s*\)`)
		methodMatches := methodRegex.FindAllStringSubmatch(serviceContent, -1)
		for _, methodMatch := range methodMatches {
			methodName := methodMatch[1]
			inputType := methodMatch[2]
			outputType := methodMatch[3]

			method := &LightMethod{
				Name:       methodName,
				InputType:  inputType,
				OutputType: outputType,
				Options:    make(map[string]string),
			}

			service.Methods = append(service.Methods, method)
		}

		result.Services = append(result.Services, service)
	}

	// Extract enums (simplified)
	enumRegex := regexp.MustCompile(`enum\s+([a-zA-Z0-9_]+)\s*{([^}]*)}`)
	enumMatches := enumRegex.FindAllStringSubmatch(cleanedContent, -1)
	for _, match := range enumMatches {
		enumName := match[1]
		enumContent := match[2]

		enum := &LightEnum{
			Name:    enumName,
			Values:  make(map[string]int),
			Options: make(map[string]string),
		}

		// Extract enum values
		valueRegex := regexp.MustCompile(`([a-zA-Z0-9_]+)\s*=\s*(\d+)`)
		valueMatches := valueRegex.FindAllStringSubmatch(enumContent, -1)
		for _, valueMatch := range valueMatches {
			valueName := valueMatch[1]
			valueNumber := 0
			fmt.Sscanf(valueMatch[2], "%d", &valueNumber)

			enum.Values[valueName] = valueNumber
		}

		result.Enums = append(result.Enums, enum)
	}

	return result, nil
}

// ToTemplateData converts the lightweight Protocol Buffer file to a template-friendly format
func (f *LightProtoFile) ToTemplateData() map[string]interface{} {
	data := make(map[string]interface{})
	data["Package"] = f.Package
	
	// Convert messages
	messages := make([]map[string]interface{}, 0, len(f.Messages))
	for _, message := range f.Messages {
		msgData := make(map[string]interface{})
		msgData["Name"] = message.Name
		msgData["GoName"] = message.Name
		msgData["GoIdent"] = map[string]string{
			"GoName": message.Name,
		}
		
		// Convert fields
		fields := make([]map[string]interface{}, 0, len(message.Fields))
		for _, field := range message.Fields {
			fieldData := make(map[string]interface{})
			fieldData["Name"] = field.Name
			fieldData["GoName"] = field.Name
			fieldData["Type"] = field.Type
			fieldData["Number"] = field.Number
			fieldData["Repeated"] = field.Repeated
			fieldData["Optional"] = field.Optional
			
			fields = append(fields, fieldData)
		}
		msgData["Fields"] = fields
		
		messages = append(messages, msgData)
	}
	data["Messages"] = messages
	
	// Convert services
	services := make([]map[string]interface{}, 0, len(f.Services))
	for _, service := range f.Services {
		svcData := make(map[string]interface{})
		svcData["Name"] = service.Name
		svcData["GoName"] = service.Name
		
		// Convert methods
		methods := make([]map[string]interface{}, 0, len(service.Methods))
		for _, method := range service.Methods {
			methodData := make(map[string]interface{})
			methodData["Name"] = method.Name
			methodData["GoName"] = method.Name
			methodData["InputType"] = method.InputType
			methodData["OutputType"] = method.OutputType
			
			// Extract last part of type for GoIdent
			inputParts := strings.Split(method.InputType, ".")
			outputParts := strings.Split(method.OutputType, ".")
			
			methodData["Input"] = map[string]interface{}{
				"GoIdent": map[string]string{
					"GoName": inputParts[len(inputParts)-1],
				},
			}
			
			methodData["Output"] = map[string]interface{}{
				"GoIdent": map[string]string{
					"GoName": outputParts[len(outputParts)-1],
				},
			}
			
			methods = append(methods, methodData)
		}
		svcData["Methods"] = methods
		
		services = append(services, svcData)
	}
	data["Services"] = services
	
	// Convert enums
	enums := make([]map[string]interface{}, 0, len(f.Enums))
	for _, enum := range f.Enums {
		enumData := make(map[string]interface{})
		enumData["Name"] = enum.Name
		enumData["GoIdent"] = map[string]string{
			"GoName": enum.Name,
		}
		
		// Convert values
		values := make([]map[string]interface{}, 0, len(enum.Values))
		for name, number := range enum.Values {
			valueData := make(map[string]interface{})
			valueData["Name"] = name
			valueData["GoName"] = name
			valueData["Number"] = number
			
			values = append(values, valueData)
		}
		enumData["Values"] = values
		
		enums = append(enums, enumData)
	}
	data["Enums"] = enums
	
	// Add file info
	data["GoPackageName"] = strings.Replace(f.Package, ".", "_", -1)
	
	return data
}