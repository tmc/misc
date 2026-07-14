package main

// Blank-import the well-known types so their FileDescriptors are registered in
// protoregistry.GlobalFiles. When a recovered descriptor lists one of these as
// a dependency (e.g. google/protobuf/timestamp.proto), lookupKnownDep can then
// resolve it instead of falling back to an empty stub — which keeps type
// references like google.protobuf.Timestamp valid and printable.
import (
	_ "google.golang.org/protobuf/types/known/anypb"
	_ "google.golang.org/protobuf/types/known/apipb"
	_ "google.golang.org/protobuf/types/known/durationpb"
	_ "google.golang.org/protobuf/types/known/emptypb"
	_ "google.golang.org/protobuf/types/known/fieldmaskpb"
	_ "google.golang.org/protobuf/types/known/sourcecontextpb"
	_ "google.golang.org/protobuf/types/known/structpb"
	_ "google.golang.org/protobuf/types/known/timestamppb"
	_ "google.golang.org/protobuf/types/known/typepb"
	_ "google.golang.org/protobuf/types/known/wrapperspb"
)
