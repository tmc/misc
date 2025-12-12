package main

import (
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

func TestHelperLoadPrototxt(t *testing.T) {
	// 1. Setup Generator with a known message type
	g := NewGenerator(Options{
		TemplateDir: ".",
	})

	// Create a simple FileDescriptorSet with one message "TestMessage"
	// We can construct this manually using specific descriptor protos
	fd := &descriptorpb.FileDescriptorProto{
		Name:    protoString("test.proto"),
		Package: protoString("test"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: protoString("TestMessage"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:     protoString("foo"),
						Number:   protoInt32(1),
						Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
						JsonName: protoString("foo"),
					},
				},
			},
		},
	}

	fds := &descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{fd},
	}

	files, err := protodesc.NewFiles(fds)
	if err != nil {
		t.Fatalf("failed to create file descriptor: %v", err)
	}

	// Register the message type in g.types
	desc, err := files.FindDescriptorByName("test.TestMessage")
	if err != nil {
		t.Fatalf("failed to find message descriptor: %v", err)
	}
	md := desc.(protoreflect.MessageDescriptor)

	// We need to register this into g.types, which is *protoregistry.Types.
	// However, protoregistry.Types doesn't have a simple "Register" method for dynamic types easily accessible in this context
	// without implementing the right interfaces or using dynamicpb.
	// But `helperLoadPrototxt` uses `g.types.FindMessageByName`.
	// `protoregistry.Types` is a struct wrapping `proto.Type`, etc.
	// Actually `NewGenerator` initializes `g.types = new(protoregistry.Types)`.
	// `protoregistry.Types` allows looking up extensions and messages if they are registered.
	// But `protoregistry.Types` looks up in *global* registry or what's registered *to it*.
	// Wait, `protoregistry.Types` has `RegisterMessage(MessageType)`.

	mt := dynamicpb.NewMessageType(md)
	if err := g.types.RegisterMessage(mt); err != nil {
		t.Fatalf("failed to register message type: %v", err)
	}

	// 2. Create a temporary .prototxt file
	dir := t.TempDir()
	filename := filepath.Join(dir, "test.prototxt")
	content := []byte(`foo: "bar"`)
	if err := os.WriteFile(filename, content, 0644); err != nil {
		t.Fatalf("failed to write prototxt file: %v", err)
	}

	// 3. Call helperLoadPrototxt
	got, err := g.helperLoadPrototxt(filename, "test.TestMessage")
	if err != nil {
		t.Fatalf("helperLoadPrototxt failed: %v", err)
	}

	// 4. Assert result
	msg, ok := got.(*dynamicpb.Message)
	if !ok {
		t.Fatalf("expected *dynamicpb.Message, got %T", got)
	}

	// Check field "foo"
	fooField := md.Fields().ByName("foo")
	if !msg.Has(fooField) {
		t.Error("expected message to have field 'foo'")
	}
	if val := msg.Get(fooField).String(); val != "bar" {
		t.Errorf("expected foo='bar', got '%s'", val)
	}
}

func protoString(s string) *string {
	return &s
}

func protoInt32(i int32) *int32 {
	return &i
}
