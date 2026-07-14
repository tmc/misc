package main

import (
	"github.com/jhump/protoreflect/desc"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// lookupKnownDep resolves a dependency .proto path against the descriptors
// linked into this binary (the global registry), covering the well-known types
// (google/protobuf/*.proto) and anything else compiled in. Returns nil when the
// path isn't registered, in which case the caller falls back to a stub.
func lookupKnownDep(path string) *desc.FileDescriptor {
	fd, err := protoregistry.GlobalFiles.FindFileByPath(path)
	if err != nil {
		return nil
	}
	wrapped, err := desc.WrapFile(fd)
	if err != nil {
		return nil
	}
	return wrapped
}
