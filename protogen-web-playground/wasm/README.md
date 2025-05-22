# WebAssembly Port of Protoc-Gen-Anything

This directory contains the code for the WebAssembly port of protoc-gen-anything. Compiling to WebAssembly allows us to run the generator directly in the browser, eliminating the need for a backend server.

## Advantages of WebAssembly Approach

1. **Simplified Architecture**: No need for a backend server to process protobuf files, reducing deployment complexity.
2. **Better Performance**: No network latency for generation, as everything happens locally in the browser.
3. **Enhanced Privacy**: User files never leave the browser, which is great for sensitive protobuf definitions.
4. **Offline Support**: The playground can work without an internet connection once loaded.

## Implementation Details

The WebAssembly implementation includes:

1. **In-Memory Filesystem**: A complete in-memory filesystem to handle template and proto files.
2. **Protobuf Processing**: Direct integration with the protobuf compiler through Go's WebAssembly support.
3. **Template Engine**: The full Go template engine running in the browser.
4. **JavaScript Bridge**: JavaScript APIs to call the WebAssembly functions from the browser.

## Building

To build the WebAssembly binary:

```bash
make build
```

This will compile the Go code to WebAssembly and place the output in the frontend's public directory.

## JavaScript API

Once loaded in the browser, the WebAssembly module exposes the following function:

```javascript
// Generate output from proto files and templates
const output = generateFromProto({
  protoFiles: {
    "example.proto": "syntax = \"proto3\"; package example; message Test { string id = 1; }"
  },
  templates: {
    "{{.Message.GoIdent.GoName}}_extension.go.tmpl": "package {{.File.GoPackageName}}\n\nfunc (m *{{.Message.GoIdent.GoName}}) Foobar() {}"
  },
  options: {
    continueOnError: true,
    verbose: false
  }
});
```

## Limitations

Current limitations of the WebAssembly approach:

1. **Binary Size**: The WebAssembly binary may be large due to including the entire protobuf compiler.
2. **Browser Compatibility**: Requires a modern browser with WebAssembly support.
3. **Memory Constraints**: Limited by the browser's memory allocation for WebAssembly.