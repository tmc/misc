package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/tmc/misc/ts2go/internal/cdn"
	"github.com/tmc/misc/ts2go/internal/config"
	"github.com/tmc/misc/ts2go/internal/plugins"
	v8 "rogchap.com/v8go"
)

// LoadTemplate loads the template from a file or returns the default template
func LoadTemplate(templatePath string) (string, error) {
	if templatePath == "" {
		return defaultTemplate, nil
	}

	content, err := os.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("failed to read template file: %v", err)
	}
	return string(content), nil
}

var defaultTemplate = `// Code generated from {{ .Filename }}; DO NOT EDIT.
package {{ .Package }}

{{ if .Imports }}
import (
{{ range .Imports }}	"{{ . }}"
{{ end }}
)
{{ end }}

{{ range .Constants }}{{ if .Description }}// {{ .Description }}{{ end }}
const {{ .Name }} = {{ .Value }}
{{ end }}

{{ range .Types }}{{ if .Description }}// {{ formatComment .Description }}{{ else }}// {{ .Name }} represents {{ end }}
type {{ .Name }} {{ .GoType }}
{{ end }}

{{ range .Interfaces }}{{ if .Description }}// {{ formatComment .Description }}{{ else }}// {{ .Name }} represents {{ end }}
type {{ .Name }} struct {
{{ range .Fields }}	{{ if .Description }}// {{ formatComment .Description }}
	{{ end }}{{ .Name }} {{ .Type }}{{ if .JSONName }} ` + "`json:\"{{ .JSONName }}{{ if .Optional }},omitempty{{ end }}\"`" + `{{ end }}
{{ end }}
}
{{ range .Methods }}
{{ .Code }}
{{ end }}{{ if .CustomCode }}
{{ .CustomCode }}{{ end }}
{{ end }}`

type Constant struct {
	Name        string
	Value       string
	Description string
}

type TypeDef struct {
	Name        string
	GoType      string
	Description string
}

type Field struct {
	Name        string
	Type        string
	JSONName    string
	Optional    bool
	Description string
}

type Method struct {
	Name string
	Code string
}

type Interface struct {
	Name        string
	Fields      []Field
	Description string
	Methods     []Method
	CustomCode  string
}

type TemplateData struct {
	Package    string
	Constants  []Constant
	Types      []TypeDef
	Interfaces []Interface
	Imports    []string
	Filename   string
}

// Config holds configuration for type mappings and generation options
type Config struct {
	// Map of TypeScript type names to their Go equivalents
	TypeMappings map[string]string
	// Whether to use pointers for optional struct fields
	UsePointersForOptionalFields bool
	// List of known initialisms to properly format (e.g., "ID", "URL")
	Initialisms []string
	// Custom imports to include
	CustomImports []string
}

