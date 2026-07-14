# protofrompy

Recover `.proto` source files from generated Python protobuf code (`*_pb2.py`).

Every file produced by the modern protobuf Python backend embeds a complete,
serialized `FileDescriptorProto` in its `AddSerializedFile(b'...')` call. That
descriptor is the whole schema — messages, enums, fields, numbers, types,
labels, oneofs, maps, nested types, reserved ranges, and standard options. So
the original `.proto` can be reconstructed with high fidelity from the generated
Python alone: no `.proto` source, no `protoc`, and no running Python interpreter
required.

## Install / build

```
go build -o protofrompy ./
```

## Usage

```
protofrompy [flags] <file_or_dir>...
```

Each argument is a `_pb2.py` file or a directory searched recursively for
`*_pb2.py`. Multiple inputs are recovered together so cross-file type references
(a `Datum` referencing a `Tensor` in another file) resolve.

```
# Print one recovered .proto to stdout
protofrompy tinker_public_pb2.py

# Recover a whole package into a directory (mirrors descriptor file names)
protofrompy -out ./recovered ./src/tinker/proto/

# Inspect the raw FileDescriptorProto as JSON
protofrompy -json tinker_public_pb2.py
```

### Flags

| flag | meaning |
|------|---------|
| `-out dir` | write recovered `.proto` files under `dir` (default: stdout) |
| `-raw dir` | also dump the raw serialized `FileDescriptorProto` bytes per file |
| `-json` | print the `FileDescriptorProto` as JSON instead of `.proto` text |
| `-v` | verbose: report each recovered file and its type counts on stderr |

## What is recovered

Everything the descriptor retains, which is nearly the entire schema:

- package, `proto3`/`proto2` syntax
- all messages, including nested messages and nested enums
- all fields: name, number, type, label (`optional`/`repeated`), `optional`
  presence, default values
- enums and enum values (names + numbers)
- `oneof` groups
- `map<k, v>` fields (recognized from the synthesized `*Entry` messages)
- references to imported types, with the `import` statements that supply them
- well-known types (`google/protobuf/*.proto`) are linked in, so imports like
  `google.protobuf.Timestamp` resolve and print correctly

The recovered `.proto` is **semantically lossless**: compiling it with `protoc`
yields a descriptor with identical message/enum/field name/number/type/label
sets as the embedded original. (Verified by round-trip in this repo's tests and
against `tinker_public_pb2.py`.)

## What cannot be recovered

These are not stored in the descriptor, so no extractor can bring them back:

- **comments / doc strings** — discarded by `protoc` unless source-info is
  embedded (Python `_pb2.py` does not embed it)
- **original element order** — output is sorted (by kind then name) for stable
  diffs; the source's hand order is gone
- **formatting / whitespace / blank lines**
- **custom options** print by field number rather than by their extension name
  unless the extension descriptor is linked in

For most uses — re-deriving a schema to re-generate bindings in another language,
diffing wire compatibility, or porting a service to another stack — the
recovered file is a faithful, compilable substitute for the lost `.proto`.

## How it works

1. Find the `AddSerializedFile(b'...')` call and decode the Python bytes literal
   (handling `\xNN`, octal, named escapes, and implicitly-concatenated adjacent
   literals).
2. `proto.Unmarshal` the bytes into a `descriptorpb.FileDescriptorProto`.
3. Wire up dependencies (other recovered files + linked well-known types) and
   render with `jhump/protoreflect/desc/protoprint`.

The bytes-literal decoding and descriptor extraction are the only custom parts;
the rest is the standard protobuf descriptor toolchain.
