.PHONY: all build build-linux clean install-xk6 test test-tokenize vet vet-tokenize check example-loadtest example-loadtest-stream example-trace-replay

# Extension info
EXTENSION_NAME=xk6-llm-perf
EXTENSION_PATH=github.com/tmc/misc/$(EXTENSION_NAME)
VERSION=v0.1.0
SOURCE_REVISION:=$(shell git rev-parse HEAD)
SOURCE_HASH:=$(shell GOWORK=off go run ./internal/cmd/sourcehash)
SOURCE_DIRTY:=$(shell test -z "$$(git status --porcelain --untracked-files=all -- '*.go' '*.js' go.mod go.sum Makefile)" && echo false || echo true)

# Build settings
K6_VERSION ?= v2.3.0
XK6_VERSION ?= v1.4.14
BUILD_DIR=dist
BINARY_NAME=k6
LINUX_ARCH ?= amd64

# Development paths
GOBASE=$(shell pwd)
GOBIN=$(GOBASE)/bin

# Sample settings
VLLM_URL?=http://localhost:8000/v1
OPENAI_URL?=https://api.openai.com/v1
TRITON_URL?=http://localhost:8001/v1
OPENAI_KEY?=

all: clean build

install-xk6:
	@echo "Installing xk6..."
	@GOWORK=off GOBIN=$(GOBIN) go install go.k6.io/xk6/cmd/xk6@$(XK6_VERSION)

build: install-xk6
	@echo "Building k6 with $(EXTENSION_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@rm -f $(BUILD_DIR)/$(BINARY_NAME).new
	@GOWORK=off $(GOBIN)/xk6 build $(K6_VERSION) \
		--with $(EXTENSION_PATH)=. \
		--build-flags "-ldflags=-s -w -X $(EXTENSION_PATH).extensionRevision=$(SOURCE_REVISION) -X $(EXTENSION_PATH).extensionDirty=$(SOURCE_DIRTY) -X $(EXTENSION_PATH).extensionSourceHash=$(SOURCE_HASH)" \
		--output $(BUILD_DIR)/$(BINARY_NAME).new
	@test "$(SOURCE_HASH)" = "$$(GOWORK=off go run ./internal/cmd/sourcehash)"
	@test "$(SOURCE_REVISION)" = "$$(git rev-parse HEAD)"
	@if [ "$$(uname -s)" = Darwin ]; then codesign --force -s - $(BUILD_DIR)/$(BINARY_NAME).new; fi
	@mv $(BUILD_DIR)/$(BINARY_NAME).new $(BUILD_DIR)/$(BINARY_NAME)
	@$(BUILD_DIR)/$(BINARY_NAME) version | grep -F $(EXTENSION_PATH)

test: test-tokenize
	GOWORK=off go test -race ./...

build-linux: install-xk6
	@mkdir -p $(BUILD_DIR)
	@rm -f $(BUILD_DIR)/k6-linux.new
	@GOWORK=off $(GOBIN)/xk6 build $(K6_VERSION) --os linux --arch $(LINUX_ARCH) \
		--with $(EXTENSION_PATH)=. \
		--build-flags "-ldflags=-s -w -X $(EXTENSION_PATH).extensionRevision=$(SOURCE_REVISION) -X $(EXTENSION_PATH).extensionDirty=$(SOURCE_DIRTY) -X $(EXTENSION_PATH).extensionSourceHash=$(SOURCE_HASH)" \
		--output $(BUILD_DIR)/k6-linux.new
	@test "$(SOURCE_HASH)" = "$$(GOWORK=off go run ./internal/cmd/sourcehash)"
	@test "$(SOURCE_REVISION)" = "$$(git rev-parse HEAD)"
	@mv $(BUILD_DIR)/k6-linux.new $(BUILD_DIR)/k6-linux
	@go version -m $(BUILD_DIR)/k6-linux | grep -F $(EXTENSION_PATH)

test-tokenize:
	cd tools/llm-tokenize && GOWORK=off go test -race ./...

vet: vet-tokenize
	GOWORK=off go vet ./...

vet-tokenize:
	cd tools/llm-tokenize && GOWORK=off go vet ./...

check: test vet build
	K6_SMOKE_BINARY=$(abspath $(BUILD_DIR)/$(BINARY_NAME)) GOWORK=off go test -run 'TestBinary|TestBuiltK6|TestK6Live' -v ./...

clean:
	rm -rf $(BUILD_DIR) bin

example-loadtest: build
	@echo "Running load test..."
	ENDPOINT_URL=$(VLLM_URL) \
	OPENAI_API_KEY=$(OPENAI_KEY) \
	./$(BUILD_DIR)/$(BINARY_NAME) run \
		examples/sample-load-test.js -v -w

example-loadtest-stream: build
	@echo "Running streaming load test..."
	ENDPOINT_URL=$(VLLM_URL) \
	OPENAI_API_KEY=$(OPENAI_KEY) \
	STREAM_MODE=true \
	./$(BUILD_DIR)/$(BINARY_NAME) run \
		examples/sample-load-test.js -v -w

example-trace-replay: build
	@echo "Running trace replay example..."
	ENDPOINT_URL=$(VLLM_URL) \
	OPENAI_API_KEY=$(OPENAI_KEY) \
	./$(BUILD_DIR)/$(BINARY_NAME) run \
		examples/trace-replay.js