func main() {
	inFile := flag.String("in", "", "Input TypeScript file")
	outFile := flag.String("out", "", "Output Go file")
	packageName := flag.String("package", "generated", "Go package name")
	tsLib := flag.String("tslib", "", "Path to typescript.js library (optional, defaults to CDN version)")
	templatePath := flag.String("template", "", "Path to custom Go template file")
	usePointers := flag.Bool("use-pointers", true, "Use pointers for optional struct fields")
	configFile := flag.String("config", "", "Path to JSON configuration file")
	initConfig := flag.Bool("init-config", false, "Initialize default config file")
	enableMCP := flag.Bool("enable-mcp", false, "Enable MCP-specific transformations")
	verbose := flag.Bool("verbose", false, "Enable verbose logging")
	flag.Parse()

	// Handle config initialization if requested
	if *initConfig {
		configPath := "ts2go.config.json"
		if *configFile != "" {
			configPath = *configFile
		}
		if err := config.GenerateDefaultConfig(configPath); err != nil {
			log.Fatalf("Failed to generate config file: %v", err)
		}
		fmt.Printf("Generated default config file at: %s\n", configPath)
		return
	}

	if *inFile == "" {
		flag.Usage()
		fmt.Println("\nYou must specify an input TypeScript file with -in")
		os.Exit(1)
	}

	outFilePath := *outFile
	if outFilePath == "" {
		// Generate default output filename based on input
		base := filepath.Base(*inFile)
		ext := filepath.Ext(base)
		outFilePath = strings.TrimSuffix(base, ext) + ".go"
	}

	// Load configuration
	cfg := config.LoadConfig(*configFile)
	cfg.UsePointersForOptionalFields = *usePointers

	// Register transformers based on config
	if cfg.Transformers.EnableDefault {
		plugins.RegisterTransformer(plugins.NewDefaultTransformer())
		if *verbose {
			fmt.Println("Registered default transformer")
		}
	}

	if cfg.Transformers.EnableMCP || *enableMCP {
		plugins.RegisterTransformer(plugins.NewMCPTransformer())
		if *verbose {
			fmt.Println("Registered MCP transformer")
		}
	}

	// Register custom transformers from config
	for _, specialType := range cfg.SpecialTypes {
		transformer := customTransformerFromConfig(specialType)
		plugins.RegisterTransformer(transformer)
		if *verbose {
			fmt.Printf("Registered custom transformer for %s\n", specialType.TypeName)
		}
	}

	// Read the TypeScript file
	tsCode, err := os.ReadFile(*inFile)
	if err != nil {
		log.Fatalf("Failed to read input file: %v", err)
	}

	fmt.Printf("Parsing TypeScript file: %s\n", *inFile)

	// Load template
	templateContent, err := LoadTemplate(*templatePath)
	if err != nil {
		log.Fatalf("Failed to load template: %v", err)
	}

	// Extract schema using V8
	schema, err := extractSchemaWithV8(string(tsCode), *tsLib, *inFile, cfg)
	if err != nil {
		log.Fatalf("Failed to extract schema: %v", err)
	}

	// Ensure we have the required imports
	schema.Imports = ensureRequiredImports(schema, cfg)

	// Write the output
	if err := writeGoCode(outFilePath, schema, *packageName, *inFile, templateContent); err != nil {
		log.Fatalf("Failed to write Go code: %v", err)
	}

	fmt.Printf("Successfully generated Go code at: %s\n", outFilePath)
}

// customTransformerFromConfig creates a TypeTransformer from a SpecialType config
func customTransformerFromConfig(specialType config.SpecialType) plugins.TypeTransformer {
	return &customTransformer{
		typeName:     specialType.TypeName,
		goTypeName:   specialType.GoTypeName,
		isInterface:  specialType.IsInterface,
		methods:      specialType.Methods,
		fields:       specialType.Fields,
		imports:      specialType.Imports,
		customCode:   specialType.CustomCode,
	}
}

// customTransformer implements the TypeTransformer interface for config-based custom types
type customTransformer struct {
	typeName     string
	goTypeName   string
	isInterface  bool
	methods      []config.Method
	fields       []config.Field
	imports      []string
	customCode   string
}

func (c *customTransformer) Name() string {
	return fmt.Sprintf("CustomTransformer(%s)", c.typeName)
}

func (c *customTransformer) CanTransform(typeName string) bool {
	return typeName == c.typeName
}

func (c *customTransformer) Transform(typeName string, isOptional bool) (string, bool) {
	if typeName == c.typeName {
		return c.goTypeName, true
	}
	return "", false
}

func (c *customTransformer) GenerateCustomCode(typeName string) string {
	if typeName != c.typeName {
		return ""
	}

	if c.customCode != "" {
		return c.customCode
	}

	// Generate code using CodeGen if no custom code provided
	gen := plugins.NewCodeGen()
	
	if c.isInterface {
		methods := make([]string, 0, len(c.methods))
		for _, method := range c.methods {
			methods = append(methods, method.Name)
		}
		gen.AddInterface(c.goTypeName, methods, "")
	} else {
		fieldDefinitions := make([]plugins.FieldDefinition, 0, len(c.fields))
		for _, field := range c.fields {
			fieldDefinitions = append(fieldDefinitions, plugins.FieldDefinition{
				Name:        field.Name,
				Type:        field.Type,
				JSONName:    field.JSONName,
				Optional:    field.Optional,
				Description: field.Description,
			})
		}
		gen.AddStruct(c.goTypeName, fieldDefinitions, "")
	}

	// Add methods
	for _, method := range c.methods {
		if method.Signature != "" {
			gen.AddRawCode(fmt.Sprintf("%s {\n\t%s\n}", method.Signature, method.Body))
		}
	}

	return gen.String()
}

func (c *customTransformer) AdditionalImports(typeName string) []string {
	if typeName == c.typeName {
		return c.imports
	}
	return nil
}

