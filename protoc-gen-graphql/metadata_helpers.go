package main

import (
	metadata "github.com/tmc/misc/protoc-gen-graphql/proto/graphql/v1"
	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

func getMethodGraphQLOpts(m *protogen.Method) (*metadata.GraphQLOperation, bool) {
	opts, ok := m.Desc.Options().(*descriptorpb.MethodOptions)
	if !ok {
		return nil, false
	}
	extension := proto.GetExtension(opts, metadata.E_GraphqlOperation)
	mop, ok := extension.(*metadata.GraphQLOperation)
	return mop, ok
}

func getServiceGraphQLOpts(svc *protogen.Service) (*metadata.GraphQLService, bool) {
	opts, ok := svc.Desc.Options().(*descriptorpb.ServiceOptions)
	if !ok {
		return nil, false
	}
	extension := proto.GetExtension(opts, metadata.E_GraphqlService)
	mop, ok := extension.(*metadata.GraphQLService)
	return mop, ok
}

func getFieldGoogleAPIOpts(m *protogen.Field) ([]annotations.FieldBehavior, bool) {
	opts, ok := m.Desc.Options().(*descriptorpb.FieldOptions)
	if !ok {
		return nil, false
	}
	if !proto.HasExtension(opts, annotations.E_FieldBehavior) {
		return nil, false
	}
	ext := proto.GetExtension(opts, annotations.E_FieldBehavior)
	mops, ok := ext.([]annotations.FieldBehavior)
	if !ok {
		return nil, false
	}
	return mops, true
}

func isRepeated(f *protogen.Field) bool {
	return f.Desc.Cardinality() == protoreflect.Repeated
}

func hasRequiredFieldOption(f *protogen.Field) bool {
	if fOpts, ok := getFieldGoogleAPIOpts(f); ok {
		for _, v := range fOpts {
			if v == annotations.FieldBehavior_REQUIRED {
				return true
			}
		}
	}
	return false
}
