# 💡 protoc-gen-anything

A versatile protoc plugin that generates anything from protobuf files.

# Rationale

This project exists to help make guiding code generation from a mixture of Go Templates and Protocol Buffer files to, well, anything, a total breeze.

We have a lot of slop on the march, this is hopefully a piece of the puzzle that will help us tame things.

## Installation

Ensure you have [Go](https://go.dev/doc/install) installed (version 1.20+).

```bash
go install github.com/tmc/protoc-gen-anything@latest
```

## Usage

1. Define your protobuf files
2. Create template files in a directory (e.g., `templates/`)
3. Run buf (recommended) or protoc with the protoc-gen-anything plugin

### Using buf (recommended)

[buf](https://buf.build/docs/installation) is a tool for making working with protocol buffer files more reasonable.

Follow the link above to install (if you're on MacOS, you can simply `brew install bufbuild/buf/buf`).

Add the following to your `buf.gen.yaml`:

```yaml
version: v1
plugins:
  - plugin: anything # This indicates to buf that it should use `protoc-gen-anything`.
    out: gen # output directory
    opt:
      - templates=./templates
      - paths=source_relative
    strategy: all  # Important for cross-package custom extensions.
```

Then run:

```bash
buf generate
```

> **Important Note**: The `strategy: all` setting is crucial when working with custom extensions or options that are defined in different packages. This strategy ensures that all required files are processed together, allowing protoc-gen-anything to resolve cross-package references correctly. Without this, you might encounter errors when trying to access custom extensions defined in separate packages.

### Using protoc directly

<details>
<summary>Click to see protoc usage</summary>

Run protoc with the protoc-gen-anything plugin:

```bash
protoc --anything_out=templates=./templates:./output proto/*.proto
```

Options:
- `templates`: Path to your template directory
- `verbose`: Enable verbose logging
- `continue_on_error`: Continue generation even if errors occur

Note: When using protoc directly, you need to ensure all relevant .proto files (including those with custom extensions) are included in the protoc command to handle cross-package references correctly.

</details>

## Custom Extensions Handling

protoc-gen-anything provides powerful support for handling custom protobuf extensions, even across different packages.

1. Define custom extensions in a separate package:

```protobuf
// myextensions/extensions.proto
syntax = "proto3";

package myextensions;

import "google/protobuf/descriptor.proto";

extend google.protobuf.MethodOptions {
  AuthzPolicy authz = 50000;
}

message AuthzPolicy {
  bool allow_unauthenticated = 1;
}
```

2. Use extensions in your service definitions:

```protobuf
// myservice/service.proto
syntax = "proto3";

package myservice;

import "myextensions/extensions.proto";

service MyService {
  rpc MyMethod(MyRequest) returns (MyResponse) {
    option (myextensions.authz) = {
      allow_unauthenticated: true
    };
  }
}
```

3. Access extensions in templates:

```go
{{ $authz := methodExtension .Method "myextensions.authz" }}
{{ $allowUnauthenticated := fieldByName $authz "allow_unauthenticated" }}
{{ if $allowUnauthenticated }}
// This method allows unauthenticated access
func (s *MyServiceServer) {{ .Method.GoName }}(ctx context.Context, req *{{ .Method.Input.GoIdent.GoName }}) (*{{ .Method.Output.GoIdent.GoName }}, error) {
    // Implementation for unauthenticated method
    return &{{ .Method.Output.GoIdent.GoName }}{}, nil
}
{{ else }}
// This method requires authentication
func (s *MyServiceServer) {{ .Method.GoName }}(ctx context.Context, req *{{ .Method.Input.GoIdent.GoName }}) (*{{ .Method.Output.GoIdent.GoName }}, error) {
    if err := authenticateUser(ctx); err != nil {
        return nil, err
    }
    // Implementation for authenticated method
    return &{{ .Method.Output.GoIdent.GoName }}{}, nil
}
{{ end }}
```

Remember to use `strategy: all` in your buf.gen.yaml (or include all relevant .proto files when using protoc directly) to ensure proper resolution of cross-package extensions.