// loadConfig loads configuration from a JSON file or returns default config
func loadConfig(configPath string) Config {
	config := Config{
		TypeMappings: map[string]string{
			"string":    "string",
			"number":    "float64",
			"boolean":   "bool",
			"any":       "interface{}",
			"void":      "struct{}",
			"null":      "nil",
			"undefined": "nil",
			"object":    "map[string]interface{}",
		},
		UsePointersForOptionalFields: true,
		Initialisms: []string{
			"ID", "URL", "URI", "JSON", "XML", "HTTP", "HTML", "API",
			"SQL", "RPC", "TCP", "UDP", "IP", "DNS", "EOF", "UUID",
		},
		CustomImports: []string{"encoding/json"},
	}

	if configPath == "" {
		return config
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		log.Printf("Warning: Could not read config file %s: %v", configPath, err)
		return config
	}

	if err := json.Unmarshal(data, &config); err != nil {
		log.Printf("Warning: Could not parse config file %s: %v", configPath, err)
		return config
	}

	return config
}

// ensureRequiredImports makes sure we have all necessary imports
func ensureRequiredImports(data *TemplateData, config Config) []string {
	imports := make(map[string]bool)

	// Add custom imports from config
	for _, imp := range config.CustomImports {
		imports[imp] = true
	}

	// Add encoding/json if we have interfaces
	if len(data.Interfaces) > 0 {
		imports["encoding/json"] = true
	}

	// Check if we need json schema
	needsJSONSchema := false
	for _, typ := range data.Types {
		if strings.Contains(typ.GoType, "jsonschema.Schema") {
			needsJSONSchema = true
			break
		}
	}
	if needsJSONSchema {
		imports["github.com/tmc/misc/ts2go/internal/jsonschema"] = true
	}

	// Convert map to slice
	result := make([]string, 0, len(imports))
	for imp := range imports {
		result = append(result, imp)
	}

	return result
}

