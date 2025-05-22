# ts2go - TypeScript to Go Converter

ts2go is a powerful tool for converting TypeScript definitions to Go code, with a focus on preserving types, interfaces, and documentation.

## Features

- **Convert TypeScript to Go** - Automatically convert TypeScript interfaces, types, and constants to idiomatic Go
- **Plugin System** - Extensible plugin architecture for custom type transformations
- **JSON Schema Support** - Proper handling of JSON Schema types and constraints
- **Powerful Configuration** - Extensive configuration options via JSON configuration files
- **Special Type Handling** - Custom handling for complex types with special behavior
- **Interface Implementation** - Support for implementing Go interfaces and methods

## Installation

```bash
go install github.com/tmc/misc/ts2go@latest
```

## Quick Start

Convert a TypeScript file to Go:

```bash
ts2go -in schema.ts -out schema.go -package mypackage
```

Generate a default configuration file:

```bash
ts2go -init-config
```

Convert with MCP protocol support:

```bash
ts2go -in schema.ts -out schema.go -package schema -enable-mcp
```

## Command Line Options

- `-in`: Input TypeScript file (required)
- `-out`: Output Go file (default: input filename with .go extension)
- `-package`: Go package name (default: "generated")
- `-tslib`: Path to typescript.js library (optional, defaults to CDN version)
- `-template`: Path to custom Go template file (optional)
- `-use-pointers`: Whether to use pointers for optional struct fields (default: true)
- `-config`: Path to JSON configuration file
- `-init-config`: Initialize a default configuration file
- `-enable-mcp`: Enable Model Context Protocol specific transformations
- `-verbose`: Enable verbose logging

## Configuration

ts2go uses a JSON configuration file for customization. You can generate a default config file with `-init-config` or create one manually.

Example configuration:

```json
{
  "typeMappings": {
    "string": "string",
    "number": "float64",
    "boolean": "bool",
    "any": "interface{}"
  },
  "usePointersForOptionalFields": true,
  "initialisms": ["ID", "URL", "URI", "JSON"],
  "customImports": ["encoding/json"],
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
  ],
  "transformers": {
    "enableDefault": true,
    "enableMCP": false
  }
}
```

### Configuration Options

- **typeMappings**: Maps TypeScript types to Go types
- **usePointersForOptionalFields**: Uses pointers for optional struct fields
- **initialisms**: List of initialisms to preserve casing (e.g., "ID", "URL")
- **customImports**: Additional imports to include
- **specialTypes**: Custom type definitions with fields and methods
- **transformers**: Enable/disable specific transformers

## Type Conversion Rules

TypeScript types are converted to Go according to these rules:

| TypeScript | Go |
|------------|-----|
| string | string |
| number | float64 |
| boolean | bool |
| any | interface{} |
| void | struct{} |
| null | nil |
| undefined | nil |
| string[] | []string |
| number[] | []float64 |
| Record<string, T> | map[string]T |
| string literal union | string |
| T \| null \| undefined | *T (with omitempty json tag) |
| complex object | struct |

## Plugin System

ts2go includes a plugin system that allows you to customize how TypeScript types are converted to Go. The plugin system is based on the `TypeTransformer` interface:

```go
type TypeTransformer interface {
    // Name returns the name of the transformer
    Name() string
    
    // CanTransform returns true if this transformer can handle the given TypeScript type
    CanTransform(typeName string) bool
    
    // Transform converts a TypeScript type to a Go type
    Transform(typeName string, isOptional bool) (string, bool)
    
    // GenerateCustomCode returns any custom code that should be added to the Go file
    GenerateCustomCode(typeName string) string
    
    // AdditionalImports returns any additional imports needed for this type
    AdditionalImports(typeName string) []string
}
```

You can register custom transformers in your configuration file using the `specialTypes` section.

## Example: Model Context Protocol (MCP)

ts2go includes special support for the Model Context Protocol (MCP), which requires special handling for interfaces, JSON-RPC types, and more.

Enable MCP support with the `-enable-mcp` flag or in your configuration:

```json
{
  "transformers": {
    "enableMCP": true
  }
}
```

## Custom Templates

You can provide a custom Go template to change the output format with the `-template` flag. The template has access to:

- `.Package`: The Go package name
- `.Constants`: List of constants
- `.Types`: List of type definitions
- `.Interfaces`: List of interfaces
- `.Imports`: List of imports
- `.Filename`: Original TypeScript filename

The template also has access to the `formatComment` function which formats comments to be Go-idiomatic.

## License

MIT