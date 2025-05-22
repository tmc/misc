# gen - Example Packager for protoc-gen-anything

A small CLI tool to package protoc-gen-anything examples as txtar files.

## Installation

```bash
go install github.com/tmc/misc/protoc-gen-anything/cmd/gen@latest
```

## Usage

Package a single example:

```bash
# Output to stdout
gen openapi

# Save to a file
gen graphql -o graphql.txtar
```

Package all examples:

```bash
# Output all examples to stdout
gen -a

# Save all examples to a file
gen -a -o examples.txtar
```

Specify a custom examples directory:

```bash
# Use a specific examples path
gen -path=/path/to/examples openapi
```

## What are txtar files?

Txtar is a simple text-based archive format used for bundling multiple files into a single text file. It's commonly used in Go for storing examples, test data, and other file collections.

The format is simple:
- A comment section at the top
- Each file starts with a header line with the format `-- filename --`
- The file content follows until the next file header or end of archive

## Examples

Example of a txtar file containing an OpenAPI example:

```
# OpenAPI example for protoc-gen-anything

-- proto/http_service.proto --
syntax = "proto3";

package http;

option go_package = "github.com/yourusername/yourrepo/examples/http/gen/http";

import "google/api/annotations.proto";

message HTTPMessage {
  string id = 1;
  string data = 2;
}

service HTTPService {
  rpc GetMessage(HTTPMessage) returns (HTTPMessage) {
    option (google.api.http) = {
      get: "/v1/messages/{id}"
    };
  }
}

-- templates/openapi.yaml.tmpl --
# GENERATION_BEHAVIOR: overwrite
openapi: "3.1.0"
info:
  title: "{{ .File.GoPackageName }} API"
  version: "0.1.0"
  description: "API generated from Protocol Buffers using protoc-gen-anything"

paths:
  ...
```

You can extract a txtar file using the `go tool txtar extract` command:

```bash
go tool txtar extract examples.txtar
```

This will create a directory structure with all the files from the txtar archive.