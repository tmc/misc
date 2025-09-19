.PHONY: build
build:
	go build -o $(GOPATH)/bin/protoc-gen-jsonschema

.PHONY: install
install:
	go install

.PHONY: test
test: build
	go test ./...

# Basic usage examples
.PHONY: examples
examples: build
	mkdir -p testout/default
	mkdir -p testout/nullables
	mkdir -p testout/required
	mkdir -p testout/enums-as-constants
	mkdir -p testout/disallow-additional
	mkdir -p testout/prefix-package
	mkdir -p testout/with-refs
	mkdir -p testout/enforce-oneof
	mkdir -p testout/all-options

	# Default settings
	protoc \
		--jsonschema_out=testout/default \
		--proto_path=. testdata/examples.proto

	# Allow null values
	protoc \
		--jsonschema_out=allow_null_values:testout/nullables \
		--proto_path=. testdata/examples.proto

	# All fields required
	protoc \
		--jsonschema_out=all_fields_required:testout/required \
		--proto_path=. testdata/examples.proto

	# Enums as constants
	protoc \
		--jsonschema_out=enums_as_constants:testout/enums-as-constants \
		--proto_path=. testdata/examples.proto

	# Disallow additional properties
	protoc \
		--jsonschema_out=disallow_additional_properties:testout/disallow-additional \
		--proto_path=. testdata/examples.proto

	# Prefix with package name
	protoc \
		--jsonschema_out=prefix_schema_files_with_package:testout/prefix-package \
		--proto_path=. testdata/examples.proto

	# With definitions as references (default)
	protoc \
		--jsonschema_out=testout/with-refs \
		--proto_path=. testdata/examples.proto

	# Enforce oneof 
	protoc \
		--jsonschema_out=enforce_oneof:testout/enforce-oneof \
		--proto_path=. testdata/examples.proto

	# All options combined
	protoc \
		--jsonschema_out=allow_null_values,all_fields_required,disallow_additional_properties,enums_as_constants,enforce_oneof,file_extension=schema.json:testout/all-options \
		--proto_path=. testdata/examples.proto

	# Specific messages only
	mkdir -p testout/specific-messages
	protoc \
		--jsonschema_out=messages=[ExampleMessage+OptionalFields]:testout/specific-messages \
		--proto_path=. testdata/examples.proto

.PHONY: clean
clean:
	rm -rf testout