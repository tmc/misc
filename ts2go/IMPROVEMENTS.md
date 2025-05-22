# ts2go Improvements

Based on our comparison with the MCRP repository, we've implemented several improvements to the ts2go tool:

## 1. Better Type Handling

- **Improved Type Mapping**: Added customizable type mappings via configuration file
- **Pointer Usage**: Now uses pointers for optional struct fields, similar to the MCRP implementation
- **Complex Type Handling**: Better handling of union types, arrays, and maps

## 2. Comment Formatting

- **Multiline Comments**: Improved handling of multiline comments with proper formatting
- **Preservation of Structure**: Better preservation of comment structure from TypeScript to Go
- **Doc Blocks**: Support for documentation blocks that span multiple lines

## 3. Field Naming

- **Go Naming Conventions**: Improved field name transformations following Go conventions
- **Initialisms**: Better handling of initialisms (ID, URI, URL, etc.) for proper Go casing
- **Custom Transformations**: Support for custom name transformations via configuration

## 4. Content Type Handling

- **Special Types**: Added special handling for Content types similar to MCRP
- **Resource Type**: Support for embedded resources within content
- **MIME Type Support**: Proper handling of MIME types and data fields

## 5. JSON Schema Support

- **JSON Schema Types**: Added internal JSON Schema package for better type compatibility
- **Schema References**: Support for schema references with proper handling

## 6. Configuration System

- **Customizable Behavior**: Added a configuration file system to customize type mappings and other behavior
- **Custom Imports**: Support for specifying additional imports as needed
- **Optional Features**: Configuration options for optional features like pointer usage

## 7. Template Customization

- **Custom Templates**: Support for custom Go templates to change the output format
- **Template Functions**: Added template functions like `formatComment` for better output control
- **Flexible Output**: More flexible output formatting options

## 8. Project Structure

- **Modular Design**: Improved project structure with separate packages for different concerns
- **Internal Types**: Added internal types package for special type handling
- **Better Organization**: Better organization of code for maintainability

## Comparison with MCRP

While the MCRP implementation focuses on converting JSON Schema to Go with a declarative configuration system, our improved ts2go tool directly parses TypeScript files and converts them to Go, while incorporating many of the sophisticated features from MCRP:

- **Direct TypeScript Parsing**: No need for an intermediate JSON Schema step
- **V8 JavaScript Engine**: Uses the TypeScript compiler API via V8 for accurate parsing
- **Custom Configuration**: Similar configurability to MCRP but with a simpler approach
- **Special Type Handling**: Similar special handling for common patterns

These improvements make ts2go more powerful, flexible, and aligned with idiomatic Go code generation practices, while maintaining its simplicity and ease of use.

# Improvement Ideas for ts2go

## MCP Protocol Specific Improvements

1. **Interface Method Generation**
   - Add support for generating interface implementation methods like `isJSONRPCMessage()`
   - Support for Content interface methods (`contentType()`)
   - Generate implementation methods for request/response interfaces

2. **Special Type Handling**
   - Better handling of RequestID type with proper Go struct implementation
   - Specific handling for the ProgressToken and Cursor types
   - Support for special JSON marshaling methods

3. **Field Naming**
   - Improve handling of acronyms like URI, URL, JSON, etc.
   - Ensure consistent capitalization of field names

## General Improvements

1. **Comment Preservation**
   - Better handling of TypeScript comments with proper formatting
   - Support for multi-line comments and JSDoc annotations

2. **Type Conversion**
   - More accurate mapping of TypeScript types to Go
   - Better handling of union types and complex types

3. **Code Generation Template**
   - Add support for custom code generation after type declarations
   - Include interface methods in the generated output

4. **JSON Tags**
   - Better handling of JSON tag options (omitempty)
   - Support for custom JSON tag logic

5. **Code Structure**
   - Support for Go code organization (constants, types, interfaces)
   - Option to preserve the original order of declarations

## Implementation Strategies

### For RequestID Type
```go
// RequestID is a Request identifier.
// It can be either a string or a number value.
type RequestID struct {
    value interface{}
}

// StringID creates a new string request identifier.
func StringID(s string) RequestID { return RequestID{value: s} }

// Int64ID creates a new integer request identifier.
func Int64ID(i int64) RequestID { return RequestID{value: i} }

// Float64ID creates a new float64 request identifier.
func Float64ID(f float64) RequestID { return RequestID{value: f} }

// IsValid returns true if the ID is a valid identifier.
// The default value for RequestID will return false.
func (id RequestID) IsValid() bool { return id.value != nil }

// String returns a string representation of the ID.
func (id RequestID) String() string {
    if !id.IsValid() {
        return "<invalid>"
    }
    return fmt.Sprintf("%v", id.value)
}

// MarshalJSON implements the json.Marshaler interface.
func (id RequestID) MarshalJSON() ([]byte, error) {
    if !id.IsValid() {
        return []byte("null"), nil
    }
    return json.Marshal(id.value)
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (id *RequestID) UnmarshalJSON(data []byte) error {
    // Implementation details
}
```

### For Content Interface Methods
```go
// Content is an interface for different content types.
type Content interface {
    contentType() string
}

// TextContent represents text provided to or from an LLM.
type TextContent struct {
    // Fields
}

func (TextContent) contentType() string { return "text" }

// ImageContent represents an image provided to or from an LLM.
type ImageContent struct {
    // Fields
}

func (ImageContent) contentType() string { return "image" }
```

### For Client/Server Request Types
```go
// ClientRequest is an interface for all possible request types from the client.
type ClientRequest interface {
    isClientRequest()
}

func (InitializeParams) isClientRequest() {}
func (CompleteParams) isClientRequest() {}
// Other implementations
``` 