func extractSchemaWithV8(tsCode, tsLibPath, filename string, config Config) (*TemplateData, error) {
	// If no TypeScript library path provided, get it from CDN
	if tsLibPath == "" {
		var err error
		tsLibPath, err = cdn.FetchTypeScript()
		if err != nil {
			return nil, fmt.Errorf("failed to fetch TypeScript from CDN: %v", err)
		}
	}

	// Read TypeScript library
	tsSource, err := os.ReadFile(tsLibPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read typescript.js: %v", err)
	}

	// Create V8 context
	isolate := v8.NewIsolate()
	ctx := v8.NewContext(isolate)
	defer func() {
		ctx.Close()
		isolate.Dispose()
	}()

	// Load TypeScript into context
	if _, err := ctx.RunScript(string(tsSource), "typescript.js"); err != nil {
		return nil, fmt.Errorf("failed to load TypeScript: %v", err)
	}

	// Pass the configuration to the extraction script
	configJSON, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize config: %v", err)
	}

	// Prepare the extraction script
	script := fmt.Sprintf(`
	(function() {
		try {
			// Parse config
			const config = %s;
			
			// Setup TypeScript compiler
			const sourceFile = ts.createSourceFile(
				"%s",
				%s,
				ts.ScriptTarget.Latest,
				true
			);

			// Extract schema
			const schema = {
				constants: [],
				types: [],
				interfaces: [],
				specialTypes: []
			};

			// Special handling for Model Context Protocol
			const isModelContextProtocol = sourceFile.text.includes('JSONRPCMessage') && 
				sourceFile.text.includes('Model Context Protocol');

			// Build custom type mappings from config
			const customTypeMappings = {};
			if (config && config.TypeMappings) {
				for (const [tsType, goType] of Object.entries(config.TypeMappings)) {
					customTypeMappings[tsType] = goType;
				}
			}

			function getNodeComments(node) {
				let commentText = "";
				const leadingComments = ts.getLeadingCommentRanges(sourceFile.text, node.pos);
				if (leadingComments) {
					commentText = leadingComments.map(c => {
						const commentStr = sourceFile.text.substring(c.pos, c.end);
						// Clean up comments but preserve line breaks
						return commentStr
							.replace(/^\s*\/\*\*|\*\/\s*$/g, '')
							.replace(/^\s*\*\s?/gm, '')
							.replace(/^\s*\/\/\s?/gm, '')
							.trim();
					}).join(' ').trim();
				}
				return commentText;
			}

			function extractType(type, isOptional = false) {
				if (!type) return 'interface{}';

				if (type.kind) {
					const kindName = ts.SyntaxKind[type.kind];
					
					// Handle basic types
					if (kindName === 'StringKeyword') return 'string';
					if (kindName === 'NumberKeyword') return 'float64';
					if (kindName === 'BooleanKeyword') return 'bool';
					if (kindName === 'AnyKeyword') return 'interface{}';
					if (kindName === 'VoidKeyword') return 'struct{}';
					if (kindName === 'NullKeyword') return 'nil';
					if (kindName === 'UndefinedKeyword') return 'nil';
					if (kindName === 'ObjectKeyword') return 'map[string]interface{}';
					
					// Handle array types
					if (kindName === 'ArrayType') {
						return '[]' + extractType(type.elementType);
					}
					
					// Handle map/record types
					if (kindName === 'TypeLiteral' && type.members) {
						// Check if it's a record type like Record<string, T>
						const indexSignature = type.members.find(m => m.kind === ts.SyntaxKind.IndexSignature);
						if (indexSignature && indexSignature.parameters && indexSignature.parameters.length > 0) {
							const keyType = extractType(indexSignature.parameters[0].type);
							const valueType = extractType(indexSignature.type);
							
							if (keyType === 'string') {
								return 'map[string]' + valueType;
							}
						}
						return 'map[string]interface{}';
					}
					
					// Handle union types
					if (kindName === 'UnionType' && type.types) {
						// Check for string literal unions (enum-like)
						const isStringEnum = type.types.every(t => 
							t.kind === ts.SyntaxKind.LiteralType && 
							t.literal && 
							t.literal.kind === ts.SyntaxKind.StringLiteral);

						if (isStringEnum) return 'string';
						
						// Check if it's a nullable type (T | null | undefined)
						const nonNullTypes = type.types.filter(t => 
							t.kind !== ts.SyntaxKind.NullKeyword && 
							t.kind !== ts.SyntaxKind.UndefinedKeyword
						);
						
						if (nonNullTypes.length === 1) {
							// This is T | null/undefined
							return extractType(nonNullTypes[0]);
						}

						// If it's a union that contains at least one interface or object type,
						// and is an optional field, use a pointer to json.RawMessage
						if (isOptional && config.UsePointersForOptionalFields) {
							const hasComplexType = type.types.some(t => 
								t.kind === ts.SyntaxKind.TypeReference || 
								t.kind === ts.SyntaxKind.ObjectType ||
								t.kind === ts.SyntaxKind.TypeLiteral
							);
							if (hasComplexType) {
								return "*json.RawMessage";
							}
						}

						// General union becomes interface{}
						return 'interface{}';
					}
					
					// Handle reference to other types
					if (kindName === 'TypeReference' && type.typeName) {
						const typeName = type.typeName.text;
						
						// Try to get from type mappings first
						if (config.TypeMappings && config.TypeMappings[typeName]) {
							return config.TypeMappings[typeName];
						}
						
						// Handle built-in generic types
						if (typeName === 'Array' && type.typeArguments && type.typeArguments.length > 0) {
							return '[]' + extractType(type.typeArguments[0]);
						}
						
						if (typeName === 'Record' && type.typeArguments && type.typeArguments.length > 1) {
							const keyType = extractType(type.typeArguments[0]);
							const valueType = extractType(type.typeArguments[1]);
							
							if (keyType === 'string') {
								return 'map[string]' + valueType;
							}
						}
						
						// Check for custom transformers
						if (customTypeMappings[typeName]) {
							return customTypeMappings[typeName];
						}
						
						// Handle optional struct references with pointers if configured
						if (isOptional && config.UsePointersForOptionalFields) {
							// If this is a complex type and optional, use pointer
							return '*' + typeName;
						}
						
						// Pass through other type references
						return typeName;
					}
					
					// Handle other complex types
					return 'interface{}';
				}
				
				return 'interface{}';
			}

			// Format Go names according to Go conventions
			function formatGoName(name) {
				if (!name) return '';
				
				// Special handling for MCP known names
				if (isModelContextProtocol) {
					const mcpSpecialNames = {
						'uri': 'URI',
						'url': 'URL',
						'id': 'ID',
						'jsonrpc': 'JSONRPC',
						'json': 'JSON',
						'rpc': 'RPC',
						'http': 'HTTP',
						'mime': 'MIME'
					};
					
					for (const [key, value] of Object.entries(mcpSpecialNames)) {
						if (name.toLowerCase() === key) {
							return value;
						}
						
						// Handle case where it's part of a longer name
						if (name.toLowerCase().includes(key)) {
							const pattern = new RegExp(key, 'i');
							name = name.replace(pattern, value);
						}
					}
				}
				
				// Format according to Go naming conventions
				let goName = '';
				
				// PascalCase for field names (exported)
				if (name.includes('_')) {
					// handle snake_case
					goName = name.split('_')
						.map(part => part.charAt(0).toUpperCase() + part.slice(1))
						.join('');
				} else if (name.includes('-')) {
					// handle kebab-case
					goName = name.split('-')
						.map(part => part.charAt(0).toUpperCase() + part.slice(1))
						.join('');
				} else if (name.match(/^[A-Z0-9_]+$/)) {
					// ALL_CAPS constants - PascalCase them
					goName = name.toLowerCase()
						.split('_')
						.map(part => part.charAt(0).toUpperCase() + part.slice(1))
						.join('');
				} else {
					// Standard case - just capitalize first letter
					goName = name.charAt(0).toUpperCase() + name.slice(1);
				}
				
				// Fix common acronyms to be properly cased in Go
				for (const initialism of config.Initialisms) {
					// This regex finds the initialism unless it's all uppercase already
					const pattern = new RegExp(
						'([a-z])' + initialism.charAt(0) + initialism.slice(1).toLowerCase() + '([A-Z]|$)',
						'g'
					);
					goName = goName.replace(pattern, '$1' + initialism + '$2');
					
					// Also handle beginning of string
					if (goName.toLowerCase().startsWith(initialism.toLowerCase()) &&
						goName.substr(0, initialism.length) !== initialism) {
						goName = initialism + goName.substr(initialism.length);
					}
				}
				
				return goName;
			}

			// Process all nodes in the source file
			function visit(node) {
				// Extract constants
				if (node.kind === ts.SyntaxKind.VariableStatement) {
					if (node.declarationList && 
						node.declarationList.declarations && 
						node.modifiers && 
						node.modifiers.some(m => m.kind === ts.SyntaxKind.ExportKeyword)) {
						
						for (const decl of node.declarationList.declarations) {
							if (decl.name && decl.initializer && decl.name.text) {
								const name = decl.name.text;
								let value = '';
								
								// Special handling for MCP constants
								if (isModelContextProtocol) {
									if (name === 'LATEST_PROTOCOL_VERSION') {
										schema.constants.push({
											name: 'LatestProtocolVersion',
											value: '"' + decl.initializer.text + '"',
											description: 'LatestProtocolVersion is the current version of the Model Context Protocol'
										});
										continue;
									} else if (name === 'JSONRPC_VERSION') {
										schema.constants.push({
											name: 'JSONRPCVersion',
											value: '"' + decl.initializer.text + '"',
											description: 'JSONRPCVersion is the JSON-RPC version used by the protocol (2.0)'
										});
										continue;
									}
								}
								
								// Handle different initializer types
								if (decl.initializer.kind === ts.SyntaxKind.StringLiteral) {
									value = '"' + decl.initializer.text + '"';
								} else if (decl.initializer.kind === ts.SyntaxKind.NumericLiteral) {
									value = decl.initializer.text;
								} else if (decl.initializer.kind === ts.SyntaxKind.TrueKeyword) {
									value = 'true';
								} else if (decl.initializer.kind === ts.SyntaxKind.FalseKeyword) {
									value = 'false';
								} else {
									// Skip complex initializers
									continue;
								}
								
								const comments = getNodeComments(node);
								
								schema.constants.push({
									name: name,
									value: value,
									description: comments
								});
							}
						}
					}
				}
				
				// Extract type aliases
				if (node.kind === ts.SyntaxKind.TypeAliasDeclaration && 
					node.name && 
					node.modifiers && 
					node.modifiers.some(m => m.kind === ts.SyntaxKind.ExportKeyword)) {
					
					const name = node.name.text;
					const goType = extractType(node.type);
					const comments = getNodeComments(node);
					
					// Skip certain interface types in ModelContextProtocol that we'll handle separately
					if (isModelContextProtocol && 
						(name === 'JSONRPCMessage' || name === 'Content' || 
						name === 'ResourceContents' || name === 'Reference')) {
						continue;
					}
					
					// Special handling for RequestId -> RequestID with struct implementation
					if (isModelContextProtocol && name === 'RequestId') {
						// Add our custom RequestID struct
						const requestIDCode = "// StringID creates a new string request identifier.\nfunc StringID(s string) RequestID { return RequestID{value: s} }";

						schema.interfaces.push({
							name: 'RequestID',
							description: 'RequestID is a Request identifier.\\nIt can be either a string or a number value.',
							fields: [{
								name: 'value',
								type: 'interface{}',
								description: ''
							}],
							customCode: requestIDCode
						});
						continue;
					}
					
					schema.types.push({
						name: name,
						goType: goType,
						description: comments
					});
				}
				
				// Extract interfaces
				if (node.kind === ts.SyntaxKind.InterfaceDeclaration && 
					node.name && 
					node.modifiers && 
					node.modifiers.some(m => m.kind === ts.SyntaxKind.ExportKeyword)) {
					
					const name = node.name.text;
					const comments = getNodeComments(node);
					const fields = [];
					
					if (node.members) {
						for (const member of node.members) {
							if (member.name) {
								const fieldName = member.name.text;
								const jsonName = fieldName;
								const optional = !!member.questionToken;
								let goType = extractType(member.type, optional);
								const fieldComments = getNodeComments(member);
								
								// Apply pointer for optional struct fields if configured
								if (optional && config.UsePointersForOptionalFields && 
									!goType.startsWith('*') && !goType.startsWith('[]') && 
									!goType.startsWith('map[') && 
									!['string', 'bool', 'int', 'int64', 'float64', 'interface{}'].includes(goType)) {
									goType = '*' + goType;
								}
								
								// Fix Go naming conventions
								const goFieldName = formatGoName(fieldName);
								
								fields.push({
									name: goFieldName,
									type: goType,
									jsonName: jsonName,
									optional: optional,
									description: fieldComments
								});
							}
						}
					}
					
					// Interface implementation method for JSONRPCMessage types
					let interfaceMethods = [];
					if (isModelContextProtocol && name.includes('JSONRPC')) {
						interfaceMethods.push({
							name: 'isJSONRPCMessage',
							code: 'func (*' + name + ') isJSONRPCMessage() {}'
						});
					}
					
					// Client/Server request/notification/result interfaces
					if (isModelContextProtocol && 
						(name === 'ClientRequest' || name === 'ClientNotification' || 
						name === 'ClientResult' || name === 'ServerRequest' || 
						name === 'ServerNotification' || name === 'ServerResult')) {
						const methodName = 'is' + name;
						interfaceMethods.push({
							name: methodName,
							code: 'func (' + name + ') ' + methodName + '() {}'
						});
					}
					
					// Content interface type methods
					if (isModelContextProtocol && 
						(name === 'TextContent' || name === 'ImageContent' || 
						name === 'AudioContent' || name === 'EmbeddedResource')) {
						interfaceMethods.push({
							name: 'contentType',
							code: 'func (' + name + ') contentType() string { return "' + 
								name.replace('Content', '').toLowerCase() + '" }'
						});
					}
					
					// Check if this is ResourceContents type
					if (isModelContextProtocol && 
						(name === 'TextResourceContents' || name === 'BlobResourceContents')) {
						interfaceMethods.push({
							name: 'GetURI',
							code: 'func (r *' + name + ') GetURI() string { return r.URI }'
						});
						interfaceMethods.push({
							name: 'GetMimeType',
							code: 'func (r *' + name + ') GetMimeType() string { return r.MimeType }'
						});
					}
					
					// Add methods to the interface
					if (interfaceMethods.length > 0) {
						schema.interfaces.push({
							name: name,
							description: comments,
							fields: fields,
							methods: interfaceMethods
						});
					} else {
						schema.interfaces.push({
							name: name,
							description: comments,
							fields: fields
						});
					}
				}
				
				// Recursively visit all children
				ts.forEachChild(node, visit);
			}
			
			// Start the traversal
			visit(sourceFile);
			
			return JSON.stringify(schema);
		} catch (err) {
			return JSON.stringify({ error: String(err) });
		}
	})();
	`, string(configJSON), filepath.Base(filename), formatJSString(tsCode))

	// Execute the script
	result, err := ctx.RunScript(script, "extract-schema.js")
	if err != nil {
		return nil, fmt.Errorf("failed to extract schema: %v", err)
	}

	// Parse the result
	var templateData TemplateData
	// First, parse into a map
	var rawSchema map[string]interface{}
	if err := json.Unmarshal([]byte(result.String()), &rawSchema); err != nil {
		return nil, fmt.Errorf("failed to parse schema: %v", err)
	}

	// Check for errors
	if errMsg, ok := rawSchema["error"]; ok {
		return nil, fmt.Errorf("extraction error: %v", errMsg)
	}

	// Parse constants
	if constArr, ok := rawSchema["constants"].([]interface{}); ok {
		for _, c := range constArr {
			if constMap, ok := c.(map[string]interface{}); ok {
				constant := Constant{
					Name:        getString(constMap, "name"),
					Value:       getString(constMap, "value"),
					Description: getString(constMap, "description"),
				}
				templateData.Constants = append(templateData.Constants, constant)
			}
		}
	}

	// Parse types
	if typesArr, ok := rawSchema["types"].([]interface{}); ok {
		for _, t := range typesArr {
			if typeMap, ok := t.(map[string]interface{}); ok {
				typeDef := TypeDef{
					Name:        getString(typeMap, "name"),
					GoType:      getString(typeMap, "goType"),
					Description: getString(typeMap, "description"),
				}
				templateData.Types = append(templateData.Types, typeDef)
			}
		}
	}

	// Parse interfaces
	if interfacesArr, ok := rawSchema["interfaces"].([]interface{}); ok {
		for _, i := range interfacesArr {
			if ifaceMap, ok := i.(map[string]interface{}); ok {
				iface := Interface{
					Name:        getString(ifaceMap, "name"),
					Description: getString(ifaceMap, "description"),
				}

				// Parse fields
				if fieldsArr, ok := ifaceMap["fields"].([]interface{}); ok {
					for _, f := range fieldsArr {
						if fieldMap, ok := f.(map[string]interface{}); ok {
							field := Field{
								Name:        getString(fieldMap, "name"),
								Type:        getString(fieldMap, "type"),
								JSONName:    getString(fieldMap, "jsonName"),
								Optional:    getBool(fieldMap, "optional"),
								Description: getString(fieldMap, "description"),
							}
							iface.Fields = append(iface.Fields, field)
						}
					}
				}

				// Parse methods
				if methodsArr, ok := ifaceMap["methods"].([]interface{}); ok {
					for _, m := range methodsArr {
						if methodMap, ok := m.(map[string]interface{}); ok {
							method := Method{
								Name: getString(methodMap, "name"),
								Code: getString(methodMap, "code"),
							}
							iface.Methods = append(iface.Methods, method)
						}
					}
				}

				templateData.Interfaces = append(templateData.Interfaces, iface)
			}
		}
	}

	templateData.Filename = filename

	return &templateData, nil
}

