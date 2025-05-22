# Protocol Buffer to JSONSchema Generator

This tool generates [JSONSchema](https://json-schema.org/) definitions from Protocol Buffer message definitions. It can be used as a standalone protoc plugin to generate schema files directly.

## Features

- Generates JSONSchema draft-07 compatible schemas
- Converts Protocol Buffer messages to JSONSchema with proper type mapping
- Handles nested messages, enums, maps, and repeated fields
- Supports oneOf for proto oneofs
- Handles well-known protobuf types like Timestamp, Duration, etc.
- Special handling for int64/uint64 values as strings to avoid precision loss
- Supports multiple configuration options for customizing schema generation

## Installation

```bash
go install github.com/tmc/misc/protoc-gen-jsonschema@latest
```

## Usage

### Basic Example

```bash
protoc --jsonschema_out=. your_proto_file.proto
```

### With Options

```bash
protoc \
  --jsonschema_out=disallow_additional_properties,allow_null_values,file_extension=schema.json:./schemas \
  --proto_path=proto your_proto_file.proto
```

### Generate Only Specific Messages

```bash
protoc \
  --jsonschema_out=messages=[Message1+Message2+Message3]:./schemas \
  --proto_path=proto your_proto_file.proto
```

## Configuration Options

### Output Options

| Option | Description | Default |
|--------|-------------|---------|
| `output_dir` | Output directory for schema files | Same as protoc output |
| `file_extension` | File extension for generated schemas | `json` |
| `prefix_schema_files_with_package` | Prefix schema files with package name | `false` |
| `debug` | Enable debug logging | `false` |

### Schema Behavior Options

| Option | Description | Default |
|--------|-------------|---------|
| `allow_null_values` / `nullable` | Mark optional fields as nullable | `false` |
| `embed_defs` | Embed definitions instead of using references | `false` |
| `all_fields_required` | Mark all fields as required | `false` |
| `disallow_additional_properties` | Prevent additional properties in objects | `false` |
| `enforce_oneof` | Enforce oneof fields with JSON Schema oneOf | `false` |

### Field Options

| Option | Description | Default |
|--------|-------------|---------|
| `json_fieldnames` | Use JSON field names | `true` |
| `enums_as_constants` | Generate enums as constants (both string and numeric values) | `false` |
| `bigints_as_strings` | Represent 64-bit integers as strings | `true` |
| `disallow_bigints_as_strings` | Represent 64-bit integers as numbers | `false` |

## Special Type Handling

### Well-Known Types

| Protobuf Type | JSONSchema Type |
|---------------|-----------------|
| Timestamp | string with format date-time |
| Duration | string with pattern |
| Any/Struct | object with additionalProperties |
| ListValue | array |
| *Value wrappers | Corresponding type |

### Scalar Types

| Protobuf Type | JSONSchema Type |
|---------------|-----------------|
| double/float | number |
| int32/sfixed32/sint32 | integer with format int32 |
| int64/sfixed64/sint64 | string with pattern (when bigints_as_strings=true) |
| uint32/fixed32 | integer with minimum 0 |
| uint64/fixed64 | string with pattern (when bigints_as_strings=true) |
| bool | boolean |
| string | string |
| bytes | string with contentEncoding base64 |

## Using as a Library

You can also use this as a library in your own Go code:

```go
import (
    "fmt"
    
    "github.com/tmc/misc/protoc-gen-jsonschema/jsonschema"
    "google.golang.org/protobuf/compiler/protogen"
)

func generateSchema(msg *protogen.Message) {
    generator := jsonschema.NewGenerator(true, false)
    generator.DisallowAdditionalProps = true
    generator.AllFieldsRequired = true
    
    jsonData, err := generator.GenerateJSON(msg, true)
    if err != nil {
        panic(err)
    }
    fmt.Println(string(jsonData))
}
```

## Integration with protoc-gen-anything

For advanced templating and customization, this can be used with [protoc-gen-anything](https://github.com/tmc/misc/protoc-gen-anything). 
See the example in `../protoc-gen-anything/examples/jsonschema` for more details.