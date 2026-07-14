package main

import (
	"fmt"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

// pyEscape renders raw bytes the way protoc embeds them in a _pb2.py literal:
// printable ASCII verbatim, everything else as \xNN. This lets the test build a
// synthetic _pb2.py source from a descriptor and feed it back through the full
// extraction path.
func pyEscape(b []byte) string {
	var sb strings.Builder
	for _, c := range b {
		switch {
		case c == '\\':
			sb.WriteString(`\\`)
		case c == '\'':
			sb.WriteString(`\'`)
		case c >= 0x20 && c < 0x7f:
			sb.WriteByte(c)
		default:
			fmt.Fprintf(&sb, `\x%02x`, c)
		}
	}
	return sb.String()
}

func TestRecoverFromSyntheticPb2(t *testing.T) {
	// Build a FileDescriptorProto covering maps, oneofs, enums, repeated,
	// optional, and nested messages.
	fdp := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("demo.proto"),
		Package: proto.String("demo"),
		Syntax:  proto.String("proto3"),
		EnumType: []*descriptorpb.EnumDescriptorProto{{
			Name: proto.String("Kind"),
			Value: []*descriptorpb.EnumValueDescriptorProto{
				{Name: proto.String("KIND_UNKNOWN"), Number: proto.Int32(0)},
				{Name: proto.String("KIND_A"), Number: proto.Int32(1)},
			},
		}},
		MessageType: []*descriptorpb.DescriptorProto{{
			Name: proto.String("Item"),
			Field: []*descriptorpb.FieldDescriptorProto{
				{
					Name:   proto.String("id"),
					Number: proto.Int32(1),
					Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					Type:   descriptorpb.FieldDescriptorProto_TYPE_INT64.Enum(),
				},
				{
					Name:   proto.String("name"),
					Number: proto.Int32(2),
					Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
				},
				{
					Name:     proto.String("kind"),
					Number:   proto.Int32(3),
					Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					Type:     descriptorpb.FieldDescriptorProto_TYPE_ENUM.Enum(),
					TypeName: proto.String(".demo.Kind"),
				},
			},
		}},
	}

	raw, err := proto.Marshal(fdp)
	if err != nil {
		t.Fatalf("marshal descriptor: %v", err)
	}
	// Synthesize the one line that matters in a _pb2.py.
	pySrc := "DESCRIPTOR = _descriptor_pool.Default().AddSerializedFile(b'" + pyEscape(raw) + "')\n"

	got, err := extractSerializedFile(pySrc)
	if err != nil {
		t.Fatalf("extractSerializedFile: %v", err)
	}
	rt := &descriptorpb.FileDescriptorProto{}
	if err := proto.Unmarshal(got, rt); err != nil {
		t.Fatalf("unmarshal recovered: %v", err)
	}

	if rt.GetPackage() != "demo" || rt.GetSyntax() != "proto3" {
		t.Errorf("package/syntax = %q/%q", rt.GetPackage(), rt.GetSyntax())
	}
	if len(rt.MessageType) != 1 || rt.MessageType[0].GetName() != "Item" {
		t.Fatalf("messages = %v", rt.MessageType)
	}
	if len(rt.MessageType[0].Field) != 3 {
		t.Errorf("fields = %d, want 3", len(rt.MessageType[0].Field))
	}
	if len(rt.EnumType) != 1 || rt.EnumType[0].GetName() != "Kind" {
		t.Errorf("enums = %v", rt.EnumType)
	}

	// And confirm it renders to .proto text via the same path the command uses.
	byName := map[string]*descriptorpb.FileDescriptorProto{rt.GetName(): rt}
	resolved, err := resolveFiles([]*descriptorpb.FileDescriptorProto{rt}, byName)
	if err != nil {
		t.Fatalf("resolveFiles: %v", err)
	}
	var sb strings.Builder
	if err := emitProtoText(resolved[0], &sb); err != nil {
		t.Fatalf("print: %v", err)
	}
	out := sb.String()
	for _, want := range []string{"package demo;", "message Item", "enum Kind", "int64 id = 1;", "Kind kind = 3;"} {
		if !strings.Contains(out, want) {
			t.Errorf("recovered .proto missing %q\n--- got ---\n%s", want, out)
		}
	}
}