func writeGoCode(outFile string, data *TemplateData, packageName string, inFile string, templateContent string) error {
	// Set package name
	data.Package = packageName

	// Create template function map
	funcMap := template.FuncMap{
		"formatComment": formatComment,
	}

	// Parse the template with the function map
	tmpl, err := template.New("gocode").Funcs(funcMap).Parse(templateContent)
	if err != nil {
		return fmt.Errorf("failed to parse template: %v", err)
	}

	// Create the output file
	f, err := os.Create(outFile)
	if err != nil {
		return fmt.Errorf("failed to create output file: %v", err)
	}
	defer f.Close()

	// Apply the template
	if err := tmpl.Execute(f, data); err != nil {
		return fmt.Errorf("failed to execute template: %v", err)
	}

	return nil
}

// formatComment formats a comment to be Go-idiomatic, preserving line breaks
func formatComment(comment string) string {
	if comment == "" {
		return ""
	}

	// Split by lines and format each line
	lines := strings.Split(comment, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}

	// Join with proper line breaks
	return strings.Join(lines, "\n// ")
}

// Helper functions
func getString(m map[string]interface{}, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}

func getBool(m map[string]interface{}, key string) bool {
	if val, ok := m[key].(bool); ok {
		return val
	}
	return false
}

func formatJSString(input string) string {
	// Escape backslashes and quotes
	escaped := strings.ReplaceAll(input, "\\", "\\\\")
	escaped = strings.ReplaceAll(escaped, "'", "\\'")
	escaped = strings.ReplaceAll(escaped, "\"", "\\\"")

	// Escape newlines for JS string
	escaped = strings.ReplaceAll(escaped, "\n", "\\n")
	escaped = strings.ReplaceAll(escaped, "\r", "\\r")

	// Wrap in quotes
	return fmt.Sprintf("\"%s\"", escaped)
}
