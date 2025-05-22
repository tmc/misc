# OpenAPI Example

This example demonstrates how to generate OpenAPI 3.1 YAML specifications from Protocol Buffer definitions using `protoc-gen-anything`.

## Features

- Converts protobuf services and methods to OpenAPI paths
- Extracts HTTP method and path information from `google.api.http` annotations
- Maps protobuf types to OpenAPI schema types
- Generates a single unified OpenAPI specification

## Usage

Generate the OpenAPI specification with:

```bash
make generate
```

This will produce an `openapi.yaml` file in the `gen` directory.

## How it Works

The example uses a file-level template that:

1. Processes all services and methods in the proto file
2. Extracts HTTP paths from `google.api.http` annotations
3. Generates appropriate request/response schemas
4. Creates a single, cohesive OpenAPI document

## Template Breakdown

The template uses several helper functions to extract and transform protobuf details:

- `methodExtension` - Extracts `google.api.http` annotations
- `getOpenAPIType` - Maps protobuf field types to OpenAPI schema types
- `isStreaming` - Detects if a method uses streaming

The template demonstrates:
- Service → OpenAPI path mapping
- Method → HTTP operation mapping
- Message → Schema mapping
- Field → Property mapping

These concepts can be extended to handle more complex proto definitions, including nested messages, enums, and custom types.