#!/bin/bash
set -e

# Check if protoc is installed
if ! command -v protoc &> /dev/null; then
    echo "Error: protoc is not installed."
    echo "Please install Protocol Buffers compiler from https://github.com/protocolbuffers/protobuf/releases"
    exit 1
fi

# Make sure we have a clean start
rm -rf gen
mkdir -p gen

# Compile the program
go build -o protoc-gen-jsonschema-anything .

# Add to PATH
export PATH=$PATH:$(pwd)

# Echo commands
set -x

# Generate standard protobuf code
buf generate

# Now manually use our custom jsonschema plugin with different options
echo "Generating schemas with different options..."

# Default
protoc \
  --plugin=protoc-gen-jsonschema-anything=./protoc-gen-jsonschema-anything \
  --jsonschema-anything_out=. \
  --proto_path=. proto/schema.proto

# With nullable fields
protoc \
  --plugin=protoc-gen-jsonschema-anything=./protoc-gen-jsonschema-anything \
  --jsonschema-anything_out=. \
  --jsonschema-anything_opt=nullable=true \
  --proto_path=. proto/schema.proto

# With enforce_oneof
protoc \
  --plugin=protoc-gen-jsonschema-anything=./protoc-gen-jsonschema-anything \
  --jsonschema-anything_out=. \
  --jsonschema-anything_opt=enforce_oneof=true \
  --proto_path=. proto/schema.proto

echo "Done!"