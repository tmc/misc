# ts2go Enhancements

We've significantly enhanced ts2go to make it a much more powerful and extensible tool for converting TypeScript to Go. This document outlines the major improvements.

## Key Improvements

### 1. Plugin System

We've added a comprehensive plugin system based on the `TypeTransformer` interface:

```go
type TypeTransformer interface {
    Name() string
    CanTransform(typeName string) bool
    Transform(typeName string, isOptional bool) (string, bool)
    GenerateCustomCode(typeName string) string
    AdditionalImports(typeName string) []string
}
```

This plugin architecture allows:
- Custom type transformations
- Special handling for complex types
- Implementation of interface methods
- Custom code generation
- Project-specific extensions

### 2. Configuration System

We've completely revamped the configuration system:

- JSON-based configuration with more options
- Support for registering custom transformers
- Special type definitions with fields and methods
- Custom code blocks for complex types
- Configuration file auto-discovery

Example:
```json
{
  "typeMappings": { "string": "string" },
  "specialTypes": [
    {
      "typeName": "RequestId",
      "goTypeName": "RequestID",
      "fields": [{"name": "value", "type": "interface{}"}],
      "methods": [
        {
          "name": "StringID",
          "signature": "func StringID(s string) RequestID",
          "body": "return RequestID{value: s}"
        }
      ]
    }
  ]
}
```

### 3. Code Generation

We've added a powerful code generation system:

- Structured code generation with the `CodeGen` helper
- Support for interface and method generation
- Full control over field definitions and JSON tags
- Support for custom method implementations
- Preservation of documentation comments

Example:
```go
gen := plugins.NewCodeGen()
gen.AddInterface("JSONRPCMessage", []string{"isJSONRPCMessage()"}, "JSON-RPC message interface")
gen.AddMethod("JSONRPCRequest", "isJSONRPCMessage", "", "", "// Implementation")
```

### 4. Type Handling Improvements

- Better support for optional fields with pointers
- Improved handling of union types
- Support for complex interface hierarchies
- Custom JSON tag generation
- Proper handling of Go naming conventions

### 5. Domain-Specific Support

We've added framework for domain-specific support:

- Built-in MCP protocol transformer
- Support for JSON-RPC message types
- Interface implementation generation
- Special content type handling
- Complex type hierarchies

### 6. Command Line Improvements

- Support for generating default config
- Verbose logging option
- Domain-specific flags
- Better error reporting
- Support for custom templates

## How It Works

### Plugin Registration

Plugins are registered at startup:

```go
// Register default transformer
plugins.RegisterTransformer(plugins.NewDefaultTransformer())

// Register MCP transformer if enabled
if cfg.Transformers.EnableMCP || *enableMCP {
    plugins.RegisterTransformer(plugins.NewMCPTransformer())
}

// Register custom transformers from config
for _, specialType := range cfg.SpecialTypes {
    transformer := customTransformerFromConfig(specialType)
    plugins.RegisterTransformer(transformer)
}
```

### Type Transformation Process

1. Extract TypeScript types using the TypeScript compiler API
2. For each type, check if a custom transformer can handle it
3. Apply the transformation to convert to Go type
4. Generate any special code needed (methods, imports, etc.)
5. Assemble the final Go code

### Custom Type Definition

Custom types can be defined in the configuration:

```json
{
  "specialTypes": [
    {
      "typeName": "RequestId",
      "goTypeName": "RequestID",
      "isInterface": false,
      "fields": [
        {
          "name": "value",
          "type": "interface{}",
          "optional": false
        }
      ],
      "methods": [
        {
          "name": "StringID",
          "signature": "func StringID(s string) RequestID",
          "body": "return RequestID{value: s}"
        }
      ],
      "imports": ["encoding/json", "fmt"]
    }
  ]
}
```

These types are then registered as custom transformers at runtime.

## Example: MCP Protocol Support

The Model Context Protocol (MCP) requires special handling:

1. Interface implementations for `JSONRPCMessage`
2. Content type interfaces and implementations
3. Special handling for `RequestID` type
4. Custom method implementations

Our new plugin system makes this easy:

```go
transformer := &MCPTransformer{
    DefaultTransformer: NewDefaultTransformer(),
}

// Register JSONRPCMessage interface
transformer.RegisterSpecialType(SpecialType{
    TypeName:   "JSONRPCMessage",
    GoTypeName: "JSONRPCMessage",
    CustomCode: "type JSONRPCMessage interface {\n\tisJSONRPCMessage()\n}",
})

// Register Content interface
transformer.RegisterSpecialType(SpecialType{
    TypeName:   "Content",
    GoTypeName: "Content",
    CustomCode: "type Content interface {\n\tcontentType() string\n}",
})
```

## Benefits

1. **Flexibility**: Easily extend ts2go for specific projects
2. **Domain-Specific Support**: Add handlers for specialized protocols
3. **Custom Type Control**: Fine-grained control over generated code
4. **Interface Implementation**: Proper handling of Go interfaces
5. **Documentation Preservation**: Better comment handling
6. **Configuration**: Rich configuration options

## Future Directions

1. **Validation**: Add validation of TypeScript against schemas
2. **Testing**: Generate tests for converted types
3. **More Domain Support**: Add support for more protocols and frameworks
4. **Code Refactoring**: Support for refactoring existing code
5. **Web Interface**: Add a web interface for easier use 